//go:build windows

package filetxn

import (
	"errors"
	"io"
	"os"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"golang.org/x/sys/windows"
)

var ErrWindowsRead = errors.New("filetxn: Windows reader unavailable or ownership unconfirmed")

// WindowsRead is a single, caller-serialized original handle owner. Retain it
// before Open, including on failure, and release it only after confirmed Close.
// There is no os.File conversion/finalizer and no retry after uncertain close.
type WindowsRead struct {
	path              string
	directory         bool
	selectedInput     bool
	limit, consumed   int64
	handle            windows.Handle
	identity          windows.ByHandleFileInformation
	acl               *WindowsFileACLSlot
	trust             fileacl.Trust
	attempted, ready  bool
	acquiring         bool
	closeAttempted    bool
	readErr, closeErr error
}

// NewWindowsRead allocates a receipt only; it acquires no native resource.
// limit includes any caller's extra byte used to detect oversized input.
func NewWindowsRead(path string, directory bool, limit int64) *WindowsRead {
	return &WindowsRead{path: path, directory: directory, limit: limit}
}

func (file *WindowsRead) Open() (finalErr error) {
	if file == nil || file.attempted || file.closeAttempted {
		return ErrWindowsRead
	}
	file.attempted = true
	defer func() {
		if recover() != nil {
			file.ready = false
			finalErr = ErrWindowsRead
		}
	}()
	if file.limit < 0 || !file.directory && file.limit == 0 || file.selectedInput && file.directory {
		return ErrWindowsRead
	}
	path, nativePath, err := windowsReadPath(file.path)
	if err != nil {
		return err
	}
	file.path = path
	if !file.selectedInput {
		file.trust, err = fileacl.CurrentTrust()
		if err != nil {
			return ErrWindowsRead
		}
	}
	name, err := windows.NewNTUnicodeString(nativePath)
	if err != nil {
		return ErrWindowsRead
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	access := uint32(windows.READ_CONTROL | windows.SYNCHRONIZE | windows.GENERIC_READ)
	sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE)
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_NON_DIRECTORY_FILE)
	if file.selectedInput {
		// GENERIC_READ also requests READ_CONTROL. Untrusted public input
		// needs data/attributes only; the OS still enforces these access rights.
		access = windows.FILE_READ_DATA | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE
		sharing = windows.FILE_SHARE_READ
	}
	if file.directory {
		access = windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE
		sharing |= windows.FILE_SHARE_WRITE
		options = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DIRECTORY_FILE
	}
	var iosb windows.IO_STATUS_BLOCK
	file.acquiring = true
	err = windows.NtCreateFile(&file.handle, access, &oa, &iosb, nil, 0, sharing, windows.FILE_OPEN, options, 0, 0)
	file.acquiring = false
	if file.handle == windows.InvalidHandle {
		file.handle = 0
	}
	if file.handle == 0 && (errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) ||
		errors.Is(err, windows.STATUS_OBJECT_PATH_NOT_FOUND) || errors.Is(err, windows.STATUS_NO_SUCH_FILE)) {
		// FILE_OPEN cannot create. On failure Information is not necessarily
		// zero (FILE_DOES_NOT_EXIST is also allowed); use the exact status and
		// absence of an original handle, never a cleanup result.
		return os.ErrNotExist
	}
	if err != nil || file.handle == 0 || iosb.Information != 1 { // FILE_OPENED
		return errors.Join(ErrWindowsRead, err)
	}
	file.identity, err = file.readInfo()
	if err != nil {
		return err
	}
	file.ready = true
	return nil
}

func (file *WindowsRead) Read(raw []byte) (n int, finalErr error) {
	if file == nil || !file.ready || file.directory || file.closeAttempted || file.handle == 0 {
		return 0, ErrWindowsRead
	}
	if file.readErr != nil {
		return 0, file.readErr
	}
	if len(raw) == 0 {
		return 0, nil
	}
	if file.consumed >= file.limit {
		return 0, ErrWindowsRead // callers must reserve their own overflow byte
	}
	if len(raw) > 32*1024 {
		raw = raw[:32*1024]
	}
	if int64(len(raw)) > file.limit-file.consumed {
		raw = raw[:int(file.limit-file.consumed)]
	}
	file.readErr = ErrWindowsRead // retain uncertainty even if the native call panics
	defer func() {
		if recover() != nil {
			n, finalErr = 0, ErrWindowsRead
		}
	}()
	var done uint32
	err := windows.ReadFile(file.handle, raw, &done, nil)
	if done > uint32(len(raw)) {
		return 0, file.readErr
	}
	file.consumed += int64(done)
	if err != nil && !errors.Is(err, windows.ERROR_HANDLE_EOF) {
		file.readErr = errors.Join(ErrWindowsRead, err)
		return int(done), file.readErr
	}
	file.readErr = nil
	if done == 0 || errors.Is(err, windows.ERROR_HANDLE_EOF) {
		return int(done), io.EOF
	}
	return int(done), nil
}

func (file *WindowsRead) Stat() (info os.FileInfo, finalErr error) {
	if file == nil || !file.ready || file.closeAttempted || file.handle == 0 || file.readErr != nil {
		return nil, ErrWindowsRead
	}
	defer func() {
		if recover() != nil {
			info, finalErr = nil, ErrWindowsRead
		}
	}()
	current, err := file.readInfo()
	if err != nil || !sameWindowsReadID(current, file.identity) {
		return nil, errors.Join(ErrWindowsRead, err)
	}
	return &windowsReadInfo{path: file.path, native: current}, nil
}

func (file *WindowsRead) Close() (finalErr error) {
	if file == nil {
		return ErrWindowsRead
	}
	if file.closeAttempted {
		return file.closeErr
	}
	file.closeAttempted, file.closeErr, file.ready = true, ErrWindowsRead, false
	defer func() {
		if recover() != nil {
			finalErr = ErrWindowsRead
		}
		file.closeErr = finalErr
	}()
	if file.acquiring {
		return ErrWindowsRead // native acquisition panicked; keep the original receipt
	}
	if file.acl != nil {
		if err := file.acl.Close(); err != nil {
			return errors.Join(ErrWindowsRead, err)
		}
	}
	if file.handle != 0 {
		if err := windows.CloseHandle(file.handle); err != nil {
			return errors.Join(ErrWindowsRead, err)
		}
		file.handle = 0
	}
	return nil
}
