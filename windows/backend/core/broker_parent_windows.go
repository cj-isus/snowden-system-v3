//go:build windows

package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"github.com/snowden-system/windows/backend/update"
	"golang.org/x/sys/windows"
)

// launch retains the original Shell structure/process/token/ancestor Job. The
// helper resource Job is a different original slot, never copied or substituted.
type brokerParentNative struct {
	launch       elevatedGUINativeRequest
	helper       elevatedGUIJob
	helperRemote windows.Handle // remote helper value, never closed in this App

	attempted                                                        bool
	ancestorAssignAttempted, ancestorAssignPending, ancestorAssigned bool
	helperAssignAttempted, helperAssignPending, helperAssigned       bool
	remoteAttempted, remoteReturned                                  [3]bool
	remotePending                                                    bool

	waitAttempted, waitPending, waitReturned, exitConfirmed bool
	exitCode                                                uint32

	retireAttempted, retirePending, retireReturned, drained bool
	retireErr                                               error
	retireSteps                                             [3]brokerParentRetireStep
}

type brokerParentRetireStep struct {
	attempted, pending, returned bool
	err                          error
}

// Closed role mapping, not a caller-injected native action. Each actual result
// is retained before the next syscall; a nonreturn cannot erase the prior step.
func (native *brokerParentNative) terminate(index int) error {
	if index < 0 || index >= len(native.retireSteps) {
		return ErrHelperCleanup
	}
	step := &native.retireSteps[index]
	if step.attempted {
		return ErrHelperCleanup
	}
	step.attempted, step.pending = true, true
	switch index {
	case 0:
		step.err = windows.TerminateJobObject(native.launch.host.job, 1)
	case 1:
		step.err = windows.TerminateJobObject(native.helper.job, 1)
	case 2:
		step.err = windows.TerminateProcess(native.launch.info.process, 1)
	}
	step.pending, step.returned = false, true
	return step.err
}

func (native *brokerParentNative) execute(ctx context.Context, image nativeImageLease, resume bool, protocol childbootstrap.BrokerProtocol, check func(context.Context) error) error {
	if native.attempted || !protocol.PinnedControl() {
		return ErrElevatedGUIUnavailable
	}
	native.attempted = true
	pair, ok := image.(*windowsNativePair)
	if !ok || pair == nil {
		return ErrNativeImageUnavailable
	}
	if err := pair.pair.WithPaths(ctx, check, func(app, _ string) error {
		// The minimal helper uses this compiled sibling name, never an argv path.
		if !strings.EqualFold(filepath.Base(app), update.ApplicationFile) {
			return ErrNativeImageUnavailable
		}
		return nativeProcessPathMatches(ctx, windows.CurrentProcess(), app)
	}); err != nil {
		return err
	}
	if err := native.launch.host.open(); err != nil {
		return err
	}
	if err := spawnprep.ConfigureBrokerJob(native.launch.host.job, spawnprep.BrokerJobAncestor, &native.launch.host.resourceLimits); err != nil {
		return err
	}
	if err := native.helper.open(); err != nil {
		return err
	}
	if err := spawnprep.ConfigureBrokerJob(native.helper.job, spawnprep.BrokerJobHelper, &native.helper.resourceLimits); err != nil {
		return err
	}
	args, err := protocol.Arguments(resume)
	if err != nil {
		return err
	}
	if err := native.launch.launchShell(ctx, image, args, check); err != nil {
		return err
	}
	if err := native.launch.classify(); err != nil {
		return err
	}
	if err := windows.SetHandleInformation(native.launch.info.process, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return err
	}
	if err := brokerMembership(native.launch.info.process, 0, false); err != nil {
		return err
	}
	if err := childbootstrap.CheckScope(ctx, check); err != nil {
		return err
	}
	native.ancestorAssignAttempted, native.ancestorAssignPending = true, true
	err = windows.AssignProcessToJobObject(native.launch.host.job, native.launch.info.process)
	native.ancestorAssignPending, native.ancestorAssigned = false, err == nil
	if err != nil {
		return err
	}
	if err := childbootstrap.CheckScope(ctx, check); err != nil {
		return err
	}
	native.helperAssignAttempted, native.helperAssignPending = true, true
	err = windows.AssignProcessToJobObject(native.helper.job, native.launch.info.process)
	native.helperAssignPending, native.helperAssigned = false, err == nil
	if err != nil {
		return err
	}
	if err := childbootstrap.CheckScope(ctx, check); err != nil {
		return err
	}
	if err := image.Validate(ctx); err != nil {
		return err
	}
	// API writes into retained target-value slots, including a partial error.
	for index := range native.remoteAttempted {
		if err := childbootstrap.CheckScope(ctx, check); err != nil {
			return err
		}
		if err := native.duplicate(index); err != nil {
			return err
		}
	}
	if !protocol.ValidHandles(native.remoteHandles()) {
		return ErrElevatedGUIUnavailable
	}
	if err := native.validate(ctx, image); err != nil {
		return err
	}
	return childbootstrap.CheckScope(ctx, check)
}

