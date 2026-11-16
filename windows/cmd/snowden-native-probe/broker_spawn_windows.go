//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

var brokerIsInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
var brokerKnownFolder = windows.NewLazySystemDLL("shell32.dll").NewProc("SHGetKnownFolderPath")

const brokerJobQueryAccess = 0x0004 // JOB_OBJECT_QUERY, same installed SDK value as the App adapter

type brokerSuccessor struct {
	process       spawnprep.Process
	pipes         spawnprep.Pipes
	pipe          *controlpipe.BrokerPipe
	protocol      childbootstrap.BrokerProtocol
	finalExchange controlpipe.ExchangeReceipt
	limits        windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	remote        [4]windows.Handle // values in the GUI table, never helper-owned handles

	duplicateAttempted, duplicateReturned [4]bool
	duplicatePending                      bool

	token                                                         windows.Token
	tokenAttempted, tokenPending, tokenReady, tokenCloseAttempted bool

	executable, command, environment, directory []uint16
	folders                                     [2]brokerFolderPath

	prepareAttempted, preparePending, ready, preparationClosed                bool
	commitAttempted, commitPending, commitReturned, ackObserved, serverClosed bool
	retireAttempted, retirePending, retireReturned, retired                   bool
	terminationErr                                                            error
}

// The x/sys convenience method hides its CoTaskMem output and uses an unbounded
// UTF16 scan. Keep each original output/one-attempt free here and cap the copy.
type brokerFolderPath struct {
	path                                               *uint16
	attempted, pending, returned, freeAttempted, freed bool
	acquired                                           bool
}

func (folder *brokerFolderPath) read(index int) (string, error) {
	if folder.attempted || index < 0 || index > 1 {
		return "", errBrokerHelper
	}
	if err := brokerKnownFolder.Find(); err != nil {
		return "", err
	}
	id := windows.FOLDERID_RoamingAppData
	if index == 1 {
		id = windows.FOLDERID_LocalAppData
	}
	folder.attempted, folder.pending = true, true
	result, _, _ := brokerKnownFolder.Call(uintptr(unsafe.Pointer(id)), 0, 0, uintptr(unsafe.Pointer(&folder.path)))
	folder.pending, folder.returned = false, true
	folder.acquired = result == 0 && folder.path != nil
	runtime.KeepAlive(folder)
	if result != 0 || folder.path == nil {
		return "", errBrokerHelper
	}
	var buffer [2049]uint16
	for index := range buffer {
		// SHGetKnownFolderPath supplies a native NUL-terminated allocation.
		// Read one element at a time, stopping before any byte beyond NUL.
		value := *(*uint16)(unsafe.Add(unsafe.Pointer(folder.path), index*2))
		if value == 0 {
			if index == 0 {
				return "", errBrokerHelper
			}
			return windows.UTF16ToString(buffer[:index]), nil
		}
		buffer[index] = value
	}
	return "", errBrokerHelper
}

func (folder *brokerFolderPath) close() error {
	if folder.pending || folder.attempted && !folder.returned {
		return errBrokerHelper
	}
	if folder.freed || folder.path == nil {
		return nil
	}
	if !folder.acquired || folder.freeAttempted {
		return errBrokerHelper
	}
	folder.freeAttempted = true
	windows.CoTaskMemFree(unsafe.Pointer(folder.path))
	folder.path, folder.freed = nil, true // actual return only, never a defer receipt
	return nil
}

func (child *brokerSuccessor) closeFolders() error {
	return errors.Join(child.folders[0].close(), child.folders[1].close())
}

func (child *brokerSuccessor) prepare(ctx context.Context, session *brokerHelperSession, resume bool) error {
	if child.prepareAttempted {
		return errBrokerHelper
	}
	child.prepareAttempted, child.preparePending = true, true
	err := child.prepareGUI(ctx, session, resume)
	child.preparePending = false // only an actual return; nonreturn retains partial original owners
	child.ready = err == nil
	return err
}

