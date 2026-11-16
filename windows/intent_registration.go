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
func (a *App) contextForIntent(registration intentRegistration) (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || !registration.currentLocked(a) {
		return nil, context.Canceled
	}
	ctx := netEpoch.scopeContext(a.ctx)
	path, _ := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	var sources core.OperationSources
	if path != nil {
		sources = core.OperationSources{Intent: registration.source, Path: &path.current, Closing: &a.closing}
	}
	ctx = core.WithOperationSources(ctx, sources)
	if !lifecyclePublicationCurrent(ctx) {
		return nil, context.Canceled
	}
	return ctx, nil
}

// App.mu and operationMu held. Reuse the one existing update operation slot
// and epoch cancellation. Unlike beginOperation this does not require running
// protection, and creates no appOperationScope/probe/native/health authority.
func (a *App) bindUpdateOperationLocked(registration intentRegistration) (context.Context, context.CancelFunc, error) {
	if a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || !registration.currentLocked(a) {
		return nil, nil, context.Canceled
	}
	ctx, cancel := netEpoch.bindOperation(a.ctx)
	operation := core.NewOperationSource()
	path, _ := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	var sources core.OperationSources
	if path != nil {
		sources = core.OperationSources{Intent: registration.source, Operation: operation, Path: &path.current, Closing: &a.closing}
	}
	ctx = core.WithOperationSources(ctx, sources)
	retire := func() {
		operation.Revoke() // before ordinary context cancellation; original cleanup remains independent
		cancel()
	}
	a.operationCancel = retire
	return ctx, retire, nil
}
