//go:build windows

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/update"
)

// The target is the fixed main sibling even when the caller is the recovery
// companion. Never confuse the companion's verified bytes with installed main.
func openInstalledPairImage(ctx context.Context, digest string) (lease nativeImageLease, finalErr error) {
	image := newNativeImage()
	lease = image
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrNativeImageUnavailable)
		}
	}()
	finalErr = image.prepareInstalled(ctx, digest)
	completed = true
	return image, finalErr
}

func (image *windowsNativePair) prepareInstalled(ctx context.Context, digest string) error {
	caller, err := os.Executable()
	if err != nil || len(caller) > 2048 || !filepath.IsAbs(caller) || filepath.Clean(caller) != caller {
		return ErrNativeImageUnavailable
	}
	return image.openAt(ctx, digest, filepath.Join(filepath.Dir(caller), update.ApplicationFile))
}

func pinApplicationImage(ctx context.Context, lease nativeImageLease, digest string, size, helperSize int64) error {
	image, ok := lease.(*windowsNativePair)
	pin, err := nativeImagePin(digest)
	if !ok || err != nil || size < 1 || size > maxApplicationImageBytes || helperSize < 1 || helperSize > maxNativeImageBytes {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(image.pair.PinApplication(ctx, pin, size, helperSize, contextError, nativePairAppIDs))
}

func installedApplicationExecutable(lease nativeImageLease) string {
	if image, ok := lease.(*windowsNativePair); ok {
		application, _ := image.pair.Paths()
		return application
	}
	return ""
}

func validateInstalledApplicationProcess(ctx context.Context, lease nativeImageLease, host processHost) error {
	image, ok := lease.(*windowsNativePair)
	if !ok {
		return ErrNativeImageUnavailable
	}
	return image.validateProcess(ctx, host, true)
}
