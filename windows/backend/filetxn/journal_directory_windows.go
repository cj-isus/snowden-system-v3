//go:build windows

package filetxn

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Reuse the bounded writer's relative opens, ACL/identity checks and checked
// handle retirement. No os.File conversion, finalizer or path-based mutation.
// One directory plus <=64 ancestors and one transient cleanup file are retained.
type nativeJournalDirectory struct {
	parents                       boundedNativeWrite
	file                          boundedNativeWriteFile
	deleting                      *nativeFileDeletion
	attempted, uncertain, removed bool
	observedPath                  string
}

func newJournalDirectory() journalDirectory {
	return &nativeJournalDirectory{parents: boundedNativeWrite{
		limit: maxFileSize, directories: make([]boundedNativeWriteFile, 0, maxWriteDirectories),
	}}
}

func (owner *nativeJournalDirectory) Uncertain() bool {
	if owner.uncertain || owner.file.acquiring || owner.file.ioPending || owner.file.closeErr != nil {
		return true
	}
	if file := owner.deleting; file != nil && file.Uncertain() {
		return true
	}
	for index := range owner.parents.directories {
		file := &owner.parents.directories[index]
		if file.acquiring || file.ioPending || file.closeErr != nil {
			return true
		}
	}
	return false
}

func (owner *nativeJournalDirectory) begin(path string, create bool) error {
	if owner.attempted {
		return ErrStagingCleanup
	}
	owner.attempted = true
	owner.parents.path = path
	return owner.parents.openDirectoriesMode(create)
}

func (owner *nativeJournalDirectory) parent() *boundedNativeWriteFile {
	return &owner.parents.directories[len(owner.parents.directories)-1]
}

func (owner *nativeJournalDirectory) Create(parent string) (string, os.FileInfo, error) {
	if err := owner.begin(filepath.Join(parent, ".import-stage-placeholder"), true); err != nil {
		return "", nil, err
	}
	if len(owner.parents.directories) >= maxWriteDirectories {
		return "", nil, ErrStagingCleanup // reserve one ancestor slot for the stage writer
	}
	for attempt := 0; attempt < 16; attempt++ {
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", nil, err
		}
		name := ".import-stage-" + hex.EncodeToString(nonce[:])
		path, _, err := windowsReadPath(filepath.Join(owner.parent().path, name))
		if err != nil {
			return "", nil, err
		}
		// Retain the native output field before NtCreateFile. Only a confirmed
		// no-create collision plus checked Close may replace this receipt.
		owner.file = boundedNativeWriteFile{path: path, directory: true, deleteDirectory: true}
		err = owner.parents.open(&owner.file, owner.parent().handle, name, windows.FILE_CREATE)
		if errors.Is(err, os.ErrExist) && !owner.file.created && owner.file.handle == 0 && !owner.file.acquiring {
			if err := owner.file.Close(); err != nil {
				return "", nil, err
			}
			continue
		}
		if err != nil {
			return owner.file.path, nil, err
		}
		info, err := owner.Inspect(owner.file.path)
		return owner.file.path, info, err
	}
	return "", nil, errors.Join(ErrStagingCleanup, os.ErrExist)
}

func (owner *nativeJournalDirectory) Open(path string) (os.FileInfo, error) {
	if err := owner.begin(path, false); err != nil {
		return nil, err // recovery never creates missing ancestors
	}
	owner.file = boundedNativeWriteFile{path: owner.parents.path, directory: true, deleteDirectory: true}
	if err := owner.parents.open(&owner.file, owner.parent().handle, filepath.Base(owner.file.path), windows.FILE_OPEN); err != nil {
		return nil, err
	}
	return owner.Inspect(owner.file.path)
}

