//go:build windows

package filetxn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"golang.org/x/sys/windows"
)

const maxWriteDirectories = 64

type boundedNativeWriteFile struct {
	writerLock                               bool
	readOnlyLock                             bool
	inspectOnly                              bool
	deleteDirectory, deleteRead              bool
	handle                                   windows.Handle
	path                                     string
	directory, created, acquiring, ioPending bool
	identity                                 windows.ByHandleFileInformation
	acl                                      *WindowsFileACLSlot
	closeAttempted                           bool
	closeErr                                 error
}

type boundedNativeWrite struct {
	selectedContext                                 context.Context
	sharedDirectory                                 string
	newOnly                                         bool
	path                                            string
	limit                                           int64
	directories                                     []boundedNativeWriteFile
	temporary                                       boundedNativeWriteFile
	trust                                           fileacl.Trust
	attempted, published, uncertain, closeAttempted bool
	closeErr                                        error
}

// No native output or finalizer exists before the slot retains this record.
func newBoundedWriteFile(path string, limit int64, newOnly bool, directory string, selectedContext context.Context) boundedWriteFile {
	return &boundedNativeWrite{path: path, limit: limit, newOnly: newOnly, sharedDirectory: directory, selectedContext: selectedContext, directories: make([]boundedNativeWriteFile, 0, maxWriteDirectories)}
}

func (writer *boundedNativeWrite) publication() WritePublication {
	return WritePublication{Published: writer.published, MayHavePublished: writer.published || writer.uncertain}
}

func (writer *boundedNativeWrite) contextError() error {
	return selectedOutputContextError(writer.selectedContext)
}

func (writer *boundedNativeWrite) trusted(sid *windows.SID) bool {
	return writer.trust.Trusted(sid)
}

func (writer *boundedNativeWrite) initTrust() error {
	var err error
	writer.trust, err = fileacl.CurrentTrust()
	if err != nil {
		return ErrBoundedWrite
	}
	return nil
}

func (writer *boundedNativeWrite) open(file *boundedNativeWriteFile, parent windows.Handle, name string, disposition uint32) error {
	if file.handle != 0 || file.acquiring || file.closeAttempted {
		return ErrBoundedWrite
	}
	if writer.selectedContext != nil && (file.writerLock || file.deleteDirectory || file.deleteRead || file.inspectOnly || writer.sharedDirectory != "") {
		return ErrBoundedWrite // selected output cannot borrow journal/storage privileges
	}
	text, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: text, Attributes: windows.OBJ_CASE_INSENSITIVE}
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE)
	sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE) // pin directory names against deletion/moves
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	attributes := uint32(windows.FILE_ATTRIBUTE_DIRECTORY)
	if file.directory {
		access |= windows.FILE_TRAVERSE
		options |= windows.FILE_DIRECTORY_FILE
		if file.deleteDirectory {
			access |= windows.DELETE
		} else if writer.sharedDirectory != "" && strings.EqualFold(file.path, writer.sharedDirectory) {
			sharing |= windows.FILE_SHARE_DELETE
		}
	} else {
		access |= windows.GENERIC_WRITE | windows.DELETE
		sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE // no concurrent data writer
		options |= windows.FILE_NON_DIRECTORY_FILE
		attributes = windows.FILE_ATTRIBUTE_NORMAL
		if file.deleteRead {
			access = windows.GENERIC_READ | windows.READ_CONTROL | windows.DELETE | windows.SYNCHRONIZE
			sharing = windows.FILE_SHARE_READ // keep original bytes/name through checked deletion
		} else if file.inspectOnly {
			access = windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE
			sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
		} else if file.writerLock {
			access = windows.GENERIC_READ | windows.GENERIC_WRITE | windows.READ_CONTROL | windows.SYNCHRONIZE
			if file.readOnlyLock {
				access &^= windows.GENERIC_WRITE
			}
			sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE // stable lock cannot be replaced/deleted
		}
	}
	if writer.selectedContext != nil {
		if file.directory {
			access = windows.FILE_READ_ATTRIBUTES | windows.FILE_TRAVERSE | windows.SYNCHRONIZE
		} else {
			// GENERIC_WRITE is required by FlushFileBuffers. This is our
			// original created temporary, never the existing destination.
			access = windows.GENERIC_WRITE | windows.FILE_READ_ATTRIBUTES | windows.DELETE | windows.SYNCHRONIZE
			sharing = windows.FILE_SHARE_READ // deny other data writers and deletion/rename
		}
	}
	var iosb windows.IO_STATUS_BLOCK
	file.acquiring = true
	err = windows.NtCreateFile(&file.handle, access, &oa, &iosb, nil, attributes, sharing, disposition, options, 0, 0)
	file.acquiring = false
	if file.handle == windows.InvalidHandle {
		file.handle = 0
	}
	file.created = disposition != windows.FILE_OPEN && iosb.Information == 2
	if disposition == windows.FILE_OPEN && file.handle == 0 && (errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) ||
		errors.Is(err, windows.STATUS_OBJECT_PATH_NOT_FOUND) || errors.Is(err, windows.STATUS_NO_SUCH_FILE)) {
		return os.ErrNotExist
	}
	if disposition == windows.FILE_CREATE && errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) && file.handle == 0 && !file.created {
		return os.ErrExist // only confirmed no-create collisions may try another name
	}
	if err != nil || file.handle == 0 || iosb.Information != 1 && iosb.Information != 2 ||
		disposition == windows.FILE_CREATE && !file.created || disposition == windows.FILE_OPEN && iosb.Information != 1 {
		return errors.Join(ErrBoundedWrite, err)
	}
	file.identity, err = writer.inspectFile(file)
	return err
}

