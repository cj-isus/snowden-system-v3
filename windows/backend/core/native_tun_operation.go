package core

import (
	"context"
	"errors"
	"time"
)

// Metadata-only view of the original native observation operation. Capture it
// outside the admission/source metadata locks it protects. Worker callers may
// retain their outer Engine operation lock; this view does not make arbitrary
// RPC/SDK/context callbacks under that outer lock safe. Never use it as an RPC
// context or context parent: only Done/deadline/core source values are supported.
// No cancellation observer, timer or successor operation is created here.
type nativeTUNOperation struct {
	original    context.Context // original custody only; getters never invoke it
	done        <-chan struct{}
	deadline    time.Time
	hasDeadline bool
	sources     any
}

func snapshotNativeTUNOperation(original context.Context) (snapshot *nativeTUNOperation, finalErr error) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			snapshot = nil
			finalErr = errors.Join(ErrNativeScopeUnavailable, finalErr)
		}
	}()
	if finalErr = contextError(original); finalErr != nil {
		returned = true
		return nil, finalErr
	}
	snapshot = &nativeTUNOperation{original: original}
	snapshot.done = original.Done()
	snapshot.deadline, snapshot.hasDeadline = original.Deadline()
	snapshot.sources = original.Value(operationSourcesContextKey{})
	finalErr = errors.Join(contextError(original), contextError(snapshot))
	if finalErr != nil {
		snapshot = nil
	}
	returned = true
	return snapshot, finalErr
}

func (operation *nativeTUNOperation) Deadline() (time.Time, bool) {
	return operation.deadline, operation.hasDeadline
}

func (operation *nativeTUNOperation) Done() <-chan struct{} { return operation.done }

// This view is used only for negative admission checks. It preserves an
// expired real deadline even before timer delivery, not a custom Err/Cause.
func (operation *nativeTUNOperation) Err() error {
	if operation.hasDeadline && !time.Now().Before(operation.deadline) {
		return context.DeadlineExceeded
	}
	select {
	case <-operation.done:
		return context.Canceled
	default:
		return nil
	}
}

func (operation *nativeTUNOperation) Value(key any) any {
	if _, ok := key.(operationSourcesContextKey); ok {
		return operation.sources // exact original pointer-valued binding, never refreshed
	}
	return nil // no arbitrary original-context callbacks under metadata locks
}
