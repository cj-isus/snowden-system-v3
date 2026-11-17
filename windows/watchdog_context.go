package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

// The original creator alone prepares this already-reserved task. No task is
// scheduled before success. Failed construction retains the slot and records
// its actual failed frame; it never reenters creator-held operationMu cleanup.
func (owner *appWatchdogTask) prepareContext() (result error) {
	transferred := false
	defer func() {
		if transferred {
			return
		}
		owner.app.watchdogBlocked.Store(true)
		defer close(owner.done) // creator's original failed frame, never a running-loop receipt
		sequence := new(appCleanupSequence)
		defer func() {
			owner.mu.Lock()
			defer owner.mu.Unlock()
			owner.fault = errors.Join(owner.fault, sequence.result)
			result = errors.Join(errWatchdogUnavailable, result, sequence.result)
		}()
		owner.mu.Lock()
		owner.running = false
		owner.fault = errors.Join(errWatchdogUnavailable, result)
		owner.mu.Unlock()
		appCleanupStep("retire failed watchdog preparation", sequence, owner.cancellation.join)
	}()
	if err := owner.constructContext(); err != nil {
		return err
	}
	transferred = true
	return nil
}

// Actual original parent methods run outside runtime/App/task/cancel locks.
// Publish the real partial cancel before any later getter/fence preparation.
func (owner *appWatchdogTask) constructContext() (result error) {
	cancellation := owner.cancellation
	returned := false
	defer func() {
		if !returned {
			_ = recover() // no context payload escapes; Goexit still unwinds original creator
			result = errWatchdogUnavailable
		}
		cancellation.prepareReturned.Store(returned)
		close(cancellation.prepared)
	}()
	if !owner.parentFence.Current() || !cancellation.operation.Current() || owner.app.closing.Load() || owner.app.watchdogBlocked.Load() {
		returned = true
		return errWatchdogUnavailable // no child constructor was entered
	}
	cancellation.attempted.Store(true)
	child, cancel := context.WithCancel(owner.parent)
	if err := cancellation.storeCancel(cancel); err != nil {
		return errWatchdogUnavailable
	}
	if !cancellation.operation.Current() || !owner.parentFence.Current() {
		returned = true
		return errWatchdogUnavailable
	}
	fence, err := core.CaptureOperationFence(child)
	if err != nil {
		returned = true
		return errWatchdogUnavailable
	}
	a := owner.app
	current := func() bool {
		runtime_.mu.Lock()
		defer runtime_.mu.Unlock()
		a.mu.Lock()
		defer a.mu.Unlock()
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if runtime_.watchdog != owner || !owner.parentFence.Current() || !a.appLifetimeCurrentLocked() || a.closing.Load() || a.quitRequested ||
			a.installedImageChanged || a.startupErr != nil || a.watchdogBlocked.Load() || owner.stopping || !cancellation.operation.Current() || !fence.Current() {
			return false
		}
		owner.ctx, owner.ctxFence, owner.contextReady = child, fence, true
		return true
	}()
	returned = true
	if !current {
		return errWatchdogUnavailable
	}
	return nil
}