func (writer *boundedNativeWrite) inspectFile(file *boundedNativeWriteFile) (windows.ByHandleFileInformation, error) {
	if writer.selectedContext != nil {
		return inspectWindowsFile(file.handle, file.path, file.directory, writer.limit, nil, inspectSelectedOutput)
	}
	if file.acl == nil {
		file.acl = new(WindowsFileACLSlot)
	}
	return InspectWindowsFileWithACL(file.acl, file.handle, file.path, file.directory, writer.limit, writer.trusted)
}

func (writer *boundedNativeWrite) inspect(file *boundedNativeWriteFile) (windows.ByHandleFileInformation, error) {
	info, err := writer.inspectFile(file)
	if err != nil || !sameWindowsReadID(info, file.identity) {
		return info, errors.Join(ErrBoundedWrite, err)
	}
	return info, nil
}

func (writer *boundedNativeWrite) validateDirectories() error {
	for index := range writer.directories {
		if _, err := writer.inspect(&writer.directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func (writer *boundedNativeWrite) openDirectories() error {
	return writer.openDirectoriesMode(writer.selectedContext == nil)
}

func (writer *boundedNativeWrite) openDirectoriesMode(create bool) error {
	path, _, err := windowsReadPath(writer.path)
	if err != nil || !windowsReadComponent(filepath.Base(path)) {
		return errors.Join(ErrBoundedWrite, err)
	}
	volume := filepath.VolumeName(path)
	localDrive := len(volume) == 2 && volume[1] == ':'
	selectedUNC := writer.selectedContext != nil && strings.HasPrefix(volume, `\\`)
	if !localDrive && !selectedUNC {
		return ErrBoundedWrite // stored state is local-drive only; selected UNC must already validate
	}
	writer.path = path
	if writer.sharedDirectory != "" && !strings.EqualFold(writer.sharedDirectory, filepath.Dir(path)) {
		return ErrBoundedWrite
	}
	root := volume + `\`
	relative := strings.TrimPrefix(filepath.Dir(path), root)
	var parts []string
	if relative != "" {
		parts = strings.Split(relative, `\`)
	}
	if len(parts)+1 > maxWriteDirectories {
		return ErrBoundedWrite
	}
	if writer.selectedContext == nil {
		if err := writer.initTrust(); err != nil {
			return err
		}
	}
	_, nativeRoot, err := windowsReadPath(root)
	if err != nil {
		return err
	}
	writer.directories = append(writer.directories, boundedNativeWriteFile{path: root, directory: true})
	if err := writer.contextError(); err != nil {
		return err
	}
	if err := writer.open(&writer.directories[0], 0, nativeRoot, windows.FILE_OPEN); err != nil {
		return err
	}
	for _, part := range parts {
		if err := writer.contextError(); err != nil {
			return err
		}
		if !windowsReadComponent(part) {
			return ErrBoundedWrite
		}
		index := len(writer.directories) - 1
		writer.directories = append(writer.directories, boundedNativeWriteFile{path: filepath.Join(writer.directories[index].path, part), directory: true})
		disposition := uint32(windows.FILE_OPEN)
		if create {
			disposition = windows.FILE_OPEN_IF
		}
		if err := writer.open(&writer.directories[index+1], writer.directories[index].handle, part, disposition); err != nil {
			return err
		}
	}
	return nil
}

func (writer *boundedNativeWrite) createTemporary() error {
	parent := &writer.directories[len(writer.directories)-1]
	for attempt := 0; attempt < 16; attempt++ {
		if err := writer.contextError(); err != nil {
			return err
		}
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		name := ".stored-write-" + hex.EncodeToString(nonce[:])
		writer.temporary = boundedNativeWriteFile{path: filepath.Join(parent.path, name)}
		err := writer.open(&writer.temporary, parent.handle, name, windows.FILE_CREATE)
		if errors.Is(err, os.ErrExist) && !writer.temporary.created && writer.temporary.handle == 0 && !writer.temporary.acquiring {
			if err := writer.temporary.Close(); err != nil {
				return err
			}
			continue
		}
		if err != nil || writer.temporary.identity.FileSizeHigh != 0 || writer.temporary.identity.FileSizeLow != 0 {
			return errors.Join(ErrBoundedWrite, err)
		}
		return nil
	}
	return errors.Join(ErrBoundedWrite, os.ErrExist)
}

func (file *boundedNativeWriteFile) flush() error {
	file.ioPending = true
	err := windows.FlushFileBuffers(file.handle)
	file.ioPending = false
	return err
}

// Same FILE_RENAME_INFORMATION layout as the existing installation/floor code.
// Only this explicit single-file replacement is allowed; no Ex/POSIX fallback.
type boundedWriteRenameInformation struct {
	ReplaceIfExists bool
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [256]uint16
}

func (writer *boundedNativeWrite) Publish(raw []byte) error {
	if writer.attempted || writer.closeAttempted || int64(len(raw)) > writer.limit {
		return ErrBoundedWrite
	}
	writer.attempted = true
	if err := writer.contextError(); err != nil {
		return err
	}
	if err := writer.openDirectories(); err != nil {
		return err
	}
	if err := writer.createTemporary(); err != nil {
		return err
	}
	file := &writer.temporary
	for offset := 0; offset < len(raw); {
		if err := writer.contextError(); err != nil {
			return err
		}
		end := min(offset+32*1024, len(raw))
		var done uint32
		file.ioPending = true
		err := windows.WriteFile(file.handle, raw[offset:end], &done, nil)
		file.ioPending = false
		if done != uint32(end-offset) {
			return errors.Join(ErrBoundedWrite, io.ErrShortWrite, err)
		}
		if err != nil {
			return errors.Join(ErrBoundedWrite, err)
		}
		offset = end
	}
	if err := writer.contextError(); err != nil {
		return err
	}
	if err := file.flush(); err != nil {
		return errors.Join(ErrBoundedWrite, err)
	}
	info, err := writer.inspect(file)
	if err != nil || info.FileSizeHigh != 0 || info.FileSizeLow != uint32(len(raw)) {
		return errors.Join(ErrBoundedWrite, err)
	}
	if err := writer.validateDirectories(); err != nil {
		return err
	}
	name, err := windows.UTF16FromString(filepath.Base(writer.path))
	if err != nil || len(name) > 256 {
		return ErrBoundedWrite
	}
	rename := boundedWriteRenameInformation{ReplaceIfExists: !writer.newOnly, RootDirectory: writer.directories[len(writer.directories)-1].handle, FileNameLength: uint32(2 * (len(name) - 1))}
	copy(rename.FileName[:], name)
	if err := writer.contextError(); err != nil {
		return err
	}
	var iosb windows.IO_STATUS_BLOCK
	writer.uncertain = true // error/panic can follow an already changed canonical name
	err = windows.NtSetInformationFile(file.handle, &iosb, (*byte)(unsafe.Pointer(&rename)), uint32(unsafe.Offsetof(rename.FileName))+rename.FileNameLength, windows.FileRenameInformation)
	if err != nil {
		if _, inspectErr := writer.inspect(file); inspectErr == nil {
			writer.uncertain = false // exact original is still at its private name
		}
		return errors.Join(ErrBoundedWrite, err)
	}
	writer.published = true
	file.path = writer.path
	if _, err := writer.inspect(file); err != nil {
		return err
	}
	if err := file.flush(); err != nil {
		return errors.Join(ErrBoundedWrite, err)
	}
	info, err = writer.inspect(file)
	if err != nil || info.FileSizeHigh != 0 || info.FileSizeLow != uint32(len(raw)) {
		return errors.Join(ErrBoundedWrite, err)
	}
	if err := writer.validateDirectories(); err != nil {
		return err
	}
	writer.uncertain = false
	return nil // the slot still requires checked original Close before success
}

func (file *boundedNativeWriteFile) Close() (finalErr error) {
	if file.closeAttempted {
		return file.closeErr
	}
	file.closeAttempted, file.closeErr = true, ErrWriteRetirement
	defer func() {
		if recover() != nil {
			finalErr = ErrWriteRetirement
		}
		file.closeErr = finalErr
	}()
	if file.acquiring || file.ioPending || file.handle == 0 && file.created {
		return ErrWriteRetirement
	}
	if file.acl != nil {
		if err := file.acl.Close(); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
	}
	if file.handle != 0 {
		if err := windows.CloseHandle(file.handle); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
		file.handle = 0
	}
	return nil
}

func (writer *boundedNativeWrite) Close() (finalErr error) {
	if writer.closeAttempted {
		return writer.closeErr
	}
	writer.closeAttempted, writer.closeErr = true, ErrWriteRetirement
	defer func() {
		if recover() != nil {
			finalErr = ErrWriteRetirement
		}
		writer.closeErr = finalErr
	}()
	if writer.uncertain || writer.temporary.acquiring || writer.temporary.ioPending {
		return ErrWriteRetirement
	}
	file := &writer.temporary
	if file.created && !writer.published {
		if _, err := writer.inspect(file); err != nil {
			return err
		}
		if err := writer.validateDirectories(); err != nil {
			return err
		}
		var iosb windows.IO_STATUS_BLOCK
		disposition := byte(1)
		writer.uncertain = true
		if err := windows.NtSetInformationFile(file.handle, &iosb, &disposition, 1, windows.FileDispositionInformation); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
		writer.uncertain = false // delete only the original created object, never a path
	}
	if err := file.Close(); err != nil {
		return err
	}
	for index := len(writer.directories) - 1; index >= 0; index-- {
		if err := writer.directories[index].Close(); err != nil {
			return err
		}
	}
	return nil
}
