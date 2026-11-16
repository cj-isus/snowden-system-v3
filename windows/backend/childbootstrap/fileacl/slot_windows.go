//go:build windows

package fileacl

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

var rawGetSecurityInfo = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetSecurityInfo")

// Slot must be retained before Validate and must not be copied after use. It
// owns at most one original descriptor, including partial/unreturned queries.
// A completed query can be repeated only after confirmed release of that buffer.
type Slot struct {
	mu            sync.Mutex
	retiring      atomic.Bool
	descriptor    *windows.SECURITY_DESCRIPTOR
	acquiring     bool
	freeAttempted bool
	fault         error
}

func (slot *Slot) release() error {
	if slot.acquiring {
		return ErrCleanup
	}
	if slot.descriptor == nil {
		return nil
	}
	if slot.freeAttempted {
		return ErrCleanup // unknown Free cannot be retried on a guessed live pointer
	}
	slot.freeAttempted = true
	remaining, err := windows.LocalFree(windows.Handle(unsafe.Pointer(slot.descriptor)))
	if err != nil || remaining != 0 {
		return errors.Join(ErrCleanup, err)
	}
	slot.descriptor, slot.freeAttempted = nil, false // confirmed release permits a fresh query
	return nil
}

// Validate borrows the file handle. trusted is a pure, transient SID predicate:
// it must not retain/mutate SID pointers or reenter this owner. No context,
// process, file handle, background task or effective-access authority is created.
func (slot *Slot) Validate(handle windows.Handle, directory bool, trusted func(*windows.SID) bool) (finalErr error) {
	if slot == nil {
		return ErrACL
	}
	if !slot.mu.TryLock() {
		return errors.Join(ErrACL, ErrBusy)
	}
	defer slot.mu.Unlock()
	if slot.retiring.Load() {
		return ErrACL
	}
	if slot.acquiring || slot.descriptor != nil || slot.freeAttempted {
		return ErrCleanup
	}
	if slot.fault != nil {
		return slot.fault
	}
	if !nativelease.ValidHandleValue(handle) || trusted == nil {
		return ErrACL
	}
	completed := false
	defer func() {
		if !completed {
			slot.fault = ErrACL // retained even on callback panic/Goexit
			finalErr = errors.Join(finalErr, ErrACL)
		}
		if err := slot.release(); err != nil {
			slot.fault = errors.Join(ErrCleanup, err)
			finalErr = errors.Join(finalErr, slot.fault)
		}
		if slot.retiring.Load() {
			finalErr = errors.Join(finalErr, ErrACL)
		}
	}()
	if err := rawGetSecurityInfo.Find(); err != nil {
		completed = true
		return errors.Join(ErrACL, err)
	}
	if slot.retiring.Load() {
		completed = true
		return ErrACL
	}
	slot.acquiring = true
	// DWORD status return, not BOOL/GetLastError. Native output writes directly
	// into the original owner even when the call returns an error.
	status, _, _ := rawGetSecurityInfo.Call(uintptr(handle), uintptr(windows.SE_FILE_OBJECT),
		uintptr(windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION), 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&slot.descriptor)))
	slot.acquiring = false
	runtime.KeepAlive(slot)
	if status != 0 {
		completed = true
		return errors.Join(ErrACL, syscall.Errno(status))
	}
	if slot.retiring.Load() {
		completed = true
		return ErrACL
	}
	finalErr = validateDescriptor(slot.descriptor, directory, trusted)
	completed = true
	return finalErr
}

// Close retires this original owner before trying its mutex. Busy/unknown cleanup
// retains the descriptor and prohibits a new query; no file handle is closed here.
func (slot *Slot) Close() error {
	if slot == nil {
		return nil
	}
	slot.retiring.Store(true)
	if !slot.mu.TryLock() {
		return errors.Join(ErrCleanup, ErrBusy)
	}
	defer slot.mu.Unlock()
	return slot.release()
}