func (owner *nativeJournalDirectory) Inspect(path string) (os.FileInfo, error) {
	absolute, _, pathErr := windowsReadPath(path)
	if pathErr != nil || !owner.attempted || owner.Uncertain() || owner.removed || owner.file.closeAttempted || !strings.EqualFold(absolute, owner.file.path) {
		return nil, ErrStagingCleanup
	}
	if err := owner.parents.validateDirectories(); err != nil {
		return nil, err
	}
	info, err := owner.parents.inspect(&owner.file)
	if err != nil {
		return nil, err
	}
	return &windowsReadInfo{path: owner.file.path, native: info}, nil
}

// Observe the original handle after an uncertain rename. This records evidence,
// never permission to retry, reopen a guessed path, delete or clear the barrier.
func (owner *nativeJournalDirectory) observeMove(from, to string) {
	defer func() { _ = recover() }()
	if owner.file.acl == nil {
		owner.file.acl = new(WindowsFileACLSlot)
	}
	for _, path := range []string{to, from} {
		info, err := InspectWindowsFileWithACL(owner.file.acl, owner.file.handle, path, true, maxFileSize, owner.parents.trusted)
		if err == nil && sameWindowsReadID(info, owner.file.identity) {
			owner.observedPath = path
			return
		}
	}
}

func (owner *nativeJournalDirectory) Move(from, to string) (finalErr error) {
	if _, err := owner.Inspect(from); err != nil || owner.deleting != nil {
		return errors.Join(ErrStagingCleanup, err)
	}
	from = owner.file.path
	path, _, err := windowsReadPath(to)
	if err != nil || !strings.EqualFold(filepath.Dir(path), owner.parent().path) || strings.EqualFold(from, path) {
		return errors.Join(ErrStagingCleanup, err)
	}
	name, err := windows.UTF16FromString(filepath.Base(path))
	if err != nil || len(name) > 256 {
		return ErrStagingCleanup
	}
	rename := boundedWriteRenameInformation{RootDirectory: owner.parent().handle, FileNameLength: uint32(2 * (len(name) - 1))}
	copy(rename.FileName[:], name) // ReplaceIfExists=false; no Ex/POSIX fallback
	defer func() {
		if recover() != nil {
			finalErr = ErrStagingCleanup
		}
		if finalErr != nil && owner.uncertain {
			owner.observeMove(from, path)
		}
	}()
	owner.uncertain = true
	var iosb windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(owner.file.handle, &iosb, (*byte)(unsafe.Pointer(&rename)),
		uint32(unsafe.Offsetof(rename.FileName))+rename.FileNameLength, windows.FileRenameInformation); err != nil {
		return errors.Join(ErrStagingCleanup, err)
	}
	owner.file.path = path
	if _, err := owner.parents.inspect(&owner.file); err != nil {
		return err
	}
	if err := owner.parents.validateDirectories(); err != nil {
		return err
	}
	owner.observedPath, owner.uncertain = path, false
	return nil
}

func (owner *nativeJournalDirectory) Remove() error {
	if _, err := owner.Inspect(owner.file.path); err != nil || owner.deleting != nil {
		return errors.Join(ErrStagingCleanup, err)
	}
	owner.uncertain = true
	var iosb windows.IO_STATUS_BLOCK
	disposition := byte(1)
	err := windows.NtSetInformationFile(owner.file.handle, &iosb, &disposition, 1, windows.FileDispositionInformation)
	if errors.Is(err, windows.STATUS_DIRECTORY_NOT_EMPTY) {
		// No traversal/forced deletion of unrecorded children. Keep the same
		// original owner for a later explicit repair; this is debris, not commit.
		owner.uncertain = false
		return ErrStagingCleanup
	}
	if err != nil {
		return errors.Join(ErrStagingCleanup, err)
	}
	owner.removed = true
	if err := owner.file.Close(); err != nil {
		return err
	}
	for index := len(owner.parents.directories) - 1; index >= 0; index-- {
		if err := owner.parents.directories[index].Close(); err != nil {
			return err
		}
	}
	owner.uncertain = false
	return nil
}
