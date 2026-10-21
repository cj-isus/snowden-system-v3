package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

func awaitProbe[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("probe operation did not reach the expected phase")
		var zero T
		return zero
	}
}

func TestAppProbeFaultCannotEscapeOrRemainBusy(t *testing.T) {
	for _, kind := range []string{"factory-panic", "run-panic", "nil-probe", "typed-nil-probe", "nil-report"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			a := localRunningApp(t, nil)
			a.probeFactory = func(*render.ClientConfig) core.Probe {
				switch kind {
				case "factory-panic":
					panic("synthetic private factory payload")
				case "run-panic":
					return localProbe(func(context.Context) *core.ProbeReport { panic("synthetic private run payload") })
				case "typed-nil-probe":
					return (*core.ProtectedProbe)(nil)
				case "nil-report":
					return localProbe(func(context.Context) *core.ProbeReport { return nil })
				default:
					return nil
				}
			}
			m := runtime_.manager
			var err error
			var escaped any
			func() {
				defer func() { escaped = recover() }()
				err = a.RunProbe()
			}()
			state := a.GetState()
			if escaped != nil || err == nil || state.ProbeRunning || state.Probe == nil || state.Probe.OK || state.ProbeLastAt != "" {
				t.Fatalf("invalid diagnostic escaped or left false facts: panic=%v err=%v busy=%v report=%+v", escaped, err, state.ProbeRunning, state.Probe)
			}
			if m.State() != core.StateRunning || state.State != "running" || runtime_.manager != m {
				t.Fatal("unconfirmed diagnostic fault tore down the existing session")
			}
			a.mu.Lock()
			logText := fmt.Sprint(a.logs)
			a.mu.Unlock()
			if strings.Contains(err.Error()+logText+fmt.Sprint(state.Probe), "synthetic private") {
				t.Fatal("arbitrary panic payload escaped into diagnostics")
			}
		})
	}
}

func TestAppProbeNormalizationKeepsFactsAndSourceImmutable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		report   *core.ProbeReport
		ok, leak bool
	}{
		{"nil", nil, false, false},
		{"empty", &core.ProbeReport{}, false, false},
		{"pass", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "echo", Passed: true, Status: "pass"}}}, true, false},
		{"aggregate failed", &core.ProbeReport{Results: []core.ProbeResult{{Name: "echo", Passed: true, Status: "pass"}}}, false, false},
		{"failed step marked pass", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "echo", Status: "pass"}}}, false, false},
		{"unavailable", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "dns", Passed: true, Status: "unavailable"}}}, false, false},
		{"skipped", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "dns", Passed: true, Status: "skipped"}}}, false, false},
		{"unknown status", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "dns", Passed: true, Status: "unknown"}}}, false, false},
		{"leak flag", &core.ProbeReport{AllPassed: true, LeakConfirmed: true}, false, true},
		{"leak step", &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "route", Passed: true, Status: "leak"}}}, false, true},
		{"failed step", &core.ProbeReport{Results: []core.ProbeResult{{Name: "echo", Detail: "synthetic failure"}}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before *core.ProbeReport
			if tc.report != nil {
				copy := *tc.report
				copy.Results = append([]core.ProbeResult(nil), tc.report.Results...)
				before = &copy
			}
			out := normalizeProbeReport(tc.report)
			if out.AllPassed != tc.ok || out.LeakConfirmed != tc.leak || (!tc.ok && len(out.Results) == 0) {
				t.Fatalf("facts not respected: %+v", out)
			}
			if !reflect.DeepEqual(out, normalizeProbeReport(out)) {
				t.Fatal("normalization added duplicate facts")
			}
			view := toView(out)
			if view.OK != tc.ok || !reflect.DeepEqual(tc.report, before) {
				t.Fatal("view changed aggregate or mutated the input")
			}
			for _, step := range out.Results {
				if !step.Passed && step.Status == "pass" {
					t.Fatal("failed fact still displayed as a passing step")
				}
			}
			out.Results[0].Detail = "changed only in the consumer"
			if !reflect.DeepEqual(tc.report, before) {
				t.Fatal("result slices still alias")
			}
		})
	}
}

