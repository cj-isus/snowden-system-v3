//go:build windows

package nativelease

import (
	"context"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var leaseKernel32 = windows.NewLazySystemDLL("kernel32.dll")
var leaseIsProcessInJob = leaseKernel32.NewProc("IsProcessInJob")
var leaseGetHandleInformation = leaseKernel32.NewProc("GetHandleInformation")

// Lease is a one-attempt child owner. Job stays open until process exit; parent
// closes only after its original Wait returns signaled. No arbitrary input handle
// is closed on failed adoption. Unknown native results burn all positive reuse.
type Lease struct {
	mu      sync.Mutex
	claimed [2]uint64 // exact original seed values, including rejected/partial input
	job     windows.Handle
	parent  windows.Handle

	adoptAttempted, adopted, awaitAttempted, parentCloseAttempted, failed bool
}

// ValidHandleValue checks representation only, never object type or authority.
// Real creation results cannot be zero or a reserved negative pseudo handle.
func ValidHandleValue(handle windows.Handle) bool {
	return handle != 0 && handle < windows.Handle(^uintptr(15))
}

func (lease *Lease) retainFailure(completed *bool) {
	if !*completed {
		lease.failed = true // also on panic/Goexit; mutex is still held
	}
}

func (lease *Lease) Adopt(handles [2]uint64) error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrLease
	}
	defer lease.mu.Unlock()
	if lease.adoptAttempted || lease.failed {
		return ErrLease
	}
	lease.adoptAttempted, lease.claimed = true, handles
	completed := false
	defer lease.retainFailure(&completed)
	if handles[0] == handles[1] {
		return ErrLease
	}
	for _, raw := range handles {
		if uint64(windows.Handle(raw)) != raw || !ValidHandleValue(windows.Handle(raw)) {
			return ErrLease
		}
		var flags uint32
		if leaseGetHandleInformation.Find() != nil {
			return ErrLease
		}
		ok, _, _ := leaseGetHandleInformation.Call(uintptr(windows.Handle(raw)), uintptr(unsafe.Pointer(&flags)))
		if ok == 0 || flags&windows.HANDLE_FLAG_INHERIT != 0 {
			return ErrLease
		}
	}
	// PID is an object-type/nonself comparison only, never an acquisition source.
	parent, err := windows.GetProcessId(windows.Handle(handles[1]))
	if err != nil || parent == 0 || parent == windows.GetCurrentProcessId() {
		return ErrLease
	}
	lease.job, lease.parent = windows.Handle(handles[0]), windows.Handle(handles[1])
	if err := lease.validateJob(); err != nil {
		return err // original values remain; no Close on unvalidated input
	}
	lease.adopted = true
	completed = true
	return nil
}

func (lease *Lease) validateJob() error {
	if !ValidHandleValue(lease.job) || leaseIsProcessInJob.Find() != nil {
		return ErrLease
	}
	var member int32
	ok, _, _ := leaseIsProcessInJob.Call(uintptr(windows.CurrentProcess()), uintptr(lease.job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 || member == 0 {
		return ErrLease
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var returned uint32
	err := windows.QueryInformationJobObject(lease.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), &returned)
	want := uint32(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK)
	if err != nil || returned != uint32(unsafe.Sizeof(limits)) || limits.BasicLimitInformation.LimitFlags != want {
		return ErrLease
	}
	return nil
}

func (lease *Lease) ValidateJob() error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrLease
	}
	defer lease.mu.Unlock()
	if !lease.adopted || lease.failed {
		return ErrLease
	}
	completed := false
	defer lease.retainFailure(&completed)
	if err := lease.validateJob(); err != nil {
		return err
	}
	completed = true
	return nil
}

// WithParent lends the original handle only during this locked callback. It must
// neither close/retain the handle nor reenter Lease. AwaitParent cannot close it
// concurrently. Peer/image/SID validation stays in the caller's original owner.
func (lease *Lease) WithParent(use func(windows.Handle) error) error {
	if lease == nil || use == nil || !lease.mu.TryLock() {
		return ErrLease
	}
	defer lease.mu.Unlock()
	if !lease.adopted || lease.failed || lease.awaitAttempted || lease.parentCloseAttempted || !ValidHandleValue(lease.parent) {
		return ErrLease
	}
	completed := false
	defer lease.retainFailure(&completed)
	if err := use(lease.parent); err != nil {
		return err
	}
	completed = true
	return nil
}

func (lease *Lease) AwaitParent(ctx context.Context, scope func(context.Context) error) error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrLease
	}
	defer lease.mu.Unlock()
	if !lease.adopted || lease.failed || lease.awaitAttempted || !ValidHandleValue(lease.parent) {
		return ErrLease
	}
	lease.awaitAttempted = true
	completed := false
	defer lease.retainFailure(&completed)
	if err := checkScope(ctx, scope); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		if err := checkScope(ctx, scope); err != nil {
			return err
		}
		state, err := windows.WaitForSingleObject(lease.parent, 100)
		if err != nil || state != windows.WAIT_OBJECT_0 && state != uint32(windows.WAIT_TIMEOUT) {
			return ErrLease
		}
		if err := checkScope(ctx, scope); err != nil {
			return err
		}
		if state == windows.WAIT_OBJECT_0 {
			lease.parentCloseAttempted = true
			if err := windows.CloseHandle(lease.parent); err != nil {
				return ErrLease // retain original value, never retry an uncertain Close
			}
			lease.parent = 0 // original Wait returned; no outstanding waiter uses it
			if err := lease.validateJob(); err != nil {
				return err
			}
			if err := checkScope(ctx, scope); err != nil {
				return err
			}
			completed = true
			return nil
		}
	}
}
