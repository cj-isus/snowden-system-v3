//go:build windows

package core

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

// Layout and flags: installed SDK10.0.26100.0 um/shellapi.h:395-503.
// Only the exact W ABI is used; optional association/IDLIST/async inputs stay zero.
type elevatedShellInfo struct {
	size, mask             uint32
	window                 windows.Handle
	verb                   *uint16
	file                   *uint16
	parameters             *uint16
	directory              *uint16
	show                   int32
	instance               windows.Handle
	idList                 unsafe.Pointer
	class                  *uint16
	classKey               windows.Handle
	hotKey                 uint32
	iconOrMonitor, process windows.Handle
}

var elevatedShellExecute = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type elevatedGUIJob struct {
	job                                  windows.Handle
	attempted, acquiring, closeAttempted bool
	resourceLimits                       windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
}

type elevatedGUINativeRequest struct {
	info         elevatedShellInfo // API writes its original hProcess HERE, including partial failure
	host         elevatedGUIJob    // original legacy GUI Job or broker ancestor Job; fixed by the request's role
	token        windows.Token
	jobRemote    windows.Handle // target-process value, never CloseHandle in this parent
	parentRemote windows.Handle // original parent query/sync value in the target process
	expected     string
	abortErr     error

	tokenAttempted, tokenAcquiring, tokenCloseAttempted, processCloseAttempted bool
	tokenReady                                                                 bool

	pin    runtime.Pinner
	pinned bool

	callStarted, callReturned, callOK, declined                               bool
	comStarted, comReturned, comOwned, uninitStarted, uninitReturned          bool
	classified, pathMatched, assigned, drained, ranBeforeJob, unassignedAbort bool
}

func (native *elevatedGUINativeRequest) releasePins() {
	if native.pinned {
		native.pin.Unpin()
		native.pinned = false
	}
}

func (native *elevatedGUINativeRequest) execute(ctx context.Context, image nativeImageLease, resume bool, nonce string, guard func(context.Context) error) error {
	if native.callStarted || native.host.attempted {
		return ErrElevatedGUIUnavailable
	}
	if err := native.host.open(); err != nil {
		return err
	}
	if err := configureOriginalJobLimits(native.host.job, &native.host.resourceLimits, ownedProcessLimits{processes: 1, successorGUI: true}); err != nil {
		return err
	}
	digest, err := nativeImagePin(nonce)
	if err != nil {
		return err
	}
	protocol, err := childbootstrap.NewApplicationProtocol(elevatedGUIArgument, digest)
	if err != nil {
		return err
	}
	args, err := protocol.ApplicationArguments(true, resume)
	if err != nil {
		return err
	}
	if err := native.launchShell(ctx, image, args, guard); err != nil {
		return err
	}
	if err := native.classify(); err != nil {
		return err
	}
	if err := windows.SetHandleInformation(native.info.process, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return err
	}
	// This is a RUNNING UAC process. The pre-attachment interval is explicit;
	// a failed request cannot use later empty Job accounting to erase that gap.
	// ShellExecute cannot request CREATE_BREAKAWAY_FROM_JOB. An inherited or
	// broker Job of unknown lifetime must not kill this child on parent exit.
	if updateIsProcessInJob.Find() != nil {
		return ErrElevatedGUIUnavailable
	}
	var alreadyInJob int32
	jobQueryOK, _, _ := updateIsProcessInJob.Call(uintptr(native.info.process), 0, uintptr(unsafe.Pointer(&alreadyInJob)))
	if jobQueryOK == 0 || alreadyInJob != 0 {
		return ErrElevatedGUIUnavailable
	}
	if err := windows.AssignProcessToJobObject(native.host.job, native.info.process); err != nil {
		return err
	}
	native.assigned = true
	if err := guard(ctx); err != nil {
		return err
	}
	if err := windows.DuplicateHandle(windows.CurrentProcess(), native.host.job, native.info.process, &native.jobRemote, updateJobQueryAccess, false, 0); err != nil {
		return err
	}
	if !createdProcessHandle(native.jobRemote) {
		return ErrElevatedGUIUnavailable
	}
	if err := windows.DuplicateHandle(windows.CurrentProcess(), windows.CurrentProcess(), native.info.process, &native.parentRemote,
		windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, 0); err != nil {
		return err
	}
	if !createdProcessHandle(native.parentRemote) {
		return ErrElevatedGUIUnavailable
	}
	return nil // original Shell COM/pinning cleanup completed before assignment/seed
}

