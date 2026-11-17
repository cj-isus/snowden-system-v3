package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

var errAppLifetimeUnavailable = errors.New("original application lifetime unavailable")

// Fixed internal dispatchers construct a fresh restricted App without Wails
// startup. Their original partial-owner slot is reserved before this callback;
// bind lifetime before any native/storage acquisition, outside value locks.
func (a *App) bindHeadlessLifetime(ctx context.Context) error {
	if a == nil {
		return errAppLifetimeUnavailable
	}
	fence, err := core.CaptureOperationFence(ctx)
	if err != nil {
		return errAppLifetimeUnavailable
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx != nil || a.startupAttempted || !a.installedImageChanged || a.closing.Load() || a.quitRequested || !fence.Current() {
		return errAppLifetimeUnavailable
	}
	a.ctx, a.ctxFence = ctx, fence
	return nil
}

// App.mu held. Lifetime only: this is not readiness, trust, retirement or a
// forwarding grant. Startup publishes one original fence; zero never falls
// back to calling the original Context while a value lock is held.
func (a *App) appLifetimeCurrentLocked() bool {
	return a != nil && a.ctx != nil && a.ctxFence.Current()
}

// Capture before taking any registry/owner lock that follows App.mu in order.
// The immutable copy still observes original cancellation/source revocation.
func (a *App) appLifetimeFence() core.OperationFence {
	if a == nil {
		return core.OperationFence{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		return core.OperationFence{}
	}
	return a.ctxFence
}

// No App/registry/owner value lock may already be held by this caller.
func (a *App) appLifetimeCurrent() bool {
	return a.appLifetimeFence().Current()
}
