//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

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
	batch           *nativeWFPBatch // original policy batch, bound before Begin; nil for legacy/read-only callers
	readContext     context.Context // immutable original scoped read; negative Abort never depends on it
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
	return owner.beginTransactionBound(readOnly, nil)
}

func (owner *nativeWFPSessionOwner) beginTransactionBound(readOnly bool, batch *nativeWFPBatch) error {
	return owner.beginTransactionScoped(readOnly, batch, nil)
}

func (owner *nativeWFPSessionOwner) beginReadTransaction(ctx context.Context) error {
	if ctx == nil {
		return core.ErrNativeScopeUnavailable
	}
	return owner.beginTransactionScoped(true, nil, ctx)
}

func (owner *nativeWFPSessionOwner) beginTransactionScoped(readOnly bool, batch *nativeWFPBatch, readContext context.Context) error {
	if readContext != nil && (!readOnly || batch != nil) {
		return core.ErrNativeScopeUnavailable
	}
	if batch != nil {
		if readOnly || batch.transaction != nil || batch.session != nil {
			return core.ErrNativeScopeUnavailable
		}
		if err := batch.operationScope(); err != nil {
			return err
		}
	}
	if !owner.callsReturned() || owner.handle == 0 || owner.closed || owner.closeStarted || owner.transaction != nil && owner.transaction.phase != wfpTransactionFinished {
		return core.ErrNativeScopeUnavailable
	}
	transaction := &nativeWFPTransaction{phase: wfpTransactionStarting, readOnly: readOnly, batch: batch, readContext: readContext}
	owner.transaction = transaction // original receipt exists BEFORE any native call
	if batch != nil {
		batch.transaction = transaction // retain the exact partial Begin, never adopt an earlier transaction on error
		batch.session, owner.queryBatch = owner, batch
		batch.queryActive.Store(true) // this original batch can be bound only once
	}
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
	if transaction.readContext != nil {
		if err := owner.queryScope(transaction.readContext); err != nil {
			transaction.phase = wfpTransactionFinished // no original Begin was issued
			return err
		}
	}
	if batch != nil {
		if err := batch.operationScope(); err != nil {
			transaction.phase = wfpTransactionFinished // no original Begin was attempted
			return err
		}
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
	if transaction.readContext != nil {
		return owner.queryScope(transaction.readContext) // preserve the original late Begin for independent Abort
	}
	if batch != nil {
		return batch.operationScope() // a late Begin is still owned and independently abortable
	}
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
	if err := transaction.operationScope(ctx); err != nil {
		return err // original batch source must still authorize the native commit
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
	return transaction.operationScope(ctx) // late source failure cannot erase the recorded original commit
}

type wfpPolicyAction uint8

const (
	wfpPolicyArm wfpPolicyAction = iota
	wfpPolicyGrant
	wfpPolicyRevoke // removal of general permits only; no barrier or failed-grant reconciliation proof
	wfpPolicyRestrict
	wfpPolicyRelease
)

// A bounded batch owns every native input and filter-ID output before the API.
// The first failed grant receipt remains retained until a fresh restrictive
// transaction confirms its general permits absent. No failed result is upgraded
// to success by comparing configuration or rereading the same stable keys.
type nativeWFPBatch struct {
	action       wfpPolicyAction
	operation    context.Context        // immutable original request; a later caller cannot refresh this batch
	session      *nativeWFPSessionOwner // original query polarity/source backlink, assigned before Begin
	queryActive  atomic.Bool            // revoked at original writer exit, not a policy/health receipt
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
	if operation == nil || action > wfpPolicyRelease {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpPolicyOperationScope(ctx, action); err != nil {
		return err
	}
	if !wfpPolicyWriter.TryLock() {
		return errors.Join(core.ErrHelperBusy, core.ErrHelperCleanup)
	}
	defer wfpPolicyWriter.Unlock()
	if err := wfpPolicyOperationScope(ctx, action); err != nil {
		return err
	}
	var owner *nativeWFPPolicyOwner
	var currentBatch *nativeWFPBatch
	var restrictedGrant *nativeWFPBatch
	var restrictedPending bool
	defer func() {
		if owner == nil {
			return
		}
		defer func() {
			if finalErr != nil && restrictedPending {
				// Restrict's receipt is not retired until the image finalizer
				// also returned normally. Keep the exact first failed grant.
				owner.pendingGrant, owner.failedGrant = true, restrictedGrant
			}
			owner.retainFailedGrant(finalErr)
		}()
		defer wfpCleanupStep(&finalErr, func() error { return wfpPolicyOperationScope(ctx, action) })
		// Runs AFTER the original writer finalizer, before unlocking. Holding
		// the image across this batch prevents path replacement during AppID/
		// Add/verification, without blocking a later owned update rename.
		wfpCleanupStep(&finalErr, owner.session.retireApplicationImage)
	}()
	complete := false
	defer func() {
		defer func() {
			if currentBatch != nil {
				currentBatch.queryActive.Store(false)
			}
		}() // after all original finalization, including abnormal unwind
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
			if owner.pendingGrant && action != wfpPolicyRevoke || owner.session.closeStarted && !owner.session.closed {
				finalErr = errors.Join(finalErr, core.ErrHelperCleanup)
			}
		}()
		owner.retainFailedGrant(finalErr) // before cleanup can panic/Goexit or lose the caller assignment
		cleanup := cleanupWFPWriterTransaction(owner)
		cleanupReturned = true
		finalErr = errors.Join(finalErr, cleanup, wfpPolicyOperationScope(ctx, action))
		if action == wfpPolicyRestrict && complete && finalErr == nil {
			// Preserve the original failed grant until the full closed-base
			// post-commit snapshot, original cleanup and final negative caller
			// scope all returned normally. A later cleanup failure cannot lose it.
			restrictedPending, restrictedGrant = owner.pendingGrant, owner.failedGrant
			owner.pendingGrant, owner.failedGrant = false, nil
		}
		if action == wfpPolicyRelease && complete && finalErr == nil {
			// Keep the original owner and failed grant through the full reader/
			// transaction finalizer. Only a normally confirmed original Close
			// permits unpublication; a late caller error cannot undo retirement.
			if wfpPolicyWriter.owner != owner {
				finalErr = core.ErrHelperCleanup
			} else if err := guardedWFPReadClose(&owner.session); err != nil {
				finalErr = errors.Join(core.ErrHelperCleanup, err)
			} else if !owner.session.closed || owner.session.handle != 0 || !owner.session.callsReturned() {
				finalErr = core.ErrHelperCleanup
			} else {
				owner.pendingGrant, owner.failedGrant = false, nil
				wfpPolicyWriter.owner = nil
				finalErr = wfpPolicyOperationScope(ctx, action)
			}
		}
		// Revoke is only a negative cleanup stage. Keeping pendingGrant is
		// required there and never authorizes a later Arm/Grant or base proof.
		if cleanup != nil || owner.pendingGrant && action != wfpPolicyRevoke || owner.session.closeStarted && !owner.session.closed {
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
		var err error
		if action == wfpPolicyArm || action == wfpPolicyGrant {
			err = owner.session.openSessionModeContext(ctx, false, "Snowden persistent policy owner")
		} else {
			err = owner.session.openSessionMode(false, "Snowden persistent policy owner")
		}
		if err != nil {
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
	if action == wfpPolicyArm || action == wfpPolicyGrant {
		if err := owner.session.retireApplicationImage(); err != nil {
			return err // no successor image after an unresolved original Close
		}
	}
	if err := wfpPolicyOperationScope(ctx, action); err != nil {
		return err
	}
	owner.batch = &nativeWFPBatch{action: action, operation: ctx}
	currentBatch = owner.batch
	if err := owner.session.beginTransactionBound(false, owner.batch); err != nil {
		return err
	}
	if err := operation(owner); err != nil {
		return err
	}
	if err := verifyWFPPolicyAction(ctx, owner, action); err != nil {
		return err
	}
	if err := owner.batch.operationScope(); err != nil {
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
	if err := owner.batch.operationScope(); err != nil {
		return err
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
