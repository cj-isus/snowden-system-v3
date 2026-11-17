//go:build windows

package main

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
	"github.com/tailscale/wf"
)

// One original mutable diagnostic owner. A failed acquisition/cleanup is never
// adopted, replaced or removed by a later same-key rule. No public setter.
var diagnosticWFP struct {
	sync.Mutex
	owner *nativeTUNWFPLease
}

type nativeTUNWFPPolicy struct{}

type nativeTUNWFPLease struct {
	mu             sync.Mutex
	sealed         atomic.Bool
	call           *nativeTUNWFPCall
	actor          *core.NativeTUNActor
	descriptor     core.NativeTUNDescriptor
	luid           uint64
	spec           wfpRuleSpec
	native         nativeDiagnosticWFP
	opened         bool
	closed         bool
	acquireStarted bool
	claimed        bool
}

func diagnosticWFPRule(appID string, port uint16, luid uint64) wfpRuleSpec {
	return wfpRuleSpec{30, "one-flow native TUN diagnostic", wf.LayerALEAuthConnectV4, 14, wf.ActionPermit, []*wf.Match{
		{Field: wf.FieldALEAppID, Op: wf.MatchTypeEqual, Value: appID},
		{Field: wf.FieldIPLocalInterface, Op: wf.MatchTypeEqual, Value: luid},
		{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: wf.IPProtoTCP},
		{Field: wf.FieldIPLocalPort, Op: wf.MatchTypeEqual, Value: port},
		{Field: wf.FieldIPRemoteAddress, Op: wf.MatchTypeEqual, Value: netip.MustParseAddr("1.1.1.1")},
		{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeEqual, Value: uint16(443)},
	}}
}

// Pure allocation: no global claim, native query/session/filter or callback.
// The capture can retain this owner before any in-place Acquire is entered.
func (nativeTUNWFPPolicy) NewLease(actor *core.NativeTUNActor) core.NativeTUNLeasePreparation {
	return &nativeTUNWFPLease{actor: actor}
}

// Compatibility entrypoint; production capture uses NewLease then Acquire.
func (nativeTUNWFPPolicy) Acquire(ctx context.Context, actor *core.NativeTUNActor) (core.NativeTUNLease, error) {
	lease := &nativeTUNWFPLease{actor: actor}
	err := lease.Acquire(ctx)
	if err != nil && !lease.hasCustody() {
		return nil, err // returned normally before this lease claimed any resource
	}
	return lease, err
}

func (lease *nativeTUNWFPLease) Acquire(ctx context.Context) (finalErr error) {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	call, err := lease.beginCall(nativeTUNWFPAcquire, ctx)
	if err != nil {
		return err
	}
	returned := false
	defer lease.finishCall(call, &returned, &finalErr)
	finalErr = call.prepareContext()
	if finalErr == nil {
		finalErr = lease.acquireOwned(call)
	}
	returned = true
	return finalErr
}

