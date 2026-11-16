//go:build windows

package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const maxInstallationNameAttempts = 16
const ntFileOpened = 1
const ntFileCreated = 2 // IO_STATUS_BLOCK.Information, distinct from FILE_CREATE disposition

// file is already stored in the original root's bounded slot. Native output
// goes directly to that receipt before validation or any wrapper conversion.
func createInstallationObject(parent windows.Handle, file *nativeImageFile, access, share uint32) (created, collision bool, finalErr error) {
	name := filepath.Base(file.path)
	if parent == 0 || !nativeImageComponent(name) {
		return false, false, ErrInstallationRootUnavailable
	}
	un, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return false, false, err
	}
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	attributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if file.directory {
		options |= windows.FILE_DIRECTORY_FILE
		attributes = windows.FILE_ATTRIBUTE_DIRECTORY
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE | windows.FILE_SEQUENTIAL_ONLY
	}
	information, err := file.openRelative(parent, un, access, attributes, share, windows.FILE_CREATE, options)
	created = file.handle() != 0 || information == ntFileCreated
	if errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) && !created {
		return false, true, errors.Join(os.ErrExist, err)
	}
	if err != nil || file.handle() == 0 || information != ntFileCreated {
		return created, false, errors.Join(ErrInstallationRootUnavailable, err)
	}
	return true, false, nil
}

func (owner *windowsInstallationRoot) CreateDirectory(ctx context.Context, path string) (created bool, finalErr error) {
	if owner.Validate(ctx, owner.root) != nil || !installationDirectoryPath(owner.root, path) || owner.directoryParent(filepath.Dir(path)) == 0 {
		return false, ErrInstallationRootUnavailable
	}
	if owner.directoryIndex(path) >= 0 {
		return false, os.ErrExist // an existing pin is never a new-object receipt
	}
	if len(owner.directories.files) >= maxInstallationDirectories {
		return false, ErrInstallationRootUnavailable
	}
	index := len(owner.directories.files)
	owner.directories.files = append(owner.directories.files, nativeImageFile{path: path, directory: true})
	file := &owner.directories.files[index]
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			created = created || file.handle() != 0
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
		}
	}()
	var collision bool
	created, collision, finalErr = createInstallationObject(owner.directoryParent(filepath.Dir(path)), file,
		windows.FILE_TRAVERSE|windows.FILE_READ_ATTRIBUTES|windows.READ_CONTROL|windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if collision {
		owner.directories.files[index] = nativeImageFile{}
		owner.directories.files = owner.directories.files[:index] // only a confirmed no-create collision
		completed = true
		return false, finalErr
	}
	if finalErr == nil {
		finalErr = owner.directories.readFileIdentity(file, &file.identity)
	}
	if finalErr == nil {
		finalErr = owner.Validate(ctx, owner.root)
	}
	completed = true
	return created, finalErr // retain original output on every partial result
}

func (owner *windowsInstallationRoot) CreateUniqueDirectory(ctx context.Context, prefix string) (path string, created bool, finalErr error) {
	if prefix != ".snowden-update-stage-" && prefix != ".snowden-update-accepted-" {
		return "", false, ErrInstallationRootUnavailable
	}
	for attempt := 0; attempt < maxInstallationNameAttempts; attempt++ {
		if contextError(ctx) != nil {
			return "", false, contextError(ctx)
		}
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", false, err
		}
		path = filepath.Join(owner.root, prefix+hex.EncodeToString(nonce[:]))
		created, finalErr = owner.CreateDirectory(ctx, path)
		if errors.Is(finalErr, os.ErrExist) && !created {
			continue
		}
		return path, created, finalErr
	}
	return "", false, errors.Join(ErrInstallationRootBusy, os.ErrExist)
}
