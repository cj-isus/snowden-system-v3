//go:build windows

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const installationRootLockName = ".snowden-update.lock"

type windowsInstallationRoot struct {
	image           windowsNativeImage // shared ACL/path/NTFS/ID/close primitives; no EXE pin
	directories     windowsNativeImage // bounded original transaction directory objects
	root            string
	lockIndex       int
	locked          bool
	moving          bool
	payloads        []*installationPayloadFile
	payloadRetiring bool
	staging         []*installationStagingFile
	stagingRetiring bool
}

func newInstallationRoot() nativeInstallationPreparation {
	return &windowsInstallationRoot{image: windowsNativeImage{files: make([]nativeImageFile, 0, 65)},
		directories: windowsNativeImage{files: make([]nativeImageFile, 0, maxInstallationDirectories)}, lockIndex: -1}
}

func openInstallationRoot(ctx context.Context, root string) (lease nativeInstallationRoot, finalErr error) {
	owner := newInstallationRoot()
	lease = owner
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
		}
	}()
	finalErr = owner.open(ctx, root)
	completed = true
	return
}

func installationLockError(err error) error {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.STATUS_SHARING_VIOLATION) || errors.Is(err, windows.STATUS_FILE_LOCK_CONFLICT) || errors.Is(err, windows.STATUS_LOCK_NOT_GRANTED) {
		return ErrInstallationRootBusy
	}
	return ErrInstallationRootUnavailable
}

func (owner *windowsInstallationRoot) open(ctx context.Context, root string) error {
	exe, err := os.Executable()
	if err != nil || !strings.EqualFold(filepath.Dir(exe), root) {
		return ErrInstallationRootUnavailable // scope cannot target another installation
	}
	if err := owner.image.openDirectories(ctx, root); err != nil {
		return errors.Join(ErrInstallationRootUnavailable, err)
	}
	owner.root = root
	owner.directories.trust = owner.image.trust
	path := filepath.Join(root, installationRootLockName)
	name, err := windows.NewNTUnicodeString(installationRootLockName)
	if err != nil || contextError(ctx) != nil || len(owner.image.files) == 0 {
		return ErrInstallationRootUnavailable
	}
	// Persistent name, ephemeral kernel ownership. Never truncate, delete a
	// "stale" file, trust a PID, queue, or replace an uncertain original handle.
	// Capture native output directly in the original receipt BEFORE acquisition.
	// FILE_OPEN_IF preserves a prior zero-byte cooperative lock; no truncate.
	owner.lockIndex = len(owner.image.files)
	parent := owner.image.files[owner.lockIndex-1].handle()
	owner.image.files = append(owner.image.files, nativeImageFile{path: path})
	file := &owner.image.files[owner.lockIndex]
	information, err := file.openRelative(parent, name, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.READ_CONTROL|windows.SYNCHRONIZE,
		windows.FILE_ATTRIBUTE_NORMAL, 0, windows.FILE_OPEN_IF,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if err != nil {
		return errors.Join(installationLockError(err), err)
	}
	if file.handle() == 0 || information != ntFileOpened && information != ntFileCreated {
		return ErrInstallationRootUnavailable
	}
	if err := owner.image.readFileIdentity(file, &file.identity); err != nil || file.identity.FileSizeHigh != 0 || file.identity.FileSizeLow != 0 {
		return ErrInstallationRootUnavailable
	}
	if err := windows.LockFileEx(file.handle(), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		return installationLockError(err)
	}
	owner.locked = true
	return owner.Validate(ctx, root)
}

func (owner *windowsInstallationRoot) Validate(ctx context.Context, root string) error {
	if !owner.locked || owner.lockIndex < 1 || owner.lockIndex != len(owner.image.files)-1 || !strings.EqualFold(root, owner.root) ||
		!filepath.IsAbs(root) || filepath.Clean(root) != root {
		return ErrInstallationRootUnavailable
	}
	if err := owner.image.validateFiles(ctx); err != nil {
		return errors.Join(ErrInstallationRootUnavailable, err)
	}
	if owner.moving {
		return ErrInstallationRootUnavailable
	}
	if owner.payloadRetiring {
		return ErrInstallationRootUnavailable
	}
	if owner.stagingRetiring {
		return ErrInstallationRootUnavailable
	}
	for _, file := range owner.staging {
		if err := file.validate(ctx); err != nil {
			return err
		}
	}
	for _, file := range owner.payloads {
		if !file.closed {
			if err := file.validate(ctx); err != nil {
				return err
			}
		}
	}
	if len(owner.directories.files) != 0 {
		if err := owner.directories.validateFiles(ctx); err != nil {
			return errors.Join(ErrInstallationRootUnavailable, err)
		}
	}
	return contextError(ctx)
}

func (owner *windowsInstallationRoot) Close() error {
	if err := owner.ReleaseStagingFiles(); err != nil {
		return err // no root/payload release under uncertain original writer ownership
	}
	if err := owner.ReleasePayloads(); err != nil {
		return err // no directory/root unlock under uncertain regular-file custody
	}
	if err := owner.directories.Close(); err != nil {
		return err // retain root lock until every child handle is confirmed closed
	}
	owner.directories.files = owner.directories.files[:0]
	if owner.locked {
		if owner.lockIndex < 0 || owner.lockIndex >= len(owner.image.files) {
			return ErrHelperCleanup
		}
		handle := owner.image.files[owner.lockIndex].handle()
		if handle == 0 || windows.UnlockFileEx(handle, 0, 1, 0, &windows.Overlapped{}) != nil {
			return ErrHelperCleanup // retain this exact lock and all original directory handles
		}
		owner.locked = false
	}
	return owner.image.Close() // same reverse original-handle retirement as image owner
}