func TestAppProbeManualOutcomeUsesFactualGate(t *testing.T) {
	for _, kind := range []string{"pass", "fail", "empty", "contradictory-step", "leak-flag", "leak-step"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			args := os.Args
			os.Args = []string{"synthetic.exe", "--tun"}
			t.Cleanup(func() { os.Args = args })
			input := &core.ProbeReport{AllPassed: true, Results: []core.ProbeResult{{Name: "echo", Passed: true}}}
			switch kind {
			case "fail":
				input.AllPassed = false
			case "empty":
				input = &core.ProbeReport{}
			case "contradictory-step":
				input.Results[0].Passed, input.Results[0].Status = false, "pass"
			case "leak-flag":
				input.LeakConfirmed = true
			case "leak-step":
				input.Results[0].Status = "leak"
			}
			a := localRunningApp(t, localProbe(func(context.Context) *core.ProbeReport { return input }))
			policy := &fakeProtection{}
			a.policy = policy
			a.state.ProbeLastAt = "previous-success"
			manager := runtime_.manager
			err := a.RunProbe()
			state := a.GetState()
			if state.ProbeRunning || state.Probe == nil || state.Probe.OK != (kind == "pass") || (err == nil) != (kind == "pass") {
				t.Fatalf("incorrect manual outcome: err=%v state=%+v", err, state)
			}
			if kind == "pass" {
				if state.ProbeLastAt == "previous-success" {
					t.Fatal("successful proof time was not updated")
				}
			} else if state.ProbeLastAt != "previous-success" {
				t.Fatal("failed probe received a fresh success timestamp")
			}
			leak := strings.HasPrefix(kind, "leak")
			if leak {
				if manager.State() != core.StateStopped || policy.blocks != 1 || policy.releases != 0 || state.State != "error" {
					t.Fatalf("confirmed leak did not retain protection: %+v state=%s", policy, state.State)
				}
			} else if manager.State() != core.StateRunning || policy.blocks != 0 || policy.releases != 0 {
				t.Fatal("unconfirmed diagnostic outcome tore down the existing session")
			}
		})
	}
}

func TestAppProbeCancellationNeverPublishesLatePassOrLeak(t *testing.T) {
	for _, leak := range []bool{false, true} {
		t.Run(fmt.Sprint(leak), func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			args := os.Args
			os.Args = []string{"synthetic.exe", "--tun"}
			t.Cleanup(func() { os.Args = args })
			entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			a := localRunningApp(t, localProbe(func(ctx context.Context) *core.ProbeReport {
				close(entered)
				<-ctx.Done()
				close(cancelled)
				<-release
				return &core.ProbeReport{AllPassed: true, LeakConfirmed: leak}
			}))
			policy := &fakeProtection{}
			a.policy = policy
			defer func() {
				a.mu.Lock()
				cancel := a.operationCancel
				a.mu.Unlock()
				if cancel != nil {
					cancel()
				}
				releaseOnce.Do(func() { close(release) })
			}()
			a.state.ProbeLastAt = "previous-success"
			probeDone, stopDone := make(chan error, 1), make(chan error, 1)
			go func() { probeDone <- a.RunProbe() }()
			awaitProbe(t, entered)
			go func() { stopDone <- a.Stop() }()
			awaitProbe(t, cancelled)
			releaseOnce.Do(func() { close(release) })
			if err := awaitProbe(t, probeDone); !errors.Is(err, context.Canceled) {
				t.Fatalf("late probe lost cancellation: %v", err)
			}
			if err := awaitProbe(t, stopDone); err != nil {
				t.Fatal(err)
			}
			state := a.GetState()
			if state.ProbeRunning || state.ProbeLastAt != "previous-success" || state.Probe != nil || state.State != "stopped" || policy.blocks != 0 || policy.releases != 1 {
				t.Fatalf("late report changed Disconnect outcome: state=%+v policy=%+v", state, policy)
			}
		})
	}
}

func TestAppProbeGuardContextAndFactorySnapshot(t *testing.T) {
	for _, kind := range []string{"nil-context", "cancelled", "nil-config", "cancel-in-factory", "cancel-in-run", "factory-snapshot"} {
		t.Run(kind, func(t *testing.T) {
			a := NewApp()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			config := &render.ClientConfig{ExpectedEgress: "192.0.2.1", ProbeTargets: []string{"https://one.invalid", "https://two.invalid"}}
			factoryCalls, runCalls := 0, 0
			a.probeFactory = func(got *render.ClientConfig) core.Probe {
				factoryCalls++
				if got.ExpectedEgress != "192.0.2.1" || got.ProbeTargets[0] != "https://one.invalid" {
					t.Error("probe adopted mutated configuration inputs")
				}
				if kind == "cancel-in-factory" {
					cancel()
				}
				return localProbe(func(context.Context) *core.ProbeReport {
					runCalls++
					if kind == "cancel-in-run" {
						cancel()
					}
					return &core.ProbeReport{AllPassed: true}
				})
			}
			if kind == "cancelled" {
				cancel()
			} else if kind == "nil-context" {
				ctx = nil
			} else if kind == "nil-config" {
				config = nil
			}
			probe := a.protectedProbe(config)
			if config != nil {
				config.ExpectedEgress = "192.0.2.2"
				config.ProbeTargets[0] = "https://changed.invalid"
			}
			a.probeFactory = func(*render.ClientConfig) core.Probe { t.Fatal("probe adopted a later factory"); return nil }
			report := probe.Run(ctx)
			if report == nil || report.AllPassed != (kind == "factory-snapshot") {
				t.Fatalf("context/factory outcome mismatch: %+v", report)
			}
			if (kind == "nil-context" || kind == "cancelled" || kind == "nil-config") && factoryCalls != 0 {
				t.Fatal("invalid input reached factory")
			}
			if kind == "cancel-in-factory" && (factoryCalls != 1 || runCalls != 0) {
				t.Fatal("cancelled factory still started the probe")
			}
		})
	}
	cc := &render.ClientConfig{ProbeTargets: []string{"https://one.invalid", "https://two.invalid"}}
	probe := newAppProbe(cc)
	cc.ProbeTargets[0] = "https://changed.invalid"
	if probe.Targets[0] != "https://one.invalid" {
		t.Fatal("probe target snapshot aliases its config")
	}
}

