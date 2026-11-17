package main

import (
	"context"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

// Applied configs are immutable after publication. Keep both their exact owner
// pointer and an owned clone: later UI/profile/selector reads cannot retarget a
// request. Neither this subject nor its worker/LUID identities leave the App.
type nativeProbeScope struct {
	applied *render.ClientConfig
	config  *render.ClientConfig
	manager *core.Manager
	engine  *core.Engine
	clock   *metadata.ObservedClock
	intent  uint64
	source  *core.OperationSource // exact original App intent; own operation cancel does not retag cached facts
	path    *networkPathSource    // original continuity source, independent of this action's local cancel
	epoch   int
	pin     string
	lease   tunnelLease
	started time.Time
}

type nativeProbeObservation struct {
	scope   *nativeProbeScope
	view    NativeProbeReportView // guarded by App.mu, no mutable slices/pointers
	expires time.Time
}

// lockCurrent uses only in-memory state (including Manager's local worker
// status). No IPC, file, interface, WFP, events or callbacks run inside this
// publication gate. Order matches observer_publication.go. Caller must release
// before invoking any lifecycle/native work, or acquiring App.mu again.
func (s *nativeProbeScope) lockCurrent(a *App, ctx context.Context) (func(), time.Time, error) {
	if a == nil || s == nil || s.applied == nil || s.config == nil || s.manager == nil || s.engine == nil || s.clock == nil || s.path == nil || s.epoch <= 0 {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	publication := capturePublicationContext(ctx)
	if publication == nil {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	runtime_.mu.Lock()
	if runtime_.cc != s.applied || runtime_.manager != s.manager || runtime_.engine != s.engine {
		runtime_.mu.Unlock()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	releaseOwner, ok := s.manager.GuardRunningOwner(s.config.OwnerID, s.config.OwnerGeneration)
	if !ok {
		runtime_.mu.Unlock()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	a.mu.Lock()
	netEpoch.mu.Lock()
	release := func() {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
	}
	handedOff := false
	defer func() {
		if !handedOff {
			release() // error/panic/Goexit cannot retain this acquired value-only lease
		}
	}()
	cc := s.applied
	if !publication.current() || !a.protectionDesired || a.startupErr != nil || a.watchdogBlocked.Load() || a.profileTransition || a.closing.Load() || a.trustBlocked || a.operationID != s.intent || a.intentSource != s.source || !s.source.Current() || a.observedClock != s.clock ||
		a.probeFactory != nil || a.tunnelBoundary != nil || a.policy != nil || nativeProbeHelperSHA256 != s.pin || !canonicalApprovalSHA256(s.pin) ||
		netEpoch.num != s.epoch || netEpoch.path != s.path || !s.path.current.Load() || netEpoch.knownDown || netEpoch.powerBlocked.Load() ||
		cc.SessionID == 0 || cc.SessionID != s.config.SessionID || cc.OwnerID != s.config.OwnerID || cc.OwnerGeneration != s.config.OwnerGeneration ||
		cc.TUNName != render.TUNInterfaceName || cc.TUNName != s.config.TUNName || cc.SelectorDefault != s.config.SelectorDefault || cc.Profile != s.config.Profile || cc.Validity != s.config.Validity {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	now, err := s.clock.Now(s.config.Validity.ClockFloor)
	if err == nil {
		_, err = s.config.Validity.Remaining(now)
	}
	if err != nil || !s.started.IsZero() && now.Before(s.started) || !publication.current() || !s.source.Current() || !s.path.current.Load() || a.closing.Load() || a.watchdogBlocked.Load() || netEpoch.powerBlocked.Load() {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	handedOff = true
	return release, now, nil
}

// Caller holds operationMu. Only this explicit native action may reach the OS;
// injected/local probe components opt out before runtime or interface queries.
func (a *App) captureNativeProbeScope(ctx context.Context, intent uint64) (*nativeProbeScope, error) {
	publication := capturePublicationContext(ctx)
	if publication == nil || !publication.hasOperation || !publication.hasPath || publication.path == nil {
		return nil, core.ErrNativeScopeUnavailable
	}
	operation := publication.operation
	if operation == nil || operation.app != a || operation.intent != intent || !operation.intentSource.Current() {
		return nil, core.ErrNativeScopeUnavailable
	}
	a.mu.Lock()
	custom := a.probeFactory != nil || a.tunnelBoundary != nil || a.policy != nil
	a.mu.Unlock()
	if custom || !canonicalApprovalSHA256(nativeProbeHelperSHA256) {
		return nil, core.ErrNativeScopeUnavailable
	}
	clock := a.trustClock()
	runtime_.mu.Lock()
	applied, manager, engine := runtime_.cc, runtime_.manager, runtime_.engine
	runtime_.mu.Unlock()
	if applied == nil || manager == nil || engine == nil || applied.TUNName != render.TUNInterfaceName {
		return nil, core.ErrNativeScopeUnavailable
	}
	scope := &nativeProbeScope{applied: applied, config: applied.Clone(), manager: manager, engine: engine, clock: clock,
		intent: intent, source: operation.intentSource, path: publication.path, epoch: currentChannelEpoch(), pin: nativeProbeHelperSHA256}
	release, started, err := scope.lockCurrent(a, ctx)
	if err != nil {
		return nil, err
	}
	release()
	scope.started = started
	actualEngine, identity, err := currentTunnelRuntime(ctx, scope.config)
	if err != nil || actualEngine != engine {
		return nil, core.ErrNativeScopeUnavailable
	}
	luid, err := waitInterfaceLUIDContext(ctx, scope.config.TUNName, 250*time.Millisecond)
	if err != nil || luid == 0 {
		return nil, core.ErrNativeScopeUnavailable
	}
	scope.lease = tunnelLease{engine: engine, runtime: identity, luid: luid}
	if err := scope.guard(a, ctx); err != nil {
		return nil, err
	}
	return scope, nil
}

// Full before/after guard for CaptureNativeDNS: current intent/source/owner,
// worker config+selector, observed interface identity, existing policy shape,
// epoch and clock. It observes policy; it never establishes a new TUN permit.
func (s *nativeProbeScope) guard(a *App, ctx context.Context) error {
	release, _, err := s.lockCurrent(a, ctx)
	if err != nil {
		return err
	}
	release()
	engine, identity, err := currentTunnelRuntime(ctx, s.config)
	if err != nil || engine != s.lease.engine || identity != s.lease.runtime || s.lease.luid == 0 {
		return core.ErrNativeScopeUnavailable
	}
	luid, err := waitInterfaceLUIDContext(ctx, s.config.TUNName, 250*time.Millisecond)
	if err != nil || luid != s.lease.luid {
		return core.ErrNativeScopeUnavailable
	}
	if verifyWFPPolicyContext(ctx, true, luid) != nil {
		return core.ErrNativeScopeUnavailable
	}
	release, _, err = s.lockCurrent(a, ctx)
	if err == nil {
		release()
	}
	return err
}
