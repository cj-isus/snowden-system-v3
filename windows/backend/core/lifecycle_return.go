package core

import (
	"context"
	"errors"
	"fmt"
)

// Same original goroutine: Goexit cannot set this ordinary-return witness.
func (operation *lifecycleOperation) runBody(body func() error) error {
	err := body()
	operation.bodyReturned = true
	return err
}

func abnormalLifecycleError(name string, payload any) error {
	return errors.Join(ErrEngineCleanupFailed, fmt.Errorf("%s lifecycle did not return normally (%T)", name, payload))
}

// Must be deferred directly by the original lifecycle caller so recover sees
// its panic. A missing ordinary return includes Goexit, even with nil payload.
func (manager *Manager) finalizeLifecycle(result *error, state *State, ctx *context.Context, operation *lifecycleOperation, name string, engine *Engine, proxy systemProxy, probes ...Probe) {
	payload := recover()
	returned, negative := false, false
	defer func() {
		if !returned {
			payload := recover()
			*state = StateError
			*result = errors.Join(revokeVerifiedRollback(*result), abnormalLifecycleError(name+" terminal", payload))
			if !negative {
				manager.finishAbnormalLifecycle(operation, result, engine, proxy, probes...)
			}
		}
	}()
	if !operation.bodyReturned {
		*state = StateError
		*result = errors.Join(revokeVerifiedRollback(*result), abnormalLifecycleError(name, payload))
		negative = true // the negative owner registers its own cleanup/join/finalization order
		manager.finishAbnormalLifecycle(operation, result, engine, proxy, probes...)
		returned = true
		return
	}
	terminalErr := manager.finishLifecycle(*ctx, operation, *state, engine, proxy, probes...)
	if *state != StateRunning || terminalErr != nil {
		*result = revokeVerifiedRollback(*result)
	}
	if terminalErr != nil {
		*result = errors.Join(*result, terminalErr)
	}
	returned = true
}

// Register the whole negative unwind before invoking a cleanup callback. Every
// stage runs in the original goroutine; no rescue worker or timeout receipt.
func (manager *Manager) finishAbnormalLifecycle(operation *lifecycleOperation, result *error, engine *Engine, proxy systemProxy, probes ...Probe) {
	operation.cancel()
	manager.mu.Lock()
	ours := manager.active == operation
	if ours {
		manager.state = StateStopping
	}
	manager.mu.Unlock()
	if !ours {
		*result = errors.Join(*result, ErrInvalidState)
		return // never retire a successor for an obsolete original frame
	}
	defer func() {
		if receipt := operation.cleanupResult; receipt != nil {
			*result = errors.Join(*result, receipt.result)
		}
		signal := func() chan<- struct{} {
			manager.mu.Lock()
			defer manager.mu.Unlock()
			if manager.active != operation {
				*result = errors.Join(*result, ErrInvalidState)
				return nil
			}
			manager.state = StateError
			manager.markTerminalFailureLocked(*result)
			if !operation.lifetimeReturned() || !operation.runningLifetimeReturned() {
				*result = errors.Join(*result, ErrHelperCleanup)
				return nil // original active and done remain retained
			}
			if operation.runningLifetime != nil && manager.lifetimeWatch == operation.runningLifetime {
				manager.lifetimeWatch = nil // original full task receipt, never cancellation alone
			}
			if !manager.completeOperationLocked(operation) {
				*result = errors.Join(*result, ErrHelperCleanup)
			}
			return manager.lifetimeSignal
		}()
		signalManagerFault(signal)
	}()
	defer func() {
		if operation.lifetime != nil {
			operation.lifetime.requestStop()
			<-operation.lifetime.done
		}
		if operation.runningLifetime != nil {
			<-operation.runningLifetime.done
		}
	}()
	if receipt := operation.cleanupResult; receipt == nil || !receipt.drained {
		*result = errors.Join(*result, manager.cleanup(engine, proxy, probes...))
	}
}

func (manager *Manager) stopOwnedOperation(ctx, wait context.Context, operation *lifecycleOperation, engine *Engine, proxy systemProxy) (err error) {
	state := StateError
	terminal := context.Background() // Stop's cleanup remains independent of the caller's interest budget
	defer manager.finalizeLifecycle(&err, &state, &terminal, operation, "stop", engine, proxy)
	return operation.runBody(func() error {
		cleanupErr := manager.cleanup(engine, proxy)
		cleanupErr = errors.Join(cleanupErr, manager.retireRunningLifetime(wait, operation, engine, proxy, true))
		if cleanupErr == nil {
			state = StateStopped
		}
		return errors.Join(ctx.Err(), cleanupErr)
	})
}

func signalManagerFault(signal chan<- struct{}) {
	_ = cleanupCall("lifecycle fault signal", func() error {
		select {
		case signal <- struct{}{}:
		default:
		}
		return nil
	})
}
