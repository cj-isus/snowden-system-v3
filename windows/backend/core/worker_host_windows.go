//go:build windows

package core

import (
	"context"
	"os"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

type windowsProcessHost struct {
	native         spawnprep.Process // original native owner, published before any acquisition
	preparation    windowsSpawnResources
	resourceLimits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION // product policy remains here
}

func (h *windowsProcessHost) ProcessCreated() bool { return h.native.Created() }

func createdProcessHandle(handle windows.Handle) bool { return nativelease.ValidHandleValue(handle) }

func (h *windowsProcessHost) EnsureWait(p *engineProcess) {
	if p == nil || !h.native.ReserveWait() {
		return
	}
	if p.done == nil {
		p.done = make(chan struct{})
	}
	go p.awaitExit()
}

func (h *windowsProcessHost) ClosePreparation() error { return h.preparation.ClosePreparation() }

type jobAccounting = spawnprep.Accounting

func (h *windowsProcessHost) Wait() error               { return h.native.Wait() }
func (h *windowsProcessHost) ExitCode() (uint32, error) { return h.native.ExitCode() }
func (h *windowsProcessHost) Terminate() error          { return h.native.Terminate() }
func (h *windowsProcessHost) Empty() (bool, error)      { return h.native.Empty() }

func (h *windowsProcessHost) Close() error {
	if err := h.ClosePreparation(); err != nil {
		return err
	}
	return h.native.Close() // original Wait completion/root exit/empty Job/one-attempt Close
}

func workerPipesReady() bool {
	for _, file := range []*os.File{os.Stdin, os.Stdout} {
		kind, err := windows.GetFileType(windows.Handle(file.Fd()))
		if err != nil || kind != windows.FILE_TYPE_PIPE {
			return false
		}
	}
	return true
}

func (process *engineProcess) spawnEngine(ctx context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return ErrEngineWorkerUnavailable
	}
	// Pinned sing-tun may spawn netsh for DNS fallback. Preserve its existing
	// eight-process ceiling and do not impose a new engine memory policy.
	return process.spawn(ctx, executable, []string{EngineWorkerArgument}, ownedProcessLimits{processes: 8})
}

func spawnEngineProcess() (process *engineProcess, err error) {
	process = newOwnedProcess()
	err = process.spawnEngine(context.Background())
	return
}
