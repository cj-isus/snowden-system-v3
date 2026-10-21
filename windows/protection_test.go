package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

type fakeProtection struct {
	arms, blocks, releases int
	err                    error
}

func (p *fakeProtection) Arm() error { p.arms++; return p.err }

func (p *fakeProtection) BlockDirect() error { p.blocks++; return p.err }
func (p *fakeProtection) Release() error     { p.releases++; return p.err }

type localProbe func(context.Context) *core.ProbeReport

func (p localProbe) Run(ctx context.Context) *core.ProbeReport { return p(ctx) }

// A real local engine, fake registry/probe; no server or OS proxy changes.
func localRunningApp(t *testing.T, p core.Probe, proxy ...*retryCleanupProxy) *App {
	t.Helper()
	old := runtime_
	runtime_ = &coreRuntime{}
	t.Cleanup(func() { runtime_ = old })
	a := NewApp()
	a.protectionDesired = true
	a.operationID = 1
	a.state.State = "running"
	a.probeFactory = func(*render.ClientConfig) core.Probe { return p }
	cc := &render.ClientConfig{Raw: []byte(`{"log":{"disabled":true},"inbounds":[{"type":"mixed","tag":"socks-in","listen":"127.0.0.1","listen_port":0}],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`)}
	e := core.NewEngine(cc.Raw)
	m := core.NewManager()
	m.SetEngine(e)
	if len(proxy) > 0 {
		m.SetSystemProxy(proxy[0])
	} else {
		m.SetSystemProxy(noopProxy{})
	}
	m.SetProbe(localProbe(func(context.Context) *core.ProbeReport { return &core.ProbeReport{AllPassed: true} }))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime_.manager, runtime_.engine, runtime_.cc = m, e, cc
	t.Cleanup(func() {
		_ = m.Stop(context.Background())
		a.mu.Lock()
		a.cancelFailClosedRetryLocked()
		a.mu.Unlock()
	})
	return a
}

type retryCleanupProxy struct {
	restoreErr error
	restores   int
}

// sing-box 1.13.19 Start internally calls Close on failure but discards its
// error. The following Close returns os.ErrClosed; quiescence is unproved.
// Use a real loopback bind collision and a fake policy to verify the whole
// Engine -> Manager -> App chain cannot turn that uncertainty into Release.
func TestUnconfirmedEngineCleanupCannotReleaseProtection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	args, previous := os.Args, runtime_
	os.Args, runtime_ = []string{"synthetic.exe", "--tun"}, &coreRuntime{}
	t.Cleanup(func() { os.Args, runtime_ = args, previous })
	raw := []byte(fmt.Sprintf(`{"log":{"disabled":true},"inbounds":[{"type":"mixed","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`, listener.Addr().(*net.TCPAddr).Port))
	e, m := core.NewEngine(raw), core.NewManager()
	m.SetEngine(e)
	m.SetSystemProxy(noopProxy{})
	m.SetProbe(localProbe(func(context.Context) *core.ProbeReport { return &core.ProbeReport{AllPassed: true} }))
	if err := m.Start(context.Background()); !errors.Is(err, core.ErrEngineCleanupFailed) {
		t.Fatalf("upstream early-close uncertainty was not retained: %v", err)
	}
	a := NewApp()
	policy := &fakeProtection{}
	a.policy, a.protectionDesired = policy, true
	runtime_.manager, runtime_.engine = m, e
	for attempt := 0; attempt < 2; attempt++ {
		if err := a.Stop(); !errors.Is(err, core.ErrEngineCleanupFailed) {
			t.Fatalf("Stop %d discarded uncertainty: %v", attempt, err)
		}
		if policy.releases != 0 || runtime_.manager != m || runtime_.engine != e || a.GetState().State != "error" {
			t.Fatal("unconfirmed engine cleanup released protection or discarded owner")
		}
	}
}

func (*retryCleanupProxy) Snapshot() error       { return nil }
func (*retryCleanupProxy) SetProxy(string) error { return nil }
func (p *retryCleanupProxy) Restore() error      { p.restores++; return p.restoreErr }

