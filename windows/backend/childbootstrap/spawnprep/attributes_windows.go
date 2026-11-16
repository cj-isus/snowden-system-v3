//go:build windows

package spawnprep

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var initializeAttributes = kernel32.NewProc("InitializeProcThreadAttributeList")
var updateAttributes = kernel32.NewProc("UpdateProcThreadAttribute")
var deleteAttributes = kernel32.NewProc("DeleteProcThreadAttributeList")

// Windows SDK 10.0.26100.0 winbase.h: JobList=13, input=0x00020000,
// thread=false, additive=false. x/sys v0.44.0 does not export this constant.
const processAttributeJobList uintptr = 0x0002000d

// Publish the zero Attributes in the original owner before Initialize. Do not
// copy it after use. Both original vectors must remain immutable and reachable
// until Close confirms deletion/unpin/free. The caller retains the original Job
// throughout CreateProcess and cleanup; the stored value owns no duplicate.
// Caller serializes all use.
type Attributes struct {
	allocation                          uintptr
	list                                *windows.ProcThreadAttributeList
	jobs                                [1]windows.Handle
	initialized, deleteStarted, deleted bool
	pinner                              runtime.Pinner
	pinned                              bool
	prepareAttempted, preparing         bool
	freeAttempted, unpinAttempted       bool
	ready, useAttempted, borrowing      bool
	retiring                            bool
}

func (a *Attributes) Initialize(handles *[3]windows.Handle, job windows.Handle) error {
	if a == nil || handles == nil || !nativelease.ValidHandleValue(job) || a.retiring || a.prepareAttempted || a.preparing || a.allocation != 0 || a.initialized || a.pinned {
		return ErrResources
	}
	a.prepareAttempted, a.preparing = true, true
	a.jobs[0] = job
	err := a.prepare(handles)
	a.preparing = false // only an actual return establishes completion, including rejection
	a.ready = err == nil
	return err
}

func (a *Attributes) prepare(handles *[3]windows.Handle) error {
	for _, procedure := range []*windows.LazyProc{initializeAttributes, updateAttributes, deleteAttributes} {
		if err := procedure.Find(); err != nil {
			return ErrResources
		}
	}
	var size uintptr
	ok, _, callErr := initializeAttributes.Call(0, 2, 0, uintptr(unsafe.Pointer(&size)))
	if ok != 0 || !errors.Is(callErr, windows.ERROR_INSUFFICIENT_BUFFER) || size == 0 || size > 64<<10 {
		return ErrResources
	}
	var err error
	a.allocation, err = windows.LocalAlloc(windows.LMEM_FIXED, uint32(size))
	if err != nil || a.allocation == 0 {
		return ErrResources
	}
	a.list = (*windows.ProcThreadAttributeList)(unsafe.Pointer(a.allocation))
	ok, _, _ = initializeAttributes.Call(uintptr(unsafe.Pointer(a.list)), 2, 0, uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ErrResources // original allocation survives failed initialization
	}
	a.initialized = true
	a.pinned = true // retain a partial Pin before it can exit without returning
	a.pinner.Pin(&handles[0])
	a.pinner.Pin(&a.jobs[0])
	ok, _, _ = updateAttributes.Call(uintptr(unsafe.Pointer(a.list)), 0, windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		uintptr(unsafe.Pointer(&handles[0])), unsafe.Sizeof(*handles), 0, 0)
	runtime.KeepAlive(handles)
	if ok == 0 {
		return ErrResources
	}
	ok, _, _ = updateAttributes.Call(uintptr(unsafe.Pointer(a.list)), 0, processAttributeJobList,
		uintptr(unsafe.Pointer(&a.jobs[0])), unsafe.Sizeof(a.jobs), 0, 0)
	runtime.KeepAlive(a)
	if ok == 0 {
		return ErrResources // never fall back to assigning an already-created child
	}
	return nil
}

// WithStartup lends the attribute list for one synchronous CreateProcess call.
// The callback must not retain the pointer, reenter Attributes or close/mutate
// its original handle vector. It writes process outputs into its own retained
// owner. Panic/Goexit leaves borrowing sticky and prevents native release.
func (a *Attributes) WithStartup(use func(*windows.StartupInfoEx) error) error {
	if a == nil || use == nil || !a.ready || a.retiring || a.preparing || a.useAttempted || a.borrowing {
		return ErrResources
	}
	a.useAttempted, a.borrowing = true, true
	startup := windows.StartupInfoEx{ProcThreadAttributeList: a.list}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// Both original vectors are pinned during Initialize. Standard handle values
	// and window flags are supplied by the caller in this synchronous callback.
	err := use(&startup)
	a.borrowing = false // not deferred: a nonreturning callback grants no release fact
	return err
}

func (a *Attributes) Close() error {
	if a == nil {
		return ErrResources
	}
	a.retiring, a.ready = true, false
	if a.preparing || a.borrowing {
		return ErrResources
	}
	if a.initialized && !a.deleted {
		if a.deleteStarted {
			return ErrResources
		}
		a.deleteStarted = true
		deleteAttributes.Call(uintptr(unsafe.Pointer(a.list)))
		a.deleted = true
	}
	if a.pinned {
		if a.unpinAttempted {
			return ErrResources
		}
		a.unpinAttempted = true
		a.pinner.Unpin()
		a.pinned = false
	}
	if a.allocation != 0 {
		if a.freeAttempted {
			return ErrResources
		}
		a.freeAttempted = true
		remaining, err := windows.LocalFree(windows.Handle(a.allocation))
		if err != nil || remaining != 0 {
			return ErrResources
		}
		a.allocation, a.list = 0, nil
	}
	return nil
}
