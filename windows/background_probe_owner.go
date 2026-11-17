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
	app      *App
	ctx      context.Context
	cancel   context.CancelFunc
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	stopping bool
	returned bool // original loop and cancel ordinary return; read after done
	fault    error
	attempt  *backgroundProbeAttempt
}

type backgroundProbeScope struct {
	app           *App
	owner         *backgroundProbeTask
	ctx           context.Context
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
	return !owner.stopping && owner.fault == nil && owner.ctx.Err() == nil
}

func (owner *backgroundProbeTask) fail() {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.fault = errBackgroundProbeUnavailable // fixed local fault, never a channel failure
}

func (a *App) startBgProbes() {
	ctx := func() context.Context {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.appLifetimeCurrentLocked() || a.closing.Load() || a.quitRequested || a.startupErr != nil {
			return nil
		}
		return a.ctx
	}()
	if ctx == nil {
		return
	}
	lifetime, cancel := context.WithCancel(ctx)
	handedOff := false
	defer func() {
		if !handedOff {
			cancel()
		}
	}()
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	if bgReg.started || a.backgroundProbe != nil || a.closing.Load() || lifetime.Err() != nil {
		return
	}
	owner := &backgroundProbeTask{app: a, ctx: lifetime, cancel: cancel, stop: make(chan struct{}), done: make(chan struct{})}
	a.backgroundProbe = owner // before scheduling; no failed/stopped replacement
	bgReg.started, bgReg.stop = true, owner.stop
	handedOff = true
	go owner.run()
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
	cancelReturned := false
	defer func() {
		cancelPayload := recover()
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.returned = *returned && payload == nil && cancelReturned && cancelPayload == nil
		if !owner.returned || owner.attempt != nil {
			owner.fault = errBackgroundProbeUnavailable
		}
	}()
	owner.mu.Lock()
	unexpected := !*returned || payload != nil || (!owner.stopping && owner.ctx.Err() == nil && !owner.app.closing.Load())
	if unexpected {
		owner.fault = errBackgroundProbeUnavailable
	}
	owner.mu.Unlock()
	owner.cancel()
	cancelReturned = true
}

func (a *App) cancelBackgroundProbes() <-chan struct{} {
	owner := a.backgroundProbeOwner()
	if owner == nil {
		return nil
	}
	if owner.app != a {
		return owner.done // no cancellation against another App; result-aware join rejects it
	}
	owner.stopOnce.Do(func() {
		owner.mu.Lock()
		owner.stopping = true
		owner.mu.Unlock()
		defer close(owner.stop) // an abnormal cancel does not skip the original negative stop signal
		owner.cancel()
	})
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
	if !owner.returned || owner.fault != nil || owner.attempt != nil {
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
	a.mu.Lock()
	defer a.mu.Unlock()
	scope := &backgroundProbeScope{app: a, owner: owner, registration: a.intentRegistrationLocked(), activeID: a.state.ActiveID}
	scope.sourceCurrent = scope.registration.source.Current()
	if a.closing.Load() || a.profileTransition || a.trustBlocked || a.startupErr != nil || a.installedImageChanged ||
		a.restartCommit != nil || a.releaseCommit != nil || !scope.registration.sameLocked(a) {
		return nil, errBackgroundProbeUnavailable
	}
	scope.ctx = netEpoch.scopeContext(owner.ctx)
	scope.path, _ = scope.ctx.Value(networkPathContextKey{}).(*networkPathSource)
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	scope.epoch = netEpoch.num
	scope.control = netEpoch.controlSource
	if scope.epoch <= 0 || scope.path == nil || scope.path != netEpoch.path || !scope.path.current.Load() ||
		!scope.control.Current() || netEpoch.knownDown || netEpoch.powerBlocked.Load() {
		return nil, errBackgroundProbeUnavailable
	}
	return scope, nil
}

func (scope *backgroundProbeScope) current() bool {
	if scope.ctx.Err() != nil || scope.app.backgroundProbeOwner() != scope.owner || !scope.owner.available() || tunRequested() {
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
