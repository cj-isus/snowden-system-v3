//go:build !windows

package core

import "context"

func openInstalledPairImage(context.Context, string) (nativeImageLease, error) {
	return nil, ErrNativeImageUnavailable
}

func pinApplicationImage(context.Context, nativeImageLease, string, int64, int64) error {
	return ErrNativeImageUnavailable
}

func installedApplicationExecutable(nativeImageLease) string { return "" }

func validateInstalledApplicationProcess(context.Context, nativeImageLease, processHost) error {
	return ErrNativeImageUnavailable
}
