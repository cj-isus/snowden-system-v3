package core

import (
	"errors"
	"time"
)

// Exactly one sequential native observer under the original running owner. Its
// goroutine return is distinct from native-key/resource retirement. Parent trust
// checks never wait on this observer until after requesting lifecycle cleanup.
type runningProxyWatch struct {
	watch   *lifetimeWatch
	done    chan struct{}
	started bool  // parent-owned reservation/launch witness, never a native completion receipt
	cause   error // original child writes; parent reads only after done
}

func (observer *runningProxyWatch) run() {
	defer close(observer.done)
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			observer.cause = errors.Join(observer.cause, ErrProxyObservationUnavailable, ErrProxyStateUncertain)
		}
	}()
	for observer.watch.ctx.Err() == nil {
		if observer.cause = observer.watch.sampleAppliedProxy(); observer.cause != nil {
			returned = true
			return
		}
		timer := time.NewTimer(time.Second) // schedule target only; native calls have no claimed hard deadline
		select {
		case <-observer.watch.ctx.Done():
			timer.Stop()
			returned = true
			return
		case <-timer.C:
		}
	}
	returned = true
}

func (watch *lifetimeWatch) proxyObservationDone() <-chan struct{} {
	if watch.proxyObserver == nil {
		return nil
	}
	return watch.proxyObserver.done
}

// Called only after the actual original child's done was received.
func (watch *lifetimeWatch) proxyObservationCause() error {
	cause := watch.proxyObserver.cause
	if cause == nil && watch.ctx.Err() == nil {
		return errors.Join(ErrProxyObservationUnavailable, ErrProxyStateUncertain)
	}
	return cause
}

// Original parent only. Active lifecycle operations may start their existing
// negative cleanup on this finding without waiting for an unreturned native
// child. This never closes done, frees a slot or grants positive admission.
func (watch *lifetimeWatch) publishFault(cause error) {
	if cause == nil || watch.faultPublished || watch.fault == nil {
		return
	}
	watch.earlyCause, watch.faultPublished = cause, true
	close(watch.fault)
}

func (watch *lifetimeWatch) joinProxyObserver() {
	watch.cancel()
	observer := watch.proxyObserver
	if observer == nil || !observer.started {
		return
	}
	defer watch.finishProxyObserver(observer) // even panic/Goexit must wait for the original native child
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			if watch.trustCheckPending {
				watch.cause = errors.Join(watch.cause, &TrustFaultError{Reason: "trust_unavailable"})
			} else {
				watch.cause = errors.Join(watch.cause, ErrHelperCleanup)
			}
			watch.publishFault(watch.cause)
			watch.manager.retireExpiredLifetime(watch, watch.cause)
		}
	}()
	for watch.cause == nil {
		select {
		case <-observer.done:
			returned = true
			return
		default:
		}
		// Cancellation ends positive interest. The old engine may still be
		// running while Reload joins a blocked native child, so the original
		// parent continues immutable deny-only trust checks until retirement.
		_, watch.trustCheckPending = watch.probe.(LifetimeProbe)
		remaining, err := lifetimeRemaining(watch.probe)
		watch.trustCheckPending = false
		if err != nil {
			watch.cause = err
			watch.publishFault(err)
			watch.manager.retireExpiredLifetime(watch, err) // active successor consumes fault instead
			break
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		timer := time.NewTimer(remaining)
		select {
		case <-observer.done:
			timer.Stop()
			returned = true
			return
		case <-timer.C:
		}
	}
	returned = true
}

func (watch *lifetimeWatch) finishProxyObserver(observer *runningProxyWatch) {
	<-observer.done // cancellation/timeout cannot stand in for the original native callback's return
	if observer.cause == nil {
		return
	}
	watch.cause = errors.Join(watch.cause, observer.cause)
	watch.publishFault(watch.cause) // retain a late negative child result for an already waiting lifecycle
	manager := watch.manager
	if manager == nil {
		return
	}
	manager.mu.Lock()
	publish := manager.lifetimeWatch == watch && manager.state == StateError && manager.active == nil
	if publish {
		manager.lifetimeErr = errors.Join(manager.lifetimeErr, observer.cause)
		manager.markTerminalFailureLocked(observer.cause)
	}
	manager.mu.Unlock()
	if publish {
		signalManagerFault(watch.signal)
	}
}

// Manager read lock held. This binds the negative observation to the actual
// original running session; it never permits settings writes or forwarding.
func (watch *lifetimeWatch) proxyOwnerCurrentLocked() bool {
	manager := watch.manager
	if manager == nil {
		return false
	}
	native, ok := manager.proxy.(*ProxyManager)
	return ok && native == watch.proxy && manager.lifetimeWatch == watch && manager.generation == watch.generation &&
		manager.active == nil && manager.state == StateRunning && manager.engine == watch.engine && manager.proxyActive
}

// Only the one registered proxy child calls this method. There is no overlapping
// native read, replacement observer, guessed port or scheduler query. The parent
// remains free to expire trust and request cleanup while this native call waits.
func (watch *lifetimeWatch) sampleAppliedProxy() error {
	if watch.proxy == nil || watch.ctx.Err() != nil {
		return nil
	}
	manager := watch.manager
	if manager == nil {
		return errors.Join(ErrProxyObservationUnavailable, ErrProxyStateUncertain)
	}
	manager.mu.RLock()
	current, conflict := watch.proxyOwnerCurrentLocked(), manager.proxyConflict
	manager.mu.RUnlock()
	if conflict {
		return errors.Join(ErrProxyObservationUnavailable, ErrProxyOwnershipConflict)
	}
	if !current {
		if watch.ctx.Err() != nil {
			return nil // a normal canceled predecessor cannot sample a successor
		}
		return errors.Join(ErrProxyObservationUnavailable, ErrProxyStateUncertain)
	}
	if watch.ctx.Err() != nil {
		return nil
	}
	err := cleanupCall("running system proxy observation", watch.proxy.ValidateAppliedProxy) // no Manager mutex over native work
	manager.mu.Lock()
	native, sameNative := manager.proxy.(*ProxyManager)
	if sameNative && native == watch.proxy && errors.Is(err, ErrProxyOwnershipConflict) {
		manager.proxyConflict = true // negative finding survives cancellation; never grants overwrite authority
	}
	current = watch.proxyOwnerCurrentLocked()
	manager.mu.Unlock()
	if err != nil {
		return errors.Join(ErrProxyObservationUnavailable, err) // keep an original unknown call even after cancel
	}
	if !current && watch.ctx.Err() == nil {
		return errors.Join(ErrProxyObservationUnavailable, ErrProxyStateUncertain)
	}
	return nil // exact tuple observation only; no network/protection/OS-CAS claim
}
