//go:build windows

package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/snowden-system/windows/backend/update"
	"golang.org/x/sys/windows"
)

type windowsRecoveryImage struct {
	image windowsNativeImage // shared original ACL/NTFS/nonreparse/single-link/ID/close primitives
	ready bool
}

func newRecoveryNativeImage() recoveryImagePreparation {
	return &windowsRecoveryImage{image: windowsNativeImage{files: make([]nativeImageFile, 0, 65)}}
}

func openRecoveryImage(ctx context.Context, digest string, size int64) (lease nativeImageLease, finalErr error) {
	owner := newRecoveryNativeImage()
	lease = owner // before the first native acquisition
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrNativeImageUnavailable)
		}
	}()
	finalErr = owner.open(ctx, digest, size)
	completed = true
	return lease, finalErr
}

func (owner *windowsRecoveryImage) open(ctx context.Context, digest string, size int64) error {
	pin, err := nativeImagePin(digest)
	caller, callerErr := os.Executable()
	if err != nil || callerErr != nil || contextError(ctx) != nil || runtime.GOARCH != "amd64" ||
		size < 1 || size > maxApplicationImageBytes || !filepath.IsAbs(caller) || filepath.Clean(caller) != caller || len(caller) > 2048 {
		return ErrNativeImageUnavailable
	}
	// Exact byte-copy sibling, never a caller-supplied executable path or hard
	// link to the mutable main. Old code/journal facts cannot choose an image.
	path := filepath.Join(filepath.Dir(caller), update.RecoveryApplicationFile)
	if len(path) > 2048 {
		return ErrNativeImageUnavailable
	}
	if err := owner.image.openDirectories(ctx, filepath.Dir(caller)); err != nil {
		return err
	}
	owner.image.path = path
	if err := owner.image.openFile(ctx, path, false); err != nil {
		return err
	}
	file := owner.image.files[len(owner.image.files)-1]
	actualSize := int64(file.identity.FileSizeHigh)<<32 | int64(file.identity.FileSizeLow)
	if actualSize != size {
		return ErrNativeImageUnavailable
	}
	reader := &nativeImageReader{ctx: ctx, handle: file.handle(), size: size, limit: maxApplicationImageBytes}
	hash := sha256.New()
	n, err := io.CopyBuffer(hash, io.NewSectionReader(reader, 0, size), make([]byte, 32<<10))
	if err != nil || n != size || !bytes.Equal(hash.Sum(nil), pin[:]) || nativeImagePEKind(reader, size, true) != nil {
		return ErrNativeImageUnavailable
	}
	owner.ready = true
	return owner.Validate(ctx)
}

func (owner *windowsRecoveryImage) Executable() string { return owner.image.path }

func (owner *windowsRecoveryImage) Validate(ctx context.Context) error {
	if !owner.ready || owner.image.path == "" {
		return ErrNativeImageUnavailable
	}
	return owner.image.validateFiles(ctx)
}

func (owner *windowsRecoveryImage) ValidateProcess(ctx context.Context, host processHost) error {
	owned, ok := host.(*windowsProcessHost)
	if !ok || owned == nil {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(owned.native.WithAssigned(func(job, process windows.Handle) error {
		if owner.Validate(ctx) != nil {
			return ErrNativeImageUnavailable
		}
		return nativeProcessPathMatches(ctx, process, owner.image.path)
	}))
}

func (owner *windowsRecoveryImage) Close() error {
	owner.ready = false
	return owner.image.Close()
}
