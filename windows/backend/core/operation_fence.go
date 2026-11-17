package core

import (
	"context"
	"errors"
)

// OperationFence is immutable original-context metadata for negative value
// checks. It is not a Context, RPC parent, cancellation owner or admission.
// Copies share the original Done/source pointers, never a refreshed subject.
type OperationFence struct{ operation *nativeTUNOperation }

// Capture outside every value lock protected by the resulting fence. Only
// preparation invokes the original context, including its custom Err method.
func CaptureOperationFence(ctx context.Context) (fence OperationFence, err error) {
	err = cleanupCall("capture operation fence", func() error {
		if ctx == nil {
			return errors.New("operation fence context unavailable")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		operation, err := snapshotNativeTUNOperation(ctx)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := contextError(operation); err != nil {
			return err
		}
		fence.operation = operation
		return nil
	})
	if err != nil {
		fence = OperationFence{}
	}
	return fence, err
}

func (fence OperationFence) Current() bool {
	return fence.operation != nil && contextError(fence.operation) == nil
}
