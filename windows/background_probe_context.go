package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

// The original reserved creator owns this preparation through actual cleanup.
// A failed setup never fabricates the return of the background probing loop.
func (owner *backgroundProbeTask) prepareContext(parent appTaskParent) (result error) {
	transferred, returned := false, false
	defer func() {
		if transferred {
			return
		}
		if !returned {
			_ = recover()
			result = errors.Join(result, core.ErrEngineCleanupFailed)
		}
		owner.fail()
		defer close(owner.done) // original failed setup frame, never loop/diagnostic entry
		sequence := new(appCleanupSequence)
		var cleanupResult error
		defer func() {
			owner.mu.Lock()
			defer owner.mu.Unlock()
			owner.retirement = appProducerReceipt{preparationOnly: true, preparationReturned: returned, cleanupReturned: sequence.drained && cleanupResult == nil, cleanupResult: cleanupResult}
			result = errors.Join(errBackgroundProbeUnavailable, result, cleanupResult)
		}()
		defer sequence.finish(&cleanupResult)
		defer appCleanupStep("retire original background context preparation", sequence, owner.contextState.cancellation.join)
	}()
	result = owner.prepareContextBody(parent)
	returned = true
	transferred = result == nil
	return result
}

func (owner *backgroundProbeTask) prepareContextBody(parent appTaskParent) error {
	ctx, fence, err := owner.contextState.construct(parent)
	if err != nil {
		return errors.Join(errBackgroundProbeUnavailable, err)
	}
	a := owner.app
	a.mu.Lock()
	defer a.mu.Unlock()
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if parent.app != a || !parent.fence.Current() || !a.appLifetimeCurrentLocked() || a.closing.Load() || a.quitRequested ||
		a.startupErr != nil || a.installedImageChanged || a.backgroundProbe != owner || !bgReg.started || bgReg.stop != owner.stop ||
		owner.stopping || owner.fault != nil || !owner.contextState.cancellation.operation.Current() || !fence.Current() {
		return errBackgroundProbeUnavailable
	}
	owner.ctx = ctx
	owner.contextState.parentFence, owner.contextState.childFence, owner.contextState.ready = parent.fence, fence, true
	return nil
}

// App -> epoch values only. Capture before the first real parent getter so a
// reentrant callback cannot make this round adopt a newer request or path.
func (a *App) captureBackgroundProbeScopeValues(owner *backgroundProbeTask) (*backgroundProbeScope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	scope := &backgroundProbeScope{app: a, owner: owner, registration: a.intentRegistrationLocked(), activeID: a.state.ActiveID}
	scope.sourceCurrent = scope.registration.source.Current()
	if a.closing.Load() || a.profileTransition || a.trustBlocked || a.startupErr != nil || a.installedImageChanged ||
		a.restartCommit != nil || a.releaseCommit != nil || !scope.registration.sameLocked(a) {
		return nil, errBackgroundProbeUnavailable
	}
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	scope.path = netEpoch.pathSourceLocked()
	scope.epoch, scope.control = netEpoch.num, netEpoch.controlSource
	if scope.epoch <= 0 || scope.path == nil || !scope.path.current.Load() || !scope.control.Current() || netEpoch.knownDown || netEpoch.powerBlocked.Load() {
		return nil, errBackgroundProbeUnavailable
	}
	return scope, nil
}

func (scope *backgroundProbeScope) prepareContext() (prepared *backgroundProbeScope, result error) {
	returned := false
	defer func() {
		if !returned {
			_ = recover() // no arbitrary parent payload leaves this optional task boundary
			scope.owner.fail()
			prepared, result = nil, errBackgroundProbeUnavailable
		}
	}()
	result = scope.prepareContextBody()
	returned = true
	if result != nil {
		return nil, result
	}
	return scope, nil
}

func (scope *backgroundProbeScope) prepareContextBody() error {
	if !scope.valuesCurrent() {
		return errBackgroundProbeUnavailable
	}
	parent := scope.owner.ctx // immutable actual child, published before this task can become available
	value := parent.Value(networkPathContextKey{})
	if value != nil {
		original, ok := value.(*networkPathSource)
		if !ok || original != scope.path {
			return errBackgroundProbeUnavailable
		}
	}
	ctx := parent
	if value == nil {
		ctx = context.WithValue(parent, networkPathContextKey{}, scope.path)
	}
	fence, err := core.CaptureOperationFence(ctx) // actual methods outside App/registry/epoch/task locks
	if err != nil {
		return errBackgroundProbeUnavailable
	}
	scope.ctx, scope.fence = ctx, fence
	if !scope.current() {
		return errBackgroundProbeUnavailable
	}
	return nil
}
