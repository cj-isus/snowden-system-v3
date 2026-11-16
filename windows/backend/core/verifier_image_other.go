//go:build !windows

package core

import "context"

func newVerifierApplication() verifierApplicationLease { return nil }

func openVerifierApplication(context.Context) (nativeImageLease, error) {
	return nil, ErrNativeImageUnavailable
}
