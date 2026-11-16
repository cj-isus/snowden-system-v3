//go:build windows

package spawnprep

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Accounting is JOBOBJECT_BASIC_ACCOUNTING_INFORMATION: four LARGE_INTEGERs and
// four DWORDs. Values are measurements, never resource-policy or image authority.
type Accounting struct {
	TotalUserTime, TotalKernelTime, PeriodUserTime, PeriodKernelTime int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}

// QueryAccounting borrows an original Job handle, or zero for the calling Job.
// It neither opens a named Job/PID nor resets its accounting or CPU allowance.
func QueryAccounting(job windows.Handle) (accounting Accounting, err error) {
	var returned uint32
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), &returned); err != nil || returned != uint32(unsafe.Sizeof(accounting)) {
		return accounting, ErrProcess
	}
	return accounting, nil
}
