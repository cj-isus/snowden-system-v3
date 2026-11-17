package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// No App value lock is held. On success the caller owns operationMu and must
// release or explicitly transfer it. Failure never transfers the serializer.
func (a *App) acquireStopRequest(request stopRequest) (result error) {
	acquired, transferred, returned := false, false, false
	defer func() {
		if acquired && !transferred {
			a.operationMu.Unlock()
		}
	}()
	// Direct defer before the first actual callback and before the mutex wait.
	// It publishes a failed original registration before any acquired unlock.
	defer a.finishStopPreparation(request.intentRegistration, &returned, &result)
	request.cancelOriginal()
	a.operationMu.Lock()
	acquired = true
	result = request.joinOriginal()
	returned = true
	if result != nil {
		return result
	}
	transferred = true
	return nil
}

// A returned join error is still unconfirmed cancellation. Panic/Goexit also
// cannot skip the original negative publication or become a handoff/Release.
func (a *App) finishStopPreparation(registration intentRegistration, returned *bool, result *error) {
	payload := recover()
	if *returned && payload == nil && !localRetirementFailure(*result) {
		return
	}
	*result = errors.Join(*result, core.ErrEngineCleanupFailed)
	if !*returned || payload != nil {
		*result = errors.Join(*result, fmt.Errorf("подготовка остановки не вернулась нормально (%T)", payload))
	}
	a.publishStopFailure(registration, result)
}

// Direct defer of the original serialized Stop. Goexit has no panic payload
// and never becomes a Go/RPC return; it must still leave a negative App state.
func (a *App) finishStopOutcome(registration intentRegistration, returned *bool, cleanup *appCleanupSequence, result *error) {
	payload := recover()
	if *returned && payload == nil {
		return
	}
	*result = errors.Join(*result, cleanup.result, core.ErrEngineCleanupFailed,
		fmt.Errorf("остановка прервана до обычного возврата (%T); итог требует проверки", payload))
	a.publishStopFailure(registration, result)
}

// Negative values/events only. Safe before or while owning operationMu; this
// does not run native cleanup, release protection or clear any operation slot.
func (a *App) publishStopFailure(registration intentRegistration, result *error) {
	if registration.source != nil {
		registration.source.Revoke() // original only; never revoke a successor's source
	}
	publish := func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !registration.sameLocked(a) {
			return false
		}
		a.state.State = "error"
		a.state.CoreReady, a.state.ProbeRunning = false, false
		a.state.Error = "Остановка прервана; результат очистки требует проверки."
		a.state.BlockedReason = "cleanup_unconfirmed"
		a.nativeObservation = nil
		a.cancelFailClosedRetryLocked()
		a.sm.markBlocked(time.Now())
		return true
	}()
	if publish {
		// State is already negative. A failed UI event must not retain App.mu
		// or manufacture successful Stop/Release/native/RPC completion.
		notification := &appCleanupSequence{result: *result}
		defer notification.finish(result)
		appCleanupStep("publish interrupted stop", notification, func() error {
			a.pushState()
			return nil
		})
	}
}
