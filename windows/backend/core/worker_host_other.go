//go:build !windows

package core

import (
	"context"
	"errors"
)

func newOwnedProcess() *engineProcess { return nil }

func (process *engineProcess) spawn(context.Context, string, []string, ownedProcessLimits) error {
	return ErrEngineWorkerUnavailable
}

func (process *engineProcess) spawnEngine(context.Context) error { return ErrEngineWorkerUnavailable }

func spawnOwnedProcess(context.Context, string, []string, ownedProcessLimits) (*engineProcess, error) {
	return nil, errors.New("owned native helper unavailable on this platform")
}

func spawnEngineProcess() (*engineProcess, error) {
	return nil, errors.New("isolated engine requires the Windows process host")
}

func workerPipesReady() bool { return false }

func updateCodeCheckResourcesReady() bool { return false }
