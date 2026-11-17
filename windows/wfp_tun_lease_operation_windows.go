//go:build windows

package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

type nativeTUNWFPCallKind uint8

const (
	nativeTUNWFPAcquire nativeTUNWFPCallKind = iota
	nativeTUNWFPValidate
	nativeTUNWFPClose
)

type nativeTUNWFPCallKey struct{}

// One original operation, retained by its lease before any external callback.
// Only the admitted caller writes ctx/deadline/stage. The lease mutex guards
// pending/fault and the original pointer; native payload stays with that caller.
type nativeTUNWFPCall struct {
	lease       *nativeTUNWFPLease
	kind        nativeTUNWFPCallKind
	source      *core.OperationSource
	ctx         context.Context
	deadline    time.Time
	hasDeadline bool
	stage       string
	pending     bool
	fault       bool
}

func (call *nativeTUNWFPCall) current() bool {
	return call != nil && call.lease != nil && (call.kind == nativeTUNWFPAcquire || call.kind == nativeTUNWFPValidate) &&
		call.source.Current() && !call.lease.sealed.Load() && !call.lease.actor.Revoked()
}

// Optional negative fence only. It cannot supply original App/actor authority.
func diagnosticWFPLeaseSourceError(ctx context.Context) error {
	if ctx == nil {
		return core.ErrNativeScopeUnavailable
	}
	value := ctx.Value(nativeTUNWFPCallKey{})
	if value == nil {
		return nil
	}
	call, ok := value.(*nativeTUNWFPCall)
	if !ok || !call.current() {
		return core.ErrNativeScopeUnavailable
	}
	return nil
}

func (call *nativeTUNWFPCall) prepareContext() error {
	call.stage = "context"
	if call.ctx == nil || call.ctx.Value(nativeTUNWFPCallKey{}) != nil {
		return core.ErrNativeScopeUnavailable // never replace/revive an earlier call source
	}
	call.ctx = context.WithValue(call.ctx, nativeTUNWFPCallKey{}, call)
	call.deadline, call.hasDeadline = call.ctx.Deadline()
	return wfpDiagnosticOperationScope(call.ctx)
}

func (lease *nativeTUNWFPLease) beginCall(kind nativeTUNWFPCallKind, ctx context.Context) (*nativeTUNWFPCall, error) {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if kind == nativeTUNWFPClose {
		lease.sealed.Store(true)
		lease.opened = false
		if lease.call != nil {
			lease.call.source.Revoke() // atomic original source; no cancellation callbacks
		}
	}
	if lease.call != nil {
		return nil, errors.Join(core.ErrHelperBusy, core.ErrHelperCleanup)
	}
	if lease.closed {
		if kind == nativeTUNWFPClose {
			return nil, nil
		}
		return nil, core.ErrNativeScopeUnavailable
	}
	if kind != nativeTUNWFPClose && lease.sealed.Load() || kind > nativeTUNWFPClose {
		return nil, core.ErrNativeScopeUnavailable
	}
	if kind == nativeTUNWFPAcquire {
		if lease.acquireStarted {
			return nil, core.ErrNativeScopeUnavailable
		}
		lease.acquireStarted = true
	} else if kind == nativeTUNWFPValidate && !lease.opened {
		return nil, core.ErrNativeScopeUnavailable
	}
	call := &nativeTUNWFPCall{lease: lease, kind: kind, source: core.NewOperationSource(), ctx: ctx, stage: "admitted", pending: true}
	lease.call = call
	return call, nil
}

func (lease *nativeTUNWFPLease) claimDiagnostic(call *nativeTUNWFPCall) error {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.call != call || !call.current() {
		return core.ErrNativeScopeUnavailable
	}
	diagnosticWFP.Lock()
	defer diagnosticWFP.Unlock()
	if diagnosticWFP.owner != nil {
		return core.ErrHelperBusy
	}
	diagnosticWFP.owner, lease.claimed = lease, true
	return nil // original capture/lease and global owner precede all native calls
}

// Registered directly as a defer. Both the body and final context fence must
// return normally before this original slot can be cleared. No context, actor,
// image or native callback runs under lease.mu, including during finalization.
func (lease *nativeTUNWFPLease) finishCall(call *nativeTUNWFPCall, returned *bool, result *error) {
	payload := recover()
	checked := false
	defer func() {
		fencePayload := recover()
		lease.mu.Lock()
		defer lease.mu.Unlock()
		if payload != nil || !*returned || !checked || fencePayload != nil || lease.call != call {
			call.source.Revoke()
			call.fault = true
			lease.sealed.Store(true)
			lease.opened = false
			*result = errors.Join(*result, core.ErrNativeScopeUnavailable, core.ErrHelperCleanup)
			return // exact pending call/context/source and native owner stay retained
		}
		if call.kind == nativeTUNWFPClose {
			if *result == nil {
				diagnosticWFP.Lock() // metadata identity commit only; never native I/O
				owned := diagnosticWFP.owner == lease
				if lease.claimed && (!owned || !lease.native.closed || lease.native.handle != 0 || !lease.native.callsReturned()) || !lease.claimed && owned {
					*result = core.ErrHelperCleanup
				} else {
					if lease.claimed {
						diagnosticWFP.owner = nil
					}
					lease.closed = true
				}
				diagnosticWFP.Unlock()
			}
		} else {
			if !call.current() || call.hasDeadline && !time.Now().Before(call.deadline) {
				*result = errors.Join(*result, core.ErrNativeScopeUnavailable)
			}
			if call.kind == nativeTUNWFPAcquire && *result == nil {
				lease.opened = true
			}
		}
		call.source.Revoke()
		call.pending = false
		lease.call = nil // normal callback AND final fence, not native retirement
	}()
	if payload != nil || !*returned {
		return
	}
	if call.kind != nativeTUNWFPClose {
		*result = errors.Join(*result, wfpDiagnosticOperationScope(call.ctx))
	}
	checked = true
}

func (lease *nativeTUNWFPLease) hasCustody() bool {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return lease.call != nil || lease.claimed // do not read payload while a call may be pending
}
