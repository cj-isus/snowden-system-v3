package main

import (
	"context"

	"github.com/snowden-system/windows/backend/core"
)

type appOperationKind uint8

const (
	appForegroundOperation appOperationKind = iota + 1
	appUpdateOperation
	appCommittedUpdateOperation
	appProxyRecoveryOperation
)

// Immutable preparation inputs for the existing operation slot. This record
// is not a Context, worker, native lease or positive forwarding authority.
type appOperationPreparation struct {
	app          *App
	owner        *appOperationCancellation
	kind         appOperationKind
	parent       context.Context
	lifetime     core.OperationFence
	registration intentRegistration
	path         *networkPathSource
	proxy        *appProxyRecovery
	handoff      *restartHandoff
	recovery     bool
}

// App.mu and operationMu held. Reserve before any constructor/getter callback.
// Callers perform their mode admission first; build repeats it at publication.
func (a *App) reserveOperationContextLocked(registration intentRegistration, kind appOperationKind, proxy *appProxyRecovery, handoff *restartHandoff, recovery bool) (*appOperationPreparation, error) {
	if !a.appLifetimeCurrentLocked() || a.closing.Load() || !registration.currentLocked(a) {
		return nil, context.Canceled
	}
	switch a.operationSlotBlockLocked() {
	case "lifecycle_busy":
		return nil, errAppOperationBusy
	case "cleanup_unconfirmed":
		return nil, core.ErrEngineCleanupFailed
	case "":
		// Continue through the original mode/lifetime/path gates.
	default:
		return nil, core.ErrEngineCleanupFailed
	}
	preparation := &appOperationPreparation{app: a, kind: kind, parent: a.ctx, lifetime: a.ctxFence, registration: registration, proxy: proxy, handoff: handoff, recovery: recovery}
	if kind != appProxyRecoveryOperation {
		preparation.path = netEpoch.pathSource() // App -> epoch, passive continuity values only
	}
	preparation.owner = newAppOperationCancellation(core.NewOperationSource())
	a.operationCancellation = preparation.owner
	a.operationCancel = preparation.owner.request
	return preparation, nil
}

// App.mu held. All predicates use captured identities, atomics or prepared
// fences; no original Context method or arbitrary admission callback runs here.
func (preparation *appOperationPreparation) currentLocked(publication *publicationContext) bool {
	a, owner := preparation.app, preparation.owner
	if a.operationCancellation != owner || a.operationCancel == nil || !owner.operation.Current() ||
		!preparation.lifetime.Current() || !a.appLifetimeCurrentLocked() || a.closing.Load() ||
		!preparation.registration.currentLocked(a) || !publication.current() {
		return false
	}
	switch preparation.kind {
	case appForegroundOperation:
		if a.startupErr != nil || a.watchdogBlocked.Load() || !a.protectionDesired || a.profileTransition || a.proxyConflict || a.restartCommit != nil || a.installedImageChanged || preparation.registration.intent == 0 {
			return false
		}
	case appUpdateOperation:
		if a.restartCommit != nil || a.profileTransition || a.installedImageChanged != preparation.recovery {
			return false
		}
	case appCommittedUpdateOperation:
		h := preparation.handoff
		if h == nil || h.app != a || a.restartCommit != h || !h.request.currentLocked(a) || a.installedImageChanged {
			return false
		}
	case appProxyRecoveryOperation:
		return preparation.proxy != nil && preparation.proxy.currentLocked(a)
	default:
		return false
	}
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	return preparation.path != nil && netEpoch.path == preparation.path && preparation.path.current.Load() &&
		!netEpoch.knownDown && !netEpoch.powerBlocked.Load() && netEpoch.operationOwner == owner
}

// Only the original constructor frame records its actual return/unwind. Real
// contexts inherit the actual parent; fence snapshots never become parents.
func (preparation *appOperationPreparation) construct() (ctx context.Context, publication *publicationContext, finalErr error) {
	owner := preparation.owner
	returned := false
	defer func() {
		if !returned {
			_ = recover() // no arbitrary context payload leaves this boundary; Goexit still unwinds caller
			ctx, publication, finalErr = nil, nil, core.ErrEngineCleanupFailed
		}
		owner.prepareReturned.Store(returned)
		close(owner.prepared)
	}()
	if !owner.operation.Current() || !preparation.lifetime.Current() || preparation.path != nil && !preparation.path.current.Load() {
		returned = true
		return nil, nil, context.Canceled // no child constructor was entered
	}
	owner.attempted.Store(true)
	child, cancel := context.WithCancel(preparation.parent)
	retire := cancel
	if preparation.path != nil {
		retire = func() {
			cancel()
			netEpoch.releaseOperationOwner(owner) // only after actual original cancel returned
		}
	}
	if err := owner.storeCancel(retire); err != nil {
		return nil, nil, err // abnormal ownership: do not abandon or overwrite the first partial child
	}
	if !owner.operation.Current() || !preparation.lifetime.Current() {
		returned = true
		return nil, nil, context.Canceled // retire known canceled child before another original parent getter
	}
	ctx = child
	if path := preparation.path; path != nil {
		value := preparation.parent.Value(networkPathContextKey{})
		if value != nil {
			original, ok := value.(*networkPathSource)
			if !ok || original != path {
				returned = true
				return nil, nil, context.Canceled // existing old/unknown binding never adopts a newer route
			}
		}
		ctx = context.WithValue(ctx, networkPathContextKey{}, path)
		if !netEpoch.bindOperationOwner(owner, path) {
			returned = true
			return nil, nil, context.Canceled
		}
	}
	sources := core.OperationSources{Intent: preparation.registration.source, Operation: owner.operation, Closing: &preparation.app.closing}
	if preparation.kind == appProxyRecoveryOperation {
		sources.Path = &preparation.proxy.current
	} else {
		sources.Path = &preparation.path.current
	}
	if preparation.kind == appForegroundOperation {
		sources.Blocked = &preparation.app.watchdogBlocked
		scope := &appOperationScope{app: preparation.app, intent: preparation.registration.intent, intentSource: preparation.registration.source, operation: owner.operation}
		ctx = context.WithValue(ctx, appOperationContextKey{}, scope)
	}
	ctx = core.WithOperationSources(ctx, sources)
	publication = capturePublicationContext(ctx) // all actual getters outside App/epoch/owner value locks
	returned = true
	if publication == nil {
		return nil, nil, context.Canceled
	}
	return ctx, publication, nil
}

// operationMu held; App.mu must be released before entry. Until success, this
// original frame owns retirement even if the factory panics or calls Goexit.
func (preparation *appOperationPreparation) build() (ctx context.Context, cancel context.CancelFunc, finalErr error) {
	transferred := false
	defer func() {
		if transferred {
			return
		}
		ctx, cancel = nil, nil
		sequence := new(appCleanupSequence)
		appCleanupStep("retire original operation preparation", sequence, func() error {
			preparation.app.endOperation(preparation.owner.request)
			return nil
		})
		if sequence.result != nil {
			finalErr = core.ErrEngineCleanupFailed
		}
	}()
	ctx, publication, err := preparation.construct()
	if err != nil {
		return nil, nil, err
	}
	current := func() bool {
		preparation.app.mu.Lock()
		defer preparation.app.mu.Unlock()
		return preparation.currentLocked(publication)
	}()
	if !current {
		return nil, nil, context.Canceled
	}
	transferred = true
	return ctx, preparation.owner.request, nil
}
