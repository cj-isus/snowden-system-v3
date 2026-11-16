//go:build windows

package filetxn

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type nativeJournalWriterLock struct {
	readOnly                   bool
	parents                    boundedNativeWrite
	file                       boundedNativeWriteFile
	lockRequest, unlockRequest windows.Overlapped
	attempted, locked, pending bool
	closeAttempted             bool
	closeErr                   error
}

func newJournalWriterLock(path string) journalWriterLock {
	return newJournalScopeLock(path, false)
}

func newJournalReaderLock(path string) journalWriterLock {
	return newJournalScopeLock(path, true)
}

func newJournalScopeLock(path string, readOnly bool) journalWriterLock {
	return &nativeJournalWriterLock{readOnly: readOnly, parents: boundedNativeWrite{
		path: path, limit: maxFileSize, directories: make([]boundedNativeWriteFile, 0, maxWriteDirectories),
	}}
}

func (owner *nativeJournalWriterLock) Acquire() error {
	if owner.attempted || owner.closeAttempted {
		return ErrWriteRetirement
	}
	owner.attempted = true
	if err := owner.parents.openDirectoriesMode(!owner.readOnly); err != nil {
		if owner.readOnly && errors.Is(err, os.ErrNotExist) {
			return ErrReadScopeUnavailable
		}
		return err
	}
	parent := &owner.parents.directories[len(owner.parents.directories)-1]
	owner.file = boundedNativeWriteFile{path: owner.parents.path, writerLock: true, readOnlyLock: owner.readOnly}
	disposition := uint32(windows.FILE_OPEN_IF)
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY | windows.LOCKFILE_EXCLUSIVE_LOCK)
	if owner.readOnly {
		disposition = windows.FILE_OPEN
		flags = windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	if err := owner.parents.open(&owner.file, parent.handle, filepath.Base(owner.file.path), disposition); err != nil {
		if owner.readOnly && errors.Is(err, os.ErrNotExist) {
			return ErrReadScopeUnavailable
		}
		return err
	}
	if owner.file.identity.FileSizeHigh != 0 || owner.file.identity.FileSizeLow != 0 {
		return ErrRecoveryRequired // no truncation, stale/PID cleanup or replacement
	}
	owner.pending = true
	err := windows.LockFileEx(owner.file.handle, flags, 0, 1, 0, &owner.lockRequest)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		owner.pending = false // synchronous, fail-immediate confirmed contention
		return errors.Join(ErrWriterBusy, err)
	}
	if err != nil {
		return errors.Join(ErrWriteRetirement, err)
	}
	owner.locked, owner.pending = true, false
	return owner.Validate()
}

func (owner *nativeJournalWriterLock) Validate() error {
	if !owner.locked || owner.pending || owner.closeAttempted {
		return ErrWriteRetirement
	}
	if err := owner.parents.validateDirectories(); err != nil {
		return err
	}
	info, err := owner.parents.inspect(&owner.file)
	if err != nil || info.FileSizeHigh != 0 || info.FileSizeLow != 0 {
		return errors.Join(ErrRecoveryRequired, err)
	}
	return nil
}

func (owner *nativeJournalWriterLock) Close() (finalErr error) {
	if owner.closeAttempted {
		return owner.closeErr
	}
	owner.closeAttempted, owner.closeErr = true, ErrWriteRetirement
	defer func() {
		if recover() != nil {
			finalErr = ErrWriteRetirement
		}
		owner.closeErr = finalErr
	}()
	if owner.pending {
		return ErrWriteRetirement
	}
	if owner.locked {
		owner.pending = true
		if err := windows.UnlockFileEx(owner.file.handle, 0, 1, 0, &owner.unlockRequest); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
		owner.locked, owner.pending = false, false
	}
	if err := owner.file.Close(); err != nil {
		return err
	}
	for index := len(owner.parents.directories) - 1; index >= 0; index-- {
		if err := owner.parents.directories[index].Close(); err != nil {
			return err
		}
	}
	return nil
}
