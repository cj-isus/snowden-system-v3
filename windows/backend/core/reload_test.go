package core

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestManagerReloadRestoresVerifiedPreviousPath(t *testing.T) {
	oldProbe, candidateProbe := passProbe(), failProbe()
	m := newTestManager(t, oldProbe)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	oldRaw := append([]byte(nil), m.engine.rawJSON...)
	err := m.Reload(context.Background(), []byte(protectedFixture(t)), WithReloadProbe(candidateProbe))
	if !errors.Is(err, ErrSwitchRolledBack) || m.State() != StateRunning || m.engine.Status() != "running" {
		t.Fatalf("verified previous path not retained: err=%v manager=%s engine=%s", err, m.State(), m.engine.Status())
	}
	if oldProbe.calls != 2 || candidateProbe.calls != 1 || !bytes.Equal(m.engine.rawJSON, oldRaw) || m.probe != oldProbe {
		t.Fatalf("old config/probe not reapplied: old calls=%d candidate calls=%d", oldProbe.calls, candidateProbe.calls)
	}
	if m.proxy.(*fakeProxy).restore != 0 {
		t.Fatal("working rollback restored system proxy")
	}
}

func startedReloadManager(t *testing.T, old Probe, factory func(context.Context, []byte) (boxInstance, error)) *Manager {
	t.Helper()
	e := NewEngine([]byte(`{}`))
	if factory == nil {
		factory = func(context.Context, []byte) (boxInstance, error) { return &faultBox{}, nil }
	}
	e.newBox = factory
	m := NewManager()
	m.SetEngine(e)
	m.SetSystemProxy(&faultProxy{})
	m.SetProbe(old)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	return m
}

func TestReloadRollbackProofMatrix(t *testing.T) {
	for _, kind := range []string{"pass", "fail", "nil", "leak", "cancel", "panic"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			oldCalls, candidateCalls, creates := 0, 0, 0
			old := probeFunc(func(context.Context) *ProbeReport {
				oldCalls++
				if oldCalls == 1 {
					return &ProbeReport{AllPassed: true}
				}
				switch kind {
				case "fail":
					return &ProbeReport{}
				case "nil":
					return nil
				case "leak":
					return &ProbeReport{AllPassed: true, LeakConfirmed: true}
				case "cancel":
					cancel()
				case "panic":
					panic("synthetic rollback probe panic")
				}
				return &ProbeReport{AllPassed: true}
			})
			m := startedReloadManager(t, old, func(context.Context, []byte) (boxInstance, error) { creates++; return &faultBox{}, nil })
			candidate := probeFunc(func(context.Context) *ProbeReport { candidateCalls++; return &ProbeReport{} })
			err := m.Reload(ctx, []byte(protectedFixture(t)), WithReloadProbe(candidate))
			if oldCalls != 2 || candidateCalls != 1 || creates != 3 {
				t.Fatalf("wrong proof/config sequence: old=%d candidate=%d creates=%d", oldCalls, candidateCalls, creates)
			}
			var rollback *VerifiedRollbackError
			if kind == "pass" {
				if !errors.As(err, &rollback) || !errors.Is(err, ErrSwitchRolledBack) || !rollback.Restarted || !probePassed(rollback.Report) || m.State() != StateRunning || m.proxy.(*faultProxy).restoreCalls.Load() != 0 {
					t.Fatalf("fresh proof not retained: %v", err)
				}
			} else {
				if err == nil || errors.As(err, &rollback) || m.State() != StateError || m.engine.Status() != "stopped" || m.active != nil {
					t.Fatalf("unproved rollback accepted: kind=%s err=%v state=%s", kind, err, m.State())
				}
				if kind == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
				if kind == "panic" && !strings.Contains(err.Error(), "protected probe failed") {
					t.Fatalf("rollback panic lost the original candidate failure: %v", err)
				}
			}
		})
	}
}

func TestReloadRejectsCandidateWithoutReplacingLiveInstance(t *testing.T) {
	old := passProbe()
	m := newTestManager(t, old)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	box, raw := m.engine.box, m.engine.rawSnapshot()
	err := m.Reload(context.Background(), []byte(`{invalid`))
	if !errors.Is(err, ErrReloadRejected) || m.engine.box != box || !bytes.Equal(raw, m.engine.rawSnapshot()) || old.calls != 1 || m.State() != StateRunning {
		t.Fatalf("rejected input changed live session: %v", err)
	}
}

