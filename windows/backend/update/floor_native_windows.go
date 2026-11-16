//go:build windows

package update

import (
	"errors"
	"os"
	"strings"
	"unsafe"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

type floorNativeFile struct {
	handle    windows.Handle
	path      string
	directory bool
	identity  windows.ByHandleFileInformation
	acl       *filetxn.WindowsFileACLSlot
	closeErr  error
}

func floorNativeComponent(name string) bool {
	return name != "" && len(name) <= 255 && name != "." && name != ".." &&
		!strings.ContainsAny(name, "\\/:*?\"<>|\x00") && !strings.HasSuffix(name, ".") && !strings.HasSuffix(name, " ")
}

// Native output goes directly to a pre-reserved original record. FILE_OPEN_IF
// is restricted to fixed app directories/lock; temporary data uses FILE_CREATE.
func (owner *floorNativeCustody) openFile(file *floorNativeFile, parent windows.Handle, name string, disposition uint32, temporary bool) (created bool, finalErr error) {
	if file.handle != 0 || file.closeErr != nil {
		return false, ErrFloorUnavailable
	}
	un, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return false, err
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: un, Attributes: windows.OBJ_CASE_INSENSITIVE}
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE)
	sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE) // no ancestor/lock deletion
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	attributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if file.directory {
		access |= windows.FILE_TRAVERSE
		options |= windows.FILE_DIRECTORY_FILE
		attributes = windows.FILE_ATTRIBUTE_DIRECTORY
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
		access |= windows.GENERIC_READ | windows.GENERIC_WRITE
		if temporary {
			access |= windows.DELETE
			sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE
		}
	}
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&file.handle, access, &oa, &iosb, nil, attributes, sharing, disposition, options, 0, 0)
	if file.handle == windows.InvalidHandle {
		file.handle = 0
	}
	created = iosb.Information == 2
	if temporary && errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) && file.handle == 0 && !created {
		return false, errors.Join(os.ErrExist, err)
	}
	if err != nil || file.handle == 0 || iosb.Information != 1 && iosb.Information != 2 || disposition == windows.FILE_CREATE && !created {
		return created, errors.Join(ErrFloorUnavailable, err)
	}
	file.identity, finalErr = owner.readInfo(file)
	return created, finalErr
}

func (owner *floorNativeCustody) readInfo(file *floorNativeFile) (info windows.ByHandleFileInformation, finalErr error) {
	if file.handle == 0 || file.closeErr != nil {
		return info, ErrFloorUnavailable
	}
	kind, err := windows.GetFileType(file.handle)
	if err != nil || kind != windows.FILE_TYPE_DISK || windows.GetFileInformationByHandle(file.handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != file.directory ||
		info.FileIndexHigh == 0 && info.FileIndexLow == 0 || !file.directory && info.NumberOfLinks != 1 {
		return info, ErrFloorUnavailable
	}
	path, err := floorNativePath(file.handle)
	if err != nil || !strings.EqualFold(path, file.path) {
		return info, ErrFloorUnavailable
	}
	var filesystem [16]uint16
	if file.acl == nil {
		file.acl = new(filetxn.WindowsFileACLSlot)
	}
	if windows.GetVolumeInformationByHandle(file.handle, nil, 0, nil, nil, nil, &filesystem[0], uint32(len(filesystem))) != nil ||
		!strings.EqualFold(windows.UTF16ToString(filesystem[:]), "NTFS") {
		return info, ErrFloorUnavailable
	}
	if err := filetxn.ValidateWindowsFileACLOwned(file.acl, file.handle, file.directory, owner.trusted); err != nil {
		return info, errors.Join(ErrFloorUnavailable, err)
	}
	return info, nil
}

func floorNativePath(handle windows.Handle) (string, error) {
	var path [2049]uint16
	n, err := windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), 0)
	if err != nil || n == 0 || n >= uint32(len(path)) {
		return "", ErrFloorUnavailable
	}
	return strings.TrimPrefix(windows.UTF16ToString(path[:n]), `\\?\`), nil
}

func (owner *floorNativeCustody) validateFile(file *floorNativeFile) error {
	info, err := owner.readInfo(file)
	if err != nil || info.VolumeSerialNumber != file.identity.VolumeSerialNumber ||
		info.FileIndexHigh != file.identity.FileIndexHigh || info.FileIndexLow != file.identity.FileIndexLow {
		return errors.Join(ErrFloorUnavailable, err)
	}
	return nil
}

func (file *floorNativeFile) Close() error {
	if file.closeErr != nil {
		return file.closeErr
	}
	if file.acl != nil {
		if err := file.acl.Close(); err != nil {
			file.closeErr = errors.Join(ErrFloorUnavailable, err)
			return file.closeErr
		}
	}
	if file.handle == 0 {
		return nil
	}
	file.closeErr = ErrFloorUnavailable
	if err := windows.CloseHandle(file.handle); err != nil {
		return errors.Join(file.closeErr, err)
	}
	file.handle, file.closeErr = 0, nil
	return nil
}
