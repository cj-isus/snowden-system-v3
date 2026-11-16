//go:build windows

package spawnprep

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

var processIsInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

// Process must be published in its original caller owner before any native work
// and never copied. Caller serializes every method except the single reserved
// Wait. A blocking Wait borrows its immutable original handle; Close cannot free
// it until that native call has returned. There is no PID/path-based acquisition.
type Process struct {
	job         windows.Handle
	information windows.ProcessInformation // actual CreateProcess output, including partial failure

	jobAttempted, jobOpening, jobOpened             bool
	createAttempted, creating                       bool
	created, confirmAttempted, confirming, assigned bool
	resumeAttempted, resuming, borrowing, retiring  bool
	drained, closed, waitReserved                   bool
	closeAttempted                                  [3]bool // thread/process/Job
	waitEntered, waitFinished                       atomic.Bool
}

func (p *Process) usable() bool {
	return p != nil && !p.retiring && p.jobOpened && !p.jobOpening && !p.creating && !p.confirming && !p.resuming && !p.borrowing && nativelease.ValidHandleValue(p.job)
}

func (p *Process) OpenJob() error {
	if p == nil || p.retiring || p.jobAttempted || p.createAttempted || p.job != 0 {
		return ErrProcess
	}
	p.jobAttempted, p.jobOpening = true, true
	var err error
	p.job, err = windows.CreateJobObject(nil, nil) // unnamed and non-inheritable
	p.jobOpened = err == nil && nativelease.ValidHandleValue(p.job)
	p.jobOpening = false
	if !p.jobOpened {
		return errors.Join(ErrProcess, err)
	}
	return nil
}

// CreateSuspended invokes one trusted synchronous native adapter. It must make
// exactly one CreateProcess call with CREATE_SUSPENDED and the original Job in
// its JOB_LIST attribute, use this output pointer directly and never retain it
// or reenter Process. ConfirmJob must verify membership before Resume.
// Role/image/argument/flag and resource checks stay with that compiled caller.
// Callback success alone grants
// no signing, installation, child-tree or forwarding authority.
func (p *Process) CreateSuspended(create func(*windows.ProcessInformation) error) error {
	if !p.usable() || create == nil || p.createAttempted {
		return ErrProcess
	}
	p.createAttempted, p.creating = true, true
	err := create(&p.information)
	p.created = err == nil || p.information.Process != 0 || p.information.Thread != 0
	p.creating = false // an unreturned call leaves the original output and uncertainty retained
	return err
}

func (p *Process) Created() bool {
	return p != nil && (p.creating || p.created || p.information.Process != 0 || p.information.Thread != 0)
}

// Borrow callbacks are synchronous and nonreentrant. They may neither retain nor
// close/reassign handles. On panic/Goexit borrowing stays sticky, preventing any
// positive reuse or native release; cleanup may still terminate the known tree.
func (p *Process) WithJob(use func(windows.Handle) error) error {
	if !p.usable() || use == nil {
		return ErrProcess
	}
	p.borrowing = true
	err := use(p.job)
	p.borrowing = false
	if !p.usable() {
		return errors.Join(ErrProcess, err)
	}
	return err
}

func (p *Process) WithAssigned(use func(job, process windows.Handle) error) error {
	if !p.usable() || use == nil || !p.assigned || !nativelease.ValidHandleValue(p.information.Process) {
		return ErrProcess
	}
	p.borrowing = true
	err := use(p.job, p.information.Process)
	p.borrowing = false
	if !p.usable() {
		return errors.Join(ErrProcess, err)
	}
	return err
}

// ConfirmJob queries the original handles after creation-time association. It
// cannot attach an uncontained child or recover from a missing JOB_LIST.
func (p *Process) ConfirmJob() error {
	if !p.usable() || p.confirmAttempted || !p.Created() || !nativelease.ValidHandleValue(p.information.Process) || !nativelease.ValidHandleValue(p.information.Thread) {
		return ErrProcess
	}
	p.confirmAttempted, p.confirming = true, true
	err := p.confirmJob()
	p.assigned = err == nil
	p.confirming = false // an unreturned query cannot become a membership receipt
	return err
}

