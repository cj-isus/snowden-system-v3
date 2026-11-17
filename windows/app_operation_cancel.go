package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/core"
)

var errAppOperationBusy = errors.New("Операция приложения ещё выполняется; дождитесь её завершения.")

// One App-owned context's original construction/cancellation. Its outer owner
// protects the slot; callbacks and joining run outside value locks. Do not copy.
type appOperationCancellation struct {
	operation       *core.OperationSource
	mu              sync.Mutex // partial cancel publication only; never encloses a callback
	cancel          context.CancelFunc
	attempted       atomic.Bool
	prepared        chan struct{} // original factory unwound, not evidence of ordinary return
	prepareReturned atomic.Bool
	started         atomic.Bool
	returned        atomic.Bool
	done            chan struct{} // original frame unwound; returned separately proves ordinary completion
}

func newAppOperationCancellation(operation *core.OperationSource) *appOperationCancellation {
	return &appOperationCancellation{operation: operation, prepared: make(chan struct{}), done: make(chan struct{})}
}

func (owner *appOperationCancellation) storeCancel(cancel context.CancelFunc) error {
	owner.mu.Lock()
	if cancel == nil || owner.cancel != nil {
		owner.mu.Unlock()
		return core.ErrEngineCleanupFailed
	}
	owner.cancel = cancel // original partial child, before any later Context getter
	owner.mu.Unlock()
	if !owner.operation.Current() {
		owner.request() // a request before publication is not lost; callback runs outside the copy mutex
	}
	return nil
}

func (owner *appOperationCancellation) originalCancel() context.CancelFunc {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.cancel
}

// Notification only: a duplicate/reentrant request must not wait on the first
// callback. Its return never proves that the original cancellation returned.
func (owner *appOperationCancellation) request() {
	if owner == nil || owner.operation == nil || owner.prepared == nil || owner.done == nil {
		panic(core.ErrEngineCleanupFailed)
	}
	owner.operation.Revoke() // passive negative fence before any actual callback
	cancel := owner.originalCancel()
	if cancel == nil {
		return // original factory is pending or did not yield a child; never completion
	}
	if !owner.started.CompareAndSwap(false, true) {
		return
	}
	defer close(owner.done) // panic/Goexit still signals unwind, never ordinary completion
	cancel()
	owner.returned.Store(true)
}

// Retain the original operation/task owner while joining. This observes only
// its preparation/cancel return; it acquires no outer serializer and does not
// itself authorize a slot release. A duplicate CancelFunc is not completion.
func (owner *appOperationCancellation) join() error {
	if owner == nil || owner.operation == nil || owner.prepared == nil || owner.done == nil {
		return core.ErrEngineCleanupFailed
	}
	owner.request()
	<-owner.prepared // original factory alone closes this, including abnormal unwind
	owner.request()  // also retire a partial child published after the first request
	if owner.originalCancel() != nil {
		<-owner.done
		if !owner.returned.Load() {
			return core.ErrEngineCleanupFailed
		}
	}
	if !owner.prepareReturned.Load() || owner.attempted.Load() && owner.originalCancel() == nil {
		return core.ErrEngineCleanupFailed
	}
	return nil
}

// Passive denial classification only. Check both original unwind notifications
// independently: cancellation may fail while construction is still pending.
// Open notifications are not a failure/timeout or proof of eventual completion.
func (owner *appOperationCancellation) knownFailure() bool {
	if owner == nil || owner.operation == nil || owner.prepared == nil || owner.done == nil {
		return true
	}
	select {
	case <-owner.prepared:
		if !owner.prepareReturned.Load() {
			return true
		}
	default:
	}
	select {
	case <-owner.done:
		if !owner.returned.Load() {
			return true
		}
	default:
	}
	return false
}
