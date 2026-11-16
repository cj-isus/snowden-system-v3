package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

type observedSwitch struct {
	intent, observation, session, attempt uint64
	epoch                                 int
}

func (p *failoverPolicy) currentAttempt() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.attempting {
		return 0
	}
	return p.attemptGeneration
}

func (p *failoverPolicy) endScopedAttempt(token uint64, success bool, to string, limit int) (accepted, blocked bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if token == 0 || !p.attempting || p.attemptGeneration != token {
		return false, false
	}
	return true, p.endAttemptLocked(success, to, limit)
}

// Local failure ends only its original in-flight work. Preserve earlier
// switches, last error and lastAttempt/cooldown; do not record channel success
// or failure, and do not let a stale completion clear a successor's token.
func (p *failoverPolicy) deferScopedAttempt(token uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if token == 0 || !p.attempting || p.attemptGeneration != token {
		return false
	}
	p.attempting = false
	p.state = "open"
	return true
}

func (a *App) reserveObservedSwitch(cc *render.ClientConfig, report *core.ProbeReport, failureClass string) (observedSwitch, bool) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if !a.publishObservationLocked(cc, report, observationSwitching, failureClass) {
		return observedSwitch{}, false
	}
	token := a.fpol.currentAttempt()
	return observedSwitch{intent: report.IntentGeneration, observation: report.ObservationID, session: cc.SessionID, attempt: token, epoch: report.NetworkEpoch}, token != 0
}

// The reserved attempt, the decision evidence and the resulting applied
// snapshot are separate identities. A late completion cannot reset a newer
// attempt or turn a freshly recovered path back into DEGRADED/BLOCKED.
func (a *App) finishObservedSwitch(receipt observedSwitch, target string, candidates int, err error) (switched, blocked bool) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.fpol.currentAttempt() != receipt.attempt || receipt.attempt == 0 {
		return false, false
	}
	discard := func() {
		a.fpol.endScopedAttempt(receipt.attempt, true, "", candidates)
	}
	if !a.operationCurrent(receipt.intent) || receipt.epoch != currentChannelEpoch() {
		discard()
		return false, false
	}
	if channelRetryBlocked(err) || trustFailureCode(err) != "" || errors.Is(err, errInstalledImageChanged) {
		a.fpol.deferScopedAttempt(receipt.attempt)
		return false, false // retain lifecycle BLOCKED; no candidate hint/budget or positive rollback
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, errSessionSuperseded) {
		discard()
		return false, false
	}
	runtime_.mu.Lock()
	cc, manager := runtime_.cc, runtime_.manager
	runtime_.mu.Unlock()
	rollback, verified := verifiedRollback(err)
	if err == nil || verified {
		var report *core.ProbeReport
		phase := observationHealthy
		if verified {
			report, phase = rollback.Report, observationRecovered
		} else if manager != nil {
			report = manager.LastProbeReport()
		}
		if report != nil && report.IntentGeneration == receipt.intent && report.NetworkEpoch == receipt.epoch && a.publishObservationLocked(cc, report, phase, "") {
			if verified {
				a.appendLog("warn", "failover: новый канал не применён; прежний путь повторно подтверждён")
				return false, false
			}
			a.appendLog("info", "failover: канал переключён на "+target)
			return true, false
		}
		discard() // no fresh current proof, not a synthetic successful publication
		return false, false
	}

	runtime_.mu.Lock()
	cc, manager = runtime_.cc, runtime_.manager
	running := cc != nil && manager != nil && manager.State() == core.StateRunning
	a.mu.Lock()
	netEpoch.mu.Lock()
	current := a.protectionDesired && !a.profileTransition && !a.closing.Load() && a.operationID == receipt.intent && netEpoch.num == receipt.epoch && a.lastObservation <= receipt.observation
	if running && cc.SessionID != receipt.session {
		current = false
	}
	if !current || a.fpol.currentAttempt() != receipt.attempt {
		netEpoch.mu.Unlock()
		a.mu.Unlock()
		runtime_.mu.Unlock()
		discard()
		return false, false
	}
	if !errors.Is(err, core.ErrReloadRejected) {
		a.fpol.noteAttemptFailure(target, receipt.epoch, time.Now())
	}
	_, exhausted := a.fpol.endScopedAttempt(receipt.attempt, false, target, candidates)
	if running {
		a.sm.markSwitchFailed(time.Now())
	} // stopped paths keep the lifecycle's RECOVERING state/timer
	netEpoch.mu.Unlock()
	runtime_.mu.Unlock()
	a.pushStateLocked()
	a.mu.Unlock()
	a.appendLog("warn", "failover: "+err.Error())
	return false, exhausted && !running // candidate errors do not invalidate a retained live path
}
