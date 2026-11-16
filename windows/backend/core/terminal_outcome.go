package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Caller holds Manager.mu. Only the live terminal decision may publish Running.
// Own context retirement happens afterwards, so it cannot hide a prior cancel.
func (manager *Manager) finishLocked(operation *lifecycleOperation, state State) bool {
	if operation == nil || manager.active != operation {
		return false
	}
	manager.state = state
	return manager.completeOperationLocked(operation)
}

// Caller has fixed terminal state and still owns Manager.mu/the active op.
// Close done only after every terminal source lease has actually released.
func (manager *Manager) completeOperationLocked(operation *lifecycleOperation) bool {
	if operation == nil || manager.active != operation {
		return false
	}
	if (manager.state == StateRunning || manager.state == StateStopped) && !operation.bodyReturned {
		manager.state = StateError
		return false // no positive terminal outcome without ordinary body return
	}
	if !operation.lifetimeReturned() || !operation.runningLifetimeReturned() {
		manager.state = StateError
		return false // retain the original active owner; cancel is not sampler return
	}
	var watch *lifetimeWatch
	if manager.state == StateRunning {
		var err error
		watch, err = manager.prepareLifetimeLocked()
		if err != nil {
			manager.state, manager.lifetimeErr = StateError, err
			return false
		}
	} else {
		manager.cancelLifetimeLocked()
	}
	manager.active = nil
	if watch != nil {
		go manager.watchLifetime(watch) // reserved before spawn; active already released under this lock
	}
	close(operation.done)
	return true
}

func terminalContextCause(ctx context.Context) error {
	if ctx == nil {
		return contextError(ctx)
	}
	err := errors.Join(contextError(ctx), operationLifetimeCause(ctx))
	if deadline, bounded := ctx.Deadline(); bounded && !time.Now().Before(deadline) {
		err = errors.Join(err, context.DeadlineExceeded) // also cover delayed timer delivery
	}
	return err
}

// Stop and the Running decision serialize on the same Manager mutex. The
// original active owner remains registered until failed completion cleanup
// returns; a waiting Stop cannot adopt/restart around uncertain resources.
func (manager *Manager) finishLifecycle(ctx context.Context, operation *lifecycleOperation, state State, engine *Engine, proxy systemProxy, probes ...Probe) error {
	locked := false
	lockManager := func() { manager.mu.Lock(); locked = true }
	unlockManager := func() {
		if locked {
			locked = false
			manager.mu.Unlock()
		}
	}
	defer func() {
		if locked {
			manager.state = StateError // never expose a provisional Running on abnormal unlock
			unlockManager()
		}
	}()
	lifetimeErr, lifetimeCleanup := manager.retireOperationLifetime(ctx, operation, engine, proxy, probes...)
	var trustErr error
	var sourceErr error
	var proxyErr error
	var currentProbe Probe
	if state == StateRunning && !lifetimeCleanup {
		manager.mu.RLock()
		ours := manager.active == operation
		currentProbe = manager.probe
		manager.mu.RUnlock()
		if !ours {
			return ErrInvalidState
		}
		_, trustErr = lifetimeRemaining(currentProbe) // callback outside Manager locks, including restored-old constraints
		sourceErr = validateLifecycleSource(ctx, currentProbe)
		if trustErr == nil && sourceErr == nil && terminalContextCause(ctx) == nil {
			proxyErr = manager.validateAppliedProxy(ctx) // original native handle, never under terminal value locks
		}
	}
	lockManager()
	if operation == nil || manager.active != operation {
		unlockManager()
		return ErrInvalidState
	}
	cause := errors.Join(terminalContextCause(ctx), lifetimeErr, trustErr, sourceErr, proxyErr)
	if operation.stopRequested {
		cause = errors.Join(cause, context.Canceled)
	}
	if state == StateRunning && cause == nil {
		if engine == nil || manager.engine != engine {
			cause = ErrEngineWorkerUnavailable
		} else {
			var releaseSource func()
			releaseSource, cause = guardLifecycleCompletion(ctx, currentProbe, manager)
			if cause == nil {
				previous := manager.state
				manager.state = StateRunning // terminal decision while source lease is held
				cause = releaseLifecycleCompletion(releaseSource)
				if cause == nil {
					if manager.completeOperationLocked(operation) {
						unlockManager()
						operation.cancel()
						return nil
					}
					cause = ErrHelperCleanup // original sampler receipt is mandatory
				}
				manager.state = previous // active owner/done retained for actual failure cleanup
			}
		}
	}
	if state == StateRunning && cause != nil {
		manager.markTerminalFailureLocked(cause)
		unlockManager()
		var cleanupErr error
		if !lifetimeCleanup {
			cleanupErr = manager.cleanup(engine, proxy, probes...)
		}
		lockManager()
		if manager.active == operation && manager.lastReport != nil {
			manager.lastReport = manager.lastReport.Clone()
			manager.lastReport.CleanupUnconfirmed = manager.lastReport.CleanupUnconfirmed || errors.Is(cleanupErr, ErrGUIAncestorUnavailable) || errors.Is(cleanupErr, ErrEngineCleanupFailed) || errors.Is(cleanupErr, ErrHelperCleanup) || errors.Is(cleanupErr, ErrHelperBusy)
		}
		unlockManager()
		manager.finish(operation, StateError)
		return errors.Join(cause, cleanupErr)
	}
	finished := manager.finishLocked(operation, state)
	unlockManager()
	operation.cancel() // terminal state/ownership already fixed; not a caller cancellation
	if !finished {
		return errors.Join(cause, ErrHelperCleanup)
	}
	return cause
}

