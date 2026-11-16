//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/snowden-system/windows/backend/core"
	"golang.org/x/sys/windows"
)

type wfpTransactionPhase uint8

const (
	wfpTransactionStarting wfpTransactionPhase = iota
	wfpTransactionActive
	wfpTransactionCommitting
	wfpTransactionAborting
	wfpTransactionFinished
)

type nativeWFPTransaction struct {
	phase           wfpTransactionPhase
	readOnly        bool
	beginAttempted  bool
	beginReturned   bool
	commitAttempted bool
	commitReturned  bool
	abortAttempted  bool
	abortReturned   bool
	commitConfirmed bool
	sessionRetired  bool
	lastCode        uint32
}

func (transaction *nativeWFPTransaction) callsReturned() bool {
	return transaction == nil || (!transaction.beginAttempted || transaction.beginReturned) &&
		(!transaction.commitAttempted || transaction.commitReturned) && (!transaction.abortAttempted || transaction.abortReturned)
}

// Pure original-call graph check; known syscall return does not prove commit,
// successful cleanup or resource retirement. No alternate handle is queried.
func (owner *nativeWFPSessionOwner) callsReturned() bool {
	return owner != nil && (!owner.openAttempted || owner.openReturned) && (!owner.closeStarted || owner.closeReturned) && !owner.writePending &&
		owner.reader.callsReturned() && owner.transaction.callsReturned()
}

var (
	wfpTransactionBegin  = diagnosticWFPDLL.NewProc("FwpmTransactionBegin0")
	wfpTransactionCommit = diagnosticWFPDLL.NewProc("FwpmTransactionCommit0")
	wfpTransactionAbort  = diagnosticWFPDLL.NewProc("FwpmTransactionAbort0")
)

func (owner *nativeWFPSessionOwner) beginTransaction(readOnly bool) error {
	if !owner.callsReturned() || owner.handle == 0 || owner.closed || owner.closeStarted || owner.transaction != nil && owner.transaction.phase != wfpTransactionFinished {
		return core.ErrNativeScopeUnavailable
	}
	transaction := &nativeWFPTransaction{phase: wfpTransactionStarting, readOnly: readOnly}
	owner.transaction = transaction // original receipt exists BEFORE any native call
	for _, proc := range []*windows.LazyProc{wfpTransactionBegin, wfpTransactionCommit, wfpTransactionAbort} {
		if err := proc.Find(); err != nil {
			transaction.phase = wfpTransactionFinished
			return err
		}
	}
	flags := uintptr(0)
	if readOnly {
		flags = 1
	}
	transaction.beginAttempted = true
	code, _, _ := wfpTransactionBegin.Call(uintptr(owner.handle), flags)
	transaction.beginReturned = true
	runtime.KeepAlive(owner)
	transaction.lastCode = uint32(code)
	if code != 0 {
		return fmt.Errorf("WFP transaction begin unavailable (%d)", uint32(code))
	}
	transaction.phase = wfpTransactionActive
	return nil
}

// Only rollback/absence of an active transaction is established here. After an
// unknown Commit, NO_TXN does NOT establish whether the writes were committed.
// Retrying Abort on the SAME live private session is restrictive; Close is never
// retried after an unknown result, nor is a replacement session opened for it.
func (owner *nativeWFPSessionOwner) abortTransaction() error {
	if !owner.callsReturned() {
		return core.ErrHelperCleanup // no Abort while an original Begin/Commit/query/write/Free may be unreturned
	}
	transaction := owner.transaction
	if transaction == nil || transaction.phase == wfpTransactionFinished {
		return nil
	}
	if !transaction.beginAttempted {
		transaction.phase = wfpTransactionFinished
		return nil
	}
	if owner.handle == 0 || owner.closed || owner.closeStarted {
		return core.ErrHelperCleanup
	}
	transaction.phase = wfpTransactionAborting
	transaction.abortAttempted, transaction.abortReturned = true, false
	code, _, _ := wfpTransactionAbort.Call(uintptr(owner.handle))
	transaction.abortReturned = true
	runtime.KeepAlive(owner)
	transaction.lastCode = uint32(code)
	if code != 0 && uint32(code) != uint32(windows.FWP_E_NO_TXN_IN_PROGRESS) {
		return errors.Join(core.ErrHelperCleanup, fmt.Errorf("WFP transaction abort unconfirmed (%d)", uint32(code)))
	}
	transaction.phase = wfpTransactionFinished
	return nil
}

func (owner *nativeWFPSessionOwner) commitTransaction(ctx context.Context) error {
	transaction := owner.transaction
	if ctx == nil || ctx.Err() != nil || !owner.callsReturned() || transaction == nil || transaction.phase != wfpTransactionActive || transaction.readOnly || owner.handle == 0 || owner.closed || owner.closeStarted {
		return core.ErrNativeScopeUnavailable
	}
	transaction.phase, transaction.commitAttempted = wfpTransactionCommitting, true
	code, _, _ := wfpTransactionCommit.Call(uintptr(owner.handle))
	transaction.commitReturned = true
	runtime.KeepAlive(owner)
	transaction.lastCode = uint32(code)
	if code != 0 {
		return fmt.Errorf("WFP transaction commit unconfirmed (%d)", uint32(code))
	}
	transaction.phase, transaction.commitConfirmed = wfpTransactionFinished, true
	return ctx.Err() // a known committed operation can still have a canceled caller
}

type wfpPolicyAction uint8

const (
	wfpPolicyArm wfpPolicyAction = iota
	wfpPolicyGrant
	wfpPolicyRestrict
	wfpPolicyRelease
)

