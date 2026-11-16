package core

import (
	"context"
	"errors"
	"time"
)

// The Manager retains this original task through sampling, expiry cleanup and
// its nonblocking notification. Only done permits replacement/refund. cause is
// written by this task alone and read by other callers only after done.
type lifetimeWatch struct {
	ctx               context.Context
	cancel            context.CancelFunc
	done              chan struct{}
	fault             chan struct{} // early negative finding only; never whole observer/resource return
	earlyCause        error         // parent writes once before fault close; readers wait for fault
	faultPublished    bool          // original parent goroutine only
	generation        uint64
	probe             Probe
	manager           *Manager
	engine            *Engine
	proxy             *ProxyManager // only the actual native owner; no observer alias or public bool adapter
	proxyObserver     *runningProxyWatch
	trustCheckPending bool
	signal            chan<- struct{}
	cause             error
}

// Manager.mu held. Cancellation never joins or clears the original owner.
func (manager *Manager) cancelLifetimeLocked() *lifetimeWatch {
	watch := manager.lifetimeWatch
	if watch != nil {
		watch.cancel()
	}
	return watch
}

// Reserve only. The terminal caller launches the task AFTER clearing active,
// under the same Manager lock, so an immediate sampled failure is not lost to
// the just-completed operation. No old task is replaced before actual return.
func (manager *Manager) prepareLifetimeLocked() (*lifetimeWatch, error) {
	if previous := manager.lifetimeWatch; previous != nil {
		select {
		case <-previous.done:
			if previous.cause != nil {
				return nil, previous.cause
			}
			manager.lifetimeWatch = nil
		default:
			return nil, ErrHelperCleanup
		}
	}
	_, trust := manager.probe.(LifetimeProbe)
	native, nativeProxy := manager.proxy.(*ProxyManager)
	if !nativeProxy || !manager.proxyActive {
		native = nil
	}
	if !trust && native == nil {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	watch := &lifetimeWatch{ctx: ctx, cancel: cancel, done: make(chan struct{}), fault: make(chan struct{}),
		generation: manager.generation, probe: manager.probe, signal: manager.lifetimeSignal,
		manager: manager, engine: manager.engine, proxy: native}
	if native != nil {
		watch.proxyObserver = &runningProxyWatch{watch: watch, done: make(chan struct{})} // reserved before either goroutine
	}
	manager.lifetimeWatch = watch // reserve before go; this original Manager owns the slot
	return watch, nil
}

func (manager *Manager) watchLifetime(watch *lifetimeWatch) {
	defer close(watch.done)
	defer watch.joinProxyObserver() // cleanup/expiry handling below runs before waiting on native I/O
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			if watch.trustCheckPending {
				watch.cause = errors.Join(watch.cause, &TrustFaultError{Reason: "trust_unavailable"})
			} else {
				watch.cause = errors.Join(watch.cause, ErrHelperCleanup) // local sampler/cleanup failure is not invented trust expiry
			}
			watch.publishFault(watch.cause)
			manager.retireExpiredLifetime(watch, watch.cause)
		}
	}()
	if watch.proxyObserver != nil {
		watch.proxyObserver.started = true
		go watch.proxyObserver.run() // one retained child, no replacement native calls
	}
	watch.cause = watch.sample()
	if watch.cause != nil {
		watch.publishFault(watch.cause) // an already active Reload/Stop owns cleanup, not this sampler
		manager.retireExpiredLifetime(watch, watch.cause)
	}
	returned = true
}

func (watch *lifetimeWatch) sample() error {
	for watch.ctx.Err() == nil {
		_, watch.trustCheckPending = watch.probe.(LifetimeProbe)
		remaining, err := lifetimeRemaining(watch.probe)
		watch.trustCheckPending = false
		if err != nil {
			return err // a late sampled fault remains available to the joining original operation
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		timer := time.NewTimer(remaining)
		select {
		case <-watch.ctx.Done():
			timer.Stop()
			return nil
		case <-watch.proxyObservationDone():
			timer.Stop()
			return watch.proxyObservationCause()
		case <-timer.C:
		}
	}
	return nil
}

func (operation *lifecycleOperation) runningLifetimeReturned() bool {
	if operation == nil {
		return false
	}
	if operation.runningLifetime == nil {
		return true
	}
	select {
	case <-operation.runningLifetime.done:
		return true
	default:
		return false
	}
}

// Join outside Manager locks before a new operation samples/mutates anything.
// Stop has already attempted its own engine/protection cleanup; it joins without
// treating a disposed session's trust fault as failure of a completed Disconnect.
func (manager *Manager) retireRunningLifetime(ctx context.Context, operation *lifecycleOperation, engine *Engine, proxy systemProxy, stopped bool) error {
	watch := operation.runningLifetime
	if watch == nil {
		return nil
	}
	var cause, cleanupErr error
	cleaned := stopped
	cleanup := func() error {
		manager.mu.Lock()
		ours := manager.active == operation
		if ours {
			manager.state = StateStopping
		}
		manager.mu.Unlock()
		if !ours {
			return ErrInvalidState
		}
		return manager.cleanup(engine, proxy)
	}
	if !stopped {
		cause = terminalContextCause(ctx)
		if cause == nil {
			select {
			case <-watch.done:
			case <-watch.fault:
				cause = watch.earlyCause // actual immutable finding; full child/native return is still pending
			case <-ctx.Done():
				cause = terminalContextCause(ctx)
			}
		}
		if cause != nil {
			cleanupErr, cleaned = cleanup(), true
		}
	}
	<-watch.done // a canceled wait is not original sampler/cleanup return
	if !stopped {
		cause = errors.Join(cause, watch.cause)
		if cause != nil && !cleaned {
			cleanupErr = cleanup()
		}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.active != operation || manager.lifetimeWatch != watch {
		return errors.Join(cause, cleanupErr, ErrHelperCleanup)
	}
	manager.lifetimeWatch = nil // original done received; successor may now reserve a new watch
	if !stopped && watch.cause != nil {
		manager.lifetimeErr = errors.Join(watch.cause, cleanupErr)
	}
	return errors.Join(cause, cleanupErr)
}

// Expiry owns cleanup but never joins itself. Its lifecycle operation may
// complete after cleanup; the separate original watch slot remains retained
// through this function and the final nonblocking notification until done.
func (manager *Manager) retireExpiredLifetime(watch *lifetimeWatch, cause error) {
	manager.mu.Lock()
	if manager.lifetimeWatch != watch || manager.generation != watch.generation || manager.active != nil || manager.state != StateRunning {
		manager.mu.Unlock()
		return
	}
	manager.cancelLifetimeLocked()
	operation := &lifecycleOperation{cancel: func() {}, done: make(chan struct{})}
	manager.active = operation
	manager.generation++
	manager.state, manager.lifetimeErr = StateStopping, cause
	engine, proxy, probe := manager.engine, manager.proxy, manager.probe
	manager.mu.Unlock()
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			cause = errors.Join(cause, ErrEngineCleanupFailed)
		}
		if receipt := operation.cleanupResult; receipt != nil {
			cause = errors.Join(cause, receipt.result)
		}
		watch.cause = cause
		manager.mu.Lock()
		if manager.active == operation {
			manager.lifetimeErr = cause
			manager.markTerminalFailureLocked(cause)
		}
		manager.mu.Unlock()
		manager.finish(operation, StateError)
		signalManagerFault(watch.signal)
	}()
	cause = errors.Join(cause, manager.cleanup(engine, proxy, probe))
	returned = true
}
