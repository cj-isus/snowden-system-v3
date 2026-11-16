//go:build windows

package update

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Same FILE_RENAME_INFORMATION ABI as the existing installer; only this fixed
// control-file publication permits replacement. No Ex/POSIX/path fallback.
type floorRenameInformation struct {
	ReplaceIfExists bool
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [64]uint16
}

func writeFloorFile(path string, data []byte) (finalErr error) {
	owner := floorNativeOwner
	if owner == nil || owner.path != path || owner.writeErr != nil || len(data) == 0 || len(data) > 64 || owner.Validate() != nil {
		return ErrFloorUnavailable
	}
	owner.writeErr = ErrFloorUnavailable // sticky until the complete native publication/close is confirmed
	defer func() {
		if recover() != nil {
			finalErr = errors.Join(finalErr, ErrFloorUnavailable)
		}
	}()
	version, err := ParseVersion(string(data))
	if err != nil || version.String() != string(data) {
		return ErrFloorUnavailable
	}
	if err := owner.createTemporary(); err != nil {
		return err
	}
	var written uint32
	if err := windows.WriteFile(owner.temporary.handle, data, &written, nil); err != nil || written != uint32(len(data)) {
		return errors.Join(ErrFloorUnavailable, io.ErrShortWrite, err)
	}
	if err := windows.FlushFileBuffers(owner.temporary.handle); err != nil {
		return errors.Join(ErrFloorUnavailable, err)
	}
	info, err := owner.readInfo(&owner.temporary)
	if err != nil || info.FileSizeHigh != 0 || info.FileSizeLow != uint32(len(data)) || owner.validateFile(&owner.temporary) != nil {
		return errors.Join(ErrFloorUnavailable, err)
	}
	current, err := ReadFloor() // fresh file+registry maximum while retaining the real writer lock
	if err != nil {
		return err
	}
	if current.Compare(version) >= 0 {
		owner.writeErr = nil
		return nil // original temporary is deleted by checked owner cleanup
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	name, err := windows.UTF16FromString(filepath.Base(path))
	if err != nil || len(name) > 64 {
		return ErrFloorUnavailable
	}
	rename := floorRenameInformation{ReplaceIfExists: true, RootDirectory: owner.parent(), FileNameLength: uint32(2 * (len(name) - 1))}
	copy(rename.FileName[:], name)
	var iosb windows.IO_STATUS_BLOCK
	owner.tempKeep = true // a failed native call does not prove the canonical name stayed unchanged
	err = windows.NtSetInformationFile(owner.temporary.handle, &iosb, (*byte)(unsafe.Pointer(&rename)),
		uint32(unsafe.Offsetof(rename.FileName))+rename.FileNameLength, windows.FileRenameInformation)
	actual, lookupErr := floorNativePath(owner.temporary.handle)
	if lookupErr != nil || !strings.EqualFold(actual, path) && !strings.EqualFold(actual, owner.temporary.path) {
		owner.cleanupErr = ErrFloorUnavailable // retain exact handles/lock; no cleanup at a guessed location
		return errors.Join(owner.cleanupErr, err, lookupErr)
	}
	if strings.EqualFold(actual, owner.temporary.path) {
		owner.tempKeep = false // exact original object remains at its private temporary name
		return errors.Join(ErrFloorUnavailable, err)
	}
	owner.temporary.path = path
	if err != nil || owner.validateFile(&owner.temporary) != nil || owner.Validate() != nil {
		return errors.Join(ErrFloorUnavailable, err)
	}
	// Retire the original WRITE access before the checked readback opens it.
	if err := owner.temporary.Close(); err != nil {
		return err
	}
	owner.writeErr = nil
	return nil
}

func (owner *floorNativeCustody) createTemporary() error {
	if owner.temporary.handle != 0 || owner.temporary.path != "" || owner.tempOwned || owner.tempKeep {
		return ErrFloorUnavailable
	}
	for attempt := 0; attempt < 16; attempt++ {
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		name := ".update-floor-" + hex.EncodeToString(nonce[:])
		owner.temporary = floorNativeFile{path: filepath.Join(filepath.Dir(owner.path), name)}
		created, err := owner.openFile(&owner.temporary, owner.parent(), name, windows.FILE_CREATE, true)
		owner.tempOwned = created
		if errors.Is(err, os.ErrExist) && !created && owner.temporary.handle == 0 {
			owner.temporary = floorNativeFile{} // only a confirmed no-create collision
			continue
		}
		if err != nil || !created || owner.temporary.identity.FileSizeHigh != 0 || owner.temporary.identity.FileSizeLow != 0 {
			return errors.Join(ErrFloorUnavailable, err)
		}
		return nil
	}
	return errors.Join(ErrFloorUnavailable, os.ErrExist)
}

func (owner *floorNativeCustody) retireTemporary() error {
	file := &owner.temporary
	if file.closeErr != nil {
		return file.closeErr
	}
	if file.handle == 0 {
		return nil
	}
	if owner.tempOwned && !owner.tempKeep {
		if err := owner.Validate(); err != nil {
			owner.cleanupErr = err
			return err
		}
		if err := owner.validateFile(file); err != nil {
			owner.cleanupErr = err
			return err
		}
		// Delete only this original, still-private object; never remove by path.
		var iosb windows.IO_STATUS_BLOCK
		disposition := byte(1) // FILE_DISPOSITION_INFORMATION.DeleteFile
		owner.cleanupErr = ErrFloorUnavailable
		if err := windows.NtSetInformationFile(file.handle, &iosb, &disposition, 1, windows.FileDispositionInformation); err != nil {
			return errors.Join(owner.cleanupErr, err)
		}
		owner.cleanupErr = nil
	}
	return file.Close()
}
