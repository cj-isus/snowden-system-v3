package main

import (
	"context"
	"errors"
	"sync"

	"github.com/snowden-system/windows/backend/core"
)

var errBackgroundProbeUnavailable = errors.New("original background probe outcome or retirement unconfirmed")

// One original process-lifetime producer. Registry flags are scheduling data;
// only this owner retains its original serial attempt and cleanup uncertainty.
type backgroundProbeTask struct {
	app          *App
	contextState appTaskContext
	ctx          context.Context
	cancel       context.CancelFunc
	stop         chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	mu           sync.Mutex
	stopping     bool
	returned     bool // original loop and cancel ordinary return; read after done
	retirement   appProducerReceipt
	fault        error
	attempt      *backgroundProbeAttempt
}

type backgroundProbeScope struct {
	app           *App
	owner         *backgroundProbeTask
	ctx           context.Context
	fence         core.OperationFence
	registration  intentRegistration
	sourceCurrent bool // captured state of the latest intent, never positive authority
	path          *networkPathSource
	control       *core.OperationSource
	epoch         int
	activeID      string
}

func (a *App) backgroundProbeOwner() *backgroundProbeTask {
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	return a.backgroundProbe
}

func (owner *backgroundProbeTask) available() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return !owner.stopping && owner.fault == nil && owner.contextState.currentLocked()
}

func (owner *backgroundProbeTask) fail() {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.fault = errBackgroundProbeUnavailable // fixed local fault, never a channel failure
}

func (a *App) startBgProbes() error {
	parent := a.captureAppTaskParent()
	if !parent.fence.Current() || a.closing.Load() {
		return errBackgroundProbeUnavailable
	}
	bgReg.mu.Lock()
	if bgReg.started || a.backgroundProbe != nil {
		owner := a.backgroundProbe
		bgReg.mu.Unlock()
		if owner != nil && owner.app == a && owner.available() {
			return nil // existing prepared owner only, not loop entry or a diagnostic result
		}
		return errBackgroundProbeUnavailable
	}
	state := newAppTaskContext()
	owner := &backgroundProbeTask{app: a, contextState: state, cancel: state.cancellation.request, stop: make(chan struct{}), done: make(chan struct{})}
	a.backgroundProbe = owner // reserve before the real constructor; never replace
	bgReg.started, bgReg.stop = true, owner.stop
	bgReg.mu.Unlock()
	if err := owner.prepareContext(parent); err != nil {
		return err
	}
	go owner.run()
	return nil
}

func (owner *backgroundProbeTask) run() {
	returned := false
	defer owner.finish(&returned)
	owner.app.bgProbeLoop(owner.stop)
	returned = true
}

func (owner *backgroundProbeTask) finish(returned *bool) {
	payload := recover()
	defer close(owner.done)
	bodyReturned := *returned && payload == nil
	sequence := new(appCleanupSequence)
	var cleanupResult error
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.retirement = appProducerReceipt{bodyReturned: bodyReturned, cleanupReturned: sequence.drained && cleanupResult == nil, cleanupResult: cleanupResult}
		owner.returned = bodyReturned && owner.retirement.cleanupKnown()
		if !owner.returned || owner.attempt != nil {
			owner.fault = errBackgroundProbeUnavailable
		}
	}()
	defer sequence.finish(&cleanupResult)
	defer appCleanupStep("retire original background context", sequence, owner.contextState.cancellation.join)
	owner.mu.Lock()
	unexpected := !bodyReturned || (!owner.stopping && owner.contextState.currentLocked() && !owner.app.closing.Load())
	if unexpected {
		owner.fault = errBackgroundProbeUnavailable
	}
	owner.mu.Unlock()
}

func (a *App) cancelBackgroundProbes() <-chan struct{} {
	owner := a.backgroundProbeOwner()
	if owner == nil {
		return nil
	}
	if owner.app != a {
		return owner.done // no cancellation against another App; result-aware join rejects it
	}
	owner.mu.Lock()
	owner.stopping = true
	owner.mu.Unlock()
	owner.stopOnce.Do(func() { close(owner.stop) }) // passive notification only; no callback holds Once's mutex
	owner.cancel()                                  // duplicates notify; the original finalizer joins actual return
	return owner.done
}

func (a *App) backgroundProbeResult(done <-chan struct{}) error {
	owner := a.backgroundProbeOwner()
	if owner == nil {
		if done == nil {
			return nil
		}
		return errBackgroundProbeUnavailable
	}
	if done != owner.done || owner.app != a {
		return errBackgroundProbeUnavailable
	}
	select {
	case <-owner.done:
	default:
		return errBackgroundProbeUnavailable
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.attempt != nil || owner.retirement.terminalResult(errBackgroundProbeUnavailable) != nil {
		return errBackgroundProbeUnavailable
	}
	if owner.retirement.preparationOnly {
		return nil // confirmed original unstarted setup cleanup, never a successful running loop
	}
	if !owner.returned || owner.fault != nil {
		return errBackgroundProbeUnavailable
	}
	return nil
}

func (a *App) joinBackgroundProbes(ctx context.Context, done <-chan struct{}) error {
	owner := a.backgroundProbeOwner()
	if owner != nil && (owner.app != a || done != owner.done) || owner == nil && done != nil {
		return errBackgroundProbeUnavailable
	}
	if err := joinAppProducerContext(ctx, done, errBackgroundProbeUnavailable); err != nil {
		return err
	}
	return a.backgroundProbeResult(done)
}

func (a *App) stopBgProbes() error {
	done := a.cancelBackgroundProbes()
	if owner := a.backgroundProbeOwner(); owner != nil && owner.app != a {
		return errBackgroundProbeUnavailable
	}
	if err := joinAppProducer(done, errBackgroundProbeUnavailable); err != nil {
		return err
	}
	return a.backgroundProbeResult(done)
}

func (a *App) captureBackgroundProbeScope() (*backgroundProbeScope, error) {
	owner := a.backgroundProbeOwner()
	if owner == nil || owner.app != a || !owner.available() || tunRequested() {
		return nil, errBackgroundProbeUnavailable
	}
	scope, err := a.captureBackgroundProbeScopeValues(owner)
	if err != nil {
		return nil, err
	}
	return scope.prepareContext()
}

func (scope *backgroundProbeScope) current() bool {
	return scope.ctx != nil && scope.fence.Current() && scope.valuesCurrent()
}

func (scope *backgroundProbeScope) valuesCurrent() bool {
	if scope.app.backgroundProbeOwner() != scope.owner || !scope.owner.available() || tunRequested() {
		return false
	}
	current := func() bool {
		scope.app.mu.Lock()
		defer scope.app.mu.Unlock()
		return scope.registration.sameLocked(scope.app) && scope.registration.source.Current() == scope.sourceCurrent &&
			scope.app.state.ActiveID == scope.activeID && !scope.app.closing.Load() &&
			!scope.app.profileTransition && !scope.app.trustBlocked && scope.app.startupErr == nil && !scope.app.installedImageChanged &&
			scope.app.restartCommit == nil && scope.app.releaseCommit == nil
	}()
	if !current {
		return false
	}
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	return scope.epoch == netEpoch.num && scope.path == netEpoch.path && scope.path.current.Load() &&
		scope.control == netEpoch.controlSource && scope.control.Current() && !netEpoch.knownDown && !netEpoch.powerBlocked.Load()
}