func TestFailedCleanupRetainsOwnershipUntilExplicitRecovery(t *testing.T) {
	args := os.Args
	os.Args = []string{"synthetic.exe", "--tun"}
	t.Cleanup(func() { os.Args = args })
	proxy := &retryCleanupProxy{restoreErr: errors.New("synthetic restore failure")}
	a := localRunningApp(t, nil, proxy)
	a.state.CoreReady, a.state.ProbeRunning = true, true
	policy := &fakeProtection{}
	a.policy = policy
	manager, cc := runtime_.manager, runtime_.cc
	if err := a.Stop(); err == nil {
		t.Fatal("failed proxy cleanup reported success")
	}
	if policy.releases != 0 || runtime_.manager != manager || runtime_.cc != cc {
		t.Fatal("failed cleanup released protection or lost recovery ownership")
	}
	if a.GetState().State != "error" || a.protectionDesired || a.retry.timer != nil {
		t.Fatal("explicit Stop must disable retry even when cleanup fails")
	}
	if a.GetState().CoreReady || a.GetState().ProbeRunning {
		t.Fatal("cleanup failure published stale core readiness")
	}
	proxy.restoreErr = nil
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if proxy.restores != 2 || policy.releases != 1 || runtime_.manager != nil || a.GetState().State != "stopped" {
		t.Fatal("second explicit Stop did not complete recovery")
	}
}

func TestAutomaticBarrierFailureCannotReleaseOrLoseSession(t *testing.T) {
	args := os.Args
	os.Args = []string{"synthetic.exe", "--tun"}
	t.Cleanup(func() { os.Args = args })
	a := localRunningApp(t, nil)
	policy := &fakeProtection{err: errors.New("synthetic barrier failure")}
	a.policy = policy
	manager := runtime_.manager
	if err := a.stopCoreFor(stopFailure); err == nil {
		t.Fatal("barrier error was swallowed")
	}
	if policy.releases != 0 || runtime_.manager != manager || !a.protectionDesired || a.GetState().State != "error" {
		t.Fatal("automatic failure lost protection intent or cleanup ownership")
	}
	policy.err = nil
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if policy.releases != 1 || runtime_.manager != nil {
		t.Fatal("explicit recovery did not clear retained ownership")
	}
}

func TestActiveRunProbeCompletes(t *testing.T) {
	a := localRunningApp(t, localProbe(func(context.Context) *core.ProbeReport {
		return &core.ProbeReport{AllPassed: true}
	}))
	done := make(chan error, 1)
	go func() { done <- a.RunProbe() }()
	select {
	case err := <-done:
		if err != nil || a.GetState().ProbeRunning || a.GetState().ProbeLastAt == "" {
			t.Fatalf("probe completion: err=%v state=%+v", err, a.GetState())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active RunProbe deadlocked")
	}
}

func TestStopCancelsActiveProbe(t *testing.T) {
	entered := make(chan struct{})
	a := localRunningApp(t, localProbe(func(ctx context.Context) *core.ProbeReport {
		close(entered)
		<-ctx.Done()
		return &core.ProbeReport{AllPassed: true} // obsolete PASS must not publish
	}))
	probeDone := make(chan error, 1)
	go func() { probeDone <- a.RunProbe() }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not enter")
	}
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := <-probeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("obsolete result: %v", err)
	}
	if a.GetState().State != "stopped" || a.GetState().ProbeLastAt != "" || a.protectionDesired {
		t.Fatal("Stop did not win over old probe")
	}
}

func TestStopReasonProtectionPolicy(t *testing.T) {
	args := os.Args
	os.Args = []string{"synthetic.exe", "--tun"}
	t.Cleanup(func() { os.Args = args })
	for _, reason := range []StopReason{stopFailure, stopQuit, stopRestart, stopDisconnect} {
		t.Run(string(rune('0'+reason)), func(t *testing.T) {
			a := localRunningApp(t, nil)
			p := &fakeProtection{}
			a.policy = p
			if err := a.stopCoreFor(reason); err != nil {
				t.Fatal(err)
			}
			if reason.releasesProtection() {
				if p.releases != 1 || p.blocks != 0 || a.protectionDesired {
					t.Fatalf("Disconnect policy=%+v desired=%v", p, a.protectionDesired)
				}
			} else if p.releases != 0 || p.blocks != 1 || !a.protectionDesired {
				t.Fatalf("automatic teardown opened protection: %+v", p)
			}
		})
	}
}