// A bounded batch owns every native input and filter-ID output before the API.
// The first failed grant receipt remains retained until a fresh restrictive
// transaction confirms its general permits absent. No failed result is upgraded
// to success by comparing configuration or rereading the same stable keys.
type nativeWFPBatch struct {
	action       wfpPolicyAction
	transaction  *nativeWFPTransaction
	writes       [40]*nativeWFPWrite
	count        int
	expectedLUID uint64
}

type nativeWFPPolicyOwner struct {
	session      nativeWFPSessionOwner
	batch        *nativeWFPBatch
	failedGrant  *nativeWFPBatch
	pendingGrant bool
}

var wfpPolicyWriter struct {
	sync.Mutex
	owner *nativeWFPPolicyOwner
}

// A successful Arm/grant keeps this ORIGINAL non-dynamic session for the policy
// lifetime. Persistent objects survive client exit as before. Only explicit
// Release closes it, AFTER actual removal; an unknown Close retains custody.
func withWFPPolicyWriter(ctx context.Context, action wfpPolicyAction, operation func(*nativeWFPPolicyOwner) error) (finalErr error) {
	if ctx == nil || ctx.Err() != nil || operation == nil || action > wfpPolicyRelease {
		return core.ErrNativeScopeUnavailable
	}
	if !wfpPolicyWriter.TryLock() {
		return errors.Join(core.ErrHelperBusy, core.ErrHelperCleanup)
	}
	defer wfpPolicyWriter.Unlock()
	var owner *nativeWFPPolicyOwner
	complete := false
	defer func() {
		if recovered := recover(); recovered != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
		if owner == nil {
			return
		}
		cleanupReturned, finalized := false, false
		defer func() {
			if !cleanupReturned || !finalized {
				finalErr = errors.Join(finalErr, core.ErrHelperCleanup)
			}
			owner.retainFailedGrant(finalErr)
			if owner.pendingGrant || owner.session.closeStarted && !owner.session.closed {
				finalErr = errors.Join(finalErr, core.ErrHelperCleanup)
			}
		}()
		owner.retainFailedGrant(finalErr) // before cleanup can panic/Goexit or lose the caller assignment
		cleanup := cleanupWFPWriterTransaction(owner)
		cleanupReturned = true
		finalErr = errors.Join(finalErr, cleanup, ctx.Err())
		if cleanup != nil || owner.pendingGrant || owner.session.closeStarted && !owner.session.closed {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup, cleanup)
		}
		finalized = true
	}()
	owner = wfpPolicyWriter.owner
	if owner != nil && (!owner.session.openReturned || owner.session.openCode != 0) {
		// An unsuccessful open never executed a policy mutation. Retire that
		// original partial session before allocating the next owner's outputs.
		if err := guardedWFPReadClose(&owner.session); err != nil {
			return errors.Join(core.ErrHelperCleanup, err)
		}
		wfpPolicyWriter.owner, owner = nil, nil
	}
	if owner == nil { // an unresolved original owner is never replaced by allocation
		owner = &nativeWFPPolicyOwner{}
		wfpPolicyWriter.owner = owner
		if err := owner.session.openSessionMode(false, "Snowden persistent policy owner"); err != nil {
			return errors.Join(core.ErrHelperCleanup, err)
		}
	}
	if owner.session.handle == 0 || owner.session.closed || owner.session.closeStarted {
		return core.ErrHelperCleanup
	}
	if err := cleanupWFPWriterTransaction(owner); err != nil {
		return errors.Join(core.ErrHelperCleanup, err)
	}
	if owner.pendingGrant && (action == wfpPolicyArm || action == wfpPolicyGrant) {
		return core.ErrHelperCleanup // restrictive cleanup must precede any new grant
	}
	owner.batch = &nativeWFPBatch{action: action}
	if err := owner.session.beginTransaction(false); err != nil {
		owner.batch.transaction = owner.session.transaction
		return err
	}
	owner.batch.transaction = owner.session.transaction
	if err := operation(owner); err != nil {
		return err
	}
	if err := verifyWFPPolicyAction(ctx, owner, action); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.session.commitTransaction(ctx); err != nil {
		return err
	}
	// Post-commit verification uses a NEW explicit read snapshot on the SAME
	// original session. It does not excuse any previous failed Commit result.
	if err := verifyWFPPolicyAction(ctx, owner, action); err != nil {
		return err
	}
	if action == wfpPolicyRestrict || action == wfpPolicyRelease {
		owner.pendingGrant, owner.failedGrant = false, nil
	}
	if action == wfpPolicyRelease {
		if err := guardedWFPReadClose(&owner.session); err != nil {
			return errors.Join(core.ErrHelperCleanup, err)
		}
		wfpPolicyWriter.owner = nil
	}
	complete = true
	return nil
}

func cleanupWFPWriterTransaction(owner *nativeWFPPolicyOwner) (finalErr error) {
	if owner == nil || !owner.session.callsReturned() {
		return core.ErrHelperCleanup
	}
	// Both attempts are registered before callbacks. An unreturned predecessor
	// only permits the next step's negative graph check, never another syscall.
	defer wfpCleanupStep(&finalErr, owner.session.abortTransaction)
	defer wfpCleanupStep(&finalErr, func() error { return owner.session.reader.cleanup(owner.session.handle) })
	return nil
}

func (owner *nativeWFPPolicyOwner) retainFailedGrant(cause error) {
	if cause != nil && owner.batch != nil && owner.batch.action == wfpPolicyGrant && owner.batch.transaction != nil && owner.batch.transaction.commitAttempted {
		owner.pendingGrant = true
		if owner.failedGrant == nil {
			owner.failedGrant = owner.batch
		}
	}
}
