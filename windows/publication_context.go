package main

import (
	"context"
	"reflect"

	"github.com/snowden-system/windows/backend/core"
)

// Only preparation reads the source Context. These private App values and the
// opaque core fence are checked under publication locks without callbacks.
type publicationContext struct {
	fence        core.OperationFence
	operation    *appOperationScope
	hasOperation bool
	path         *networkPathSource
	hasPath      bool
}

func capturePublicationContext(ctx context.Context) (captured *publicationContext) {
	returned := false
	defer func() {
		if !returned {
			_ = recover() // arbitrary context payloads are never diagnostics
			captured = nil
		}
	}()
	fence, err := core.CaptureOperationFence(ctx)
	if err != nil {
		returned = true
		return nil
	}
	operation, hasOperation := ctx.Value(appOperationContextKey{}).(*appOperationScope)
	if operation != nil {
		original := *operation // freeze values, retain only original negative source pointers
		operation = &original
	}
	path, hasPath := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	captured = &publicationContext{fence: fence, operation: operation, hasOperation: hasOperation, path: path, hasPath: hasPath}
	if ctx.Err() != nil || !captured.current() {
		captured = nil
	}
	returned = true
	return captured
}

func (captured *publicationContext) current() bool {
	if captured == nil || !captured.fence.Current() || captured.path != nil && !captured.path.current.Load() {
		return false
	}
	original := captured.operation
	return !captured.hasOperation || original != nil && original.intentSource.Current() && original.operation.Current()
}

// Exact optional-observer identity only. Comparable checks the actual value,
// including nested interface fields; Type.Comparable alone is insufficient.
// Unknown identity is not replaced by equal Done channels or deadlines.
func sameContextIdentity(current, original context.Context) bool {
	return current != nil && original != nil && reflect.ValueOf(current).Comparable() && reflect.ValueOf(original).Comparable() && current == original
}
