//go:build windows

package spawnprep

import (
	"errors"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

type BrokerJobRole uint8

const (
	BrokerJobAncestor BrokerJobRole = iota + 1
	BrokerJobHelper
	BrokerJobGUI
)

// Initial, unmeasured helper-only ceilings. The GUI must leave this resource
// Job while remaining in the ancestor Job; never weaken flags to permit spawn.
const BrokerHelperCommitBytes uintptr = 128 << 20
const BrokerHelperUserTime = 20 * time.Second

func BrokerJobInformation(role BrokerJobRole) (windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, error) {
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	switch role {
	case BrokerJobAncestor:
		// Holds the whole descendant lifetime without imposing helper limits.
	case BrokerJobHelper:
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK |
			windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS | windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_JOB_TIME
		limits.BasicLimitInformation.ActiveProcessLimit = 1
		limits.BasicLimitInformation.PerJobUserTimeLimit = int64(BrokerHelperUserTime / UserTimeTick)
		limits.JobMemoryLimit = BrokerHelperCommitBytes
	case BrokerJobGUI:
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	default:
		return windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}, ErrLimits
	}
	return limits, nil
}

// Borrow only a fresh original Job and retained expected slot. Publishing the
// expected value burns this configure attempt even if a later native call fails.
// No retry may reset CPU accounting or replace an uncertain original owner.
func ConfigureBrokerJob(job windows.Handle, role BrokerJobRole, expected *windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION) error {
	if !nativelease.ValidHandleValue(job) || expected == nil || expected.BasicLimitInformation.LimitFlags != 0 {
		return ErrLimits
	}
	limits, err := BrokerJobInformation(role)
	if err != nil {
		return err
	}
	*expected = limits
	accounting, err := QueryAccounting(job)
	if err != nil || accounting.TotalProcesses != 0 || accounting.ActiveProcesses != 0 || accounting.TerminatedProcesses != 0 {
		return errors.Join(ErrLimits, err)
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(expected)), uint32(unsafe.Sizeof(*expected))); err != nil {
		return errors.Join(ErrLimits, err)
	}
	if role == BrokerJobHelper {
		var action jobTimeAction
		if _, err := windows.SetInformationJobObject(job, windows.JobObjectEndOfJobTimeInformation, uintptr(unsafe.Pointer(&action)), uint32(unsafe.Sizeof(action))); err != nil {
			return errors.Join(ErrLimits, err)
		}
	}
	_, err = QueryBrokerJobLimits(job, role)
	return err
}

func QueryBrokerJobLimits(job windows.Handle, role BrokerJobRole) (actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, err error) {
	expected, err := BrokerJobInformation(role)
	if err != nil || !nativelease.ValidHandleValue(job) {
		return actual, ErrLimits
	}
	var returned uint32
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&actual)), uint32(unsafe.Sizeof(actual)), &returned); err != nil || returned != uint32(unsafe.Sizeof(actual)) {
		return actual, errors.Join(ErrLimits, err)
	}
	if actual.BasicLimitInformation.LimitFlags != expected.BasicLimitInformation.LimitFlags ||
		actual.BasicLimitInformation.ActiveProcessLimit != expected.BasicLimitInformation.ActiveProcessLimit ||
		actual.BasicLimitInformation.PerJobUserTimeLimit != expected.BasicLimitInformation.PerJobUserTimeLimit || actual.JobMemoryLimit != expected.JobMemoryLimit {
		return actual, ErrLimits
	}
	if role == BrokerJobHelper {
		var action jobTimeAction
		returned = 0
		if err := windows.QueryInformationJobObject(job, windows.JobObjectEndOfJobTimeInformation, uintptr(unsafe.Pointer(&action)), uint32(unsafe.Sizeof(action)), &returned); err != nil || returned != uint32(unsafe.Sizeof(action)) || action.action != 0 {
			return actual, errors.Join(ErrLimits, err)
		}
	}
	return actual, nil
}
