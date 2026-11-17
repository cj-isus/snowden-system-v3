package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Value copies of appProtectedProbe share one preparation slot. Periodic Run
// never writes it; lifecycle preparation and consumption are each one-shot.
type appNativeCommitSlot struct {
	mu      sync.Mutex
	lease   *tunnelLease
	pending *appNativeProbePublication // private candidate; take cannot consume it
}

func (slot *appNativeCommitSlot) clear() {
	if slot != nil {
		slot.mu.Lock()
		slot.lease = nil
		slot.pending = nil // revoke delayed probe publication before negative cleanup
		slot.mu.Unlock()
	}
}

func (slot *appNativeCommitSlot) take() *tunnelLease {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	lease := slot.lease
	slot.lease = nil
	slot.pending = nil // consumption also burns any unfinished preparation
	return lease
}

type nativeTUNCommitPhase uint8

const (
	nativeTUNPrepared nativeTUNCommitPhase = iota
	nativeTUNWFPReady
	nativeTUNAdmitted
)

func (scope *nativeTUNLifecycleScope) runtimePhase(ctx context.Context, phase nativeTUNCommitPhase) error {
	engine, identity, err := currentTunnelRuntime(ctx, scope.config)
	expected := scope.runtime
	expected.ForwardingAdmitted = phase == nativeTUNAdmitted
	if err != nil || engine != scope.engine || identity != expected {
		return core.ErrNativeScopeUnavailable
	}
	luid, err := waitInterfaceLUIDContext(ctx, scope.config.TUNName, 250*time.Millisecond)
	if err != nil || luid != scope.luid || luid == 0 {
		return core.ErrNativeScopeUnavailable
	}
	return nil
}

// Explicit phase comparison changes only ForwardingAdmitted, never the original
// instance/config/tag/epoch. All native/IPC work runs outside the value locks.
func (scope *nativeTUNLifecycleScope) validateCommitPhase(ctx context.Context, phase nativeTUNCommitPhase) error {
	if scope == nil || ctx == nil || scope.operation == nil || ctx.Done() == nil || ctx.Done() != scope.operation.Done() || phase > nativeTUNAdmitted {
		return core.ErrNativeScopeUnavailable
	}
	release, _, err := scope.lockCurrent(ctx)
	if err != nil {
		return err
	}
	release()
	if err := scope.runtimePhase(ctx, phase); err != nil {
		return err
	}
	if phase == nativeTUNPrepared {
		err = verifyWFPClosedContext(ctx)
	} else {
		err = verifyWFPPolicyContext(ctx, true, scope.luid)
	}
	if err != nil {
		return err
	}
	if err := scope.runtimePhase(ctx, phase); err != nil {
		return err
	}
	release, _, err = scope.lockCurrent(ctx)
	if err == nil {
		release()
	}
	return err
}

// Only the compiled App/strict WFP path consumes a prepared lifecycle lease.
// A native probe, injected boundary or public report cannot call this with a
// valid private Manager commit context. The acceptance gate remains closed.
func (probe appProtectedProbe) CommitNativeForwarding(ctx context.Context, engine *core.Engine) (admission core.NativeTUNAdmission, finalErr error) {
	attempted, completed := false, false
	defer func() {
		if recovered := recover(); recovered != nil {
			completed = false
			finalErr = errors.Join(finalErr, fmt.Errorf("native TUN commit panic (%T)", recovered))
		}
		if completed {
			return
		}
		admission = core.NativeTUNAdmission{}
		if finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
		if attempted {
			finalErr = errors.Join(finalErr, restrictNativeTUNAfterFailure())
		}
	}()
	lease := probe.nativeCommit.take() // burn root preparation before every outcome
	subject, valid := probe.lifecycleSubject(ctx)
	if !valid || !subject.current(probe.app, ctx, probe.config) {
		return admission, core.ErrNativeScopeUnavailable
	}
	boundary, compiled := probe.boundary.(nativeTunnelBoundary)
	_, strict := boundary.policy.(strictWFPPolicy)
	if !core.NativeTUNForwardingAvailable() || lease == nil || !compiled || !strict || probe.app == nil || probe.config == nil || probe.factory != nil ||
		boundary.app != probe.app || lease.native == nil || engine == nil || lease.engine != engine || lease.native.engine != engine ||
		lease.runtime != lease.native.runtime || lease.luid == 0 || lease.luid != lease.native.luid || lease.native.app != probe.app || lease.native.candidate != probe.config {
		return admission, core.ErrNativeScopeUnavailable
	}
	scope := lease.native
	if err := engine.ValidateNativeForwardingContext(ctx, scope.observation); err != nil {
		return admission, err
	}
	if err := scope.validatePrepared(ctx, probe.config); err != nil {
		return admission, err
	}
	attempted = true // register possible mutation BEFORE the native writer call
	if err := permitPreparedWFPTUNContext(ctx, scope.luid); err != nil {
		return admission, err
	}
	if err := scope.validateCommitPhase(ctx, nativeTUNWFPReady); err != nil {
		return admission, err
	}
	admission, finalErr = engine.CommitPreparedNative(ctx, scope.observation)
	if finalErr != nil {
		return admission, finalErr
	}
	if err := scope.validateCommitPhase(ctx, nativeTUNAdmitted); err != nil {
		return admission, err
	}
	completed = true
	return admission, nil
}

// A failed/unknown write may have committed. Restrict the SAME writer owner
// with an independent cleanup deadline; never Release or adopt a new owner.
func restrictNativeTUNAfterFailure() (finalErr error) {
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup, fmt.Errorf("native TUN restriction did not return normally (%T)", recovered))
		}
		if finalErr != nil {
			finalErr = errors.Join(core.ErrEngineCleanupFailed, finalErr)
		}
	}()
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := revokeWFPTUNContext(cleanup); err != nil {
		returned = true
		return err
	}
	finalErr = verifyWFPClosedContext(cleanup)
	returned = true
	return finalErr
}
