//go:build windows

package delivery

import (
	"errors"
	"os"
	"unsafe"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

type workspaceFile struct {
	handle             windows.Handle
	path               string
	directory, created bool
	identity           windows.ByHandleFileInformation
	acl                *filetxn.WindowsFileACLSlot
	closeAttempted     bool
	closeErr           error
}

func (owner *nativeWorkspace) open(file *workspaceFile, parent windows.Handle, name string, create, write, remove bool) error {
	if file == nil || len(file.path) == 0 || len(file.path) > 2048 || file.handle != 0 || file.closeAttempted || owner.badErr != nil {
		return ErrTrust
	}
	text, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return ErrTrust
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: text, Attributes: windows.OBJ_CASE_INSENSITIVE}
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE)
	sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE)
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	attributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if file.directory {
		access |= windows.FILE_TRAVERSE
		options |= windows.FILE_DIRECTORY_FILE
		attributes = windows.FILE_ATTRIBUTE_DIRECTORY
		sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE // prevent directory relocation while SDK uses paths
		if create || remove {
			access |= windows.FILE_LIST_DIRECTORY
		}
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
	}
	if write {
		access |= windows.GENERIC_WRITE
	}
	if remove || create {
		access |= windows.DELETE
	}
	disposition := uint32(windows.FILE_OPEN)
	if create {
		disposition = windows.FILE_CREATE
	}
	var iosb windows.IO_STATUS_BLOCK
	owner.badErr = ErrTrust // panic/partial output stays owned before creation is classified
	err = windows.NtCreateFile(&file.handle, access, &oa, &iosb, nil, attributes, sharing, disposition, options, 0, 0)
	if file.handle == windows.InvalidHandle {
		file.handle = 0
	}
	file.created = create && iosb.Information == 2
	if create && errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) && file.handle == 0 && !file.created {
		owner.badErr = nil
		return os.ErrExist
	}
	if !create && file.handle == 0 && (errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) ||
		errors.Is(err, windows.STATUS_OBJECT_PATH_NOT_FOUND) || errors.Is(err, windows.STATUS_NO_SUCH_FILE)) {
		owner.badErr = nil
		return os.ErrNotExist
	}
	if err != nil || file.handle == 0 || create && !file.created || !create && iosb.Information != 1 {
		owner.badErr = ErrTrust // unknown acquisition cannot be repaired by guessing paths
		return errors.Join(ErrTrust, err)
	}
	file.acl = new(filetxn.WindowsFileACLSlot) // published before the descriptor query
	file.identity, err = filetxn.InspectWindowsFileWithACL(file.acl, file.handle, file.path, file.directory, MaxMetadataBytes, owner.trusted)
	if err != nil {
		owner.badErr = ErrTrust
		return ErrTrust
	}
	owner.badErr = nil
	return nil
}

func (owner *nativeWorkspace) validate(file *workspaceFile) error {
	if file == nil || file.handle == 0 || file.closeAttempted || file.acl == nil || owner.badErr != nil {
		return ErrTrust
	}
	info, err := filetxn.InspectWindowsFileWithACL(file.acl, file.handle, file.path, file.directory, MaxMetadataBytes, owner.trusted)
	if err != nil || info.VolumeSerialNumber != file.identity.VolumeSerialNumber ||
		info.FileIndexHigh != file.identity.FileIndexHigh || info.FileIndexLow != file.identity.FileIndexLow {
		return ErrTrust
	}
	return nil
}

func (file *workspaceFile) Close() (finalErr error) {
	if file.closeAttempted {
		return file.closeErr
	}
	file.closeAttempted, file.closeErr = true, ErrTrust
	defer func() {
		if recover() != nil {
			finalErr = ErrTrust
		}
		file.closeErr = finalErr
	}()
	if file.acl != nil {
		if err := file.acl.Close(); err != nil {
			return errors.Join(ErrTrust, err)
		}
	}
	if file.handle != 0 {
		if err := windows.CloseHandle(file.handle); err != nil {
			return ErrTrust
		}
		file.handle = 0
	}
	return nil
}
