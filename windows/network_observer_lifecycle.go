package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

var errNetworkPollerUnavailable = errors.New("наблюдатель сети недоступен; исходный цикл опроса должен завершиться до замены")

// One original polling task. Context cancellation is a request; only done
// confirms that the original collector/event-delivery stack has returned.
type networkPollerOwner struct {
	app                 *App
	contextState        appTaskContext
	ctx                 context.Context
	cancel              context.CancelFunc
	started             chan struct{}
	done                chan struct{}
	mu                  sync.Mutex
	running             bool
	observed            bool // at least one actual delivered event from this original poller, not task entry
	stopping            bool
	fault               error
	retirement          networkProducerReceipt // owner.mu; separate from functional readiness/fault
	power               powerSubscription
	powerWake           chan struct{}
	powerRetirement     bool   // known continuity loss, distinct from producer/cleanup failure
	powerRecoveryTried  bool   // observer-only restart budget on this original producer
	powerRecoveryIntent uint64 // a new explicit App intent may reconcile it again
}

func newNetworkPollerOwner(app *App) *networkPollerOwner {
	state := newAppTaskContext()
	return &networkPollerOwner{app: app, contextState: state, cancel: state.cancellation.request, started: make(chan struct{}), done: make(chan struct{}), powerWake: make(chan struct{}, 1)}
}

// Called before the DNS lease; neither guard covers I/O or callbacks.
func (owner *networkPollerOwner) guardRunning(app *App) (func(), bool) {
	if owner == nil {
		return nil, false
	}
	owner.mu.Lock()
	if owner.app != app || !owner.running || owner.fault != nil || !owner.contextState.currentLocked() {
		owner.mu.Unlock()
		return nil, false
	}
	return owner.mu.Unlock, true
}

func (owner *networkPollerOwner) stop() {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	owner.running, owner.stopping, owner.observed = false, true, false
	owner.mu.Unlock()
	owner.cancel()
}

func (owner *networkPollerOwner) run(watcher *netWatcher) {
	defer close(owner.done)
	completed := false
	defer owner.finishPoller(watcher, &completed)
	power, err := startNativePowerNotifications(owner)
	owner.mu.Lock()
	if owner.power == nil {
		owner.power = power // never overwrite an original pre-published partial registration
	}
	if err != nil {
		owner.fault = errors.Join(owner.fault, errNetworkPollerUnavailable, err)
		netEpoch.revokePowerContinuityValue()
	}
	owner.mu.Unlock()
	if err != nil || !netEpoch.enterPowerObservedPoller(owner) {
		completed = true
		return
	}
	close(owner.started) // task entry only, not a successful sample or OS proof
	watcher.runWithPowerOwner(owner.ctx, owner)
	completed = true
}

// Value-only revocation for the exact original producer/task pair. Closing a
// passive source invokes no user callback and does not assert task retirement.
func (w *netWatcher) revokeServiceDNSControl(owner *networkPollerOwner) {
	w.mu.Lock()
	dns := w.dns
	current := w.poller == owner && dns != nil && dns.poller == owner
	w.mu.Unlock()
	if current {
		dns.mu.Lock() // same short lease as positive observer publication
		dns.source.Revoke()
		dns.mu.Unlock()
	}
}

// The original poller fails only its linked current DNS task. No network
// epoch/up/down/health is invented; existing DNS finalization owns failCore.
func (w *netWatcher) failNetworkPoller(owner *networkPollerOwner) (err error) {
	completed := false
	var cancelDNS context.CancelFunc
	defer func() {
		if !completed {
			_ = recover()
			err = errors.Join(errNetworkPollerUnavailable, core.ErrEngineCleanupFailed)
		}
		if cancelDNS != nil {
			cancelErr := func() (failure error) {
				canceled := false
				defer func() {
					if !canceled {
						_ = recover()
						failure = core.ErrEngineCleanupFailed
					}
				}()
				cancelDNS()
				canceled = true
				return nil
			}()
			err = errors.Join(err, cancelErr)
		}
	}()
	w.mu.Lock()
	dns := w.dns
	current := w.poller == owner && dns != nil && dns.poller == owner
	w.mu.Unlock()
	if current {
		cancelDNS = dns.cancel // dependency retirement also runs if the App fence panics
		owner.mu.Lock()
		cause := owner.fault
		power := owner.powerRetirement
		owner.mu.Unlock()
		if cause == nil {
			cause = errNetworkPollerUnavailable
		}
		dns.mu.Lock()
		dns.running = false
		dns.source.Revoke()
		dns.pending = nil
		dns.powerRetirement = power
		dns.fault = errors.Join(dns.fault, cause)
		dns.mu.Unlock()
		if cancel := owner.app.invalidateFailedServiceDNS(dns); cancel != nil {
			cancel()
		}
	}
	completed = true
	return nil
}

// Reused by both owned tasks. The guard is a private value-only lease; it
// is released before returning or waiting, never around task/engine cleanup.
func awaitNetworkTaskEntry(ctx context.Context, started, done, stopped <-chan struct{}, unavailable error, guard func() (func(), bool)) error {
	if ctx == nil {
		return unavailable
	}
	wait, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	select {
	case <-started:
		release, ok := guard()
		if !ok {
			return unavailable
		}
		release()
		if wait.Err() != nil {
			return errors.Join(unavailable, wait.Err())
		}
		return nil
	case <-done:
		return unavailable
	case <-stopped:
		return unavailable
	case <-wait.Done():
		return errors.Join(unavailable, wait.Err())
	}
}

func (owner *networkPollerOwner) awaitStarted(ctx context.Context) error {
	return awaitNetworkTaskEntry(ctx, owner.started, owner.done, owner.contextState.cancellation.operation.Done(), errNetworkPollerUnavailable,
		func() (func(), bool) { return owner.guardRunning(owner.app) })
}

func joinTerminalNetworkTask(ctx context.Context, terminal bool, done <-chan struct{}, unavailable error) error {
	if !terminal {
		return nil
	}
	select {
	case <-done:
		return nil // task completion only, not engine/resource retirement
	case <-ctx.Done():
		return errors.Join(unavailable, ctx.Err())
	}
}