func (native *brokerParentNative) duplicate(index int) error {
	if index < 0 || index >= len(native.remoteAttempted) || native.remotePending || native.remoteAttempted[index] {
		return ErrElevatedGUIUnavailable
	}
	var source windows.Handle
	var target *windows.Handle
	access := uint32(updateJobQueryAccess)
	switch index {
	case 0:
		source, target = native.launch.host.job, &native.launch.jobRemote
	case 1:
		source, target = native.helper.job, &native.helperRemote
	case 2:
		source, target = windows.CurrentProcess(), &native.launch.parentRemote
		access = windows.SYNCHRONIZE | windows.PROCESS_QUERY_LIMITED_INFORMATION
	}
	native.remoteAttempted[index], native.remotePending = true, true
	err := windows.DuplicateHandle(windows.CurrentProcess(), source, native.launch.info.process, target, access, false, 0)
	native.remotePending, native.remoteReturned[index] = false, true
	return err
}

func brokerMembership(process, job windows.Handle, want bool) error {
	if updateIsProcessInJob.Find() != nil {
		return ErrElevatedGUIUnavailable
	}
	var member int32
	ok, _, _ := updateIsProcessInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 || (member != 0) != want {
		return ErrElevatedGUIUnavailable
	}
	return nil
}

func (native *brokerParentNative) validate(ctx context.Context, image nativeImageLease) error {
	root := &native.launch
	if !root.callReturned || !root.callOK || !root.pathMatched || !root.uninitReturned || root.pinned ||
		!native.ancestorAssigned || !native.helperAssigned || native.ancestorAssignPending || native.helperAssignPending ||
		native.remotePending || native.waitAttempted || root.processCloseAttempted || root.host.closeAttempted || native.helper.closeAttempted {
		return ErrElevatedGUIUnavailable
	}
	if err := native.alive(ctx); err != nil {
		return err
	}
	pair, ok := image.(*windowsNativePair)
	if !ok || pair == nil {
		return ErrNativeImageUnavailable
	}
	if err := image.Validate(ctx); err != nil {
		return err
	}
	if err := pair.pair.MatchProcess(ctx, root.info.process, false, contextError); err != nil {
		return err
	}
	if err := root.prepareToken(); err != nil {
		return err
	}
	user, err := fileacl.UserSID(root.token)
	if err != nil || !root.token.IsElevated() || !pair.pair.MatchesUser(user) {
		return ErrElevatedGUIUnavailable
	}
	for _, job := range []windows.Handle{root.host.job, native.helper.job} {
		if err := brokerMembership(root.info.process, job, true); err != nil {
			return err
		}
	}
	if err := native.checkJobs(false); err != nil {
		return err
	}
	return native.alive(ctx)
}

func (native *brokerParentNative) checkJobs(exited bool) error {
	if _, err := spawnprep.QueryBrokerJobLimits(native.launch.host.job, spawnprep.BrokerJobAncestor); err != nil {
		return err
	}
	limits, err := spawnprep.QueryBrokerJobLimits(native.helper.job, spawnprep.BrokerJobHelper)
	if err != nil {
		return err
	}
	root, rootErr := spawnprep.QueryAccounting(native.launch.host.job)
	helper, helperErr := spawnprep.QueryAccounting(native.helper.job)
	active := uint32(1)
	if exited {
		active = 0
	}
	if rootErr != nil || helperErr != nil || helper.TotalProcesses != 1 || helper.ActiveProcesses != active || helper.TerminatedProcesses != 0 ||
		helper.TotalUserTime < 0 || helper.TotalUserTime > limits.BasicLimitInformation.PerJobUserTimeLimit ||
		limits.PeakJobMemoryUsed > limits.JobMemoryLimit || limits.PeakProcessMemoryUsed > limits.JobMemoryLimit ||
		root.TotalProcesses < 1 || root.TotalProcesses > 2 || root.ActiveProcesses < 1 || root.ActiveProcesses > 2 || root.TerminatedProcesses != 0 {
		return ErrElevatedGUIUnavailable
	}
	if exited && (root.TotalProcesses != 2 || root.ActiveProcesses != 1) {
		return ErrElevatedGUIUnavailable
	}
	return nil
}

func (native *brokerParentNative) remoteHandles() [4]uint64 {
	return [4]uint64{uint64(native.launch.jobRemote), uint64(native.helperRemote), uint64(native.launch.parentRemote), 0}
}

