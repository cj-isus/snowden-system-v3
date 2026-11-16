//go:build !windows

package core

import "context"

func newRecoveryNativeImage() recoveryImagePreparation { return nil }

func openRecoveryImage(context.Context, string, int64) (nativeImageLease, error) {
	return nil, ErrNativeImageUnavailable
}
