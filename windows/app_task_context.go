package main

import (
	"context"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Capture before registry ownership. The actual original App Context remains
// the parent; this immutable fence is a negative publication check only.
type appTaskParent struct {
	app   *App
	ctx   context.Context
	fence core.OperationFence
}

func (a *App) captureAppTaskParent() appTaskParent {
	a.mu.Lock()
	defer a.mu.Unlock()
	return appTaskParent{app: a, ctx: a.ctx, fence: a.ctxFence}
}

// The containing original task's mu protects publication and ready. The
// cancellation owner is immutable from passive reservation onward.
type appTaskContext struct {
	cancellation *appOperationCancellation
	dependencies [2]*core.OperationSource // immutable original negative sources, never a parent/positive grant
	parentFence  core.OperationFence
	childFence   core.OperationFence
	ready        bool
}

func newAppTaskContext() appTaskContext {
	return appTaskContext{cancellation: newAppOperationCancellation(core.NewOperationSource())}
}

func (state *appTaskContext) currentLocked() bool {
	return state.ready && state.sourcesCurrent() &&
		state.parentFence.Current() && state.childFence.Current()
}

func (state *appTaskContext) sourcesCurrent() bool {
	if state.cancellation == nil || !state.cancellation.operation.Current() {
		return false
	}
	for _, source := range state.dependencies {
		if source != nil && !source.Current() {
			return false
		}
	}
	return true
}

// The original creator alone enters this factory, outside every value lock.
// Only this actual frame records constructor unwind and ordinary return.
func (state *appTaskContext) construct(parent appTaskParent) (ctx context.Context, fence core.OperationFence, result error) {
	return state.constructContext(parent, 0)
}

func (state *appTaskContext) constructTimeout(parent appTaskParent, timeout time.Duration) (ctx context.Context, fence core.OperationFence, result error) {
	if timeout <= 0 {
		return state.constructContext(parent, -1) // ordinary invalid input; no unbounded substitute
	}
	return state.constructContext(parent, timeout)
}

func (state *appTaskContext) constructContext(parent appTaskParent, timeout time.Duration) (ctx context.Context, fence core.OperationFence, result error) {
	owner, returned := state.cancellation, false
	defer func() {
		if !returned {
			_ = recover() // no custom-parent payload is published; Goexit still unwinds the creator
			ctx, fence, result = nil, core.OperationFence{}, core.ErrEngineCleanupFailed
		}
		owner.prepareReturned.Store(returned)
		close(owner.prepared)
	}()
	if timeout < 0 || !parent.fence.Current() || !state.sourcesCurrent() || parent.app.closing.Load() {
		returned = true
		return nil, core.OperationFence{}, context.Canceled
	}
	owner.attempted.Store(true)
	var child context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		child, cancel = context.WithTimeout(parent.ctx, timeout)
	} else {
		child, cancel = context.WithCancel(parent.ctx)
	}
	if err := owner.storeCancel(cancel); err != nil {
		return nil, core.OperationFence{}, core.ErrEngineCleanupFailed
	}
	if !state.sourcesCurrent() || !parent.fence.Current() {
		returned = true
		return nil, core.OperationFence{}, context.Canceled
	}
	fence, result = core.CaptureOperationFence(child)
	returned = true
	if result != nil {
		return nil, core.OperationFence{}, core.ErrEngineCleanupFailed // never retain arbitrary parent errors
	}
	return child, fence, nil
}

// Called only by the original creator when this factory was never entered.
// This proves no constructor attempt, not cancellation of an unknown child.
func (state *appTaskContext) skipConstruction() {
	state.cancellation.prepareReturned.Store(true)
	close(state.cancellation.prepared)
}