func (native *elevatedGUINativeRequest) launchShell(ctx context.Context, image nativeImageLease, args []string, guard func(context.Context) error) error {
	if unsafe.Sizeof(native.info) != 112 || unsafe.Offsetof(native.info.process) != 104 || native.callStarted || native.comStarted || image == nil || guard == nil {
		return ErrElevatedGUIUnavailable
	}
	if err := elevatedShellExecute.Find(); err != nil {
		return err
	}
	native.expected = image.Executable()
	var err error
	native.info.size = uint32(unsafe.Sizeof(native.info))
	native.info.mask = 0x40 | 0x100 | 0x4000 // NOCLOSEPROCESS | NOASYNC | UNICODE
	native.info.show = windows.SW_SHOWNORMAL
	if native.info.verb, err = windows.UTF16PtrFromString("runas"); err != nil {
		return err
	}
	if native.info.file, err = windows.UTF16PtrFromString(native.expected); err != nil {
		return err
	}
	parameters := windows.ComposeCommandLine(args)
	if native.info.parameters, err = windows.UTF16PtrFromString(parameters); err != nil {
		return err
	}
	if native.info.directory, err = windows.UTF16PtrFromString(filepath.Dir(native.expected)); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer func() {
		if native.comOwned && (!native.callStarted || native.callReturned) {
			native.uninitStarted = true
			windows.CoUninitialize()
			native.uninitReturned = true
		}
		if !native.callStarted || native.callReturned {
			native.releasePins()
		}
		// An unfinished thread-affine COM cleanup is not returned to the pool.
		// The retained owner blocks replacement; its eventual OS exit is terminal.
		if native.comReturned && (!native.comOwned || native.uninitReturned) {
			runtime.UnlockOSThread()
		}
	}()
	native.comStarted = true
	initErr := windows.CoInitializeEx(0, 0x2|0x4) // STA | DISABLE_OLE1DDE, installed objbase.h
	native.comReturned = true
	// x/sys0.44 returns S_FALSE as Errno(1); it still owns a COM reference.
	if initErr != nil && !errors.Is(initErr, syscall.Errno(1)) {
		return initErr
	}
	native.comOwned = true
	native.pinned = true // a partial Pin remains reachable and cleanup may unpin it
	for _, value := range []any{&native.info, native.info.verb, native.info.file, native.info.parameters, native.info.directory} {
		native.pin.Pin(value)
	}
	if err := image.Validate(ctx); err != nil {
		return err
	}
	if err := guard(ctx); err != nil {
		return err
	}
	native.callStarted = true
	ok, _, callErr := elevatedShellExecute.Call(uintptr(unsafe.Pointer(&native.info)))
	native.callReturned, native.callOK = true, ok != 0
	// Any observed execution precedes both Job assignments, even a partial failure.
	native.ranBeforeJob = ok != 0 || native.info.process != 0
	runtime.KeepAlive(native)
	if ok == 0 {
		native.declined = errors.Is(callErr, windows.ERROR_CANCELLED) && native.info.process == 0
		if native.declined {
			return errors.Join(ErrElevatedGUIDeclined, callErr)
		}
		if callErr != syscall.Errno(0) {
			return errors.Join(ErrElevatedGUIUnavailable, callErr)
		}
		return ErrElevatedGUIUnavailable
	}
	return nil // original output and COM cleanup remain in this owner
}

