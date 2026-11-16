//go:build windows

package filetxn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func windowsReadPath(path string) (absolute, native string, finalErr error) {
	if path == "" || len(path) > 2048 || strings.ContainsRune(path, 0) {
		return "", "", ErrWindowsRead
	}
	path = filepath.FromSlash(path)
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\??\`) {
		return "", "", ErrWindowsRead
	}
	absolute, err := filepath.Abs(path)
	if err != nil || len(absolute) > 2048 || !filepath.IsAbs(absolute) {
		return "", "", ErrWindowsRead
	}
	volume := filepath.VolumeName(absolute)
	if strings.HasPrefix(volume, `\\`) {
		parts := strings.Split(strings.TrimPrefix(volume, `\\`), `\`)
		if len(parts) != 2 || !windowsReadComponent(parts[0]) || !windowsReadComponent(parts[1]) {
			return "", "", ErrWindowsRead
		}
		native = `\??\UNC\` + strings.TrimPrefix(absolute, `\\`)
	} else {
		if len(volume) != 2 || volume[1] != ':' || !(volume[0] >= 'a' && volume[0] <= 'z' || volume[0] >= 'A' && volume[0] <= 'Z') {
			return "", "", ErrWindowsRead
		}
		native = `\??\` + absolute
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(absolute, volume), `\`)
	if relative != "" {
		for _, component := range strings.Split(relative, `\`) {
			if !windowsReadComponent(component) {
				return "", "", ErrWindowsRead
			}
		}
	}
	return absolute, native, nil
}

func windowsReadComponent(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		!strings.ContainsAny(name, "\\/:*?\"<>|\x00") && !strings.HasSuffix(name, ".") && !strings.HasSuffix(name, " ")
}

func (file *WindowsRead) trusted(sid *windows.SID) bool {
	return file.trust.Trusted(sid)
}

func (file *WindowsRead) readInfo() (info windows.ByHandleFileInformation, finalErr error) {
	if file.selectedInput {
		return inspectWindowsFile(file.handle, file.path, file.directory, file.limit, nil, inspectSelectedInput)
	}
	if file.acl == nil {
		file.acl = new(WindowsFileACLSlot)
	}
	return InspectWindowsFileWithACL(file.acl, file.handle, file.path, file.directory, file.limit, file.trusted)
}

// InspectWindowsFile inspects an already-owned handle; it acquires/releases no
// file handle and grants no ownership. Callers retain their original receipt.
func InspectWindowsFile(handle windows.Handle, pathName string, directory bool, limit int64, trusted func(*windows.SID) bool) (info windows.ByHandleFileInformation, finalErr error) {
	return inspectWindowsFile(handle, pathName, directory, limit, trusted, inspectStoredFile)
}

// InspectWindowsFileWithACL retains descriptor custody in the caller's per-file
// slot. The caller owns the file handle and retires slot before closing that file.
func InspectWindowsFileWithACL(slot *WindowsFileACLSlot, handle windows.Handle, pathName string, directory bool, limit int64, trusted func(*windows.SID) bool) (windows.ByHandleFileInformation, error) {
	return inspectWindowsFileUsingACL(slot, handle, pathName, directory, limit, trusted, inspectStoredFile)
}

type windowsFileInspection uint8

const (
	inspectStoredFile windowsFileInspection = iota
	inspectSelectedInput
	inspectSelectedOutput
)

// The public entrypoint always uses the trusted-store policy. Only the private
// selected input/output may omit that policy; neither grants local trust. Output
// permits held parent directories but keeps single-link checks on created files.
func inspectWindowsFile(handle windows.Handle, pathName string, directory bool, limit int64, trusted func(*windows.SID) bool, use windowsFileInspection) (info windows.ByHandleFileInformation, finalErr error) {
	return inspectWindowsFileUsingACL(&standaloneWindowsFileACL, handle, pathName, directory, limit, trusted, use)
}

func inspectWindowsFileUsingACL(slot *WindowsFileACLSlot, handle windows.Handle, pathName string, directory bool, limit int64, trusted func(*windows.SID) bool, use windowsFileInspection) (info windows.ByHandleFileInformation, finalErr error) {
	if handle == 0 || handle == windows.InvalidHandle || limit < 0 ||
		use > inspectSelectedOutput || (use == inspectStoredFile && trusted == nil) || (use == inspectSelectedInput && directory) {
		return info, ErrWindowsRead
	}
	kind, err := windows.GetFileType(handle)
	if err != nil || kind != windows.FILE_TYPE_DISK || windows.GetFileInformationByHandle(handle, &info) != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory ||
		info.FileIndexHigh == 0 && info.FileIndexLow == 0 {
		return info, ErrWindowsRead
	}
	if !directory {
		if info.NumberOfLinks == 0 || (use != inspectSelectedInput && info.NumberOfLinks != 1) || info.FileSizeHigh > 0x7fffffff {
			return info, ErrWindowsRead
		}
		if int64(uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow)) > limit {
			return info, errors.Join(ErrWindowsRead, ErrReadTooLarge)
		}
	}
	var path [2049]uint16
	n, err := windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), 0)
	if err != nil || n == 0 || n >= uint32(len(path)) {
		return info, ErrWindowsRead
	}
	finalPath := windows.UTF16ToString(path[:n])
	if strings.HasPrefix(finalPath, `\\?\UNC\`) {
		finalPath = `\\` + strings.TrimPrefix(finalPath, `\\?\UNC\`)
	} else {
		finalPath = strings.TrimPrefix(finalPath, `\\?\`)
	}
	if !strings.EqualFold(finalPath, pathName) {
		return info, ErrWindowsRead
	}
	if use == inspectStoredFile {
		if err := ValidateWindowsFileACLOwned(slot, handle, directory, trusted); err != nil {
			return info, errors.Join(ErrWindowsRead, err)
		}
	}
	return info, nil
}

// These immutable snapshots carry eager original-handle IDs. os.SameFile accepts
// only the standard library's private FileInfo type; never feed it this type or
// fall back to reopening a path after a transaction has moved it.
type windowsReadInfo struct {
	path   string
	native windows.ByHandleFileInformation
}

func (info *windowsReadInfo) Name() string { return filepath.Base(info.path) }
func (info *windowsReadInfo) Size() int64 {
	return int64(uint64(info.native.FileSizeHigh)<<32 | uint64(info.native.FileSizeLow))
}
func (info *windowsReadInfo) Mode() os.FileMode {
	mode := os.FileMode(0666)
	if info.native.FileAttributes&windows.FILE_ATTRIBUTE_READONLY != 0 {
		mode = 0444
	}
	if info.IsDir() {
		mode |= os.ModeDir | 0111
	}
	return mode
}
func (info *windowsReadInfo) ModTime() time.Time {
	return time.Unix(0, info.native.LastWriteTime.Nanoseconds())
}
func (info *windowsReadInfo) IsDir() bool {
	return info.native.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
}
func (info *windowsReadInfo) Sys() any { return info.native }

func sameWindowsReadID(a, b windows.ByHandleFileInformation) bool {
	return (a.FileIndexHigh != 0 || a.FileIndexLow != 0) && a.VolumeSerialNumber == b.VolumeSerialNumber &&
		a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
}

// SameWindowsFile compares only snapshots produced by this reader.
func SameWindowsFile(a, b os.FileInfo) bool {
	x, xOK := a.(*windowsReadInfo)
	y, yOK := b.(*windowsReadInfo)
	return xOK && yOK && x != nil && y != nil && sameWindowsReadID(x.native, y.native)
}

// MatchesWindowsFileID binds a reader snapshot to an original creator receipt.
func MatchesWindowsFileID(info os.FileInfo, volume, indexHigh, indexLow uint32) bool {
	snapshot, ok := info.(*windowsReadInfo)
	return ok && snapshot != nil && sameWindowsReadID(snapshot.native, windows.ByHandleFileInformation{
		VolumeSerialNumber: volume, FileIndexHigh: indexHigh, FileIndexLow: indexLow,
	})
}
