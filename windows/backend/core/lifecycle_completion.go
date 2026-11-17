package core

import (
	"context"
	"errors"
)

// Called with Manager.mu held. Only downstream value locks, with synchronous
// release; no original Context getters, upstream locks, I/O or native work.
type LifecycleCompletionGuard func() (func(), error)

// Optional refinement captures platform-specific context values BEFORE taking
// Manager.mu. The returned guard retains exact original sources, never a later
// global subject. Existing completion adapters receive core metadata only.
type LifecycleCompletionPreparationProbe interface {
	LifecycleCompletionProbe
	PrepareLifecycleCompletion(context.Context, *Manager) (LifecycleCompletionGuard, error)
}

// Negative metadata for the original operation, never an RPC context/parent or
// an admission. The actual source context/cause is read only during preparation.
type lifecycleCompletion struct {
	operation *nativeTUNOperation
	cause     error
	guard     LifecycleCompletionGuard
}

func prepareLifecycleCompletion(ctx context.Context, probe Probe, manager *Manager, positive bool) (prepared *lifecycleCompletion, finalErr error) {
	prepared = &lifecycleCompletion{}
	finalErr = cleanupCall("prepare lifecycle completion", func() error {
		var snapshotErr error
		prepared.operation, snapshotErr = snapshotNativeTUNOperation(ctx)
		prepared.cause = errors.Join(terminalContextCause(ctx), snapshotErr)
		if prepared.cause != nil || !positive {
			return prepared.cause
		}
		if source, ok := probe.(LifecycleCompletionPreparationProbe); ok {
			var err error
			prepared.guard, err = source.PrepareLifecycleCompletion(ctx, manager)
			if err != nil {
				return err
			}
		} else if source, ok := probe.(LifecycleCompletionProbe); ok {
			prepared.guard = func() (func(), error) {
				return source.GuardLifecycleCompletion(prepared.operation, manager)
			}
		} else if _, scoped := probe.(LifecycleSourceProbe); scoped {
			return errors.New("platform lifecycle completion guard unavailable")
		} else {
			prepared.guard = func() (func(), error) { return func() {}, nil } // explicit non-platform legacy contract
		}
		if prepared.guard == nil {
			return errors.New("lifecycle completion preparation unavailable")
		}
		return errors.Join(terminalContextCause(ctx), prepared.current())
	})
	return prepared, finalErr
}

func (prepared *lifecycleCompletion) current() error {
	if prepared == nil || prepared.operation == nil {
		return errors.New("lifecycle completion context unavailable")
	}
	return errors.Join(prepared.cause, contextError(prepared.operation))
}