func TestDisconnectWithoutManagerStillReleasesPolicy(t *testing.T) {
	args, old := os.Args, runtime_
	os.Args, runtime_ = []string{"synthetic.exe", "--tun"}, &coreRuntime{}
	t.Cleanup(func() { os.Args, runtime_ = args, old })
	p := &fakeProtection{err: errors.New("synthetic cleanup failure")}
	a := NewApp()
	a.policy = p
	a.protectionDesired = true
	if err := a.Stop(); err == nil || p.releases != 1 || a.GetState().State != "error" {
		t.Fatal("cleanup failure must be returned, including manager=nil")
	}
	p.err = nil
	if err := a.Stop(); err != nil || p.releases != 2 || a.GetState().State != "stopped" {
		t.Fatal("explicit recovery could not retry cleanup")
	}
}

func TestStaleStartCannotPublish(t *testing.T) {
	a := NewApp()
	a.protectionDesired, a.operationID = true, 7
	// Stop changes the generation before the queued operation obtains its lock.
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	a.startCoreGeneration(7) // must return before descriptors/vault/network
	if a.GetState().State != "stopped" || a.protectionDesired {
		t.Fatal("obsolete Start resurrected protection")
	}
}

func TestManualLeakTearsDownAndSchedulesRecovery(t *testing.T) {
	a := localRunningApp(t, localProbe(func(context.Context) *core.ProbeReport {
		return &core.ProbeReport{LeakConfirmed: true}
	}))
	if err := a.RunProbe(); err == nil {
		t.Fatal("leak returned success")
	}
	if a.GetState().State != "error" || a.GetState().RetryAttempt != 1 || !a.protectionDesired || runtime_.manager != nil {
		t.Fatal("unsafe path not finalized into recovery")
	}
}

func TestStaleWatchdogCannotStopNewSession(t *testing.T) {
	a := localRunningApp(t, nil)
	obsolete := &render.ClientConfig{}
	if a.failSession(obsolete, &core.ProbeReport{LeakConfirmed: true}, "old leak", false) {
		t.Fatal("obsolete report finalized a different session")
	}
	if err := a.switchChannelCoreFor("unused", obsolete); !errors.Is(err, errSessionSuperseded) {
		t.Fatalf("obsolete report could switch a different session: %v", err)
	}
	if a.GetState().State != "running" || runtime_.manager.State() != core.StateRunning {
		t.Fatal("new session was interrupted")
	}
	cc := runtime_.cc
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if a.failSession(cc, &core.ProbeReport{}, "late failure", true) || a.GetState().State != "stopped" {
		t.Fatal("late failure undid manual Disconnect")
	}
}

func TestErrorWithoutProtectionIntentDoesNotRetry(t *testing.T) {
	a := NewApp()
	a.state.State = "error"
	a.scheduleFailClosedRetryLocked(time.Now())
	if a.retry.timer != nil || a.retry.attempt != 0 {
		t.Fatal("engine error silently enabled protection/retry")
	}
}

func TestTUNAttemptsBarrierBeforeRender(t *testing.T) {
	args, old := os.Args, runtime_
	os.Args, runtime_ = []string{"synthetic.exe", "--tun"}, &coreRuntime{}
	t.Cleanup(func() { os.Args, runtime_ = args, old })
	a := NewApp()
	a.protectionDesired, a.operationID = true, 1
	p := &fakeProtection{err: errors.New("synthetic arm failure")}
	a.policy = p
	a.startCoreGeneration(1) // must fail before vault/preflight/network access
	a.mu.Lock()
	a.cancelFailClosedRetryLocked()
	a.mu.Unlock()
	if p.arms != 1 || p.releases != 0 || a.GetState().State != "error" || !a.protectionDesired {
		t.Fatal("initial failure skipped the protection policy or authorized direct")
	}
}

func TestRetryGenerationReservedBeforeAsyncLaunch(t *testing.T) {
	a := NewApp()
	a.protectionDesired, a.operationID = true, 10
	a.retryLaunch = func() {} // no engine/network; inspect the synchronous reservation
	a.mu.Lock()
	launch := a.beginRetryLaunchLocked()
	reserved := a.operationID
	a.mu.Unlock()
	launch()
	if reserved != 11 || a.GetState().State != "starting" {
		t.Fatal("retry did not reserve its generation before returning the launcher")
	}
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	a.operationMu.Lock()
	_, _, ok := a.beginOperation(reserved)
	a.operationMu.Unlock()
	if ok {
		t.Fatal("delayed retry remained valid after Disconnect")
	}
}
