package main

import (
	"context"
	"sync/atomic"
)

type profileRequestContextKey struct{}

// Original process-local negative source, not a public approval/native receipt.
// The one writer may transfer it only in its actual profile retirement under
// App.mu. Journal callbacks read atomic source values and never lock App.
type profileRequestScope struct {
	app             *App
	parent, request context.Context
	registration    atomic.Pointer[intentRegistration]
}

// Pure negative check for selected input/output and final result publication.
// It does not change Context.Err/Done semantics or grant filesystem authority.
func profileRequestCurrent(ctx context.Context) bool {
	if !lifecyclePublicationCurrent(ctx) {
		return false
	}
	value := ctx.Value(profileRequestContextKey{})
	if value == nil {
		return true
	} // existing independent non-App read contexts
	scope, ok := value.(*profileRequestScope)
	return ok && scope.current()
}

func (scope *profileRequestScope) current() bool {
	if scope == nil || scope.app == nil || scope.parent == nil || scope.parent.Err() != nil || scope.app.closing.Load() || !lifecyclePublicationCurrent(scope.request) {
		return false
	}
	registration := scope.registration.Load()
	return registration != nil && registration.source.Current()
}

// Caller owns App.mu. Scalar ID alone cannot adopt another original source.
func (scope *profileRequestScope) currentLocked(app *App) bool {
	if scope == nil || scope.app != app || !scope.current() {
		return false
	}
	registration := scope.registration.Load()
	return registration != nil && registration.currentLocked(app)
}

func (a *App) bindProfileRequest(ctx, parent context.Context, registration intentRegistration) context.Context {
	if ctx == nil || parent == nil {
		return nil
	}
	if ctx.Value(profileRequestContextKey{}) != nil {
		return ctx
	} // never overwrite an old/invalid source
	scope := &profileRequestScope{app: a, parent: parent, request: ctx}
	scope.registration.Store(&registration)
	return context.WithValue(ctx, profileRequestContextKey{}, scope)
}

// Called before decoding/KDF/serializer waits. A context supplied by an actual
// public-source flight keeps its admission registration, even if it went stale.
func (a *App) captureProfileRequest(ctx context.Context) (context.Context, *profileRequestScope, error) {
	if ctx != nil {
		if value := ctx.Value(profileRequestContextKey{}); value != nil {
			scope, ok := value.(*profileRequestScope)
			if !ok || scope == nil || scope.app != a || ctx.Err() != nil || !scope.current() {
				return nil, nil, context.Canceled
			}
			return ctx, scope, nil
		}
	}
	a.mu.Lock()
	registration, err := a.restartIntentLocked()
	parent := a.ctx
	a.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if ctx == nil {
		ctx = parent
	}
	ctx = a.bindProfileRequest(ctx, parent, registration)
	if ctx == nil {
		return nil, nil, context.Canceled
	}
	scope, ok := ctx.Value(profileRequestContextKey{}).(*profileRequestScope)
	if !ok || !scope.current() {
		return nil, nil, context.Canceled
	}
	return ctx, scope, nil
}

// Caller owns operationMu, no journal/vault lock. The actual retirement below
// performs the only source transfer; a newer manual Stop is never adopted.
func (a *App) retireProfileRequest(scope *profileRequestScope) (uint64, error) {
	if scope == nil || scope.app != a {
		return 0, context.Canceled
	}
	registration := scope.registration.Load()
	if registration == nil {
		return 0, context.Canceled
	}
	return a.retireProfileSessionRegistered(*registration, scope)
}
