package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

func prepareFixture(t *testing.T, level string) []byte {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(protectedFixture(t)), &cfg); err != nil {
		t.Fatal(err)
	}
	if level != "" {
		cfg["log"] = map[string]any{"level": level}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReloadPrepareRejectsOptionsWithoutTouchingSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ReloadOption
	}{
		{"nil option", []ReloadOption{nil}},
		{"panic", []ReloadOption{func(*reloadOpts) { panic("synthetic prepare panic") }}},
		{"panic after probe selection", []ReloadOption{WithReloadProbe(probeFunc(func(context.Context) *ProbeReport {
			t.Fatal("uncommitted probe ran")
			return nil
		})), func(*reloadOpts) { panic("synthetic prepare panic") }}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			box, raw := m.engine.box, m.engine.rawSnapshot()
			var err error
			var escaped any
			func() {
				defer func() { escaped = recover() }()
				err = m.Reload(context.Background(), prepareFixture(t, ""), tc.opts...)
			}()
			// Baseline had a panic before its finalizer and kept this lock held.
			// Repair only that proven test failure so t.Cleanup cannot deadlock.
			if m.mu.TryLock() {
				m.mu.Unlock()
			} else {
				m.mu.Unlock()
				t.Error("preparation retained the manager mutex")
			}
			if escaped != nil || !errors.Is(err, ErrReloadRejected) {
				t.Errorf("prepare must return rejection instead of panic/success: panic=%v err=%v", escaped, err)
			}
			if tc.name != "nil option" && (err == nil || !strings.Contains(err.Error(), "synthetic prepare panic")) {
				t.Error("prepare failure cause lost")
			}
			if m.State() != StateRunning || m.active != nil || m.engine.box != box || !bytes.Equal(m.engine.rawSnapshot(), raw) || m.probe != &old || m.proxy.(*faultProxy).restoreCalls.Load() != 0 {
				t.Fatal("failed preparation changed the old live session")
			}
		})
	}
}

