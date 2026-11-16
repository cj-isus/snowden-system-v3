package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Receipt for the original Start/Switch frame, not another scheduler or engine
// owner. After acquire, every field and App.lifecycleOutcome use operationMu.
type appLifecycleOutcome struct {
	app          *App
	registration intentRegistration
	stage        string
	locked       bool
	conflict     bool
	returned     bool
	cleanup      *appStopAttempt
}

// Capture before any serializer/pre-acquisition wait. Negative cleanup must
// never obtain a successor's source by looking up the current scalar ID later.
func (a *App) newLifecycleOutcome(intent uint64, stage string) *appLifecycleOutcome {
	outcome := &appLifecycleOutcome{app: a, stage: stage}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.operationID == intent && intent != 0 {
		outcome.registration = a.intentRegistrationLocked()
	}
	return outcome
}

func (outcome *appLifecycleOutcome) acquire() bool {
	a := outcome.app
	a.operationMu.Lock()
	outcome.locked = true
	if a.lifecycleOutcome != nil {
		outcome.conflict = true // an unretired frame cannot be adopted or overwritten
		return false
	}
	a.lifecycleOutcome = outcome
	return true
}

func (outcome *appLifecycleOutcome) originalCurrent() bool {
	outcome.app.mu.Lock()
	defer outcome.app.mu.Unlock()
	return outcome.registration.currentLocked(outcome.app)
}

func (outcome *appLifecycleOutcome) release() {
	if !outcome.locked {
		return
	}
	if outcome.app.lifecycleOutcome == outcome {
		outcome.app.lifecycleOutcome = nil
	}
	outcome.locked = false
	outcome.app.operationMu.Unlock()
}

// Direct defer of the original frame. Existing body defers (cancellation,
// sampler/task cleanup and startBusy) unwind before this terminal boundary.
func (outcome *appLifecycleOutcome) finish(result *error) {
	payload := recover()
	defer outcome.release()
	if outcome.returned && payload == nil && !outcome.conflict {
		return // preserve ordinary errors and verified rollback without rewrapping
	}
	if outcome.registration.source != nil {
		outcome.registration.source.Revoke()
	}
	if !outcome.locked {
		outcome.acquire() // negative cleanup still serializes after a pre-acquisition unwind
	}
	cause := errors.Join(core.ErrEngineCleanupFailed,
		fmt.Errorf("%s: операция не вернулась нормально (%T)", outcome.stage, payload))
	var localResult error
	if result == nil {
		result = &localResult
	}
	sequence := &appCleanupSequence{result: core.InvalidateLifecycleOutcome(*result, cause)}
	defer sequence.finish(result)
	// Reapply the negative state AFTER any negative Stop. A nested Stop may
	// finish normally, but cannot turn this interrupted Start/Switch into success.
	defer appCleanupStep("publish interrupted lifecycle", sequence, func() error {
		if outcome.markFailed() {
			outcome.app.pushState()
		}
		return nil
	})
	if !outcome.markFailed() || outcome.conflict {
		return
	}
	if previous := outcome.cleanup; previous != nil && previous.coversCurrentOwners(outcome.app) {
		sequence.result = errors.Join(sequence.result, previous.sequence.result)
		return // original pending/failed/drained attempt is not replayed as proof
	}
	appCleanupStep("stop interrupted original lifecycle", sequence, func() error {
		return outcome.app.stopCoreRegisteredLocked(stopFailure, outcome.registration)
	})
}

func (outcome *appLifecycleOutcome) markFailed() bool {
	a := outcome.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !outcome.registration.sameLocked(a) {
		return false
	}
	a.state.State = "error"
	a.state.Error = outcome.stage + ": операция прервана; результат очистки требует проверки."
	a.state.BlockedReason = "cleanup_unconfirmed"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.state.ProxyMode, a.state.ActiveID = "", ""
	a.state.Active = nil
	a.nativeObservation = nil
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
	return true
}
