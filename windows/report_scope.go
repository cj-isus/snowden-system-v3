package main

import (
	"errors"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Public unavailable/stale flags alone cannot reserve a Stop. The private
// marker is added only after original scoped probe/protection work was begun.
func restrictiveObservation(report *core.ProbeReport) bool {
	return report != nil && report.RestrictionObserved() && unsafeObservation(report)
}

// App.mu held. The exact original source is required even when scalar intent
// IDs happen to match. It is a negative identity, never health authority.
func (a *App) reportIntentCurrentLocked(report *core.ProbeReport) bool {
	return report != nil && report.IntentGeneration == a.operationID && report.OperationSource() == a.intentSource &&
		a.intentSource.Current() && a.protectionDesired && !a.profileTransition && !a.closing.Load()
}

// epoch.mu held. Epoch0/power gaps cannot silently adopt a replacement path.
// This helper only checks values; no locks/callbacks/native work are acquired.
func reportPathCurrentLocked(report *core.ProbeReport) bool {
	return report != nil && netEpoch.path != nil && report.ObservationPath() == &netEpoch.path.current && netEpoch.path.current.Load() &&
		!netEpoch.knownDown && !netEpoch.powerBlocked.Load() && report.NetworkEpoch >= 0 && report.NetworkEpoch == netEpoch.num
}

// The owned report carries the actor that actually underwent a protection
// check. A later epoch/intent cannot replace it with the caller's current state.
type sessionProtectionFault struct {
	report *core.ProbeReport
	cause  error
}

func (*sessionProtectionFault) Error() string { return "session protection observation failed" }
func (fault *sessionProtectionFault) Unwrap() error {
	if fault == nil {
		return nil
	}
	return fault.cause
}

func protectionFault(report *core.ProbeReport, cause error) error {
	owned := normalizeProbeReport(report)
	owned.AllPassed, owned.ProtectionUnavailable = false, true
	failure := tunnelProtectionFailure(cause)
	owned.CleanupUnconfirmed = owned.CleanupUnconfirmed || failure.CleanupUnconfirmed
	owned.Results = append(owned.Results, failure.Results...)
	owned.MarkRestrictionObserved()
	return &sessionProtectionFault{report: owned, cause: cause}
}

func (a *App) blockProtectionFailure(cc *render.ClientConfig, cause error, reason string) {
	var fault *sessionProtectionFault
	if errors.As(cause, &fault) && fault != nil && fault.report != nil {
		a.blockUnsafePath(cc, reason, fault.report, fcUnknown)
	}
}