func (manager *Manager) markTerminalFailureLocked(cause error) {
	if manager.lastReport == nil {
		return
	}
	report := manager.lastReport.Clone()
	report.AllPassed, report.Stale = false, true
	report.CleanupUnconfirmed = report.CleanupUnconfirmed || errors.Is(cause, ErrGUIAncestorUnavailable) || errors.Is(cause, ErrEngineCleanupFailed) || errors.Is(cause, ErrHelperCleanup) || errors.Is(cause, ErrHelperBusy)
	var trust *TrustFaultError
	if errors.As(cause, &trust) {
		report.TrustUnavailable, report.TrustReason = true, "trust_unavailable"
	}
	report.Results = append(report.Results, ProbeResult{Name: "завершение операции", Status: "unavailable", Detail: "Контекст, владелец операции, системный прокси или срок доверия изменились до подтверждения состояния; результат недействителен."})
	manager.lastReport = report
}

// A verified rollback is an outcome, not a permanent property of an error.
// Remove only its positive marker/report after terminal invalidation. Preserve
// independent joined custody/trust failures so callers cannot retry over them.
func revokeVerifiedRollback(err error) error {
	if err == nil {
		return nil
	}
	var rollback *VerifiedRollbackError
	if !errors.As(err, &rollback) && !errors.Is(err, ErrSwitchRolledBack) {
		return err
	}
	if rollback, direct := err.(*VerifiedRollbackError); direct {
		if rollback == nil {
			return nil
		}
		return revokeVerifiedRollback(rollback.Cause)
	}
	if err == ErrSwitchRolledBack {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		out := make([]error, 0, len(children))
		for _, child := range children {
			out = append(out, revokeVerifiedRollback(child))
		}
		return errors.Join(out...)
	}
	if child := errors.Unwrap(err); child != nil {
		filtered := revokeVerifiedRollback(child)
		if filtered == nil {
			return nil
		}
		return fmt.Errorf("lifecycle outcome invalidated: %w", filtered)
	}
	return errors.New("rollback outcome invalidated")
}

// InvalidateLifecycleOutcome removes a stale positive rollback receipt while
// preserving independent failure causes across an App cancellation/supersession.
// This helper only restricts an outcome; it cannot create forwarding authority.
func InvalidateLifecycleOutcome(err, cause error) error {
	return errors.Join(revokeVerifiedRollback(err), cause)
}
