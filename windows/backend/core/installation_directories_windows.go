//go:build windows

package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const maxInstallationDirectories = 8

func installationDataDirectory(name string) bool {
	switch name {
	case "new", "old", "displaced", "rejected", "restore":
		return true
	}
	return false
}

// A journal cannot select a different path family, volume or arbitrary child.
func installationDirectoryPath(root, path string) bool {
	if len(path) > 2048 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return false
	}
	parts := strings.Split(strings.ToLower(relative), `\`)
	for _, part := range parts {
		if !nativeImageComponent(part) {
			return false
		}
	}
	switch {
	case strings.HasPrefix(parts[0], ".snowden-update-stage-"):
		return len(parts) == 1 || len(parts) == 2 && installationDataDirectory(parts[1])
	case parts[0] == ".snowden-update":
		return len(parts) == 1 || len(parts) == 2 && parts[1] == "data" ||
			len(parts) == 3 && parts[1] == "data" && installationDataDirectory(parts[2])
	case strings.HasPrefix(parts[0], ".snowden-update-accepted-"):
		return len(parts) == 1 || len(parts) == 2 && parts[1] == "transaction" ||
			len(parts) == 3 && parts[1] == "transaction" && parts[2] == "data" ||
			len(parts) == 4 && parts[1] == "transaction" && parts[2] == "data" && installationDataDirectory(parts[3])
	}
	return false
}

func (owner *windowsInstallationRoot) directoryIndex(path string) int {
	for index := range owner.directories.files {
		if strings.EqualFold(owner.directories.files[index].path, path) {
			return index
		}
	}
	return -1
}

func (owner *windowsInstallationRoot) directoryParent(path string) windows.Handle {
	if strings.EqualFold(path, owner.root) && owner.lockIndex > 0 {
		return owner.image.files[owner.lockIndex-1].handle()
	}
	if index := owner.directoryIndex(path); index >= 0 {
		return owner.directories.files[index].handle()
	}
	return 0
}

func sameInstallationID(a, b windows.ByHandleFileInformation) bool {
	return a.VolumeSerialNumber == b.VolumeSerialNumber && a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
}

func (owner *windowsInstallationRoot) openDirectorySlot(ctx context.Context, index int, rebind bool) error {
	if contextError(ctx) != nil {
		return contextError(ctx)
	}
	file := &owner.directories.files[index]
	if file.handle() != 0 {
		return ErrInstallationRootUnavailable
	}
	name, err := windows.UTF16PtrFromString(file.path)
	if err != nil {
		return ErrInstallationRootUnavailable
	}
	access := uint32(windows.FILE_TRAVERSE | windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.DELETE | windows.SYNCHRONIZE)
	if err := file.open(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS); err != nil {
		return errors.Join(ErrInstallationRootUnavailable, err)
	}
	var current windows.ByHandleFileInformation
	if err := owner.directories.readFileIdentity(file, &current); err != nil {
		return err
	}
	if rebind && !sameInstallationID(current, file.identity) {
		return ErrInstallationRootUnavailable // never adopt a replacement object
	}
	if !rebind {
		file.identity = current
	}
	return nil
}

func (owner *windowsInstallationRoot) PinDirectory(ctx context.Context, path string) error {
	if err := owner.Validate(ctx, owner.root); err != nil {
		return err
	}
	if !installationDirectoryPath(owner.root, path) || owner.directoryParent(filepath.Dir(path)) == 0 {
		return ErrInstallationRootUnavailable
	}
	if owner.directoryIndex(path) >= 0 {
		return nil // original object was revalidated above, no duplicate handle
	}
	if len(owner.directories.files) >= maxInstallationDirectories {
		return ErrInstallationRootUnavailable
	}
	owner.directories.files = append(owner.directories.files, nativeImageFile{path: path, directory: true})
	return owner.openDirectorySlot(ctx, len(owner.directories.files)-1, false)
}

// Cleanup only: caller retains the root lock and its private creation IDs.
// Confirmed Close removes this slot; uncertain Close retains the exact handle.
func (owner *windowsInstallationRoot) ReleaseDirectory(path string) error {
	if owner.moving {
		return ErrHelperCleanup
	}
	index := owner.directoryIndex(path)
	if index < 0 {
		return nil // exact previous release may be retried during unpublished discard
	}
	file := &owner.directories.files[index]
	if err := file.close(); err != nil {
		return err
	}
	copy(owner.directories.files[index:], owner.directories.files[index+1:])
	owner.directories.files[len(owner.directories.files)-1] = nativeImageFile{}
	owner.directories.files = owner.directories.files[:len(owner.directories.files)-1]
	return nil
}

// ABI mirrors FILE_RENAME_INFORMATION (Go stdlib and Microsoft ntifs.h).
// False means the kernel must reject an existing destination. No Ex/POSIX,
// overwrite, path-based retry or fallback is requested.
type installationRenameInformation struct {
	ReplaceIfExists bool
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [256]uint16
}

func (owner *windowsInstallationRoot) MoveDirectory(ctx context.Context, from, to string) (moved bool, finalErr error) {
	if owner.hasLiveStaging() {
		return false, ErrInstallationRootUnavailable // no directory move under an original writer
	}
	if len(owner.payloads) != 0 {
		return false, ErrInstallationRootUnavailable // acquire payloads after publication; close before archive move
	}
	if err := owner.Validate(ctx, owner.root); err != nil {
		return false, err
	}
	source := owner.directoryIndex(from)
	parent := owner.directoryParent(filepath.Dir(to))
	if source < 0 || parent == 0 || !installationDirectoryPath(owner.root, to) || owner.directoryIndex(to) >= 0 ||
		strings.HasPrefix(strings.ToLower(to), strings.ToLower(from)+`\`) {
		return false, ErrInstallationRootUnavailable
	}
	name, err := windows.UTF16FromString(filepath.Base(to))
	if err != nil || len(name) > 256 {
		return false, ErrInstallationRootUnavailable
	}
	info := installationRenameInformation{RootDirectory: parent, FileNameLength: uint32(2 * (len(name) - 1))}
	copy(info.FileName[:], name)
	var next [maxInstallationDirectories]string
	for index, file := range owner.directories.files {
		if strings.EqualFold(file.path, from) {
			next[index] = to
		} else if strings.HasPrefix(strings.ToLower(file.path), strings.ToLower(from)+`\`) {
			next[index] = to + file.path[len(from):]
		}
	}
	owner.moving = true
	defer func() {
		if recover() != nil {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
		}
		// Query the original source handle even after a failed native call.
		// Only one of the two intended locations can authorize descendant rebind.
		actual, lookup := nativeFilePath(owner.directories.files[source].handle())
		if lookup != nil || !strings.EqualFold(actual, from) && !strings.EqualFold(actual, to) {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
			return // keep transition restrictive and every remaining original handle
		}
		moved = strings.EqualFold(actual, to)
		if !moved && finalErr == nil {
			finalErr = ErrInstallationRootUnavailable
		}
		if moved {
			for index, path := range next {
				if path != "" {
					owner.directories.files[index].path = path
				}
			}
		}
		// Owned cleanup may rebind after operation cancellation, at most7 known
		// objects. This soft budget is not a hard native I/O deadline.
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for index := range owner.directories.files {
			if owner.directories.files[index].handle() == 0 {
				if err := owner.openDirectorySlot(cleanup, index, true); err != nil {
					finalErr = errors.Join(finalErr, err)
					return
				}
			}
		}
		if err := owner.directories.validateFiles(cleanup); err != nil {
			finalErr = errors.Join(finalErr, err)
			return
		}
		owner.moving = false
		if ctx.Err() != nil {
			finalErr = errors.Join(finalErr, ctx.Err())
		}
	}()
	// Open descendants can prevent directory rename on Windows. Release only
	// after confirmed Close, retaining immutable private IDs and source handle.
	for index := len(owner.directories.files) - 1; index >= 0; index-- {
		if index != source && next[index] != "" {
			file := &owner.directories.files[index]
			if err := file.close(); err != nil {
				return false, err
			}
		}
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	finalErr = windows.NtSetInformationFile(owner.directories.files[source].handle(), &windows.IO_STATUS_BLOCK{},
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), windows.FileRenameInformation)
	return false, finalErr // defer derives actual location and verifies the complete owned set
}
