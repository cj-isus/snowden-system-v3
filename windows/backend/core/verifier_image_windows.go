//go:build windows

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

// Reuse original ancestor/ACL/FileID/deny-write-delete/checked-Close custody.
// This pins the current on-disk application, not its kernel loaded sections or
// a new external code-signing authority. No network helper is needed for TUF.
type windowsVerifierImage struct {
	image *windowsNativeImage
	path  string
}

func openVerifierApplication(ctx context.Context) (lease nativeImageLease, finalErr error) {
	owner := newVerifierApplication()
	lease = owner // partial native custody exists before its first acquisition
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrNativeImageUnavailable)
		}
	}()
	finalErr = owner.prepare(ctx)
	completed = true
	return owner, finalErr
}

func newVerifierApplication() verifierApplicationLease {
	return &windowsVerifierImage{image: &windowsNativeImage{files: make([]nativeImageFile, 0, 65)}}
}

func (owner *windowsVerifierImage) prepare(ctx context.Context) error {
	if owner == nil || owner.image == nil || owner.path != "" || len(owner.image.files) != 0 {
		return ErrNativeImageUnavailable
	}
	var err error
	owner.path, err = os.Executable()
	if err != nil || contextError(ctx) != nil || runtime.GOARCH != "amd64" || len(owner.path) > 2048 ||
		!filepath.IsAbs(owner.path) || filepath.Clean(owner.path) != owner.path || !nativeImageComponent(filepath.Base(owner.path)) {
		return ErrNativeImageUnavailable
	}
	if err := owner.image.openDirectories(ctx, filepath.Dir(owner.path)); err != nil {
		return err
	}
	if err := owner.image.openFile(ctx, owner.path, false); err != nil {
		return err
	}
	file := owner.image.files[len(owner.image.files)-1]
	size := int64(file.identity.FileSizeHigh)<<32 | int64(file.identity.FileSizeLow)
	if size < 1 || size > maxApplicationImageBytes {
		return ErrNativeImageUnavailable
	}
	reader := &nativeImageReader{ctx: ctx, handle: file.handle(), size: size, limit: maxApplicationImageBytes}
	if nativeImagePEKind(reader, size, true) != nil {
		return ErrNativeImageUnavailable
	}
	// There is no independently approved hash for a first-run application here.
	// Hashing its own disk bytes would not attest the currently loaded sections.
	// Keep the original deny-write/delete image/ancestor identity through spawn.
	return owner.Validate(ctx)
}

func (owner *windowsVerifierImage) Executable() string { return owner.path }

func (owner *windowsVerifierImage) Validate(ctx context.Context) error {
	if owner == nil || owner.image == nil || owner.path == "" {
		return ErrNativeImageUnavailable
	}
	if err := owner.image.validateFiles(ctx); err != nil {
		return err
	}
	return nativeProcessPathMatches(ctx, windows.CurrentProcess(), owner.path)
}

func (owner *windowsVerifierImage) ValidateProcess(ctx context.Context, host processHost) error {
	if err := owner.Validate(ctx); err != nil {
		return err
	}
	process, ok := host.(*windowsProcessHost)
	if !ok || process == nil {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(process.native.WithAssigned(func(job, handle windows.Handle) error {
		return nativeProcessPathMatches(ctx, handle, owner.path)
	}))
}

func (owner *windowsVerifierImage) Close() error { return owner.image.Close() }
