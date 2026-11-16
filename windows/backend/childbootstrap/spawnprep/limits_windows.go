//go:build windows

package spawnprep

import (
	"errors"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

type jobTimeAction struct{ action uint32 } // zero DWORD: terminate, never reset-and-run

const boundedVerifierFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS | windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_JOB_TIME

// Information projects values only. The platform still selects a fixed role and
// checks image/source/compiled caps; this struct grants no execution authority.
func (policy JobLimits) Information() windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	limits.BasicLimitInformation.ActiveProcessLimit = policy.Processes
	if policy.SuccessorGUI {
		limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
		limits.BasicLimitInformation.ActiveProcessLimit = 0
	}
	if policy.MemoryBytes != 0 {
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		limits.JobMemoryLimit = policy.MemoryBytes
	}
	if policy.UserTime != 0 {
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_TIME
		limits.BasicLimitInformation.PerJobUserTimeLimit = int64(policy.UserTime / UserTimeTick)
	}
	return limits
}

// Configure borrows a fresh empty original Job and the caller's retained expected
// output slot. Never call it on a running Job or replace/reset that expected slot:
// reapplying Job time limits could grant a fresh CPU allowance.
func (policy JobLimits) Configure(job windows.Handle, expected *windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION) error {
	if !nativelease.ValidHandleValue(job) || expected == nil || expected.BasicLimitInformation.LimitFlags != 0 ||
		policy.Processes < 1 || policy.Processes > 8 || policy.UserTime < 0 || policy.UserTime%UserTimeTick != 0 ||
		policy.SuccessorGUI && (policy.Processes != 1 || policy.MemoryBytes != 0 || policy.UserTime != 0) ||
		policy.UserTime != 0 && (policy.Processes != 1 || policy.MemoryBytes == 0) {
		return ErrLimits
	}
	*expected = policy.Information()
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(expected)), uint32(unsafe.Sizeof(*expected))); err != nil {
		return errors.Join(ErrLimits, err)
	}
	if policy.UserTime == 0 {
		return nil
	}
	var action jobTimeAction
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectEndOfJobTimeInformation, uintptr(unsafe.Pointer(&action)), uint32(unsafe.Sizeof(action))); err != nil {
		return errors.Join(ErrLimits, err)
	}
	_, err := QueryVerifierLimits(job, *expected)
	return err
}

func QueryVerifierLimits(job windows.Handle, expected windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION) (actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, err error) {
	if expected.BasicLimitInformation.LimitFlags != boundedVerifierFlags || expected.JobMemoryLimit == 0 || expected.BasicLimitInformation.PerJobUserTimeLimit <= 0 || expected.BasicLimitInformation.ActiveProcessLimit != 1 {
		return actual, ErrLimits
	}
	var returned uint32
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&actual)), uint32(unsafe.Sizeof(actual)), &returned); err != nil || returned != uint32(unsafe.Sizeof(actual)) {
		return actual, ErrLimits
	}
	// Exact flags reject either kind of breakaway and loss of any required limit.
	if actual.BasicLimitInformation.LimitFlags != expected.BasicLimitInformation.LimitFlags ||
		actual.BasicLimitInformation.ActiveProcessLimit != expected.BasicLimitInformation.ActiveProcessLimit ||
		actual.BasicLimitInformation.PerJobUserTimeLimit != expected.BasicLimitInformation.PerJobUserTimeLimit ||
		actual.JobMemoryLimit != expected.JobMemoryLimit {
		return actual, ErrLimits
	}
	var action jobTimeAction
	returned = 0
	if err := windows.QueryInformationJobObject(job, windows.JobObjectEndOfJobTimeInformation, uintptr(unsafe.Pointer(&action)), uint32(unsafe.Sizeof(action)), &returned); err != nil || returned != uint32(unsafe.Sizeof(action)) || action.action != 0 {
		return actual, ErrLimits
	}
	return actual, nil
}

// ValidateVerifierResources also counts exited/refused/terminated members and
// observed user-time/memory peaks. Zero exit alone does not grant a receipt.
// Memory is committed Job memory, not RSS; kernel/user/wall time are distinct.
func ValidateVerifierResources(job windows.Handle, expected windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, active uint32) error {
	actual, err := QueryVerifierLimits(job, expected)
	if err != nil {
		return err
	}
	accounting, err := QueryAccounting(job)
	if err != nil {
		return err
	}
	if active > 1 || accounting.ActiveProcesses != active || accounting.TotalProcesses != 1 || accounting.TerminatedProcesses != 0 ||
		accounting.TotalUserTime < 0 || accounting.TotalUserTime > expected.BasicLimitInformation.PerJobUserTimeLimit ||
		actual.PeakJobMemoryUsed > expected.JobMemoryLimit || actual.PeakProcessMemoryUsed > expected.JobMemoryLimit {
		return ErrLimits
	}
	return nil
}
