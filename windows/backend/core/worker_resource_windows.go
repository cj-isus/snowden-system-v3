//go:build windows

package core

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

func nativeJobLimits(policy ownedProcessLimits) spawnprep.JobLimits {
	return spawnprep.JobLimits{Processes: policy.processes, MemoryBytes: policy.commitBytes, UserTime: policy.userTime, SuccessorGUI: policy.successorGUI}
}

func ownedJobLimits(policy ownedProcessLimits) windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
	return nativeJobLimits(policy).Information()
}

func configureOwnedJobLimits(host *windowsProcessHost, policy ownedProcessLimits) error {
	if host == nil || host.native.Created() || host.resourceLimits.BasicLimitInformation.LimitFlags != 0 {
		return errors.New("owned job limits already initialized or owner unavailable")
	}
	return host.native.WithJob(func(job windows.Handle) error {
		return configureOriginalJobLimits(job, &host.resourceLimits, policy)
	})
}

// Shared unchanged policy for the suspended owner and the separately retained
// running UAC request. This does not transfer either process or Job ownership.
func configureOriginalJobLimits(job windows.Handle, limits *windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, policy ownedProcessLimits) error {
	return nativeJobLimits(policy).Configure(job, limits)
}

func queryJobResourceLimits(job windows.Handle, expected windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION) (windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, error) {
	return spawnprep.QueryVerifierLimits(job, expected)
}

func queryJobAccounting(job windows.Handle) (jobAccounting, error) {
	return spawnprep.QueryAccounting(job)
}

func validateOwnedJobResources(job windows.Handle, expected windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, active uint32) error {
	return spawnprep.ValidateVerifierResources(job, expected, active)
}

func (host *windowsProcessHost) ValidateResourceUsage() error {
	if host == nil {
		return errors.New("original bounded verifier job unavailable")
	}
	return host.native.WithAssigned(func(job, process windows.Handle) error {
		return validateOwnedJobResources(job, host.resourceLimits, 0)
	})
}

func updateCodeCheckResourcesReady() bool {
	// NULL queries the calling process's immediate Job, including nested Jobs.
	// No inherited/opened Job handle or caller-supplied PID/name is needed.
	// This is a resource precondition, not authentication of the parent.
	expected := ownedJobLimits(ownedProcessLimits{processes: 1, commitBytes: updateCodeCheckCommitBytes, userTime: updateCodeCheckUserTime})
	return validateOwnedJobResources(0, expected, 1) == nil
}
