//go:build windows

package filetxn

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Retained before the original relative FILE_OPEN. The later absence probe has
// its own output slot; it cannot overwrite a failed delete/Close receipt.
type nativeFileDeletion struct {
	file, absence                  boundedNativeWriteFile
	attempted, prepared, uncertain bool
}

func (deletion *nativeFileDeletion) Uncertain() bool {
	return deletion.uncertain || deletion.file.acquiring || deletion.file.ioPending || deletion.file.closeErr != nil ||
		deletion.absence.acquiring || deletion.absence.ioPending || deletion.absence.closeErr != nil
}

func (owner *nativeJournalDirectory) RemoveFile(name string, size int, sha string) error {
	if size < 0 || size > maxFileSize || owner.deleting != nil {
		return ErrStagingCleanup
	}
	if _, err := owner.Inspect(owner.file.path); err != nil {
		return err
	}
	owner.deleting = &nativeFileDeletion{}
	err := owner.deleting.erase(&owner.parents, &owner.file, name, &journalCleanupFile{name: name, size: size, sha: sha})
	if !owner.deleting.Uncertain() {
		owner.deleting = nil
	}
	return err
}

// expected is required for journal debris; nil is reserved for restoring the
// absence of a caller-fixed target after the complete old set authenticates.
// Neither mode trusts paths chosen by a journal or follows reparse/hard links.
func (deletion *nativeFileDeletion) erase(parents *boundedNativeWrite, parent *boundedNativeWriteFile, name string, expected *journalCleanupFile) (finalErr error) {
	if deletion.attempted {
		return ErrStagingCleanup
	}
	defer func() {
		if recover() != nil {
			finalErr = ErrStagingCleanup
		}
		finalErr = errors.Join(finalErr, deletion.close())
	}()
	if err := deletion.prepare(parents, parent, name, expected); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // unchanged journal cleanup semantics for exact native absence
		}
		return err
	}
	return deletion.commit(parents, parent, name)
}

// prepare retains the original deny-write/delete handle before any caller work.
// Only the enclosing original owner may retire it or commit its disposition.
func (deletion *nativeFileDeletion) prepare(parents *boundedNativeWrite, parent *boundedNativeWriteFile, name string, expected *journalCleanupFile) error {
	if deletion.attempted || !windowsReadComponent(name) || expected != nil &&
		(expected.name != name || expected.size < 0 || expected.size > maxFileSize || len(expected.sha) != 64) {
		return ErrStagingCleanup
	}
	deletion.attempted = true
	deletion.file = boundedNativeWriteFile{path: filepath.Join(parent.path, name), deleteRead: true}
	file := &deletion.file
	if err := parents.validateDirectories(); err != nil {
		return err
	}
	if _, err := parents.inspect(parent); err != nil {
		return err
	}
	if err := parents.open(file, parent.handle, name, windows.FILE_OPEN); err != nil {
		return err
	}
	if expected != nil {
		if err := deletion.match(expected); err != nil {
			return err
		}
	}
	deletion.prepared = true
	return nil
}

func (deletion *nativeFileDeletion) commit(parents *boundedNativeWrite, parent *boundedNativeWriteFile, name string) error {
	file := &deletion.file
	if !deletion.prepared || deletion.Uncertain() || !windowsReadComponent(name) || file.path != filepath.Join(parent.path, name) {
		return ErrStagingCleanup
	}
	initial := file.identity
	current, err := parents.inspect(file)
	if err != nil || current.FileSizeHigh != initial.FileSizeHigh || current.FileSizeLow != initial.FileSizeLow || current.LastWriteTime != initial.LastWriteTime {
		return errors.Join(ErrStagingCleanup, err)
	}
	if err := parents.validateDirectories(); err != nil {
		return err
	}
	if _, err := parents.inspect(parent); err != nil {
		return err
	}
	deletion.uncertain = true
	var iosb windows.IO_STATUS_BLOCK
	disposition := byte(1)
	if err := windows.NtSetInformationFile(file.handle, &iosb, &disposition, 1, windows.FileDispositionInformation); err != nil {
		return errors.Join(ErrStagingCleanup, err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	deletion.uncertain = false
	deletion.prepared = false
	return deletion.observeAbsence(parents, parent, name)
}

func (deletion *nativeFileDeletion) observeAbsence(parents *boundedNativeWrite, parent *boundedNativeWriteFile, name string) error {
	if deletion.Uncertain() || !windowsReadComponent(name) || deletion.absence.handle != 0 || deletion.absence.closeAttempted {
		return ErrStagingCleanup
	}
	if err := parents.validateDirectories(); err != nil {
		return err
	}
	if _, err := parents.inspect(parent); err != nil {
		return err
	}
	// Accepted disposition/our Close alone need not mean the name is absent:
	// other handles can delay deletion, or a competing writer can recreate it.
	// Observe through the same original parent; never delete that replacement.
	deletion.absence = boundedNativeWriteFile{path: filepath.Join(parent.path, name), inspectOnly: true}
	err := parents.open(&deletion.absence, parent.handle, name, windows.FILE_OPEN)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return errors.Join(ErrStagingCleanup, err)
}

func (deletion *nativeFileDeletion) close() error {
	if deletion.Uncertain() {
		return ErrWriteRetirement // retain both original outputs; never retry unknown I/O
	}
	if err := deletion.absence.Close(); err != nil {
		return err
	}
	return deletion.file.Close()
}

// Match cleanup bytes while the original file disallows write/delete sharing.
// The digest identifies recorded debris; it is not profile signing authority.
func (deletion *nativeFileDeletion) match(expected *journalCleanupFile) error {
	file := &deletion.file
	if file.identity.FileSizeHigh != 0 || file.identity.FileSizeLow != uint32(expected.size) {
		return ErrStagingCleanup
	}
	hash := sha256.New()
	var chunk [32 * 1024]byte
	remaining := expected.size
	for remaining > 0 {
		length := min(remaining, len(chunk))
		var done uint32
		file.ioPending = true
		err := windows.ReadFile(file.handle, chunk[:length], &done, nil)
		file.ioPending = false
		if err != nil || done == 0 || done > uint32(length) {
			return errors.Join(ErrStagingCleanup, io.ErrUnexpectedEOF, err)
		}
		_, _ = hash.Write(chunk[:done])
		remaining -= int(done)
	}
	var extra uint32
	file.ioPending = true
	err := windows.ReadFile(file.handle, chunk[:1], &extra, nil)
	file.ioPending = false
	if extra != 0 || err != nil && !errors.Is(err, windows.ERROR_HANDLE_EOF) || hex.EncodeToString(hash.Sum(nil)) != expected.sha {
		return errors.Join(ErrStagingCleanup, err)
	}
	return nil
}
