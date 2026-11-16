package main

import (
	"context"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

type observationPhase uint8

const (
	observationHealth   observationPhase = iota // health only, lifecycle Start gate
	observationMeasured                         // manual inspection, no policy reset
	observationHealthy
	observationRecovered
	observationSuspected
	observationDegraded
	observationSwitching
	observationStarted
	observationApplied // applied switch/rollback; scoped attempt owns its policy outcome
	observationStopping
	observationSafetyStopping // policy uncertainty can require Stop without proving a leak
)

// Safety facts and local protection/cleanup uncertainty bypass ordinary probe
// ordering. A local uncertainty is not proof of a leak or a network failure.
// Every report still must own intent/session/epoch.
func unsafeObservation(report *core.ProbeReport) bool {
	if report == nil {
		return false
	}
	return report.LeakConfirmed || report.ProtectionUnavailable || report.CleanupUnconfirmed || observedEgressDivergence(report) ||
		ClassifyFailure(report, 0, 0).Code == fcLeakConfirmed
}

func (a *App) publishObservation(cc *render.ClientConfig, report *core.ProbeReport, phase observationPhase, failureClass string) bool {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	return a.publishObservationLocked(cc, report, phase, failureClass)
}

// Caller owns operationMu. Lock order: runtime -> Manager -> App -> epoch ->
// poller -> DNS publication lease -> health/SM/failover (clock checked before leases).
// Callback-free in-memory commit only; no probe, Manager mutation,
// WFP, log persistence or EventsEmit while runtime/epoch locks are held.
func (a *App) publishObservationLocked(cc *render.ClientConfig, report *core.ProbeReport, phase observationPhase, failureClass string) bool {
	return a.publishObservationContextLocked(nil, cc, report, phase, failureClass)
}

func lifecyclePublicationCurrent(ctx context.Context) bool {
	if ctx == nil || ctx.Err() != nil || !core.OperationSourcesCurrent(ctx) {
		return false
	}
	if source, ok := ctx.Value(networkPathContextKey{}).(*networkPathSource); ok && source != nil && !source.current.Load() {
		return false // value-only; this helper also runs under networkEpoch.mu
	}
	deadline, bounded := ctx.Deadline()
	return !bounded || time.Now().Before(deadline)
}

// A rejected mutation retains an old session without producing a new report.
// Use historical evidence only to keep its scope, never retag it or refresh
// health/probe time. The native-only scope helper cannot cover SOCKS here.
func (a *App) publishRetainedLifecycleLocked(ctx context.Context, cc *render.ClientConfig, manager *core.Manager, intent uint64) bool {
	if !lifecyclePublicationCurrent(ctx) || cc == nil || manager == nil {
		return false
	}
	report := normalizeProbeReport(manager.LastProbeReport())
	if !report.AllPassed || report.Stale || report.RestrictionObserved() || report.SessionID == 0 || report.SessionID != cc.SessionID || report.IntentGeneration != intent {
		return false
	}
	clock := a.trustClock()
	runtime_.mu.Lock()
	if runtime_.cc != cc || runtime_.manager != manager {
		runtime_.mu.Unlock()
		return false
	}
	releaseOwner, ok := manager.GuardRunningOwner(cc.OwnerID, cc.OwnerGeneration)
	if !ok {
		runtime_.mu.Unlock()
		return false
	}
	a.mu.Lock()
	netEpoch.mu.Lock()
	valid := a.reportIntentCurrentLocked(report) && !a.trustBlocked &&
		a.observedClock == clock && a.restartCommit == nil && !a.installedImageChanged && !a.watchdogBlocked.Load() &&
		reportPathCurrentLocked(report) && lifecyclePublicationCurrent(ctx)
	if valid {
		now, err := clock.Now(cc.Validity.ClockFloor)
		if err == nil {
			_, err = cc.Validity.Remaining(now)
		}
		valid = err == nil && lifecyclePublicationCurrent(ctx)
	}
	if valid {
		releaseDNS, live := runtime_.dns.guardRunning(a)
		if live {
			a.state.State, a.state.Error = "running", ""
			releaseDNS()
		} else {
			valid = false
		}
	}
	netEpoch.mu.Unlock()
	releaseOwner()
	runtime_.mu.Unlock()
	if valid {
		a.pushStateLocked() // only App.mu remains; preserve Stop/event ordering
	}
	a.mu.Unlock()
	return valid
}

// Started/applied outcomes also own their original App operation context. A
// Manager's normally retired child context is not that publication context.
func (a *App) publishObservationContextLocked(ctx context.Context, cc *render.ClientConfig, report *core.ProbeReport, phase observationPhase, failureClass string) bool {
	lifecycle := phase == observationStarted || phase == observationApplied
	if lifecycle && !lifecyclePublicationCurrent(ctx) {
		return false
	}
	if cc == nil || report == nil || report.ObservationID == 0 || phase > observationSafetyStopping {
		return false
	}
	report = normalizeProbeReport(report) // step facts also constrain aggregate flags here
	restrictive := restrictiveObservation(report)
	if !restrictive && (report.Stale || report.CompletedAt.IsZero() || report.CompletedAt.Before(report.StartedAt)) {
		return false
	}
	if (unsafeObservation(report) || report.RestrictionObserved() || phase == observationSafetyStopping) && !restrictive ||
		restrictive && phase != observationStopping && phase != observationSafetyStopping {
		return false // private actual-work fact is required, and only restrictive publication is allowed
	}
	// Health/policy consumes the actual normalized report, never a caller's
	// stale, empty or causal classification label. This cannot create a PASS.
	if !report.AllPassed {
		failureClass = ClassifyFailure(report, 0, 0).Code
	}
	unsafe := restrictive
	clock := a.trustClock() // capture before publication locks; no App callback inside the gate
	if report.TrustUnavailable && !unsafe {
		return false // expiry/clock are local trust faults, never channel-health failures
	}
	if !unsafe {
		if _, err := a.sessionLifetime(cc); err != nil {
			return false
		}
	}
	if (phase == observationHealthy || phase == observationRecovered || phase == observationStarted || phase == observationApplied) && (!report.AllPassed || unsafe) {
		return false
	}
	if (phase == observationSuspected || phase == observationDegraded || phase == observationSwitching) && (report.AllPassed || unsafe) {
		return false // unsafe facts go directly through failSession, never debounce
	}
	if phase == observationStopping && report.AllPassed && !unsafe {
		return false // ordinary teardown requires a failure, not merely a report
	}
	runtime_.mu.Lock()
	manager := runtime_.manager
	if runtime_.cc != cc || manager == nil {
		runtime_.mu.Unlock()
		return false
	}
	owner, generation := report.OwnerID, report.OwnerGeneration
	var releaseOwner func()
	var ok bool
	if restrictive {
		releaseOwner, ok = manager.GuardRetirementOwner(owner, generation, runtime_.engine)
	} else {
		releaseOwner, ok = manager.GuardRunningOwner(owner, generation)
	}
	if !ok {
		runtime_.mu.Unlock()
		return false
	}
	a.mu.Lock()
	if !restrictive && (a.trustBlocked || a.observedClock != clock || a.restartCommit != nil || a.installedImageChanged || a.watchdogBlocked.Load()) {
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
		return false
	}
	if owner == 0 || generation == 0 || !a.reportIntentCurrentLocked(report) || report.SessionID == 0 || report.SessionID != cc.SessionID || report.OwnerID != owner || report.OwnerGeneration != generation || cc.OwnerID != owner || cc.OwnerGeneration != generation || !unsafe && report.ObservationID < a.lastObservation {
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
		return false
	}
	netEpoch.mu.Lock()
	if !restrictive && !reportPathCurrentLocked(report) {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
		return false
	}
	if restrictive && !reportPathCurrentLocked(report) {
		report.Stale, report.AllPassed = true, false // old path never produces health or a refreshed probe time
	}
	if !unsafe {
		now, err := clock.Now(cc.Validity.ClockFloor)
		if err == nil {
			_, err = cc.Validity.Remaining(now)
		}
		if err != nil {
			netEpoch.mu.Unlock()
			a.mu.Unlock()
			releaseOwner()
			runtime_.mu.Unlock()
			return false
		}
	}
	if lifecycle && !lifecyclePublicationCurrent(ctx) {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
		return false
	}
	var releaseDNS func()
	if !unsafe {
		releaseDNS, ok = runtime_.dns.guardRunning(a)
		if !ok {
			netEpoch.mu.Unlock()
			a.mu.Unlock()
			releaseOwner()
			runtime_.mu.Unlock()
			return false // local observer failure is not a channel-health observation
		}
	}
	if phase == observationSwitching && !a.fpol.beginAttemptIfDue(time.Now()) {
		if releaseDNS != nil {
			releaseDNS()
		}
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		releaseOwner()
		runtime_.mu.Unlock()
		return false
	}
	if report.ObservationID > a.lastObservation {
		a.lastObservation = report.ObservationID
	}
	// Health writes share this exact epoch/intent/session commit. ISP remains
	// unknown here: an unrelated facts-cache read is not this report's ISP proof.
	// Non-safety publication already owns poller+DNS leases. Task entry after
	// an observation gap does not revive old-epoch channel health. Unsafe
	// reports retain their stop path without making a channel-health judgment.
	writeHealth := !unsafe && runtime_.dns.poller.observed && !report.TrustUnavailable && !report.ProtectionUnavailable && !report.CleanupUnconfirmed &&
		(phase == observationHealth || phase == observationHealthy || phase == observationRecovered || phase == observationDegraded || phase == observationStarted || phase == observationApplied)
	if writeHealth && report.NetworkEpoch > 0 && report.ObservationID > a.lastHealthObservation && (report.AllPassed || !measurementOnlyFailure(failureClass)) {
		if chanHealth.observeInEpoch(report.NetworkEpoch, netEpoch.netCls, "", cc.SelectorDefault, report.AllPassed && !report.LeakConfirmed, failureClass, report.CompletedAt, srcProbe) {
			a.lastHealthObservation = report.ObservationID
		}
	}
	now := time.Now()
	switch phase {
	case observationStarted, observationApplied:
		a.trustBlocked = false
		if phase == observationStarted {
			a.cancelFailClosedRetryLocked()
			a.fpol.hardReset()
			a.sm.markStarted(now)
		}
		a.state.State = "running"
		a.state.Error = ""
		a.state.ProxyMode = "socks"
		if cc.TUNName != "" {
			a.state.ProxyMode = "tun"
		}
		a.state.ActiveID = cc.SelectorDefault
		a.state.Active = channelViewOf(cc, cc.SelectorDefault)
		a.state.CoreReady = true
		a.state.CoreBlockMsg = ""
		a.state.ProbeRunning = false
	case observationMeasured:
		a.state.ProbeRunning = false
	case observationHealthy, observationRecovered:
		a.fpol.hardReset()
		if phase == observationRecovered {
			a.sm.markRecovered(now)
		} else {
			a.sm.markTick(true, "", now)
		}
	case observationSuspected:
		a.sm.markTick(false, report.Summary(), now)
	case observationDegraded:
		a.sm.markClassified(failureClass, report.Summary(), now)
		a.sm.markDegraded(now)
	case observationSwitching:
		a.sm.markSwitching(now)
	case observationStopping, observationSafetyStopping:
		a.state.State = "stopping" // decision reservation, before external cleanup
		a.state.CoreReady, a.state.ProbeRunning = false, false
		a.nativeObservation = nil
		a.sm.markRecovering(now)
	}
	if phase != observationHealth {
		a.state.Probe = toView(report)
		if report.AllPassed && !unsafe {
			a.state.ProbeLastAt = report.CompletedAt.Format(time.RFC3339)
		}
	}
	if releaseDNS != nil {
		releaseDNS()
	}
	netEpoch.mu.Unlock()
	releaseOwner()
	runtime_.mu.Unlock()
	if phase != observationHealth {
		a.pushStateLocked() // only App.mu remains held, preserves event ordering
	}
	a.mu.Unlock()
	return true
}
