package main

import (
	"context"

	"github.com/snowden-system/windows/backend/core"
)

// Process-local registration, never a public request/receipt or forwarding
// authority. A numeric ID cannot substitute for its original revocation source.
// Initial ID0 is usable only for local maintenance; beginOperation still denies
// forwarding work with zero intent.
type intentRegistration struct {
	intent uint64
	source *core.OperationSource
}

// App.mu held. Negative cleanup needs the original registration even if its
// positive source is revoked. A successor's same scalar ID is never a match.
func (registration intentRegistration) sameLocked(app *App) bool {
	return registration.source != nil && app.operationID == registration.intent && app.intentSource == registration.source
}

func (registration intentRegistration) currentLocked(app *App) bool {
	return registration.sameLocked(app) && registration.source.Current()
}

func (a *App) intentRegistrationLocked() intentRegistration {
	return intentRegistration{intent: a.operationID, source: a.intentSource}
}

// Staging outside the serialized lifecycle slot may only inherit this exact
// request. The actual original path is a negative continuity fence, not network
// availability/proof; scopeContext never replaces an existing stale path.
func (a *App) contextForIntent(registration intentRegistration) (ctx context.Context, finalErr error) {
	returned := false
	defer func() {
		if !returned {
			_ = recover() // no context payload in a staging error; Goexit still unwinds caller
			ctx, finalErr = nil, context.Canceled
		}
	}()
	if a == nil {
		return nil, context.Canceled
	}
	parent, lifetime, path, valid := func() (context.Context, core.OperationFence, *networkPathSource, bool) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.appLifetimeCurrentLocked() || a.closing.Load() || !registration.currentLocked(a) {
			return nil, core.OperationFence{}, nil, false
		}
		return a.ctx, a.ctxFence, netEpoch.pathSource(), true // App -> epoch, value-only
	}()
	if !valid || path == nil {
		return nil, context.Canceled
	}
	ctx = parent
	if original, ok := parent.Value(networkPathContextKey{}).(*networkPathSource); ok && original != nil {
		if original != path {
			return nil, context.Canceled // an existing old binding never adopts a new route
		}
	} else {
		ctx = context.WithValue(parent, networkPathContextKey{}, path)
	}
	ctx = core.WithOperationSources(ctx, core.OperationSources{Intent: registration.source, Path: &path.current, Closing: &a.closing})
	publication := capturePublicationContext(ctx) // real context methods outside value locks
	if publication == nil {
		return nil, context.Canceled
	}
	current := func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !lifetime.Current() || !a.appLifetimeCurrentLocked() || a.closing.Load() || !registration.currentLocked(a) {
			return false
		}
		netEpoch.mu.Lock()
		defer netEpoch.mu.Unlock()
		return netEpoch.path == path && path.current.Load() && !netEpoch.knownDown && !netEpoch.powerBlocked.Load() && publication.current()
	}()
	if !current {
		return nil, context.Canceled
	}
	returned = true
	return ctx, nil
}
