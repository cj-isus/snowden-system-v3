//go:build windows

package core

import (
	"context"
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

const updateJobQueryAccess = 0x0004 // JOB_OBJECT_QUERY

var updateKernel32 = windows.NewLazySystemDLL("kernel32.dll")
var updateIsProcessInJob = updateKernel32.NewProc("IsProcessInJob")
var updateGetHandleInformation = updateKernel32.NewProc("GetHandleInformation")

func closeUpdateGUIStreams() error {
	// Remove inherited standard slots before closing their Go files: a later
	// native GUI child must not see a stale/reused pipe-handle value.
	if err := windows.SetStdHandle(windows.STD_INPUT_HANDLE, 0); err != nil {
		return ErrUpdateGUIBootstrap
	}
	if err := windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, 0); err != nil {
		return ErrUpdateGUIBootstrap
	}
	return errors.Join(os.Stdin.Close(), os.Stdout.Close())
}

// Called after Resume, before the fixed seed is sent. Duplicates belong to this
// original target process, including partial failure. A never-resumed child
// must not receive the Job reference that would retain its own lifetime.
// Parent retirement terminates/waits that original process/Job; it never closes
// numeric target-process values in its own handle table or reopens a saved PID.
func prepareUpdateGUIHandles(ctx context.Context, host processHost) (handles [2]uint64, finalErr error) {
	if err := contextError(ctx); err != nil {
		return handles, errors.Join(ErrUpdateGUIBootstrap, err)
	}
	owned, ok := host.(*windowsProcessHost)
	if !ok || owned == nil {
		return handles, ErrUpdateGUIBootstrap
	}
	finalErr = owned.native.WithAssigned(func(job, process windows.Handle) error {
		for index, item := range []struct {
			source windows.Handle
			access uint32
		}{{job, updateJobQueryAccess}, {windows.CurrentProcess(), windows.SYNCHRONIZE | windows.PROCESS_QUERY_LIMITED_INFORMATION}} {
			if err := contextError(ctx); err != nil {
				return err // prior target handles remain in this exact partial receipt
			}
			var remote windows.Handle
			err := windows.DuplicateHandle(windows.CurrentProcess(), item.source, process, &remote, item.access, false, 0)
			handles[index] = uint64(remote)
			if err != nil || !createdProcessHandle(remote) {
				return ErrUpdateGUIBootstrap
			}
		}
		return contextError(ctx)
	})
	if finalErr != nil {
		return handles, errors.Join(ErrUpdateGUIBootstrap, finalErr)
	}
	return handles, nil
}

func (lease *updateGUINativeLease) withParent(use func(windows.Handle) error) error {
	if lease == nil || lease.mode != guiLeaseLegacy {
		return ErrUpdateGUIBootstrap
	}
	return updateGUILeaseError(lease.native.WithParent(use))
}
