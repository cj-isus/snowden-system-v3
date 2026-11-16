//go:build windows

package update

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"golang.org/x/sys/windows"
)

// floorMu serializes this owner, including the registry/startup consumers.
// Unknown native retirement quarantines the original owner until process exit.
var floorNativeOwner *floorNativeCustody

type floorNativeCustody struct {
	directories                     []floorNativeFile
	lock                            floorNativeFile
	temporary                       floorNativeFile
	path                            string
	trust                           fileacl.Trust
	locked                          bool
	tempOwned                       bool
	tempKeep                        bool // rename may already have published the canonical floor
	unlockErr, cleanupErr, writeErr error
	releaseAttempted                bool
	releaseErr                      error
}

func releaseFloorLock() (finalErr error) {
	if floorNativeOwner == nil {
		return nil
	}
	return floorNativeOwner.Release()
}

func (owner *floorNativeCustody) Release() (finalErr error) {
	if owner.releaseAttempted {
		return owner.releaseErr
	}
	owner.releaseAttempted, owner.releaseErr = true, ErrFloorUnavailable
	defer func() {
		if recover() != nil {
			finalErr = errors.Join(finalErr, ErrFloorUnavailable)
		}
		owner.releaseErr = finalErr
	}()
	if floorNativeOwner != owner {
		return ErrFloorUnavailable // a stale release never closes a newer original owner
	}
	if err := owner.Close(); err != nil {
		return err
	}
	floorNativeOwner = nil
	return nil
}

func lockFloor(path string) (release func() error, finalErr error) {
	defer func() {
		if recover() != nil {
			finalErr = errors.Join(finalErr, ErrFloorUnavailable)
		}
	}()
	if err := releaseFloorLock(); err != nil {
		return release, err
	}
	expected, err := FloorPath()
	if err != nil || path != expected+".lock" {
		return release, ErrFloorUnavailable
	}
	owner := &floorNativeCustody{path: expected, directories: make([]floorNativeFile, 0, 64)}
	floorNativeOwner = owner // original owner before the first acquisition
	release = owner.Release  // bind cleanup to this exact owner, never the next global value
	return release, owner.open(path)
}

func (owner *floorNativeCustody) trusted(sid *windows.SID) bool {
	return owner.trust.Trusted(sid)
}

func (owner *floorNativeCustody) open(lockPath string) error {
	var err error
	owner.trust, err = fileacl.CurrentTrust()
	if err != nil {
		return ErrFloorUnavailable
	}
	dir := filepath.Dir(owner.path)
	volume := filepath.VolumeName(dir)
	if len(dir) > 2048 || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || len(volume) != 2 || volume[1] != ':' ||
		!(volume[0] >= 'a' && volume[0] <= 'z' || volume[0] >= 'A' && volume[0] <= 'Z') {
		return ErrFloorUnavailable // local drive custody; no UNC/device/relative fallback
	}
	root := volume + `\`
	parts := strings.Split(strings.TrimPrefix(dir, root), `\`)
	if len(parts) < 2 || len(parts) >= 64 || parts[len(parts)-2] != "snowden-system" || parts[len(parts)-1] != "update" {
		return ErrFloorUnavailable
	}
	for _, part := range parts {
		if !floorNativeComponent(part) {
			return ErrFloorUnavailable
		}
	}
	owner.directories = append(owner.directories, floorNativeFile{path: root, directory: true})
	if _, err := owner.openFile(&owner.directories[0], 0, `\??\`+root, windows.FILE_OPEN, false); err != nil {
		return err
	}
	for index, part := range parts {
		parentIndex := len(owner.directories) - 1
		path := filepath.Join(owner.directories[parentIndex].path, part)
		owner.directories = append(owner.directories, floorNativeFile{path: path, directory: true})
		disposition := uint32(windows.FILE_OPEN)
		if index >= len(parts)-2 {
			disposition = windows.FILE_OPEN_IF
		} // only fixed app directories may be created
		if _, err := owner.openFile(&owner.directories[parentIndex+1], owner.directories[parentIndex].handle, part, disposition, false); err != nil {
			return err
		}
	}
	owner.lock.path = lockPath // reserved native output before open/create
	if _, err := owner.openFile(&owner.lock, owner.parent(), filepath.Base(lockPath), windows.FILE_OPEN_IF, false); err != nil {
		return err
	}
	if owner.lock.identity.FileSizeHigh != 0 || owner.lock.identity.FileSizeLow != 0 {
		return ErrFloorUnavailable
	}
	if err := windows.LockFileEx(owner.lock.handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		return errors.Join(ErrFloorUnavailable, err)
	}
	owner.locked = true
	return owner.Validate()
}

func (owner *floorNativeCustody) parent() windows.Handle {
	if len(owner.directories) == 0 {
		return 0
	}
	return owner.directories[len(owner.directories)-1].handle
}

func (owner *floorNativeCustody) Validate() error {
	if !owner.locked || owner.unlockErr != nil || owner.cleanupErr != nil || owner.parent() == 0 {
		return ErrFloorUnavailable
	}
	for index := range owner.directories {
		if err := owner.validateFile(&owner.directories[index]); err != nil {
			return err
		}
	}
	if err := owner.validateFile(&owner.lock); err != nil {
		return err
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(owner.lock.handle, &info) != nil || info.FileSizeHigh != 0 || info.FileSizeLow != 0 {
		return ErrFloorUnavailable
	}
	return nil
}

func (owner *floorNativeCustody) Close() error {
	if owner.cleanupErr != nil || owner.unlockErr != nil {
		return errors.Join(ErrFloorUnavailable, owner.cleanupErr, owner.unlockErr)
	}
	if err := owner.retireTemporary(); err != nil {
		return err
	} // retain lock/ancestors under uncertain writer
	if owner.locked {
		owner.unlockErr = ErrFloorUnavailable
		if err := windows.UnlockFileEx(owner.lock.handle, 0, 1, 0, &windows.Overlapped{}); err != nil {
			return errors.Join(owner.unlockErr, err)
		}
		owner.locked, owner.unlockErr = false, nil
	}
	if err := owner.lock.Close(); err != nil {
		return err
	}
	for index := len(owner.directories) - 1; index >= 0; index-- {
		if err := owner.directories[index].Close(); err != nil {
			return err
		}
	}
	return nil
}
