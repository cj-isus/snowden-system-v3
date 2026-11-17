package main

import (
	"context"
	"net/netip"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

// A lifecycle candidate has not been published into runtime_.cc and its config
// OwnerGeneration may still describe a previous operation. Bind the actual
// active Manager and original worker instead of adopting running-session state.
// This private scope contains no token and is never exposed through Wails.
type nativeTUNLifecycleScope struct {
	app                       *App
	candidate, config         *render.ClientConfig
	manager                   *core.Manager
	engine                    *core.Engine
	clock                     *metadata.ObservedClock
	operation                 context.Context
	operationFence            core.OperationFence // original scope lifetime, never refreshed by callers
	path                      *networkPathSource
	owner, generation, intent uint64
	epoch                     int
	pin                       string
	expected                  netip.Addr
	runtime                   core.RuntimeIdentity
	luid                      uint64
	started                   time.Time
	observation               core.NativeTUNObservation
}

// Same lock order as existing publication scopes: runtime -> Manager -> App ->
// epoch -> clock. All native/RPC/interface/filter work happens after release.
func (scope *nativeTUNLifecycleScope) lockCurrent(ctx context.Context) (func(), time.Time, error) {
	if scope == nil || scope.app == nil || scope.candidate == nil || scope.config == nil || scope.manager == nil || scope.engine == nil || scope.clock == nil || scope.operation == nil || !scope.operationFence.Current() || scope.path == nil || scope.epoch <= 0 {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	publication := capturePublicationContext(ctx)
	if publication == nil {
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	runtime_.mu.Lock()
	if runtime_.manager != scope.manager || runtime_.engine != scope.engine {
		runtime_.mu.Unlock()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	releaseOwner, ok := scope.manager.GuardLifecycleOwner(scope.owner, scope.generation, scope.engine)
	if !ok {
		runtime_.mu.Unlock()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	a := scope.app
	a.mu.Lock()
	netEpoch.mu.Lock()
	release := func() {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
	}
	cc := scope.candidate
	if !publication.current() || !scope.operationFence.Current() || !a.protectionDesired || a.profileTransition || a.closing.Load() || a.trustBlocked || a.restartCommit != nil || a.installedImageChanged ||
		a.operationID != scope.intent || a.observedClock != scope.clock || a.probeFactory != nil || a.tunnelBoundary != nil || a.policy != nil ||
		nativeProbeHelperSHA256 != scope.pin || !canonicalApprovalSHA256(scope.pin) || netEpoch.num != scope.epoch ||
		netEpoch.path != scope.path || !scope.path.current.Load() || netEpoch.knownDown || netEpoch.powerBlocked.Load() ||
		cc.SessionID == 0 || cc.SessionID != scope.config.SessionID || cc.TUNName != render.TUNInterfaceName || cc.TUNName != scope.config.TUNName ||
		cc.SelectorDefault != scope.config.SelectorDefault || cc.ExpectedEgress != scope.config.ExpectedEgress || cc.Profile != scope.config.Profile || cc.Validity != scope.config.Validity {
		release()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	now, err := scope.clock.Now(scope.config.Validity.ClockFloor)
	if err == nil {
		_, err = scope.config.Validity.Remaining(now)
	}
	if err != nil || !scope.started.IsZero() && now.Before(scope.started) || !publication.current() || !scope.operationFence.Current() {
		release()
		return nil, time.Time{}, core.ErrNativeScopeUnavailable
	}
	return release, now, nil
}

// Called by the owned lifecycle operation, never a manual/periodic probe. No
// helper is started for development builds, injected components, stale intent,
// an admitted worker or a profile without an expected IPv4 for this IPv4 actor.
func (a *App) captureTUNLifecycleScope(ctx context.Context, cc *render.ClientConfig) (*nativeTUNLifecycleScope, error) {
	if a == nil || ctx == nil || ctx.Err() != nil || cc == nil || cc.TUNName != render.TUNInterfaceName || !canonicalApprovalSHA256(nativeProbeHelperSHA256) {
		return nil, core.ErrNativeScopeUnavailable
	}
	expected, err := netip.ParseAddr(cc.ExpectedEgress)
	if err != nil || expected.Zone() != "" || !expected.Unmap().Is4() {
		return nil, core.ErrNativeScopeUnavailable
	}
	intent, valid := a.operationIntent(ctx)
	if !valid {
		return nil, core.ErrNativeScopeUnavailable
	}
	operationFence, err := core.CaptureOperationFence(ctx)
	if err != nil {
		return nil, core.ErrNativeScopeUnavailable
	}
	runtime_.mu.Lock()
	manager, engine := runtime_.manager, runtime_.engine
	runtime_.mu.Unlock()
	if manager == nil || engine == nil {
		return nil, core.ErrNativeScopeUnavailable
	}
	owner, generation := manager.Owner()
	path, _ := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	scope := &nativeTUNLifecycleScope{app: a, candidate: cc, config: cc.Clone(), manager: manager, engine: engine,
		clock: a.trustClock(), operation: ctx, operationFence: operationFence, path: path, owner: owner, generation: generation, intent: intent,
		epoch: currentChannelEpoch(), pin: nativeProbeHelperSHA256, expected: expected.Unmap()}
	release, started, err := scope.lockCurrent(ctx)
	if err != nil {
		return nil, err
	}
	release()
	scope.started = started
	actual, identity, err := currentTunnelRuntime(ctx, scope.config)
	if err != nil || actual != engine || identity.Instance == 0 || identity.AdmissionEpoch == 0 || identity.ForwardingAdmitted {
		return nil, core.ErrNativeScopeUnavailable
	}
	scope.runtime = identity
	scope.luid, err = waitInterfaceLUIDContext(ctx, scope.config.TUNName, 250*time.Millisecond)
	if err != nil || scope.luid == 0 {
		return nil, core.ErrNativeScopeUnavailable
	}
	if err := scope.guard(ctx); err != nil {
		return nil, err
	}
	return scope, nil
}

// The compiled WFP adapter independently validates the precise diagnostic rule
// while it exists. This callback checks only current source/actor/interface:
// requiring base-only filters here would reject the actor's own narrow grant.
func (scope *nativeTUNLifecycleScope) guard(ctx context.Context) error {
	release, _, err := scope.lockCurrent(ctx)
	if err != nil {
		return err
	}
	release()
	engine, identity, err := currentTunnelRuntime(ctx, scope.config)
	if err != nil || engine != scope.engine || identity != scope.runtime || identity.ForwardingAdmitted {
		return core.ErrNativeScopeUnavailable
	}
	luid, err := waitInterfaceLUIDContext(ctx, scope.config.TUNName, 250*time.Millisecond)
	if err != nil || luid != scope.luid {
		return core.ErrNativeScopeUnavailable
	}
	release, _, err = scope.lockCurrent(ctx)
	if err == nil {
		release()
	}
	return err
}

func (scope *nativeTUNLifecycleScope) matchesEgress(value string) bool {
	address, err := netip.ParseAddr(value)
	return err == nil && address.Unmap().Is4() && address.Unmap() == scope.expected
}

// Only the original candidate and still-closed worker may use preparation.
// Final commit uses a separate explicit phase validator; this guard stays closed.
func (scope *nativeTUNLifecycleScope) validatePrepared(ctx context.Context, cc *render.ClientConfig) error {
	if scope == nil || cc != scope.candidate || scope.observation.Runtime() != scope.runtime || !scope.matchesEgress(scope.observation.Egress()) {
		return core.ErrNativeScopeUnavailable
	}
	return scope.validateCommitPhase(ctx, nativeTUNPrepared)
}
