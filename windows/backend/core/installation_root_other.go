//go:build !windows

package core

import "context"

func newInstallationRoot() nativeInstallationPreparation { return nil }

func openInstallationRoot(context.Context, string) (nativeInstallationRoot, error) {
	return nil, ErrInstallationRootUnavailable
}
