//go:build windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap/imagelease"
	"github.com/snowden-system/windows/backend/netprobe"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// The minimal pair owns original image files; core retains role/name/cap/WFP
// authority and the surrounding original process/Job custody.
type windowsNativePair struct{ pair imagelease.Pair }

func newNativeImage() nativeImagePreparation { return &windowsNativePair{} }

func openNativeImage(ctx context.Context, digest string) (lease nativeImageLease, finalErr error) {
	image := newNativeImage()
	lease = image // original partial owner before any native operation
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = ErrNativeImageUnavailable
		}
	}()
	finalErr = image.prepareCurrent(ctx, digest)
	completed = true
	return
}

func (image *windowsNativePair) prepareCurrent(ctx context.Context, digest string) error {
	parent, err := os.Executable()
	if err != nil {
		return ErrNativeImageUnavailable
	}
	return image.openAt(ctx, digest, parent)
}

func (image *windowsNativePair) openAt(ctx context.Context, digest, parent string) error {
	pin, err := nativeImagePin(digest)
	if err != nil || maxNativeImageBytes != imagelease.MaxHelperBytes || maxApplicationImageBytes != imagelease.MaxApplicationBytes {
		return ErrNativeImageUnavailable // cap drift requires an explicit compatible change
	}
	helper := filepath.Join(filepath.Dir(parent), netprobe.NativeHelperName)
	if err := image.pair.Prepare(ctx, parent, helper, pin, contextError); err != nil {
		return nativeImageInspectionError(err)
	}
	return image.Validate(ctx)
}

func (image *windowsNativePair) Executable() string {
	_, helper := image.pair.Paths()
	return helper
}

func (image *windowsNativePair) Validate(ctx context.Context) error {
	return nativeImageInspectionError(image.pair.WithPaths(ctx, contextError, nativePairAppIDs))
}

func nativePairAppIDs(parent, helper string) error {
	parentID, err := wf.AppID(parent)
	if err != nil || parentID == "" {
		return ErrNativeImageUnavailable
	}
	helperID, err := wf.AppID(helper)
	if err != nil || helperID == "" || strings.EqualFold(parentID, helperID) {
		return ErrNativeImageUnavailable
	}
	return nil
}

func (image *windowsNativePair) ValidateProcess(ctx context.Context, host processHost) error {
	return image.validateProcess(ctx, host, false)
}

func (image *windowsNativePair) validateProcess(ctx context.Context, host processHost, application bool) error {
	owned, ok := host.(*windowsProcessHost)
	if !ok || owned == nil {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(owned.native.WithAssigned(func(job, process windows.Handle) error {
		return image.pair.WithPaths(ctx, contextError, func(parent, helper string) error {
			if err := nativePairAppIDs(parent, helper); err != nil {
				return err
			}
			expected := helper
			if application {
				expected = parent
			}
			return nativeProcessPathMatches(ctx, process, expected)
		})
	}))
}

func (image *windowsNativePair) Close() error {
	return nativeImageInspectionError(image.pair.Close())
}
