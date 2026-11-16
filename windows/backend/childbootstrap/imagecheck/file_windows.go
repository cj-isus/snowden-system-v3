//go:build windows

package imagecheck

import (
	"errors"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// FileIdentity inspects an original held image/installer file. The caller must
// retain the ACL slot before this call and retire it before closing the file.
// This function acquires no file handle, changes no ownership and grants no
// signing or effective-access authority. trusted follows fileacl's pure,
// transient SID contract. Native metadata writes into the caller's original
// snapshot; facts observed with an error must not authorize use.
func FileIdentity(handle windows.Handle, expected string, directory bool, acl *fileacl.Slot, trusted func(*windows.SID) bool, info *windows.ByHandleFileInformation) error {
	if !nativelease.ValidHandleValue(handle) || acl == nil || trusted == nil || info == nil {
		return ErrImage
	}
	kind, err := windows.GetFileType(handle)
	if err != nil || kind != windows.FILE_TYPE_DISK || windows.GetFileInformationByHandle(handle, info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		(info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory {
		return ErrImage
	}
	path, err := FilePath(handle)
	if err != nil || !strings.EqualFold(path, expected) {
		return ErrImage
	}
	if info.FileIndexHigh == 0 && info.FileIndexLow == 0 || !directory && info.NumberOfLinks != 1 {
		return ErrImage
	}
	if err := acl.Validate(handle, directory, trusted); err != nil {
		return errors.Join(ErrImage, err) // retain the original typed cleanup cause
	}
	return nil
}

func SameFileID(a, b windows.ByHandleFileInformation) bool {
	return (a.FileIndexHigh != 0 || a.FileIndexLow != 0) &&
		a.VolumeSerialNumber == b.VolumeSerialNumber && a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
}

// SameSnapshot compares independently inspected original/current metadata.
// Directory entry changes are allowed; regular file size/time/link changes are not.
func SameSnapshot(original, current windows.ByHandleFileInformation, directory bool) bool {
	return SameFileID(original, current) && (directory ||
		original.FileSizeHigh == current.FileSizeHigh && original.FileSizeLow == current.FileSizeLow &&
			original.LastWriteTime == current.LastWriteTime && original.NumberOfLinks == current.NumberOfLinks)
}
