//go:build windows

package filelease

import (
	"errors"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// Lease must be retained before Open/OpenRelative and must not be copied after
// use. The original caller serializes acquisition, inspection, all borrowed I/O,
// rebind and Close. This owner does not introduce a lock around caller I/O.
type Lease struct {
	handle                                   windows.Handle
	acl                                      *fileacl.Slot
	io                                       windows.IO_STATUS_BLOCK
	acquiring, acquireFailed, closeAttempted bool
}

func (lease *Lease) begin() error {
	if lease == nil || lease.acquiring || lease.handle != 0 {
		return ErrFile
	}
	if lease.acl != nil {
		if err := lease.acl.Close(); err != nil {
			return errors.Join(ErrCleanup, err)
		}
		lease.acl = nil // only confirmed retirement permits an explicit new open
	}
	lease.io = windows.IO_STATUS_BLOCK{}
	lease.closeAttempted = false
	lease.acquiring, lease.acquireFailed = true, true
	return nil
}

func (lease *Lease) finish(err error) {
	if lease.handle == windows.InvalidHandle {
		lease.handle = 0
	}
	lease.acquiring = false // the native call returned to this exact owner
	lease.acquireFailed = err != nil || !nativelease.ValidHandleValue(lease.handle)
}

// Open preserves the existing OPEN_EXISTING Win32 acquisition. The trusted
// caller chooses access/sharing/flags; this API does not approve paths or roles.
func (lease *Lease) Open(name *uint16, access, share, flags uint32) error {
	if name == nil {
		return ErrFile
	}
	if err := lease.begin(); err != nil {
		return err
	}
	var err error
	lease.handle, err = windows.CreateFile(name, access, share, nil, windows.OPEN_EXISTING, flags, 0)
	lease.finish(err)
	if lease.acquireFailed {
		if err == nil {
			return ErrFile
		}
		return errors.Join(ErrFile, err)
	}
	return nil
}

// OpenRelative retains both native outputs in the original owner, including
// partial failures. Information is a raw creation witness, not path/ID authority.
// Parent lifetime, name/disposition/options and created/collision policy belong
// to the existing caller; no name/PID reopen or cleanup fallback is performed.
func (lease *Lease) OpenRelative(parent windows.Handle, name *windows.NTUnicodeString, access, attributes, share, disposition, options uint32) (uintptr, error) {
	if !nativelease.ValidHandleValue(parent) || name == nil {
		return 0, ErrFile
	}
	if err := lease.begin(); err != nil {
		return 0, err
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent,
		ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	err := windows.NtCreateFile(&lease.handle, access, &oa, &lease.io, nil, attributes, share, disposition, options, 0, 0)
	lease.finish(err)
	if lease.acquireFailed {
		if err == nil {
			return lease.io.Information, ErrFile
		}
		return lease.io.Information, errors.Join(ErrFile, err)
	}
	return lease.io.Information, nil
}

// HandleValue is a borrowed observation of the original value, including an
// uncertain partial/Close result. It transfers no ownership or permission to
// use/close that number. Existing caller validation and lifetime serialization
// remain required; never substitute a new owner when this one is uncertain.
func (lease *Lease) HandleValue() windows.Handle {
	if lease == nil {
		return 0
	}
	return lease.handle
}

// Ready is only the original acquisition-state fact, not fresh identity or trust.
func (lease *Lease) Ready() bool {
	return lease != nil && !lease.acquiring && !lease.acquireFailed && !lease.closeAttempted &&
		nativelease.ValidHandleValue(lease.handle)
}

// Inspect borrows the original handle and owns the ACL output before its first
// query. expected/identity/trusted stay with the caller's original receipt.
func (lease *Lease) Inspect(expected string, directory bool, identity *windows.ByHandleFileInformation, trusted func(*windows.SID) bool) error {
	if !lease.Ready() {
		return ErrFile
	}
	if lease.acl == nil {
		lease.acl = new(fileacl.Slot)
	}
	return imagecheck.FileIdentity(lease.handle, expected, directory, lease.acl, trusted, identity)
}

// Close releases ACL custody before the original handle. Unreturned acquisition,
// unknown Free or Close retains its exact state; a guessed value is not retried.
// Known handle0 still permits the caller's explicit, separately checked rebind.
func (lease *Lease) Close() error {
	if lease == nil {
		return nil
	}
	if lease.acquiring {
		return ErrCleanup
	}
	if lease.acl != nil {
		if err := lease.acl.Close(); err != nil {
			return errors.Join(ErrCleanup, err)
		}
	}
	if lease.handle == 0 {
		return nil
	}
	if lease.closeAttempted || !nativelease.ValidHandleValue(lease.handle) {
		return ErrCleanup
	}
	lease.closeAttempted = true
	if err := windows.CloseHandle(lease.handle); err != nil {
		return errors.Join(ErrCleanup, err)
	}
	lease.handle = 0
	return nil
}
