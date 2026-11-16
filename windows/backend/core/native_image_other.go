//go:build !windows

package core

import "context"

func newNativeImage() nativeImagePreparation { return nil }

func openNativeImage(context.Context, string) (nativeImageLease, error) {
	return nil, ErrNativeImageUnavailable
}
