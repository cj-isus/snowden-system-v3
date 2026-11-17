package main

import (
	"context"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Only beginOperation creates this value, before any lifecycle work. Context
// descendants retain the caller's intent even when a new Start is registered.
type appOperationContextKey struct{}
type appOperationScope struct {
	app          *App
	intent       uint64
	intentSource *core.OperationSource
	operation    *core.OperationSource
}

func (a *App) operationIntent(ctx context.Context) (uint64, bool) {
	if a == nil || ctx == nil {
		return 0, false
	}
	scope, ok := ctx.Value(appOperationContextKey{}).(*appOperationScope)
	if !ok || scope == nil {
		return 0, false
	}
	return scope.intent, scope.app == a && scope.intent != 0 && scope.intentSource.Current() && scope.operation.Current() && lifecyclePublicationCurrent(ctx)
}

// A private measurement subject, never health evidence or an admission grant.
// Periodic probes capture every field before Run. A retained lifecycle probe
// binds Manager/intent at construction, then captures that Manager's active
// generation and the original caller's path for each fresh lifecycle Run.
type appProbeSubject struct {
	operation                 bool
	manager                   *core.Manager
	engine                    *core.Engine
	applied                   *render.ClientConfig
	intent, owner, generation uint64
	intentSource              *core.OperationSource
	path                      *networkPathSource
	epoch                     int
}

func (a *App) appliedProbeSubject(cc *render.ClientConfig) appProbeSubject {
	subject := appProbeSubject{applied: cc}
	if a == nil || cc == nil || cc.SessionID == 0 {
		return subject
	}
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	manager := runtime_.manager
	if runtime_.cc != cc || manager == nil || runtime_.engine == nil {
		return subject
	}
	// The report supplies only the applied session's original identity. Its
	// old PASS/time/network cannot establish health on this probe's new path.
	report := manager.LastProbeReport()
	if report == nil || report.Stale || report.ObservationID == 0 || report.SessionID != cc.SessionID {
		return subject
	}
	subject.manager, subject.engine = manager, runtime_.engine
	subject.intent = report.IntentGeneration
	subject.intentSource = report.OperationSource() // exact original source, never a current-App lookup
	subject.owner, subject.generation = manager.Owner()
	if report.OwnerID != subject.owner || report.OwnerGeneration == 0 || report.OwnerGeneration > subject.generation {
		return appProbeSubject{applied: cc}
	}
	// A rejected mutation may retain this session under a later generation
	// without a new report. Capture that actual generation before Run; the old
	// report supplies neither a new generation nor a refreshed path result.
	netEpoch.mu.Lock()
	subject.path, subject.epoch = netEpoch.pathSourceLocked(), netEpoch.num
	netEpoch.mu.Unlock()
	return subject
}

func (p appProtectedProbe) lifecycleSubject(ctx context.Context) (appProbeSubject, bool) {
	subject := p.subject
	intent, ok := p.app.operationIntent(ctx)
	if !subject.operation || !ok || intent != subject.intent || subject.manager == nil {
		return subject, false
	}
	scope, _ := ctx.Value(appOperationContextKey{}).(*appOperationScope)
	if scope.intentSource != subject.intentSource {
		return subject, false
	}
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	if runtime_.manager != subject.manager || runtime_.engine == nil {
		return subject, false
	}
	subject.engine = runtime_.engine
	subject.owner, subject.generation = subject.manager.Owner()
	// No global-path fallback: rollback may use a fresh original caller ctx,
	// but a delayed call with a revoked ctx cannot adopt its replacement path.
	subject.path, _ = ctx.Value(networkPathContextKey{}).(*networkPathSource)
	netEpoch.mu.Lock()
	subject.epoch = netEpoch.num
	current := subject.path != nil && subject.path == netEpoch.path && subject.path.current.Load()
	netEpoch.mu.Unlock()
	return subject, current
}

func (p appProtectedProbe) runSubject(ctx context.Context) (appProbeSubject, bool) {
	if p.subject.operation {
		if !p.lifecycle {
			return p.subject, false // lifecycle binding cannot become a periodic probe
		}
		return p.lifecycleSubject(ctx)
	}
	return p.subject, !p.lifecycle
}

// Same publication lock order: runtime -> Manager -> App -> epoch. Only short
// identity/state checks run under these locks; no factory, native/IPC work,
// probe, events, cancellation callback, cleanup or external I/O.
func (subject appProbeSubject) current(a *App, ctx context.Context, config *render.ClientConfig) bool {
	release, ok := subject.lockCurrent(a, ctx, config)
	if ok {
		release()
	}
	return ok
}

// Retain the same value-only publication lease when a consumer must commit
// local state under these identities. Never perform I/O or callbacks here.
func (subject appProbeSubject) lockCurrent(a *App, ctx context.Context, config *render.ClientConfig) (func(), bool) {
	if a == nil || config == nil || config.SessionID == 0 || subject.manager == nil || subject.engine == nil || subject.intent == 0 || !subject.intentSource.Current() || subject.path == nil {
		return nil, false
	}
	publication := capturePublicationContext(ctx)
	if publication == nil {
		return nil, false
	}
	if publication.hasOperation {
		original := publication.operation
		if original == nil || original.app != a || original.intent != subject.intent || original.intentSource != subject.intentSource || !original.operation.Current() {
			return nil, false
		}
	} else if subject.operation {
		return nil, false
	}
	if publication.hasPath && publication.path != subject.path {
		return nil, false
	}
	runtime_.mu.Lock()
	handedOff := false
	defer func() {
		if !handedOff {
			runtime_.mu.Unlock()
		}
	}()
	if runtime_.manager != subject.manager || runtime_.engine != subject.engine {
		return nil, false
	}
	var releaseOwner func()
	var ok bool
	if subject.operation {
		releaseOwner, ok = subject.manager.GuardLifecycleOwner(subject.owner, subject.generation, subject.engine)
	} else {
		if runtime_.cc != subject.applied || subject.applied == nil || config.SessionID != subject.applied.SessionID ||
			config.OwnerID != subject.owner || config.OwnerGeneration != subject.generation || subject.applied.OwnerID != subject.owner || subject.applied.OwnerGeneration != subject.generation {
			return nil, false
		}
		releaseOwner, ok = subject.manager.GuardRunningOwner(subject.owner, subject.generation)
	}
	if !ok {
		return nil, false
	}
	defer func() {
		if !handedOff {
			releaseOwner()
		}
	}()
	a.mu.Lock()
	defer func() {
		if !handedOff {
			a.mu.Unlock()
		}
	}()
	netEpoch.mu.Lock()
	defer func() {
		if !handedOff {
			netEpoch.mu.Unlock()
		}
	}()
	release := func() {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
	}
	if a.operationID != subject.intent || a.intentSource != subject.intentSource || !subject.intentSource.Current() || !a.protectionDesired || a.profileTransition || a.closing.Load() || a.trustBlocked || a.restartCommit != nil || a.installedImageChanged {
		return nil, false
	}
	if subject.path != netEpoch.path || !subject.path.current.Load() || netEpoch.knownDown || netEpoch.powerBlocked.Load() ||
		subject.epoch != netEpoch.num || !publication.current() {
		return nil, false
	}
	handedOff = true
	return release, true
}

// Compatibility entrypoint. Manager prepares the guard through the separate
// preparation API before taking its terminal lock; only that returned guard
// acquires App/epoch value locks during the terminal decision.
func (p appProtectedProbe) GuardLifecycleCompletion(ctx context.Context, manager *core.Manager) (func(), error) {
	guard, err := p.PrepareLifecycleCompletion(ctx, manager)
	if err != nil {
		return nil, err
	}
	return guard()
}

// Manager calls preparation outside its lock. The returned downstream guard
// retains only these exact original value sources; it never calls ctx methods.
func (p appProtectedProbe) PrepareLifecycleCompletion(ctx context.Context, manager *core.Manager) (core.LifecycleCompletionGuard, error) {
	intent, valid := p.app.operationIntent(ctx)
	if !valid || !p.subject.operation || p.subject.manager != manager || manager == nil || intent != p.subject.intent ||
		p.config == nil || p.config.SessionID == 0 {
		return nil, errSessionSuperseded
	}
	path, _ := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	if path == nil {
		return nil, errSessionSuperseded
	}
	scope, _ := ctx.Value(appOperationContextKey{}).(*appOperationScope)
	if scope == nil || scope.intentSource != p.subject.intentSource {
		return nil, errSessionSuperseded
	}
	intentSource, operationSource := scope.intentSource, scope.operation
	a := p.app
	return func() (func(), error) {
		a.mu.Lock()
		netEpoch.mu.Lock()
		release := func() {
			netEpoch.mu.Unlock()
			a.mu.Unlock()
		}
		transferred := false
		defer func() {
			if !transferred {
				release()
			}
		}()
		if a.operationID != intent || a.intentSource != intentSource || !intentSource.Current() || !operationSource.Current() || !a.protectionDesired || a.profileTransition || a.closing.Load() || a.trustBlocked || a.restartCommit != nil || a.installedImageChanged ||
			netEpoch.path != path || !path.current.Load() || netEpoch.knownDown || netEpoch.powerBlocked.Load() {
			return nil, errSessionSuperseded
		}
		transferred = true
		return release, nil
	}, nil
}