func (child *brokerSuccessor) prepareGUI(ctx context.Context, session *brokerHelperSession, resume bool) error {
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	var binding [32]byte
	if _, err := rand.Read(binding[:]); err != nil {
		return err
	}
	var err error
	child.protocol, err = childbootstrap.NewBrokerProtocol(childbootstrap.BrokerSuccessor, binding)
	if err != nil {
		return err
	}
	child.pipe, err = controlpipe.NewBrokerPipe(child.protocol)
	if err != nil {
		return err
	}
	if err := child.pipe.Listen(ctx, brokerScope); err != nil {
		return err
	}
	if err := child.process.OpenJob(); err != nil {
		return err
	}
	if err := child.process.WithJob(func(job windows.Handle) error {
		if err := spawnprep.ConfigureBrokerJob(job, spawnprep.BrokerJobGUI, &child.limits); err != nil {
			return err
		}
		return child.pipes.PrepareDetached(job)
	}); err != nil {
		return err
	}
	if err := child.prepareEnvironment(); err != nil {
		return err
	}
	args, err := child.protocol.Arguments(resume)
	if err != nil {
		return err
	}
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	err = session.pair.WithPaths(ctx, brokerScope, func(application, _ string) error {
		var err error
		child.executable, err = windows.UTF16FromString(application)
		if err != nil {
			return err
		}
		child.command, err = windows.UTF16FromString(windows.ComposeCommandLine(append([]string{application}, args...)))
		if err != nil || len(child.command) > 32767 {
			return errBrokerHelper
		}
		return child.pipes.WithStartup(func(startup *windows.StartupInfoEx) error {
			startup.Flags |= windows.STARTF_USESHOWWINDOW
			startup.ShowWindow = windows.SW_SHOWNORMAL
			return child.process.CreateSuspended(func(information *windows.ProcessInformation) error {
				if err := brokerScope(ctx); err != nil {
					return err
				}
				flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_BREAKAWAY_FROM_JOB)
				return windows.CreateProcess(&child.executable[0], &child.command[0], nil, nil, true, flags,
					&child.environment[0], &child.directory[0], &startup.StartupInfo, information)
			})
		})
	})
	runtime.KeepAlive(child) // original immutable vectors survive the synchronous create call
	if err != nil {
		return err
	}
	if err := child.process.ConfirmJob(); err != nil {
		return err
	}
	if err := child.pipes.ClosePreparation(); err != nil {
		return err
	}
	child.preparationClosed = true
	// Effective ancestry and departure from the helper resource Job, not flags.
	if err := child.validate(ctx, session); err != nil {
		return err
	}
	if err := child.process.WithAssigned(func(job, process windows.Handle) error {
		return session.lease.WithChain(ctx, brokerScope, func(ancestor, helperJob, parent, origin windows.Handle) error {
			if parent != origin {
				return errBrokerHelper
			}
			for index, source := range [4]windows.Handle{job, windows.CurrentProcess(), ancestor, parent} {
				access := uint32(windows.SYNCHRONIZE | windows.PROCESS_QUERY_LIMITED_INFORMATION)
				if index == 0 || index == 2 {
					access = brokerJobQueryAccess
				}
				if err := child.duplicate(ctx, index, source, process, access); err != nil {
					return err
				}
			}
			return nil
		})
	}); err != nil {
		return err
	}
	if !child.protocol.ValidHandles(child.targetValues()) {
		return errBrokerHelper
	}
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	if err := child.validate(ctx, session); err != nil {
		return err
	}
	if err := child.process.Resume(); err != nil {
		return err
	}
	if err := child.validate(ctx, session); err != nil {
		return err
	}
	if err := child.process.WithAssigned(func(job, process windows.Handle) error {
		id, err := windows.GetProcessId(process)
		if err != nil || id == 0 {
			return errBrokerHelper
		}
		return child.pipe.Accept(ctx, id, brokerScope)
	}); err != nil {
		return err
	}
	seed, err := child.protocol.EncodeSeed(child.targetValues())
	if err != nil {
		return err
	}
	stages := child.protocol.Stages()
	if err := child.exchange(ctx, seed, stages.Armed); err != nil {
		return err
	}
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	if err := child.validate(ctx, session); err != nil {
		return err
	}
	goFrame := childbootstrap.Frame(stages.Go, child.protocol.Binding())
	if err := child.exchange(ctx, goFrame[:], stages.Owned); err != nil {
		return err
	}
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	return child.validate(ctx, session)
}