// Negative original-handle classification, never PID-based acquisition or a
// new positive permission. A mismatched/live unknown image is not terminated.
func (native *elevatedGUINativeRequest) classify() error {
	if !createdProcessHandle(native.info.process) {
		return ErrElevatedGUIUnavailable
	}
	id, err := windows.GetProcessId(native.info.process)
	if err != nil || id == 0 || id == windows.GetCurrentProcessId() {
		return ErrElevatedGUIUnavailable
	}
	native.classified = true
	if err := nativeProcessPathMatches(context.Background(), native.info.process, native.expected); err != nil {
		return err
	}
	native.pathMatched = true
	return nil
}

func (native *elevatedGUINativeRequest) validate(ctx context.Context, image nativeImageLease) error {
	if !native.callOK || !native.callReturned || !native.assigned || !native.pathMatched || !createdProcessHandle(native.jobRemote) || !createdProcessHandle(native.parentRemote) ||
		!native.uninitReturned || native.pinned {
		return ErrElevatedGUIUnavailable
	}
	if err := image.Validate(ctx); err != nil {
		return err
	}
	if err := nativeProcessPathMatches(ctx, native.info.process, native.expected); err != nil {
		return err
	}
	state, err := windows.WaitForSingleObject(native.info.process, 0)
	if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return ErrElevatedGUIUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := native.prepareToken(); err != nil {
		return err
	}
	if !native.token.IsElevated() {
		return ErrElevatedGUIUnavailable
	}
	self, ok := image.(*windowsVerifierImage)
	user, userErr := fileacl.UserSID(native.token)
	if !ok || self == nil || self.image == nil || userErr != nil || !self.image.trust.matchesUser(user) {
		return ErrElevatedGUIUnavailable // no over-the-shoulder credential/profile substitution
	}
	if updateIsProcessInJob.Find() != nil {
		return ErrElevatedGUIUnavailable
	}
	var member int32
	memberOK, _, _ := updateIsProcessInJob.Call(uintptr(native.info.process), uintptr(native.host.job), uintptr(unsafe.Pointer(&member)))
	if memberOK == 0 || member == 0 {
		return ErrElevatedGUIUnavailable
	}
	var actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var returned uint32
	if err := windows.QueryInformationJobObject(native.host.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&actual)), uint32(unsafe.Sizeof(actual)), &returned); err != nil ||
		returned != uint32(unsafe.Sizeof(actual)) || actual.BasicLimitInformation.LimitFlags != native.host.resourceLimits.BasicLimitInformation.LimitFlags ||
		actual.BasicLimitInformation.ActiveProcessLimit != 0 || actual.JobMemoryLimit != 0 || actual.BasicLimitInformation.PerJobUserTimeLimit != 0 {
		return ErrElevatedGUIUnavailable
	}
	state, err = windows.WaitForSingleObject(native.info.process, 0)
	if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return ErrElevatedGUIUnavailable
	}
	return contextError(ctx)
}

func (native *elevatedGUINativeRequest) remoteHandles() [2]uint64 {
	return [2]uint64{uint64(native.jobRemote), uint64(native.parentRemote)}
}
func (native *elevatedGUINativeRequest) alive(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if !createdProcessHandle(native.info.process) {
		return ErrElevatedGUIUnavailable
	}
	state, err := windows.WaitForSingleObject(native.info.process, 0)
	if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return ErrElevatedGUIUnavailable
	}
	return contextError(ctx)
}
func (native *elevatedGUINativeRequest) acceptPipe(ctx context.Context, pipe *uacControlPipe, guard func(context.Context) error) error {
	if err := native.alive(ctx); err != nil {
		return err
	}
	id, err := windows.GetProcessId(native.info.process)
	if err != nil || id == 0 {
		return ErrElevatedGUIUnavailable
	}
	return pipe.accept(ctx, id, guard)
}

