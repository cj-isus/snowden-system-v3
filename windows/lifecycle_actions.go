package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

var (
	errProfileReconciliationRequired = errors.New("Start: смена профиля не завершена; повторите импорт или явно отключите VPN")
	errAppClosing                    = errors.New("Start: приложение закрывается")
)

// App.mu held. An occupied slot always denies a replacement; metadata only
// chooses the reason. No Context/cancel getter, callback, wait or slot mutation.
func (a *App) operationSlotBlockLocked() string {
	owner, cancelPresent := a.operationCancellation, a.operationCancel != nil
	if owner == nil && !cancelPresent {
		return ""
	}
	if owner == nil || !cancelPresent || owner.knownFailure() {
		return "cleanup_unconfirmed"
	}
	return "lifecycle_busy"
}

// App.mu held. These are local request-registration predicates, not evidence
// that an Engine, profile, native boundary or protected path can be admitted.
// Both public handlers and the presentation snapshot use the same predicates.
func (a *App) startBlockLocked(explicit bool) string {
	if a.proxyConflict && !explicit {
		return "explicit_start_required"
	}
	if a.restartCommit != nil {
		return "restart_committed"
	}
	if a.releaseCommit != nil {
		return "protection_release_committed"
	}
	if a.installedImageChanged {
		return "update_restart_required"
	}
	if a.startupErr != nil {
		return "startup_unavailable"
	}
	if a.watchdogBlocked.Load() {
		return "cleanup_unconfirmed"
	}
	if a.profileTransition {
		return "profile_reconciliation_required"
	}
	switch a.state.State {
	case "starting", "running", "stopping", "reloading":
		return "lifecycle_busy"
	case "stopped", "error":
		// A fresh public request may reconcile a stopped/error predecessor.
		if reason := a.operationSlotBlockLocked(); reason != "" {
			return reason
		}
	default:
		return "state_unavailable"
	}
	if a.closing.Load() {
		return "closing"
	}
	if !a.appLifetimeCurrentLocked() {
		return "app_unavailable"
	}
	return ""
}

// Finite command errors are separate from the value-only snapshot. In
// particular, never format/unwrap the private startup cause under App.mu.
func (a *App) startRejectionLocked(explicit bool) (string, error) {
	reason := a.startBlockLocked(explicit)
	switch reason {
	case "explicit_start_required":
		return reason, core.ErrProxyOwnershipConflict
	case "restart_committed":
		return reason, errRestartCommitted
	case "protection_release_committed":
		return reason, errProtectionReleaseCommitted
	case "update_restart_required":
		return reason, errInstalledImageChanged
	case "startup_unavailable":
		return reason, errAppStartupUnavailable
	case "cleanup_unconfirmed":
		if a.operationSlotBlockLocked() == "cleanup_unconfirmed" {
			return reason, core.ErrEngineCleanupFailed
		}
		return reason, errWatchdogUnavailable
	case "profile_reconciliation_required":
		return reason, errProfileReconciliationRequired
	case "lifecycle_busy":
		return reason, errAppOperationBusy
	case "closing":
		return reason, errAppClosing
	case "app_unavailable":
		return reason, context.Canceled
	case "":
		return reason, nil
	default:
		return reason, core.ErrInvalidState // a new unmapped rejection must never admit a request
	}
}

func (a *App) stopRejectionLocked(reason StopReason) (string, error) {
	if a.restartCommit != nil {
		return "restart_committed", errRestartCommitted
	}
	if a.closing.Load() && reason != stopQuit {
		return "closing", context.Canceled
	}
	if a.releaseCommit != nil {
		return "protection_release_committed", errProtectionReleaseCommitted
	}
	return "", nil
}

// Never carry errors, paths, native objects or user data in this value. Its
// intent/lifecycle bind the display to this snapshot; neither is an OS receipt.
func (a *App) lifecycleActionsLocked(snapshot AppState) *LifecycleActionsView {
	if snapshot.DiagnosticContext == nil {
		return nil
	}
	startReason := a.startBlockLocked(true)
	stopReason, stopErr := a.stopRejectionLocked(stopDisconnect)
	return &LifecycleActionsView{
		Schema: 1, Intent: snapshot.DiagnosticContext.Intent, Lifecycle: snapshot.State,
		StartAllowed: startReason == "", StartBlockedBy: startReason,
		StopAllowed: stopErr == nil, StopBlockedBy: stopReason,
		ExplicitStartRequired: a.proxyConflict,
	}
}