func (child *brokerSuccessor) prepareEnvironment() error {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	win, err := windows.GetWindowsDirectory()
	if err != nil {
		return err
	}
	config, err := child.folders[0].read(0)
	if err != nil {
		return err
	}
	cache, err := child.folders[1].read(1)
	if err != nil {
		return err
	}
	child.environment, child.directory, err = spawnprep.BuildEnvironment(spawnprep.EnvironmentGUI, spawnprep.EnvironmentLocations{
		SystemDirectory: system, WindowsDirectory: win, ConfigDirectory: config, CacheDirectory: cache, TemporaryDirectory: filepath.Join(cache, "Temp"),
	})
	return errors.Join(err, child.closeFolders())
}

func (child *brokerSuccessor) duplicate(ctx context.Context, index int, source, process windows.Handle, access uint32) error {
	if index < 0 || index >= len(child.remote) || child.duplicateAttempted[index] || child.duplicatePending {
		return errBrokerHelper
	}
	if err := brokerScope(ctx); err != nil {
		return err
	}
	child.duplicateAttempted[index], child.duplicatePending = true, true
	err := windows.DuplicateHandle(windows.CurrentProcess(), source, process, &child.remote[index], access, false, 0)
	child.duplicatePending, child.duplicateReturned[index] = false, true
	return err
}

func (child *brokerSuccessor) targetValues() (values [4]uint64) {
	for index, value := range child.remote {
		values[index] = uint64(value)
	}
	return
}

func brokerProcessMembership(process, job windows.Handle, want bool) error {
	if err := brokerIsInJob.Find(); err != nil {
		return err
	}
	var member int32
	ok, _, _ := brokerIsInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 || (member != 0) != want {
		return errBrokerHelper
	}
	return nil
}

func (child *brokerSuccessor) validate(ctx context.Context, session *brokerHelperSession) error {
	if child.retireAttempted || child.duplicatePending || child.tokenPending || child.tokenCloseAttempted || !child.preparationClosed {
		return errBrokerHelper
	}
	if err := session.pair.Validate(ctx, brokerScope); err != nil {
		return err
	}
	return child.process.WithAssigned(func(job, process windows.Handle) error {
		state, err := windows.WaitForSingleObject(process, 0)
		if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
			return errBrokerHelper
		}
		if err := session.pair.MatchProcess(ctx, process, true, brokerScope); err != nil {
			return err
		}
		if !child.tokenAttempted {
			child.tokenAttempted, child.tokenPending = true, true
			err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &child.token)
			child.tokenPending = false
			child.tokenReady = err == nil && child.token != 0
			if err != nil {
				return err
			}
		}
		if !child.tokenReady || !child.token.IsElevated() {
			return errBrokerHelper
		}
		user, err := fileacl.UserSID(child.token)
		if err != nil || !session.pair.MatchesUser(user) {
			return errBrokerHelper
		}
		if _, err := spawnprep.QueryBrokerJobLimits(job, spawnprep.BrokerJobGUI); err != nil {
			return err
		}
		accounting, err := spawnprep.QueryAccounting(job)
		if err != nil || accounting.TotalProcesses != 1 || accounting.ActiveProcesses != 1 || accounting.TerminatedProcesses != 0 {
			return errBrokerHelper
		}
		return session.lease.WithChain(ctx, brokerScope, func(ancestor, helperJob, parent, origin windows.Handle) error {
			if err := brokerProcessMembership(process, ancestor, true); err != nil {
				return err
			}
			if err := brokerProcessMembership(process, helperJob, false); err != nil {
				return err
			}
			return brokerScope(ctx)
		})
	})
}