func (native *elevatedGUINativeRequest) retire() error {
	if native.tokenAcquiring || native.host.acquiring || native.tokenCloseAttempted && native.token != 0 || native.processCloseAttempted && native.info.process != 0 || native.host.closeAttempted && native.host.job != 0 {
		return ErrHelperCleanup
	}
	if native.callStarted && !native.callReturned || native.comStarted && !native.comReturned || native.comOwned && !native.uninitReturned {
		return ErrHelperCleanup
	}
	native.releasePins()
	if native.callStarted && !native.drained {
		if !createdProcessHandle(native.info.process) {
			if !native.declined {
				return ErrHelperCleanup
			} // no handle is not proof of no effect
		} else {
			if !native.classified || !native.pathMatched {
				if err := native.classify(); err != nil {
					return ErrHelperCleanup
				}
			}
			if !native.assigned {
				if !native.unassignedAbort {
					native.unassignedAbort = true
					native.abortErr = windows.TerminateProcess(native.info.process, 1)
				}
				return errors.Join(ErrHelperCleanup, native.abortErr) // leaf exit cannot prove descendant retirement
			}
			termination := windows.TerminateJobObject(native.host.job, 1)
			deadline := time.Now().Add(workerReapTimeout)
			for {
				state, waitErr := windows.WaitForSingleObject(native.info.process, 0)
				accounting, queryErr := queryJobAccounting(native.host.job)
				if waitErr == nil && state == windows.WAIT_OBJECT_0 && queryErr == nil && accounting.ActiveProcesses == 0 {
					native.drained = true
					break
				}
				if waitErr != nil || queryErr != nil || termination != nil || !time.Now().Before(deadline) {
					return ErrHelperCleanup
				}
				time.Sleep(min(10*time.Millisecond, time.Until(deadline)))
			}
		}
	}
	if err := native.closeReferences(); err != nil {
		return err
	}
	if native.ranBeforeJob {
		return ErrHelperCleanup
	} // preserve the pre-attachment descendant-uncertainty witness
	return nil
}

// Fresh original slots only. An unreturned acquisition or uncertain Close is
// terminal; neither a later call nor another owner may reuse that numeric value.
func (job *elevatedGUIJob) open() error {
	if job.attempted || job.job != 0 {
		return ErrElevatedGUIUnavailable
	}
	job.attempted, job.acquiring = true, true
	var err error
	job.job, err = windows.CreateJobObject(nil, nil)
	job.acquiring = false
	if err != nil || !createdProcessHandle(job.job) {
		return errors.Join(ErrElevatedGUIUnavailable, err)
	}
	return nil
}
func (job *elevatedGUIJob) close() error {
	if job.acquiring {
		return ErrHelperCleanup
	}
	return spawnprep.CloseOnce(&job.job, &job.closeAttempted)
}
func (native *elevatedGUINativeRequest) prepareToken() error {
	if native.tokenAcquiring || native.tokenCloseAttempted {
		return ErrElevatedGUIUnavailable
	}
	if native.tokenAttempted {
		if native.token == 0 || !native.tokenReady {
			return ErrElevatedGUIUnavailable
		}
		return nil
	}
	native.tokenAttempted, native.tokenAcquiring = true, true
	err := windows.OpenProcessToken(native.info.process, windows.TOKEN_QUERY, &native.token)
	native.tokenAcquiring = false
	native.tokenReady = err == nil && native.token != 0
	return err
}
func (native *elevatedGUINativeRequest) closeReferences() error {
	if native.tokenAcquiring {
		return ErrHelperCleanup
	}
	if err := spawnprep.CloseOnce((*windows.Handle)(&native.token), &native.tokenCloseAttempted); err != nil {
		return ErrHelperCleanup
	}
	if err := spawnprep.CloseOnce(&native.info.process, &native.processCloseAttempted); err != nil {
		return ErrHelperCleanup
	}
	if err := native.host.close(); err != nil {
		return ErrHelperCleanup
	}
	return nil
}
