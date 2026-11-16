package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Original synchronous preparation only. done is closed after its outcome,
// never before an abnormal return can revoke storage/network readiness.
type startupOutcome struct {
	app          *App
	owned        bool
	returned     bool
	done         chan struct{}
	registration intentRegistration
}

func (outcome *startupOutcome) finish(profileErr *error, startServices *bool) {
	payload := recover()
	if !outcome.owned {
		if !outcome.returned || payload != nil {
			*profileErr = errors.Join(core.ErrEngineCleanupFailed, fmt.Errorf("startup frame interrupted (%T)", payload))
			*startServices = false
		}
		return
	}
	if outcome.done != nil {
		defer close(outcome.done)
	}
	a := outcome.app
	if outcome.returned && payload == nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		if outcome.done != nil {
			a.updateGUIStorageReady = *profileErr == nil && a.startupErr == nil && a.ctx != nil && a.ctx.Err() == nil && !a.closing.Load() && !a.quitRequested
		}
		return
	}
	*startServices = false
	*profileErr = errors.Join(*profileErr, core.ErrEngineCleanupFailed,
		fmt.Errorf("startup preparation did not return normally (%T)", payload))
	sequence := &appCleanupSequence{result: *profileErr}
	defer sequence.finish(profileErr)
	defer appCleanupStep("publish interrupted startup", sequence, func() error {
		if a.markStartupFailure(*profileErr) {
			a.pushState()
		}
		return nil
	})
	defer appCleanupStep("retire partial startup producers", sequence, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
		defer cancel()
		return a.retireTerminalProducers(ctx)
	})
	defer appCleanupStep("stop interrupted preparation lifecycle", sequence, func() error {
		return a.stopCoreRegisteredLocked(stopFailure, outcome.registration)
	})
	a.markStartupFailure(*profileErr)
	outcome.registration.source.Revoke() // exact pre-acquisition source, never a later lookup
}

func (a *App) markStartupFailure(cause error) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startupErr == nil {
		a.startupErr = cause // one original preparation; no automatic replacement after uncertainty
	}
	a.updateGUIStorageReady = false
	a.startupIntent.source.Revoke()
	if a.closing.Load() || a.quitRequested {
		return false // preserve the independently registered terminal phase
	}
	a.state.State, a.state.BlockedReason = "error", "cleanup_unconfirmed"
	a.state.Error = "Подготовка приложения прервана. Нужен перезапуск после проверки очистки."
	a.state.CoreBlockMsg = a.state.Error
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.nativeObservation = nil
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
	return true
}
