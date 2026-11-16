//go:build windows

package core

import (
	"context"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The target-process value is NEVER closed in the parent's handle table.
// A partial/unknown duplication is reclaimed only by the retained target Job.
type nativeTUNTransfer struct {
	remote  windows.Handle
	pid     uint32
	created uint64
}

func tunActorStamp(handle windows.Handle) (uint32, uint64, error) {
	if !createdProcessHandle(handle) {
		return 0, 0, ErrNativeScopeUnavailable
	}
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return 0, 0, ErrNativeScopeUnavailable
	}
	pid, err := windows.GetProcessId(handle)
	if err != nil || pid == 0 {
		return 0, 0, ErrNativeScopeUnavailable
	}
	var creation, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user) != nil {
		return 0, 0, ErrNativeScopeUnavailable
	}
	stamp := uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	if stamp == 0 {
		return 0, 0, ErrNativeScopeUnavailable
	}
	return pid, stamp, nil
}

func duplicateTUNActor(ctx context.Context, source, target processHost, receipt *nativeTUNTransfer) error {
	actor, actorOK := source.(*windowsProcessHost)
	worker, workerOK := target.(*windowsProcessHost)
	if contextError(ctx) != nil || receipt == nil || receipt.remote != 0 || !actorOK || !workerOK || actor == nil || worker == nil || actor == worker {
		return ErrNativeScopeUnavailable
	}
	err := actor.native.WithAssigned(func(actorJob, actorProcess windows.Handle) error {
		return worker.native.WithAssigned(func(workerJob, workerProcess windows.Handle) error {
			// The reserved helper is one process with the original fixed Job budget.
			expected := ownedJobLimits(ownedProcessLimits{processes: 1, commitBytes: helperCaptureCommitBytes, cleanSystemEnvironment: true})
			var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
			var returned uint32
			if actor.resourceLimits.BasicLimitInformation != expected.BasicLimitInformation || actor.resourceLimits.JobMemoryLimit != expected.JobMemoryLimit ||
				windows.QueryInformationJobObject(actorJob, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), &returned) != nil || returned != uint32(unsafe.Sizeof(limits)) ||
				limits.BasicLimitInformation.LimitFlags != expected.BasicLimitInformation.LimitFlags || limits.BasicLimitInformation.ActiveProcessLimit != 1 || limits.JobMemoryLimit != expected.JobMemoryLimit ||
				limits.PeakJobMemoryUsed > expected.JobMemoryLimit || limits.PeakProcessMemoryUsed > expected.JobMemoryLimit {
				return ErrNativeScopeUnavailable
			}
			// Members disabled by LimitFlags are ignored by Windows; comparing their
			// incidental affinity/priority/workingset values is not a resource proof.
			accounting, err := queryJobAccounting(actorJob)
			if err != nil || accounting.ActiveProcesses != 1 || accounting.TotalProcesses != 1 || accounting.TerminatedProcesses != 0 {
				return ErrNativeScopeUnavailable
			}
			pid, created, err := tunActorStamp(actorProcess)
			if err != nil {
				return err
			}
			workerPID, _, workerErr := tunActorStamp(workerProcess)
			if workerErr != nil || workerPID == pid {
				return ErrNativeScopeUnavailable
			}
			receipt.pid, receipt.created = pid, created
			if err := contextError(ctx); err != nil {
				return err // all preceding native queries may have outlived the original operation
			}
			// Direct API output goes into the retained receipt, even on API failure.
			if windows.DuplicateHandle(windows.CurrentProcess(), actorProcess, workerProcess, &receipt.remote, windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, 0) != nil || !createdProcessHandle(receipt.remote) {
				return ErrNativeScopeUnavailable
			}
			afterPID, afterCreated, err := tunActorStamp(actorProcess)
			if err != nil || afterPID != pid || afterCreated != created || contextError(ctx) != nil {
				return ErrNativeScopeUnavailable
			}
			return nil
		})
	})
	if err != nil {
		return errors.Join(ErrNativeScopeUnavailable, err)
	}
	return nil
}

func (receipt *nativeTUNTransfer) apply(request *workerTUNScopeRequest) {
	request.ActorHandle, request.ActorPID, request.ActorCreated = uint64(receipt.remote), receipt.pid, receipt.created
}

type windowsTUNActor struct {
	handle       windows.Handle
	closeStarted bool
	closeErr     error
}

func newTUNActorResource() tunActorResource { return &windowsTUNActor{} }

func (actor *windowsTUNActor) adopt(request workerTUNScopeRequest) error {
	if actor.handle != 0 || actor.closeStarted || !request.valid() || uint64(windows.Handle(request.ActorHandle)) != request.ActorHandle || !createdProcessHandle(windows.Handle(request.ActorHandle)) {
		return ErrNativeScopeUnavailable
	}
	actor.handle = windows.Handle(request.ActorHandle) // retain BEFORE native validation
	var flags uint32
	if updateGetHandleInformation.Find() != nil {
		return ErrNativeScopeUnavailable
	}
	ok, _, _ := updateGetHandleInformation.Call(uintptr(actor.handle), uintptr(unsafe.Pointer(&flags)))
	if ok == 0 || flags&windows.HANDLE_FLAG_INHERIT != 0 {
		return ErrNativeScopeUnavailable
	}
	pid, created, err := tunActorStamp(actor.handle)
	if err != nil || pid != request.ActorPID || created != request.ActorCreated {
		return ErrNativeScopeUnavailable
	}
	return nil
}

func (actor *windowsTUNActor) alive() bool {
	if actor.closeStarted || !createdProcessHandle(actor.handle) {
		return false
	}
	state, err := windows.WaitForSingleObject(actor.handle, 0)
	return err == nil && state == uint32(windows.WAIT_TIMEOUT)
}

func (actor *windowsTUNActor) Close() error {
	if actor.closeStarted {
		return actor.closeErr
	}
	actor.closeStarted, actor.closeErr = true, ErrNativeScopeUnavailable
	if actor.handle != 0 && windows.CloseHandle(actor.handle) != nil {
		return actor.closeErr
	}
	actor.handle, actor.closeErr = 0, nil
	return nil
}