func TestReloadRollbackFailureNeverUsesUnverifiedCandidate(t *testing.T) {
	for _, phase := range []string{"candidate_close", "old_construct", "old_start", "old_cleanup"} {
		t.Run(phase, func(t *testing.T) {
			creates, oldProofs := 0, 0
			old := probeFunc(func(context.Context) *ProbeReport { oldProofs++; return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, old, func(context.Context, []byte) (boxInstance, error) {
				creates++
				if creates == 3 && phase == "old_construct" {
					return nil, errors.New("old construction failed")
				}
				b := &faultBox{}
				if creates == 2 && phase == "candidate_close" || creates == 3 && phase == "old_cleanup" {
					b.close = func() error { return errors.New("synthetic cleanup failed") }
				}
				if creates == 3 && (phase == "old_start" || phase == "old_cleanup") {
					b.start = func() error { return errors.New("old start failed") }
				}
				return b, nil
			})
			err := m.Reload(context.Background(), []byte(protectedFixture(t)), WithReloadProbe(failProbe()))
			if err == nil || errors.Is(err, ErrSwitchRolledBack) || m.State() != StateError || oldProofs != 1 {
				t.Fatalf("unverified candidate survived rollback error: %v state=%s oldProofs=%d", err, m.State(), oldProofs)
			}
			if phase == "candidate_close" && creates != 2 || phase == "old_cleanup" && creates != 3 {
				t.Fatal("replacement launched before uncertain cleanup finished")
			}
		})
	}
}

func TestConfirmedLeakCannotCommitOrRollback(t *testing.T) {
	for _, stage := range []string{"start", "reload", "switch"} {
		for _, claimedPass := range []bool{false, true} {
			t.Run(stage+map[bool]string{false: "/failed", true: "/contradictory_pass"}[claimedPass], func(t *testing.T) {
				leak := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: claimedPass, LeakConfirmed: true} })
				old := passProbe()
				var m *Manager
				var err error
				switch stage {
				case "start":
					m = newTestManager(t, leak)
					err = m.Start(context.Background())
				case "reload":
					m = startedReloadManager(t, old, nil)
					err = m.Reload(context.Background(), []byte(protectedFixture(t)), WithReloadProbe(leak))
				case "switch":
					m, _ = newSelectorManager(t, old)
					if err := m.Start(context.Background()); err != nil {
						t.Fatal(err)
					}
					err = m.SwitchChannel(context.Background(), "proxy", "chan-b", leak, old)
				}
				defer m.Stop(context.Background())
				if err == nil || errors.Is(err, ErrSwitchRolledBack) || m.State() != StateError || m.engine.Status() != "stopped" {
					t.Fatalf("unsafe path was accepted: %v state=%s", err, m.State())
				}
				if stage != "start" && old.calls != 1 {
					t.Fatal("confirmed leak initiated a rollback probe")
				}
			})
		}
	}
}

func TestStopDuringReloadRollbackRejectsLateProof(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	calls := 0
	old := probeFunc(func(context.Context) *ProbeReport {
		calls++
		if calls == 2 {
			close(entered)
			<-release
		}
		return &ProbeReport{AllPassed: true}
	})
	m := startedReloadManager(t, old, nil)
	reloaded := make(chan error, 1)
	raw := []byte(protectedFixture(t))
	go func() { reloaded <- m.Reload(context.Background(), raw, WithReloadProbe(failProbe())) }()
	awaitLifecycle(t, entered)
	cancelled := observeOperationCancel(m)
	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(context.Background()) }()
	awaitLifecycle(t, cancelled)
	unblock()
	if err := awaitLifecycle(t, reloaded); !errors.Is(err, context.Canceled) || errors.Is(err, ErrSwitchRolledBack) {
		t.Fatalf("late rollback proof accepted: %v", err)
	}
	if err := awaitLifecycle(t, stopped); err != nil || m.State() != StateStopped {
		t.Fatalf("Stop did not win: %v", err)
	}
}

func TestManagerReloadStartupRollbackRequiresFreshProbe(t *testing.T) {
	e := NewEngine([]byte(`{}`))
	creates := 0
	e.newBox = func(context.Context, []byte) (boxInstance, error) {
		creates++
		if creates == 2 {
			return nil, errors.New("synthetic candidate construction failure")
		}
		return &faultBox{}, nil
	}
	oldProbe := passProbe()
	m := NewManager()
	m.SetEngine(e)
	m.SetSystemProxy(&faultProxy{})
	m.SetProbe(oldProbe)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	err := m.Reload(context.Background(), []byte(protectedFixture(t)))
	if !errors.Is(err, ErrSwitchRolledBack) || oldProbe.calls != 2 || m.State() != StateRunning || creates != 3 {
		t.Fatalf("old path accepted without fresh proof: err=%v calls=%d state=%s creates=%d", err, oldProbe.calls, m.State(), creates)
	}
}