func (p *Process) confirmJob() error {
	if err := processIsInJob.Find(); err != nil {
		return errors.Join(ErrProcess, err)
	}
	var member int32 // Win32 BOOL output, not a Go bool
	ok, _, callErr := processIsInJob.Call(uintptr(p.information.Process), uintptr(p.job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 {
		return errors.Join(ErrProcess, callErr)
	}
	if member == 0 {
		return ErrProcess
	}
	return nil
}

func (p *Process) Resume() error {
	if !p.usable() || !p.assigned || p.resumeAttempted || !nativelease.ValidHandleValue(p.information.Thread) {
		return ErrProcess
	}
	p.resumeAttempted, p.resuming = true, true
	previous, err := windows.ResumeThread(p.information.Thread)
	p.resuming = false
	if err != nil || previous != 1 {
		return errors.Join(ErrProcess, err) // zero or >1 is not our suspended-to-running transition
	}
	return nil
}

// Reserve before launching the one Wait goroutine. An allocation/go interruption
// after reservation cannot be mistaken for a completed physical native wait.
func (p *Process) ReserveWait() bool {
	if p == nil || p.retiring || p.waitReserved || !p.Created() || !nativelease.ValidHandleValue(p.information.Process) {
		return false
	}
	p.waitReserved = true
	return true
}

func (p *Process) Wait() error {
	if p == nil || !p.waitReserved || !p.waitEntered.CompareAndSwap(false, true) || !nativelease.ValidHandleValue(p.information.Process) {
		return ErrProcess
	}
	state, err := windows.WaitForSingleObject(p.information.Process, windows.INFINITE)
	p.waitFinished.Store(true) // only an actual native return, never a defer/panic/Goexit receipt
	if err != nil || state != windows.WAIT_OBJECT_0 {
		return errors.Join(ErrProcess, err)
	}
	return nil
}

func (p *Process) knownProcess() bool {
	return p != nil && nativelease.ValidHandleValue(p.information.Process) && !p.closeAttempted[1]
}

func (p *Process) ExitCode() (uint32, error) {
	if !p.knownProcess() || p.retiring {
		return 0, ErrProcess
	}
	var code uint32
	err := windows.GetExitCodeProcess(p.information.Process, &code)
	return code, err
}

func (p *Process) Terminate() error {
	if !p.knownProcess() {
		return ErrProcess
	}
	if !nativelease.ValidHandleValue(p.job) || p.closeAttempted[2] {
		return ErrProcess
	}
	if p.assigned {
		return windows.TerminateJobObject(p.job, 1)
	}
	// Partial creation or failed confirmation may still have joined the Job.
	// Retire both original owners, even if Job termination itself fails.
	jobErr := windows.TerminateJobObject(p.job, 1)
	rootErr := windows.TerminateProcess(p.information.Process, 1)
	if rootErr != nil {
		if state, err := windows.WaitForSingleObject(p.information.Process, 0); err == nil && state == windows.WAIT_OBJECT_0 {
			rootErr = nil // only known root exit; Job/I/O/image cleanup remains separate
		}
	}
	return errors.Join(jobErr, rootErr)
}

func (p *Process) Empty() (bool, error) {
	if !p.knownProcess() {
		return false, ErrProcess
	}
	if !p.assigned {
		state, err := windows.WaitForSingleObject(p.information.Process, 0)
		if err != nil || state != windows.WAIT_OBJECT_0 {
			return false, err
		}
		// Unknown membership requires root exit AND an empty original Job.
	}
	if !nativelease.ValidHandleValue(p.job) || p.closeAttempted[2] {
		return false, ErrProcess
	}
	accounting, err := QueryAccounting(p.job)
	return err == nil && accounting.ActiveProcesses == 0, err
}

// Close rechecks root exit/empty Job before releasing native handles. It does not
// retire caller I/O/image/attributes or issue a whole-application cleanup receipt.
// Caller must keep those original owners and their completion barriers as well.
func (p *Process) Close() error {
	if p == nil {
		return ErrProcess
	}
	p.retiring = true
	if p.closed {
		return nil
	}
	if p.jobOpening || p.creating || p.confirming || p.resuming || p.borrowing || p.waitReserved && !p.waitFinished.Load() {
		return ErrProcess
	}
	handles := [3]*windows.Handle{&p.information.Thread, &p.information.Process, &p.job}
	for index, handle := range handles {
		if p.closeAttempted[index] && *handle != 0 {
			return ErrProcess // do not query, close or otherwise use an uncertain numeric value
		}
	}
	if !p.drained {
		if p.Created() {
			if !p.knownProcess() {
				return ErrProcess
			}
			state, err := windows.WaitForSingleObject(p.information.Process, 0)
			if err != nil || state != windows.WAIT_OBJECT_0 {
				return ErrProcess
			}
			if empty, err := p.Empty(); err != nil || !empty {
				return ErrProcess
			}
		}
		p.drained = true
	}
	for index, handle := range handles {
		if err := CloseOnce(handle, &p.closeAttempted[index]); err != nil {
			return err
		}
	}
	p.closed = true
	return nil
}
