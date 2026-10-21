package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

func TestChannelSwitchErrorPublishesOnlySupportedOutcomes(t *testing.T) {
	for _, outcome := range []string{"rejected", "selector_rollback", "reload_rollback", "unproved_label", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			args := os.Args
			os.Args = []string{"synthetic.exe", "--tun"}
			t.Cleanup(func() { os.Args = args })
			a := localRunningApp(t, nil)
			policy := &fakeProtection{}
			a.policy = policy
			oldCC, manager := runtime_.cc, runtime_.manager
			oldCC.SelectorDefault = "old-channel"
			a.state.ActiveID, a.state.State = "old-channel", "reloading"
			oldMetrics := &metricsState{started: true, lastUp: 500, lastDown: 1000}
			runtime_.metrics = oldMetrics
			cause := errors.New("synthetic candidate failure")
			var result error = cause
			switch outcome {
			case "rejected":
				result = errors.Join(core.ErrReloadRejected, cause)
			case "selector_rollback", "reload_rollback":
				result = &core.VerifiedRollbackError{Cause: cause, Report: &core.ProbeReport{AllPassed: true}, Restarted: outcome == "reload_rollback"}
			case "unproved_label":
				result = errors.Join(core.ErrSwitchRolledBack, cause)
			}
			a.operationMu.Lock()
			err := a.finishChannelSwitchError(context.Background(), 1, "candidate", manager, oldCC, result)
			a.operationMu.Unlock()
			if !errors.Is(err, cause) {
				t.Fatalf("original failure lost: %v", err)
			}
			if outcome == "failed" || outcome == "unproved_label" {
				if a.GetState().State != "error" || runtime_.manager != nil || policy.releases != 0 {
					t.Fatal("unsupported outcome was published as a working path")
				}
				return
			}
			if a.GetState().State != "running" || a.GetState().ActiveID != "old-channel" || runtime_.manager != manager || policy.blocks != 0 || policy.releases != 0 {
				t.Fatal("retained old path was stopped or replaced by candidate")
			}
			if outcome != "rejected" && (a.GetState().ProbeLastAt == "" || a.GetState().Probe == nil || !a.GetState().Probe.OK) {
				t.Fatal("fresh rollback proof was not published")
			}
			if outcome == "reload_rollback" {
				if runtime_.cc == oldCC || runtime_.cc.SelectorDefault != oldCC.SelectorDefault || runtime_.metrics == oldMetrics {
					t.Fatal("restarted engine reused old observation/metrics lifetime")
				}
				if a.failSession(oldCC, &core.ProbeReport{}, "obsolete watchdog", true) || a.GetState().State != "running" {
					t.Fatal("old watchdog stopped restored session")
				}
			} else if runtime_.cc != oldCC || runtime_.metrics != oldMetrics {
				t.Fatal("unchanged live instance lost its observation/metrics lifetime")
			}
		})
	}
}

func TestChannelRollbackCannotUndoStopOrNewSession(t *testing.T) {
	for _, changed := range []string{"stop", "new_session", "new_session_fatal"} {
		t.Run(changed, func(t *testing.T) {
			args := os.Args
			os.Args = []string{"synthetic.exe", "--tun"}
			t.Cleanup(func() { os.Args = args })
			a := localRunningApp(t, nil)
			a.policy = &fakeProtection{}
			oldCC, manager := runtime_.cc, runtime_.manager
			if changed == "stop" {
				if err := a.Stop(); err != nil {
					t.Fatal(err)
				}
			} else {
				next := *oldCC
				runtime_.cc = &next
			}
			current := runtime_.cc
			var result error = &core.VerifiedRollbackError{Report: &core.ProbeReport{AllPassed: true}, Restarted: true}
			if changed == "new_session_fatal" {
				result = errors.New("obsolete fatal result")
			}
			a.operationMu.Lock()
			err := a.finishChannelSwitchError(context.Background(), 1, "candidate", manager, oldCC, result)
			a.operationMu.Unlock()
			if changed == "stop" && (!errors.Is(err, context.Canceled) || a.GetState().State != "stopped") {
				t.Fatalf("rollback undid Stop: %v", err)
			}
			if changed != "stop" && !errors.Is(err, errSessionSuperseded) {
				t.Fatalf("stale rollback not rejected: %v", err)
			}
			if runtime_.cc != current {
				t.Fatal("old rollback replaced current session")
			}
		})
	}
}

func TestAutomaticSwitchOutcomesRespectFreshRollbackProof(t *testing.T) {
	for _, outcome := range []string{"committed", "rolled_back", "cancelled", "rejected", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			a := NewApp()
			a.operationID, a.protectionDesired = 1, true
			now := time.Now()
			netEpoch.bump("up", "synthetic-network", now)
			epoch, _, _ := netEpoch.current()
			chanHealth.mu.Lock()
			chanHealth.resetLocked(epoch)
			chanHealth.mu.Unlock()
			t.Cleanup(func() {
				netEpoch.resetForTest()
				chanHealth.mu.Lock()
				chanHealth.resetLocked(0)
				chanHealth.mu.Unlock()
			})
			if !a.fpol.beginAttemptIfDue(now) {
				t.Fatal("attempt not started")
			}
			var result error
			switch outcome {
			case "rolled_back":
				result = &core.VerifiedRollbackError{Report: &core.ProbeReport{AllPassed: true}}
			case "cancelled":
				result = context.Canceled
			case "rejected":
				result = core.ErrReloadRejected
			case "failed":
				result = errors.New("synthetic switch failure")
			}
			switched, blocked := a.finishFailoverAttempt(1, "candidate", 1, result)
			if switched != (outcome == "committed") || blocked != (outcome == "failed" || outcome == "rejected") {
				t.Fatalf("wrong automatic outcome: switched=%v blocked=%v", switched, blocked)
			}
			if outcome == "rolled_back" {
				state, attempts, _ := a.fpol.snapshot()
				if state != "closed" || attempts != 0 {
					t.Fatal("healthy rollback was treated as exhausted outage")
				}
			}
			_, recorded := chanHealth.stateOf(epoch, "candidate")
			if recorded != (outcome == "failed" || outcome == "rolled_back") {
				t.Fatal("candidate evidence invented or discarded")
			}
		})
	}
}

func TestLateAutomaticRollbackCannotOverwriteStopOrNewAttempt(t *testing.T) {
	for _, desired := range []bool{false, true} {
		t.Run(map[bool]string{false: "stopped", true: "new_attempt"}[desired], func(t *testing.T) {
			a := NewApp()
			a.operationID, a.protectionDesired = 2, desired
			a.sm.markStopped(time.Now())
			a.fpol.beginAttemptIfDue(time.Now())
			result := &core.VerifiedRollbackError{Report: &core.ProbeReport{AllPassed: true}}
			switched, blocked := a.finishFailoverAttempt(1, "obsolete", 1, result)
			if switched || blocked {
				t.Fatal("obsolete automatic result produced a channel outcome")
			}
			state, _, _ := a.fpol.snapshot()
			if desired && state != "half-open" || !desired && state != "closed" {
				t.Fatal("old attempt changed the new policy owner")
			}
		})
	}
}
