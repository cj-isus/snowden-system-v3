package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var errWatchdogUnavailable = errors.New("контроль туннеля недоступен; исходная задача watchdog не подтверждена")

// One original task per process runtime, retained after cancellation/failure.
// started is entry only; done includes the original loop and fault finalizers.
type appWatchdogTask struct {
	app      *App
	ctx      context.Context
	cancel   context.CancelFunc
	started  chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	running  bool
	stopping bool
	fault    error
	cleanup  *appCleanupSequence
}

func (a *App) ensureWatchdog() error {
	runtime_.mu.Lock()
	a.mu.Lock()
	ctx := a.ctx
	allowed := ctx != nil && ctx.Err() == nil && !a.closing.Load() && !a.quitRequested &&
		!a.installedImageChanged && a.startupErr == nil && !a.watchdogBlocked.Load()
	owner := runtime_.watchdog
	created := false
	if allowed && owner == nil {
		lifetime, cancel := context.WithCancel(ctx)
		owner = &appWatchdogTask{app: a, ctx: lifetime, cancel: cancel,
			started: make(chan struct{}), done: make(chan struct{})}
		runtime_.watchdog, created = owner, true // reserve before scheduling, never replace
		runtime_.watchOn = true                  // legacy reservation data only; consumers use original task state
	}
	a.mu.Unlock()
	runtime_.mu.Unlock()
	if !allowed || owner == nil || owner.app != a {
		return errWatchdogUnavailable
	}
	if created {
		go owner.run()
	}
	return awaitNetworkTaskEntry(ctx, owner.started, owner.done, owner.ctx.Done(), errWatchdogUnavailable,
		func() (func(), bool) { return owner.guardRunning(a) })
}

// Value-only lease: runtime -> original task, no App/native/cleanup callbacks.
func (owner *appWatchdogTask) guardRunning(a *App) (func(), bool) {
	state := runtime_
	state.mu.Lock()
	if state.watchdog != owner || owner.app != a {
		state.mu.Unlock()
		return nil, false
	}
	owner.mu.Lock()
	if !owner.running || owner.stopping || owner.fault != nil || owner.ctx.Err() != nil || a.closing.Load() || a.watchdogBlocked.Load() {
		owner.mu.Unlock()
		state.mu.Unlock()
		return nil, false
	}
	return func() { owner.mu.Unlock(); state.mu.Unlock() }, true
}

func (owner *appWatchdogTask) run() {
	returned := false
	defer owner.finish(&returned)
	owner.loopBody()
	returned = true
}

func (owner *appWatchdogTask) loopBody() {
	owner.mu.Lock()
	if owner.stopping || owner.ctx.Err() != nil || owner.app.closing.Load() {
		owner.mu.Unlock()
		return
	}
	owner.mu.Unlock()
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()
	owner.mu.Lock()
	if owner.stopping || owner.ctx.Err() != nil || owner.app.closing.Load() {
		owner.mu.Unlock()
		return
	}
	owner.running = true
	owner.mu.Unlock()
	close(owner.started)
	owner.app.watchCoreTicks(owner.ctx, ticker.C)
}

func (owner *appWatchdogTask) finish(returned *bool) {
	payload := recover()
	defer close(owner.done)
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if owner.cleanup != nil {
			owner.fault = owner.cleanup.result // this original sequence already includes the initial fault
		}
	}()
	owner.mu.Lock()
	unexpected := !*returned || (!owner.stopping && owner.ctx.Err() == nil && !owner.app.closing.Load())
	owner.running = false
	if unexpected {
		owner.fault = errors.Join(errWatchdogUnavailable, fmt.Errorf("watchdog task interrupted (%T)", payload))
	}
	cause := owner.fault
	owner.mu.Unlock()
	owner.app.watchdogBlocked.Store(true) // negative fence before App/lifecycle mutex waits
	owner.cancel()
	if cause != nil {
		owner.failOriginalScope(cause) // same original goroutine; never joins itself
	}
}

func (owner *appWatchdogTask) stop() {
	owner.app.watchdogBlocked.Store(true)
	owner.mu.Lock()
	owner.running, owner.stopping = false, true
	owner.mu.Unlock()
	owner.cancel()
}

func (a *App) cancelWatchdog() (<-chan struct{}, error) {
	runtime_.mu.Lock()
	owner := runtime_.watchdog
	runtime_.mu.Unlock()
	if owner == nil {
		return nil, nil
	}
	if owner.app != a {
		return nil, errWatchdogUnavailable
	}
	owner.stop()
	return owner.done, nil
}

func (a *App) joinWatchdog(ctx context.Context, done <-chan struct{}) error {
	runtime_.mu.Lock()
	owner := runtime_.watchdog
	runtime_.mu.Unlock()
	if owner == nil && done == nil {
		return nil
	}
	if owner == nil || owner.app != a || done != owner.done {
		return errWatchdogUnavailable
	}
	if err := joinAppProducerContext(ctx, done, errWatchdogUnavailable); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.fault // actual done, not cancel/deadline or an inferred healthy task
}

func (a *App) watchdogStatus() (bool, error) {
	runtime_.mu.Lock()
	owner := runtime_.watchdog
	runtime_.mu.Unlock()
	if owner == nil {
		return false, nil
	}
	if release, ok := owner.guardRunning(a); ok {
		release()
		return true, nil
	}
	return false, errWatchdogUnavailable
}
