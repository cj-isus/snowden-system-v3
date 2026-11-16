package main

import (
	"context"
	"time"
)

// A local producer fault, not a channel measurement, trust result or new intent.
// Capture the exact registration before waiting for the existing serializer.
func (owner *appWatchdogTask) invalidateScope() (intentRegistration, context.CancelFunc, bool) {
	a := owner.app
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	if runtime_.watchdog != owner {
		return intentRegistration{}, nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.protectionDesired || a.profileTransition || a.closing.Load() || a.quitRequested {
		return intentRegistration{}, nil, false // explicit Disconnect/Close owns its negative phase
	}
	registration := a.intentRegistrationLocked()
	registration.source.Revoke()
	a.state.State, a.state.BlockedReason = "error", "cleanup_unconfirmed"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.state.Error = "Контроль туннеля прерван; результат очистки требует проверки."
	a.nativeObservation = nil
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
	return registration, a.operationCancel, true
}

func (owner *appWatchdogTask) failOriginalScope(cause error) {
	sequence := &appCleanupSequence{result: cause}
	owner.mu.Lock()
	owner.cleanup = sequence // original result retained even if a later stage calls Goexit
	owner.mu.Unlock()
	registration, cancel, current := owner.invalidateScope()
	if !current {
		return
	}
	defer appCleanupStep("publish original watchdog failure", sequence, func() error {
		if owner.markFailure(registration) {
			owner.app.pushState()
		}
		return nil
	})
	defer appCleanupStep("stop original watchdog-fault lifecycle", sequence, func() error {
		a := owner.app
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		return a.stopCoreRegisteredLocked(stopFailure, registration)
	})
	defer appCleanupStep("cancel original watchdog-fault operation", sequence, func() error {
		if cancel != nil {
			cancel()
		}
		return nil
	})
}

func (owner *appWatchdogTask) markFailure(registration intentRegistration) bool {
	a := owner.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !registration.sameLocked(a) || !a.protectionDesired || a.profileTransition || a.closing.Load() || a.quitRequested {
		return false
	}
	a.state.State, a.state.BlockedReason = "error", "cleanup_unconfirmed"
	a.state.Error = "Контроль туннеля прерван; результат очистки требует проверки."
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.nativeObservation = nil
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
	return true
}