func TestAppProbeFastReprobeUsesGuardAndDropsSupersededResult(t *testing.T) {
	for _, kind := range []string{"pass", "fail", "panic", "nil", "leak", "session-changed", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			a := localRunningApp(t, nil)
			cc := runtime_.cc
			a.probeFactory = func(*render.ClientConfig) core.Probe {
				return localProbe(func(context.Context) *core.ProbeReport {
					calls++
					switch kind {
					case "panic":
						panic("synthetic private run payload")
					case "nil":
						return nil
					case "session-changed":
						runtime_.mu.Lock()
						runtime_.cc = &render.ClientConfig{}
						runtime_.mu.Unlock()
					case "cancelled":
						cancel()
					}
					return &core.ProbeReport{AllPassed: kind != "fail", LeakConfirmed: kind == "leak"}
				})
			}
			recovered, report := a.fastReprobeAfter(ctx, cc, 0)
			if recovered != (kind == "pass") {
				t.Fatalf("false recovery: kind=%s report=%+v", kind, report)
			}
			if kind == "session-changed" || kind == "cancelled" {
				if calls != 1 || report != nil {
					t.Fatal("late/cancelled result escaped fast reprobe")
				}
			} else if report == nil || (report.LeakConfirmed != (kind == "leak")) {
				t.Fatalf("guarded report lost: %+v", report)
			}
			if (kind == "pass" || kind == "leak") && calls != 1 {
				t.Fatal("successful path or confirmed leak was unnecessarily retried")
			}
		})
	}
}

func TestAppProbeWatchdogAndNetworkUseGuard(t *testing.T) {
	for _, entry := range []string{"watchdog-pass", "watchdog-leak", "network-leak"} {
		t.Run(entry, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			args := os.Args
			os.Args = []string{"synthetic.exe"}
			t.Cleanup(func() { os.Args = args })
			leak := entry != "watchdog-pass"
			if leak {
				os.Args = append(os.Args, "--tun")
			}
			calls := 0
			a := localRunningApp(t, localProbe(func(context.Context) *core.ProbeReport {
				calls++
				return &core.ProbeReport{AllPassed: true, LeakConfirmed: leak}
			}))
			policy := &fakeProtection{}
			a.policy = policy
			if entry == "network-leak" {
				a.netChangeReprobe()
			} else {
				ticks := make(chan time.Time, 1)
				ticks <- time.Now()
				close(ticks)
				a.watchCoreTicks(context.Background(), ticks)
			}
			state := a.GetState()
			if calls != 1 || state.Probe == nil || state.Probe.OK == leak {
				t.Fatalf("entrypoint bypassed guard: calls=%d state=%+v", calls, state)
			}
			if leak && (policy.blocks != 1 || policy.releases != 0 || state.State != "error") {
				t.Fatalf("unsafe route protection not retained: %+v state=%s", policy, state.State)
			}
		})
	}
}

func TestAppProbeLeakCannotProducePass(t *testing.T) {
	view := toView(&core.ProbeReport{AllPassed: true, LeakConfirmed: true, Results: []core.ProbeResult{{Name: "echo", Passed: true}}})
	if view == nil || view.OK {
		t.Fatal("confirmed leak displayed as PASS")
	}
	for _, step := range view.Steps {
		if step.Status == "leak" {
			return
		}
	}
	t.Fatal("confirmed leak has no explicit failing evidence step")
}

func TestAppProbeLateResultCannotClearNewOwnerBusyFlag(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := localRunningApp(t, nil)
	a.probeFactory = func(*render.ClientConfig) core.Probe {
		return localProbe(func(context.Context) *core.ProbeReport {
			a.mu.Lock()
			a.operationID++
			a.state.ProbeRunning = true // a newer reserved operation owns this flag
			a.state.Probe = &ProbeReportView{OK: false, Steps: []ProbeStepView{{Name: "new owner"}}}
			a.mu.Unlock()
			return &core.ProbeReport{AllPassed: true}
		})
	}
	err := a.RunProbe()
	state := a.GetState()
	if !errors.Is(err, context.Canceled) || !state.ProbeRunning || state.Probe == nil || len(state.Probe.Steps) != 1 || state.Probe.Steps[0].Name != "new owner" {
		t.Fatalf("late result changed new-owner facts: err=%v busy=%v report=%+v", err, state.ProbeRunning, state.Probe)
	}
}
