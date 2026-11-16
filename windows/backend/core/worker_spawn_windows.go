//go:build windows

package core

import (
	"context"
	"errors"
	"os"
	"runtime"
	"unicode/utf16"

	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

func newOwnedProcess() *engineProcess {
	host := &windowsProcessHost{}
	return &engineProcess{
		ancestor: processGUIAncestor, host: host,
		input: host.preparation.Input(), output: host.preparation.Output(), done: make(chan struct{}),
	}
}

func spawnOwnedProcess(ctx context.Context, executable string, args []string, policy ownedProcessLimits) (process *engineProcess, finalErr error) {
	process = newOwnedProcess()
	finalErr = process.spawn(ctx, executable, args, policy)
	return
}

// Caller publishes this pure owner before the one in-place native attempt.
func (process *engineProcess) spawn(ctx context.Context, executable string, args []string, policy ownedProcessLimits) (finalErr error) {
	if process == nil || process.spawnAttempted || process.unusable || process.drained {
		return ErrEngineWorkerUnavailable
	}
	host, hostOK := process.host.(*windowsProcessHost)
	if !hostOK || host == nil || !host.preparation.Owns(process.input, process.output) {
		return ErrEngineWorkerUnavailable
	}
	process.spawnAttempted = true
	process.successorGUI = policy.successorGUI

	resumed := false
	defer func() {
		if recovered := recover(); recovered != nil || !resumed && finalErr == nil {
			finalErr = errors.New("owned process spawn interrupted") // retain a created owner
		}
	}()
	if err := process.ancestorScope(ctx, false); err != nil {
		return err
	}
	if err := validateOwnedProcessCommand(executable, args); err != nil || policy.processes < 1 || policy.processes > 8 ||
		policy.combinedOutput && (policy.deliveryVerifier || policy.applicationEnvironment || policy.successorGUI || !policy.cleanSystemEnvironment ||
			policy.beforeResume != nil || policy.processes != 1 || policy.commitBytes != helperCaptureCommitBytes || policy.userTime != 0) ||
		policy.userTime < 0 || policy.userTime%ownedProcessUserTimeTick != 0 ||
		policy.userTime != 0 && (!policy.applicationEnvironment && !policy.deliveryVerifier || policy.successorGUI) ||
		policy.deliveryVerifier && (policy.applicationEnvironment || !policy.cleanSystemEnvironment || policy.processes != 1 || policy.beforeResume == nil ||
			policy.commitBytes != updateCodeCheckCommitBytes || policy.userTime != updateCodeCheckUserTime) ||
		policy.applicationEnvironment && (!policy.cleanSystemEnvironment || policy.processes != 1 || policy.beforeResume == nil) ||
		policy.successorGUI && (!policy.applicationEnvironment || policy.commitBytes != 0 || policy.userTime != 0) {
		return errors.New("owned process command or limits invalid")
	}
	if policy.deliveryVerifier {
		if len(args) != 2 || args[0] != deliveryWorkerArgument {
			return errors.New("SDK worker requires its fixed mode")
		}
		if _, err := nativeImagePin(args[1]); err != nil {
			return err
		}
	}
	if policy.applicationEnvironment {
		if policy.successorGUI && !validUpdateGUIArguments(args) ||
			!policy.successorGUI && (len(args) != 2 || !updateCodeCheckArgument(args[0]) ||
				policy.commitBytes != updateCodeCheckCommitBytes || policy.userTime != updateCodeCheckUserTime) {
			return errors.New("updater requires its fixed acceptance mode")
		}
		if _, err := nativeImagePin(args[1]); err != nil {
			return err
		}
	}
	app, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return ErrEngineWorkerUnavailable
	}
	argv := append([]string{executable}, args...)
	commandLine := windows.ComposeCommandLine(argv)
	if len(utf16.Encode([]rune(commandLine)))+1 > 32767 {
		return errors.New("owned process command line exceeds Windows limit")
	}
	command, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return ErrEngineWorkerUnavailable
	}
	// The complete original owner is already retained by the caller.
	if err := host.native.OpenJob(); err != nil {
		return errors.New("engine job creation failed")
	}
	if err := configureOwnedJobLimits(host, policy); err != nil {
		return err
	}
	if err := host.native.WithJob(func(job windows.Handle) error {
		if policy.combinedOutput {
			return host.preparation.PrepareCombined(job)
		}
		return host.preparation.Prepare(job)
	}); err != nil {
		return err
	}
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW | windows.EXTENDED_STARTUPINFO_PRESENT)
	if policy.successorGUI {
		// An enclosing Job that disallows breakaway fails creation. Never retry
		// inside that Job: closing the old GUI must not kill its successor.
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	var environment, workingDirectoryData []uint16
	var environmentPointer, workingDirectory *uint16
	if policy.cleanSystemEnvironment {
		systemDirectory, err := windows.GetSystemDirectory()
		if err != nil {
			return errors.New("native helper system directory unavailable")
		}
		windowsDirectory, err := windows.GetWindowsDirectory()
		if err != nil {
			return errors.New("native helper Windows directory unavailable")
		}
		scope := spawnprep.EnvironmentSystem
		locations := spawnprep.EnvironmentLocations{SystemDirectory: systemDirectory, WindowsDirectory: windowsDirectory}
		if policy.applicationEnvironment {
			scope = spawnprep.EnvironmentPublicStore
			locations.ConfigDirectory, err = os.UserConfigDir()
			if err != nil {
				return errors.New("updater public SDK store location unavailable")
			}
			locations.TemporaryDirectory = os.TempDir()
			if policy.successorGUI {
				scope = spawnprep.EnvironmentGUI
				locations.CacheDirectory, err = os.UserCacheDir()
				if err != nil {
					return errors.New("update GUI cache location unavailable")
				}
			}
		}
		environment, workingDirectoryData, err = spawnprep.BuildEnvironment(scope, locations)
		if err != nil {
			return err
		}
		environmentPointer, workingDirectory = &environment[0], &workingDirectoryData[0]
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	err = host.preparation.WithStartup(func(startup *windows.StartupInfoEx) error {
		startup.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
		startup.ShowWindow = windows.SW_HIDE
		return host.native.CreateSuspended(func(information *windows.ProcessInformation) error {
			return windows.CreateProcess(app, command, nil, nil, true, flags, environmentPointer, workingDirectory, &startup.StartupInfo, information)
		})
	})
	runtime.KeepAlive(&host.preparation)
	runtime.KeepAlive(environment)
	runtime.KeepAlive(workingDirectoryData)
	if err != nil {
		return errors.New("engine process creation failed")
	}
	host.EnsureWait(process)
	if err := host.native.ConfirmJob(); err != nil {
		return errors.New("engine creation-time job membership unconfirmed") // child is still suspended
	}
	if err := host.ClosePreparation(); err != nil {
		return err // still suspended; no Resume with unknown auxiliary custody
	}
	if err := contextError(ctx); err != nil {
		return err // assigned but still suspended; caller retains/retires it
	}
	if policy.beforeResume != nil {
		if err := policy.beforeResume(ctx, host); err != nil {
			return err // image/scope failure never resumes the child
		}
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if policy.userTime != 0 {
		if err := host.native.WithAssigned(func(job, process windows.Handle) error {
			return validateOwnedJobResources(job, host.resourceLimits, 1)
		}); err != nil {
			return err // still suspended; no resource-limit fallback
		}
	}
	if err := process.ancestorScope(ctx, true); err != nil {
		return err
	}
	if err := host.native.Resume(); err != nil {
		return errors.New("engine process resume failed")
	}
	resumed = true
	return process.ancestorScope(ctx, true)
}
