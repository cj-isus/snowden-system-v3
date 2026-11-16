package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// Caller owns operationMu. Trust/credential updates retire the complete old
// engine before publication: Reload rollback must never resurrect revoked trust.
// A later Start builds only from the committed profile and runs the ordinary
// protected probe. Preview never calls this path.
func (a *App) retireProfileSession(expected uint64) (uint64, error) {
	a.mu.Lock()
	registration := a.intentRegistrationLocked()
	a.mu.Unlock()
	if registration.intent != expected {
		return 0, context.Canceled
	}
	return a.retireProfileSessionRegistered(registration, nil)
}

func (a *App) retireProfileSessionRegistered(expected intentRegistration, scope *profileRequestScope) (uint64, error) {
	a.mu.Lock()
	if !expected.currentLocked(a) || a.closing.Load() || scope != nil && !scope.currentLocked(a) {
		a.mu.Unlock()
		return 0, context.Canceled
	}
	if err := a.advanceOperationLocked(); err != nil {
		a.mu.Unlock()
		return 0, err
	}
	registration := a.intentRegistrationLocked()
	id := registration.intent
	if scope != nil {
		// Original writer's own transition only, before releasing App.mu. This
		// transfers a negative source, not proof that native retirement succeeded.
		scope.registration.Store(&registration)
	}
	a.profileTransition = true
	a.cancelFailClosedRetryLocked()
	if a.operationCancel != nil {
		a.operationCancel()
	}
	a.state.CoreReady = false
	a.state.ProbeRunning = false
	a.state.BlockedReason = "profile_update_pending"
	a.pushStateLocked()
	a.mu.Unlock()
	runtime_.mu.Lock()
	hasManager := runtime_.manager != nil
	orphanedEngine := !hasManager && runtime_.engine != nil
	runtime_.mu.Unlock()
	if orphanedEngine {
		err := errors.New("engine has no lifecycle owner; profile cannot be committed")
		a.finishProfileTransition(id, false, err)
		return id, err
	}
	// A first import on a stopped device must not arm/revoke system policy.
	// Only an existing Manager can own a session that needs retirement.
	if hasManager {
		if err := a.stopCoreLocked(stopFailure); err != nil {
			a.finishProfileTransition(id, false, err)
			return id, fmt.Errorf("onboarding: previous engine retirement failed: %w", err)
		}
	}
	a.mu.Lock()
	current := registration.currentLocked(a) && !a.closing.Load() && (scope == nil || scope.currentLocked(a))
	if current {
		a.state.BlockedReason = "profile_update_pending"
		a.pushStateLocked()
	}
	a.mu.Unlock()
	if !current {
		return id, context.Canceled
	}
	return id, nil
}

// Returns a generation-fenced launch, to be invoked after commit. It waits for
// operationMu in startCoreGeneration; a concurrent Disconnect invalidates it.
// Failure keeps protection/ownership and suppresses all automatic old-profile
// retries. Explicit Disconnect is the recovery escape, never an error cleanup.
func (a *App) finishProfileTransition(id uint64, committed bool, cause error) func() {
	proxyMessage := lifecycleFailureText("запуск", core.ErrProxyOwnershipConflict, false)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.operationID != id || a.closing.Load() {
		return nil
	}
	if !committed {
		a.profileTransition = true
		a.state.State = "error"
		a.state.CoreReady = false
		a.state.ProbeRunning = false
		a.state.BlockedReason = "profile_reconciliation_required"
		a.state.Error = "Смена профиля не завершена; повторите импорт или явно отключите VPN"
		if errors.Is(cause, errProfileStorageQuarantine) {
			a.trustBlocked = true
			a.state.BlockedReason = "profile_storage_recovery_required"
			a.state.Error = profileStorageQuarantineMessage
			a.cancelFailClosedRetryLocked()
		}
		if cause != nil && !errors.Is(cause, context.Canceled) {
			a.appendLogLocked("error", "onboarding: profile transaction did not finish; previous session is not resumed")
		}
		a.pushStateLocked()
		return nil
	}
	a.profileTransition = false
	a.trustBlocked = false
	a.fpol.clearAttemptHints()
	a.state.Error = ""
	if a.installedImageChanged {
		a.blockChangedImageLocked()
		a.pushStateLocked()
		return nil // new trust may commit; the old mapped App may not resume a worker
	}
	if !a.protectionDesired {
		a.state.State = "stopped"
		a.state.BlockedReason = ""
		a.pushStateLocked()
		return nil
	}
	if a.proxyConflict {
		a.state.State = "error"
		a.state.Error = proxyMessage
		a.state.BlockedReason = "proxy_conflict"
		a.cancelFailClosedRetryLocked()
		a.sm.markBlocked(time.Now())
		a.pushStateLocked()
		return nil // new profile is committed, but automatic resume cannot acknowledge the settings change
	}
	a.state.State = "starting"
	a.state.BlockedReason = ""
	a.pushStateLocked()
	return func() { go a.startCoreGeneration(id) }
}
