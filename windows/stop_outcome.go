package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Direct defer of the original serialized Stop. Goexit has no panic payload
// and never becomes a Go/RPC return; it must still leave a negative App state.
func (a *App) finishStopOutcome(registration intentRegistration, returned *bool, cleanup *appCleanupSequence, result *error) {
	payload := recover()
	if *returned && payload == nil {
		return
	}
	*result = errors.Join(*result, cleanup.result, core.ErrEngineCleanupFailed,
		fmt.Errorf("остановка прервана до обычного возврата (%T); итог требует проверки", payload))
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