func (native *brokerParentNative) alive(ctx context.Context) error {
	if native.waitAttempted || native.launch.processCloseAttempted {
		return ErrElevatedGUIUnavailable
	}
	return native.launch.alive(ctx)
}

func (native *brokerParentNative) acceptPipe(ctx context.Context, pipe *controlpipe.BrokerPipe, check func(context.Context) error) error {
	if err := native.alive(ctx); err != nil {
		return err
	}
	id, err := windows.GetProcessId(native.launch.info.process)
	if err != nil || id == 0 {
		return ErrElevatedGUIUnavailable
	}
	return pipe.Accept(ctx, id, check) // comparison only, no PID acquisition
}

func (native *brokerParentNative) awaitExit(ctx context.Context, image nativeImageLease, check func(context.Context) error) error {
	if native.waitAttempted || native.launch.processCloseAttempted || !native.helperAssigned || !native.ancestorAssigned {
		return ErrElevatedGUIUnavailable
	}
	native.waitAttempted = true
	deadline, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		if err := childbootstrap.CheckScope(deadline, check); err != nil {
			return err
		}
		native.waitPending = true
		state, err := windows.WaitForSingleObject(native.launch.info.process, 100)
		native.waitPending, native.waitReturned = false, true
		if err != nil || state != windows.WAIT_OBJECT_0 && state != uint32(windows.WAIT_TIMEOUT) {
			return errors.Join(ErrElevatedGUIUnavailable, err)
		}
		if state == windows.WAIT_OBJECT_0 {
			native.exitConfirmed = true // actual fact precedes any later guard/error
			if err := windows.GetExitCodeProcess(native.launch.info.process, &native.exitCode); err != nil {
				return err
			}
			if native.exitCode != 0 {
				return ErrElevatedGUIUnavailable
			}
			break
		}
	}
	if err := image.Validate(deadline); err != nil {
		return err
	}
	if err := native.checkJobs(true); err != nil {
		return err
	}
	return childbootstrap.CheckScope(deadline, check)
}

func (native *brokerParentNative) retire() error {
	root := &native.launch
	if root.callStarted && !root.callReturned || root.comStarted && !root.comReturned || root.comOwned && !root.uninitReturned ||
		root.host.acquiring || native.helper.acquiring || root.tokenAcquiring || native.ancestorAssignPending || native.helperAssignPending ||
		native.remotePending || native.waitPending || native.retirePending || root.processCloseAttempted && root.info.process != 0 ||
		root.tokenCloseAttempted && root.token != 0 || root.host.closeAttempted && root.host.job != 0 || native.helper.closeAttempted && native.helper.job != 0 {
		return ErrHelperCleanup
	}
	root.releasePins()
	if root.callStarted && !native.drained {
		if !createdProcessHandle(root.info.process) {
			if !root.declined {
				return ErrHelperCleanup
			}
		} else {
			if !root.pathMatched {
				if err := root.classify(); err != nil {
					return ErrHelperCleanup
				}
			}
			if !native.retireAttempted {
				native.retireAttempted, native.retirePending = true, true
				// Both original Jobs, even after failed/uncertain assignment. Root
				// termination covers the known helper, never an unknown descendant.
				a := native.terminate(0)
				b := native.terminate(1)
				state, waitErr := windows.WaitForSingleObject(root.info.process, 0)
				var leaf error
				if waitErr != nil || state != windows.WAIT_OBJECT_0 {
					leaf = native.terminate(2)
				}
				native.retirePending, native.retireReturned = false, true
				native.retireErr = errors.Join(a, b, leaf)
			}
			if !native.retireReturned {
				return ErrHelperCleanup
			}
			deadline := time.Now().Add(workerReapTimeout)
			for {
				state, waitErr := windows.WaitForSingleObject(root.info.process, 0)
				a, aErr := spawnprep.QueryAccounting(root.host.job)
				b, bErr := spawnprep.QueryAccounting(native.helper.job)
				if waitErr == nil && state == windows.WAIT_OBJECT_0 && aErr == nil && bErr == nil && a.ActiveProcesses == 0 && b.ActiveProcesses == 0 {
					native.drained = true
					break
				}
				if waitErr != nil || aErr != nil || bErr != nil || native.retireErr != nil || !time.Now().Before(deadline) {
					return ErrHelperCleanup
				}
				time.Sleep(min(10*time.Millisecond, time.Until(deadline)))
			}
		}
	}
	if err := native.helper.close(); err != nil {
		return ErrHelperCleanup
	}
	if err := root.closeReferences(); err != nil {
		return ErrHelperCleanup
	}
	if root.ranBeforeJob {
		return ErrHelperCleanup
	} // even zero exit/empty Jobs cannot erase the initial gap
	return nil
}
