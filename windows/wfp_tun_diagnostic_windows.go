//go:build windows

package main

import (
	"context"
	"net/netip"
	"os"
	"sync"
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
	if err != nil && !lease.claimed {
		return nil, err // returned normally before this lease claimed any resource
	}
	return lease, err
}

func (lease *nativeTUNWFPLease) Acquire(ctx context.Context) (finalErr error) {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed || lease.acquireStarted {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err // do not claim a global owner after waiting past the source/deadline
	}
	lease.acquireStarted = true
	complete := false
	defer func() {
		if recovered := recover(); recovered != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
	}()
	diagnosticWFP.Lock()
	if diagnosticWFP.owner != nil {
		diagnosticWFP.Unlock()
		complete = true
		return core.ErrHelperBusy
	}
	diagnosticWFP.owner, lease.claimed = lease, true // original capture and global owner precede native calls
	diagnosticWFP.Unlock()
	descriptor, err := lease.actor.Descriptor(ctx)
	if err != nil || descriptor.Runtime.TUNName != render.TUNInterfaceName {
		return core.ErrNativeScopeUnavailable
	}
	lease.descriptor = descriptor
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	helperApp, err := lease.native.imageAppID(ctx, descriptor.Executable)
	if err != nil || helperApp == "" {
		return core.ErrNativeImageUnavailable
	}
	engineApp, err := lease.native.imageAppID(ctx, exe)
	if err != nil || engineApp == "" || helperApp == engineApp {
		return core.ErrNativeScopeUnavailable // same-image engine bypass is not TUN evidence
	}
	lease.luid, err = waitInterfaceLUIDContext(ctx, descriptor.Runtime.TUNName, 250*time.Millisecond)
	if err != nil {
		return err
	}
	lease.spec = diagnosticWFPRule(helperApp, descriptor.SourcePort, lease.luid)
	if err := lease.native.openSessionContext(ctx); err != nil {
		return err
	}
	// Reject ANY general LUID permit and any diagnostic/unknown owned filter.
	// Acquisition cannot silently adopt a previous session's same stable ID.
	if err := verifyWFPDiagnosticContext(ctx, &lease.native.nativeWFPSessionOwner, nil); err != nil {
		return err
	}
	if current, err := lease.actor.Descriptor(ctx); err != nil || current != descriptor {
		return core.ErrNativeScopeUnavailable
	}
	if err := lease.native.open(ctx, helperApp, descriptor.SourcePort, lease.luid); err != nil {
		return err
	}
	if err := lease.validateLocked(ctx, false); err != nil {
		return err
	}
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	lease.opened, complete = true, true
	return nil
}

func (lease *nativeTUNWFPLease) Validate(ctx context.Context) error {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return lease.validateLocked(ctx, true)
}

func (lease *nativeTUNWFPLease) validateLocked(ctx context.Context, requireOpened bool) error {
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	if !lease.claimed || !lease.acquireStarted || requireOpened && !lease.opened || lease.closed || lease.native.closed || lease.native.closeStarted || !lease.native.openReturned || lease.native.openCode != 0 || lease.native.handle == 0 || lease.luid == 0 {
		return core.ErrNativeScopeUnavailable
	}
	current, err := lease.actor.Descriptor(ctx)
	if err != nil || current != lease.descriptor {
		return core.ErrNativeScopeUnavailable
	}
	luid, err := waitInterfaceLUIDContext(ctx, current.Runtime.TUNName, 250*time.Millisecond)
	if err != nil || luid != lease.luid {
		return core.ErrNativeScopeUnavailable
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

func (lease *nativeTUNWFPLease) Close() error {
	if lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed {
		return nil
	}
	lease.opened = false
	diagnosticWFP.Lock()
	owned := diagnosticWFP.owner == lease
	diagnosticWFP.Unlock()
	if !lease.claimed {
		if owned {
			return core.ErrNativeScopeUnavailable
		}
		lease.closed = true // pure allocation or a returned busy refusal; never touch another lease
		return nil
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
	diagnosticWFP.Lock()
	defer diagnosticWFP.Unlock()
	if diagnosticWFP.owner != lease {
		return core.ErrNativeScopeUnavailable
	}
	lease.closed, diagnosticWFP.owner = true, nil
	return nil
}