// Only this lease's admitted caller accesses its native payload. All actor,
// image, interface and WFP work runs outside the lease metadata mutex.
func (lease *nativeTUNWFPLease) acquireOwned(call *nativeTUNWFPCall) error {
	ctx := call.ctx
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	call.stage = "claim"
	if err := lease.claimDiagnostic(call); err != nil {
		return err
	}
	call.stage = "descriptor"
	descriptor, err := lease.actor.Descriptor(ctx)
	if err != nil || descriptor.Runtime.TUNName != render.TUNInterfaceName {
		return core.ErrNativeScopeUnavailable
	}
	lease.descriptor = descriptor
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	call.stage = "image"
	helperApp, err := lease.native.imageAppID(ctx, descriptor.Executable)
	if err != nil || helperApp == "" {
		return errors.Join(core.ErrNativeImageUnavailable, err)
	}
	engineApp, err := lease.native.applicationAppID(ctx, "")
	if err != nil || engineApp == "" || helperApp == engineApp {
		return errors.Join(core.ErrNativeScopeUnavailable, err) // same-image engine bypass is not TUN evidence
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	call.stage = "interface"
	lease.luid, err = waitInterfaceLUIDContext(ctx, descriptor.Runtime.TUNName, 250*time.Millisecond)
	if err != nil {
		return err
	}
	lease.spec = diagnosticWFPRule(helperApp, descriptor.SourcePort, lease.luid)
	call.stage = "session"
	if err := lease.native.openSessionContext(ctx); err != nil {
		return err
	}
	// Reject ANY general LUID permit and any diagnostic/unknown owned filter.
	// Acquisition cannot silently adopt a previous session's same stable ID.
	call.stage = "base"
	if err := verifyWFPDiagnosticContext(ctx, &lease.native.nativeWFPSessionOwner, nil); err != nil {
		return err
	}
	if current, err := lease.actor.Descriptor(ctx); err != nil || current != descriptor {
		return core.ErrNativeScopeUnavailable
	}
	call.stage = "filter"
	if err := lease.native.open(ctx, helperApp, descriptor.SourcePort, lease.luid); err != nil {
		return err
	}
	call.stage = "validate"
	if err := lease.validateOwned(ctx); err != nil {
		return err
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	return nil
}

func (lease *nativeTUNWFPLease) Validate(ctx context.Context) (finalErr error) {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	call, err := lease.beginCall(nativeTUNWFPValidate, ctx)
	if err != nil {
		return err
	}
	returned := false
	defer lease.finishCall(call, &returned, &finalErr)
	finalErr = call.prepareContext()
	if finalErr == nil {
		call.stage = "validate"
		finalErr = lease.validateOwned(call.ctx)
	}
	returned = true
	return finalErr
}

func (lease *nativeTUNWFPLease) validateOwned(ctx context.Context) error {
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	if !lease.claimed || !lease.acquireStarted || lease.native.closed || lease.native.closeStarted || !lease.native.openReturned || lease.native.openCode != 0 || lease.native.handle == 0 || lease.luid == 0 {
		return core.ErrNativeScopeUnavailable
	}
	current, err := lease.actor.Descriptor(ctx)
	if err != nil || current != lease.descriptor {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	luid, err := waitInterfaceLUIDContext(ctx, current.Runtime.TUNName, 250*time.Millisecond)
	if err != nil || luid != lease.luid {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	if err := verifyWFPDiagnosticContext(ctx, &lease.native.nativeWFPSessionOwner, &lease.spec); err != nil {
		return err
	}
	current, err = lease.actor.Descriptor(ctx)
	if err != nil || current != lease.descriptor {
		return core.ErrNativeScopeUnavailable
	}
	return wfpDiagnosticOperationScope(ctx)
}

func (lease *nativeTUNWFPLease) Close() (finalErr error) {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	call, err := lease.beginCall(nativeTUNWFPClose, nil)
	if err != nil || call == nil {
		return err
	}
	returned := false
	defer lease.finishCall(call, &returned, &finalErr)
	call.stage = "native-close"
	finalErr = lease.closeOwned()
	returned = true
	return finalErr
}

func (lease *nativeTUNWFPLease) closeOwned() error {
	diagnosticWFP.Lock()
	owned := diagnosticWFP.owner == lease
	diagnosticWFP.Unlock()
	if !lease.claimed {
		if owned {
			return core.ErrNativeScopeUnavailable
		}
		return nil // finalizer retires pure allocation without touching another lease
	}
	if !owned || lease.native.addAttempted && !lease.native.addReturned {
		return core.ErrNativeScopeUnavailable // no native Close while original Add may be unresolved
	}
	if err := lease.native.close(); err != nil {
		return err
	}
	// A successful original dynamic-session Close removes its objects by the
	// BFE contract. Do not reopen a session/adopt a same-key filter as a cleanup
	// substitute. This receipt says nothing about another actor's base changes;
	// any later forwarding gate must verify the complete policy afresh.
	// Original identity/unpublication is a value-only final commit, after this
	// native callback returned normally; a concurrent Close cannot pass pending.
	return nil
}