func (child *brokerSuccessor) exchange(ctx context.Context, send []byte, wantStage string) error {
	var reply [childbootstrap.FrameSize]byte
	want := childbootstrap.Frame(wantStage, child.protocol.Binding())
	var err error
	if wantStage == child.protocol.Stages().Ack {
		err = child.pipe.ExchangeObserved(ctx, send, reply[:], &child.finalExchange, brokerScope)
		child.ackObserved = child.finalExchange.ReplyMatches(send, want[:])
	} else {
		err = child.pipe.Exchange(ctx, send, reply[:], brokerScope)
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(reply[:], want[:]) {
		return errBrokerHelper
	}
	if wantStage == child.protocol.Stages().Ack {
		child.ackObserved = true // matched bytes precede any later cancellation
	}
	return brokerScope(ctx)
}

func (child *brokerSuccessor) commit(ctx context.Context, session *brokerHelperSession) error {
	if !child.ready || child.commitAttempted {
		return errBrokerHelper
	}
	child.commitAttempted, child.commitPending = true, true
	defer func() {
		stages := child.protocol.Stages()
		use := childbootstrap.Frame(stages.Use, child.protocol.Binding())
		ack := childbootstrap.Frame(stages.Ack, child.protocol.Binding())
		child.ackObserved = child.ackObserved || child.finalExchange.ReplyMatches(use[:], ack[:])
	}() // effect data only; never a deferred successful native-call/commit receipt
	err := child.commitGUI(ctx, session)
	child.commitPending, child.commitReturned = false, true
	return err
}

func (child *brokerSuccessor) commitGUI(ctx context.Context, session *brokerHelperSession) error {
	if err := session.validateChain(ctx); err != nil {
		return err
	}
	stages := child.protocol.Stages()
	use := childbootstrap.Frame(stages.Use, child.protocol.Binding())
	if err := child.exchange(ctx, use[:], stages.Ack); err != nil {
		return err
	}
	if err := session.validateChain(ctx); err != nil {
		return err
	}
	if err := child.pipe.Close(); err != nil {
		return err
	}
	child.serverClosed = true // GUI now waits for BOTH original App and helper exits
	return brokerScope(ctx)
}

func (child *brokerSuccessor) retire() error {
	if child.retired {
		return nil
	}
	if child.preparePending || child.commitPending || child.duplicatePending || child.tokenPending || child.retirePending {
		return errBrokerHelper
	}
	if !child.retireAttempted {
		child.retireAttempted, child.retirePending = true, true
		if child.process.Created() {
			child.terminationErr = child.process.Terminate()
		}
		child.retirePending, child.retireReturned = false, true
	}
	if !child.retireReturned {
		return errBrokerHelper
	}
	pipeErr := child.pipe.Close()
	preparationErr := child.pipes.ClosePreparation()
	folderErr := child.closeFolders()
	if child.process.Created() {
		deadline := time.Now().Add(4 * time.Second)
		for {
			empty, err := child.process.Empty()
			if err == nil && empty {
				break
			}
			if err != nil || child.terminationErr != nil || !time.Now().Before(deadline) {
				return errors.Join(errBrokerHelper, err, child.terminationErr, pipeErr, preparationErr, folderErr)
			}
			time.Sleep(min(10*time.Millisecond, time.Until(deadline)))
		}
	}
	if err := child.process.Close(); err != nil {
		return errors.Join(errBrokerHelper, err, pipeErr, preparationErr, folderErr)
	}
	if err := spawnprep.CloseOnce((*windows.Handle)(&child.token), &child.tokenCloseAttempted); err != nil {
		return err
	}
	if err := errors.Join(pipeErr, preparationErr, folderErr); err != nil {
		return err
	}
	child.retired = true
	return nil // these local resources only, not ancestor-tree or forwarding acceptance
}