func TestReloadPrepareStopAndNewOwnerRejectLateCandidate(t *testing.T) {
	for _, action := range []string{"stop", "stop-start", "replace-engine", "another-reload", "selector-panic"} {
		t.Run(action, func(t *testing.T) {
			old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			candidateCalls := 0
			candidate := probeFunc(func(context.Context) *ProbeReport {
				candidateCalls++
				return &ProbeReport{AllPassed: true}
			})
			go func() {
				done <- m.Reload(context.Background(), prepareFixture(t, "debug"), func(ro *reloadOpts) {
					close(entered)
					<-release
					ro.probe = candidate
				})
			}()
			awaitLifecycle(t, entered)
			observed := make(chan State, 1)
			go func() { observed <- m.State() }()
			if awaitLifecycle(t, observed) != StateRunning {
				t.Fatal("uncommitted preparation changed state")
			}
			if action == "another-reload" {
				if err := m.Reload(context.Background(), prepareFixture(t, "warn")); err != nil {
					t.Fatal(err)
				}
			} else if action == "selector-panic" {
				// faultBox deliberately has no outbound manager: exercise the
				// newer operation's panic/finalizer, then reject the late prepare.
				if err := m.SwitchChannel(context.Background(), "proxy", "candidate", &old, &old); err == nil || !strings.Contains(err.Error(), "switch panic") || m.State() != StateError {
					t.Fatalf("expected injected selector panic: %v", err)
				}
			} else {
				stopped := make(chan error, 1)
				go func() { stopped <- m.Stop(context.Background()) }()
				if err := awaitLifecycle(t, stopped); err != nil {
					t.Fatalf("Stop waited for uncommitted callback: %v", err)
				}
				if action != "stop" {
					if action == "replace-engine" {
						m.SetEngine(engineWithBox(&faultBox{}))
					}
					m.SetProbe(&old)
					if err := m.Start(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
			}
			box, raw, state, probe := m.engine.box, m.engine.rawSnapshot(), m.State(), m.probe
			restored := m.proxy.(*faultProxy).restoreCalls.Load()
			releaseOnce.Do(func() { close(release) })
			err := awaitLifecycle(t, done)
			if !errors.Is(err, ErrReloadRejected) || !errors.Is(err, ErrInvalidState) {
				t.Fatalf("late prepare not rejected: %v", err)
			}
			if m.State() != state || m.active != nil || m.engine.box != box || !bytes.Equal(m.engine.rawSnapshot(), raw) || m.probe != probe || m.proxy.(*faultProxy).restoreCalls.Load() != restored || candidateCalls != 0 {
				t.Fatal("late candidate mutated the latest lifecycle owner")
			}
		})
	}
}

func TestReloadPrepareCancellationRejectsWithoutCleanup(t *testing.T) {
	for _, kind := range []string{"before-call", "in-option", "panic-and-cancel", "blocked-option"} {
		t.Run(kind, func(t *testing.T) {
			old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			box, raw := m.engine.box, m.engine.rawSnapshot()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			nextCalled := false
			var err error
			if kind == "before-call" {
				cancel()
				err = m.Reload(ctx, prepareFixture(t, ""), func(*reloadOpts) { nextCalled = true })
			} else if kind == "blocked-option" {
				entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
				var once sync.Once
				defer once.Do(func() { close(release) })
				go func() {
					done <- m.Reload(ctx, prepareFixture(t, ""), func(*reloadOpts) {
						close(entered)
						<-release
					}, func(*reloadOpts) { nextCalled = true })
				}()
				awaitLifecycle(t, entered)
				cancel()
				if m.State() != StateRunning {
					t.Fatal("cancelled prepare changed the running state")
				}
				once.Do(func() { close(release) })
				err = awaitLifecycle(t, done)
			} else {
				err = m.Reload(ctx, prepareFixture(t, ""), func(*reloadOpts) {
					cancel()
					if kind == "panic-and-cancel" {
						panic("synthetic cancelled prepare")
					}
				}, func(*reloadOpts) { nextCalled = true })
			}
			if !errors.Is(err, context.Canceled) || (kind != "before-call" && !errors.Is(err, ErrReloadRejected)) || nextCalled {
				t.Fatalf("cancelled preparation continued or lost cancellation: next=%v err=%v", nextCalled, err)
			}
			if kind == "panic-and-cancel" && !strings.Contains(err.Error(), "synthetic cancelled prepare") {
				t.Fatal("panic cause was lost")
			}
			if m.State() != StateRunning || m.active != nil || m.engine.box != box || !bytes.Equal(m.engine.rawSnapshot(), raw) || m.probe != &old || m.proxy.(*faultProxy).restoreCalls.Load() != 0 {
				t.Fatal("cancelled preparation cleaned up or mutated the old session")
			}
		})
	}
}

func TestReloadPrepareCanReenterManagerBeforeCommit(t *testing.T) {
	for _, action := range []string{"state", "stop", "stop-start", "nested-reload"} {
		t.Run(action, func(t *testing.T) {
			old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			var callbackErr error
			err := m.Reload(context.Background(), prepareFixture(t, "debug"), func(*reloadOpts) {
				if m.State() != StateRunning {
					callbackErr = errors.New("prepare claimed running lifecycle")
					return
				}
				switch action {
				case "stop", "stop-start":
					callbackErr = m.Stop(context.Background())
					if callbackErr == nil && action == "stop-start" {
						callbackErr = m.Start(context.Background())
					}
				case "nested-reload":
					callbackErr = m.Reload(context.Background(), prepareFixture(t, "warn"))
				}
			})
			if callbackErr != nil {
				t.Fatal(callbackErr)
			}
			if action == "state" {
				if err != nil || m.State() != StateRunning {
					t.Fatalf("read-only reentry failed: %v", err)
				}
			} else if !errors.Is(err, ErrReloadRejected) || !errors.Is(err, ErrInvalidState) {
				t.Fatalf("reentrant new owner was overwritten: %v", err)
			}
			if action == "nested-reload" && !strings.Contains(string(m.engine.rawSnapshot()), "warn") {
				t.Fatal("outer preparation overwrote the nested committed config")
			}
		})
	}
}

func TestReloadPrepareUncommittedRejectionDoesNotSupersedeAnotherPrepare(t *testing.T) {
	old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
	m := startedReloadManager(t, &old, nil)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var once sync.Once
	defer once.Do(func() { close(release) })
	go func() {
		done <- m.Reload(context.Background(), prepareFixture(t, "debug"), func(*reloadOpts) {
			close(entered)
			<-release
		})
	}()
	awaitLifecycle(t, entered)
	if err := m.Reload(context.Background(), prepareFixture(t, ""), nil); !errors.Is(err, ErrReloadRejected) {
		t.Fatalf("second uncommitted candidate was not rejected: %v", err)
	}
	once.Do(func() { close(release) })
	if err := awaitLifecycle(t, done); err != nil || m.State() != StateRunning || !strings.Contains(string(m.engine.rawSnapshot()), "debug") {
		t.Fatalf("uncommitted rejection superseded a valid candidate: %v", err)
	}
}

func TestReloadPrepareProbeOptionsCommitInOrder(t *testing.T) {
	for _, kind := range []string{"default", "nil-override", "last-wins"} {
		t.Run(kind, func(t *testing.T) {
			oldCalls, firstCalls, lastCalls := 0, 0, 0
			old := probeFunc(func(context.Context) *ProbeReport { oldCalls++; return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			first := probeFunc(func(context.Context) *ProbeReport { firstCalls++; return &ProbeReport{AllPassed: true} })
			last := probeFunc(func(context.Context) *ProbeReport {
				lastCalls++
				if m.State() != StateReloading {
					t.Error("candidate probe ran before commit")
				}
				return &ProbeReport{AllPassed: true}
			})
			var opts []ReloadOption
			if kind == "nil-override" {
				opts = []ReloadOption{WithReloadProbe(nil)}
			} else if kind == "last-wins" {
				opts = []ReloadOption{WithReloadProbe(&first), WithReloadProbe(&last)}
			}
			if err := m.Reload(context.Background(), prepareFixture(t, ""), opts...); err != nil {
				t.Fatal(err)
			}
			if kind == "last-wins" {
				if oldCalls != 1 || firstCalls != 0 || lastCalls != 1 || m.probe != &last {
					t.Fatal("ordered probe options not committed correctly")
				}
			} else if oldCalls != 2 || m.probe != &old {
				t.Fatal("default probe was not preserved")
			}
		})
	}
}

func TestReloadPrepareValidationRunsBeforeOptions(t *testing.T) {
	for _, raw := range []string{"", `{}`, `null`, `{"unexpected":true}`} {
		t.Run(raw, func(t *testing.T) {
			old := probeFunc(func(context.Context) *ProbeReport { return &ProbeReport{AllPassed: true} })
			m := startedReloadManager(t, &old, nil)
			box, previous := m.engine.box, m.engine.rawSnapshot()
			called := false
			err := m.Reload(context.Background(), []byte(raw), func(*reloadOpts) { called = true })
			if !errors.Is(err, ErrReloadRejected) || called || m.State() != StateRunning || m.active != nil || m.probe != &old || m.engine.box != box || !bytes.Equal(m.engine.rawSnapshot(), previous) || m.proxy.(*faultProxy).restoreCalls.Load() != 0 {
				t.Fatalf("invalid input reached options/commit or changed the active session: %v", err)
			}
		})
	}
}
