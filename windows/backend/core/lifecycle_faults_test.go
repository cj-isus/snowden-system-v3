package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
)

// These fakes sit at the external lifecycle boundary. No registry, WFP, TUN,
// remote service or user configuration participates in these failure cases.
type faultBox struct {
	start, close func() error
	closeCalls   atomic.Int32
}

func (b *faultBox) Start() error {
	if b.start != nil {
		return b.start()
	}
	return nil
}
func (b *faultBox) Close() error {
	b.closeCalls.Add(1)
	if b.close != nil {
		return b.close()
	}
	return nil
}
func (*faultBox) Outbound() adapter.OutboundManager { return nil }

type faultProxy struct {
	snapshot, set, restore func() error
	setCalls, restoreCalls atomic.Int32
}

func (p *faultProxy) Snapshot() error {
	if p.snapshot != nil {
		return p.snapshot()
	}
	return nil
}
func (p *faultProxy) SetProxy(string) error {
	p.setCalls.Add(1)
	if p.set != nil {
		return p.set()
	}
	return nil
}
func (p *faultProxy) Restore() error {
	p.restoreCalls.Add(1)
	if p.restore != nil {
		return p.restore()
	}
	return nil
}

type probeFunc func(context.Context) *ProbeReport

func (fn probeFunc) Run(ctx context.Context) *ProbeReport { return fn(ctx) }

func engineWithBox(b boxInstance) *Engine {
	e := NewEngine([]byte(`{}`))
	e.newBox = func(context.Context, []byte) (boxInstance, error) { return b, nil }
	return e
}

func awaitLifecycle[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle operation did not reach the expected phase")
		var zero T
		return zero
	}
}

func TestEngineCleanupFailureCannotBecomeSuccessfulRetry(t *testing.T) {
	for _, panicClose := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panicClose], func(t *testing.T) {
			cause := errors.New("synthetic close failure")
			b := &faultBox{close: func() error {
				if panicClose {
					panic(cause)
				}
				return cause
			}}
			e := engineWithBox(b)
			if err := e.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				err := e.Stop()
				if !errors.Is(err, ErrEngineCleanupFailed) || !strings.Contains(err.Error(), cause.Error()) {
					t.Fatalf("cleanup lost its failure: %v", err)
				}
			}
			if b.closeCalls.Load() != 1 || e.box != b || e.Status() != "error" {
				t.Fatalf("owner discarded or Close retried: calls=%d status=%s", b.closeCalls.Load(), e.Status())
			}
			if err := e.Start(context.Background()); !errors.Is(err, ErrEngineCleanupFailed) {
				t.Fatalf("Start accepted an unretired owner: %v", err)
			}
			if err := e.Reload(context.Background(), []byte(`{}`)); !errors.Is(err, ErrEngineCleanupFailed) {
				t.Fatalf("Reload accepted an unretired owner: %v", err)
			}
			if _, err := e.SwitchDefault("proxy", "candidate"); !errors.Is(err, ErrEngineCleanupFailed) {
				t.Fatalf("Switch accepted an unretired owner: %v", err)
			}
			if _, err := e.CurrentDefault("proxy"); !errors.Is(err, ErrEngineCleanupFailed) {
				t.Fatalf("CurrentDefault hid an unretired owner: %v", err)
			}
		})
	}
}

func TestEngineFailedStartPreservesBothErrorsAndOwner(t *testing.T) {
	for _, panicStart := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panicStart], func(t *testing.T) {
			startErr := errors.New("synthetic start failure")
			closeErr := errors.New("synthetic cleanup failure")
			b := &faultBox{start: func() error {
				if panicStart {
					panic(startErr)
				}
				return startErr
			}, close: func() error { return closeErr }}
			e := engineWithBox(b)
			err := e.Start(context.Background())
			if !errors.Is(err, ErrEngineCleanupFailed) || !errors.Is(err, closeErr) ||
				!strings.Contains(err.Error(), startErr.Error()) {
				t.Fatalf("original or cleanup failure lost: %v", err)
			}
			if e.box != b || b.closeCalls.Load() != 1 {
				t.Fatal("partially started box lost its owner")
			}
			if err := e.Stop(); !errors.Is(err, ErrEngineCleanupFailed) {
				t.Fatalf("retry reported successful teardown: %v", err)
			}
		})
	}
}

func TestEngineReloadDoesNotCreateReplacementAfterCloseFailure(t *testing.T) {
	b := &faultBox{close: func() error { return errors.New("old box close failure") }}
	e := engineWithBox(b)
	var creates int
	e.newBox = func(context.Context, []byte) (boxInstance, error) { creates++; return b, nil }
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := e.Reload(context.Background(), []byte(`{"log":{"level":"error"}}`)); !errors.Is(err, ErrEngineCleanupFailed) {
		t.Fatalf("reload lost Close failure: %v", err)
	}
	if creates != 1 || string(e.rawJSON) != `{}` || e.Status() != "error" {
		t.Fatal("new candidate replaced an incompletely closed engine")
	}
}

func TestEngineStartFailureWithConfirmedCleanupAllowsRetry(t *testing.T) {
	var starts int
	b := &faultBox{start: func() error {
		starts++
		if starts == 1 {
			return errors.New("recoverable synthetic start failure")
		}
		return nil
	}}
	e := engineWithBox(b)
	if err := e.Start(context.Background()); err == nil || errors.Is(err, ErrEngineCleanupFailed) {
		t.Fatalf("ordinary failure should retain confirmed cleanup: %v", err)
	}
	if e.Status() != "stopped" || e.box != nil {
		t.Fatal("successfully closed instance was retained")
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatalf("clean retry failed: %v", err)
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerReloadCleanupFailureCannotPublishRunning(t *testing.T) {
	var creates int
	e := NewEngine([]byte(`{}`))
	e.newBox = func(context.Context, []byte) (boxInstance, error) {
		creates++
		if creates == 1 {
			return &faultBox{}, nil
		}
		return &faultBox{
			start: func() error { return errors.New("candidate start failed") },
			close: func() error { return errors.New("candidate close failed") },
		}, nil
	}
	m := NewManager()
	m.SetEngine(e)
	m.SetSystemProxy(&faultProxy{})
	m.SetProbe(passProbe())
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Reload(context.Background(), []byte(protectedFixture(t)))
	if !errors.Is(err, ErrEngineCleanupFailed) || m.State() != StateError || m.active != nil {
		t.Fatalf("candidate teardown failure published running or stranded operation: %v", err)
	}
	if creates != 2 {
		t.Fatal("rollback launched another box before retiring the failed candidate")
	}
	if err := m.Stop(context.Background()); !errors.Is(err, ErrEngineCleanupFailed) {
		t.Fatalf("repeat Stop lost candidate ownership: %v", err)
	}
}

func TestManagerRestorePanicFinishesOperationAndCanRecover(t *testing.T) {
	m := NewManager()
	p := &faultProxy{}
	p.restore = func() error {
		if p.restoreCalls.Load() == 1 {
			panic("synthetic restore panic")
		}
		return nil
	}
	m.SetEngine(engineWithBox(&faultBox{}))
	m.SetSystemProxy(p)
	m.SetProbe(passProbe())
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Stop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "restore panic") || m.State() != StateError {
		t.Fatalf("cleanup panic escaped or disappeared: %v state=%s", err, m.State())
	}
	m.mu.RLock()
	active, proxyActive := m.active, m.proxyActive
	m.mu.RUnlock()
	if active != nil || !proxyActive {
		t.Fatal("operation stranded or proxy ownership discarded")
	}
	if err := m.Stop(context.Background()); err != nil || m.State() != StateStopped {
		t.Fatalf("explicit recovery failed: %v state=%s", err, m.State())
	}
}

func TestManagerPrimaryPanicSurvivesCleanupPanic(t *testing.T) {
	m := NewManager()
	p := &faultProxy{restore: func() error { panic("secondary restore panic") }}
	m.SetEngine(engineWithBox(&faultBox{close: func() error { panic("secondary close panic") }}))
	m.SetSystemProxy(p)
	m.SetProbe(panicProbe{})
	err := m.Start(context.Background())
	for _, message := range []string{"synthetic probe panic", "secondary close panic", "secondary restore panic"} {
		if err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("missing %q from failure chain: %v", message, err)
		}
	}
	if m.State() != StateError || m.active != nil || p.restoreCalls.Load() != 1 {
		t.Fatal("panic prevented remaining cleanup/finalization")
	}
	if err := m.Stop(context.Background()); !errors.Is(err, ErrEngineCleanupFailed) || m.State() != StateError {
		t.Fatalf("repeat Stop must retain engine failure: %v", err)
	}
}

// Observe actual cancellation, not a sleep or goroutine scheduling guess.
// The tested phase remains gated while this wrapper is installed under mu.
func observeOperationCancel(m *Manager) <-chan struct{} {
	cancelled := make(chan struct{})
	var once sync.Once
	m.mu.Lock()
	op := m.active
	cancel := op.cancel
	op.cancel = func() { cancel(); once.Do(func() { close(cancelled) }) }
	m.mu.Unlock()
	return cancelled
}

func TestManagerStopAcrossStartAndReloadPhases(t *testing.T) {
	for _, phase := range []string{"snapshot", "start", "set_proxy", "probe", "reload_probe"} {
		t.Run(phase, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			gate := func() { close(entered); <-release }
			p := &faultProxy{}
			m := NewManager()
			e := NewEngine([]byte(`{}`))
			var starts, probes atomic.Int32
			e.newBox = func(context.Context, []byte) (boxInstance, error) {
				return &faultBox{start: func() error {
					starts.Add(1)
					if phase == "start" {
						gate()
					}
					return nil
				}}, nil
			}
			if phase == "snapshot" {
				p.snapshot = func() error { gate(); return nil }
			}
			if phase == "set_proxy" {
				p.set = func() error { gate(); return nil }
			}
			probe := probeFunc(func(context.Context) *ProbeReport {
				probes.Add(1)
				if phase == "probe" || phase == "reload_probe" {
					gate()
				}
				return &ProbeReport{AllPassed: true} // late PASS cannot commit
			})
			m.SetEngine(e)
			m.SetSystemProxy(p)
			m.SetProbe(passProbe())
			var operation func() error
			if phase == "reload_probe" {
				if err := m.Start(context.Background()); err != nil {
					t.Fatal(err)
				}
				raw := []byte(protectedFixture(t))
				operation = func() error { return m.Reload(context.Background(), raw, WithReloadProbe(probe)) }
			} else {
				m.SetProbe(probe)
				operation = func() error { return m.Start(context.Background()) }
			}
			result := make(chan error, 1)
			go func() { result <- operation() }()
			awaitLifecycle(t, entered)
			cancelled := observeOperationCancel(m)
			stopped := make(chan error, 1)
			go func() { stopped <- m.Stop(context.Background()) }()
			awaitLifecycle(t, cancelled)
			unblock()
			if err := awaitLifecycle(t, result); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled phase committed or lost cancellation: %v", err)
			}
			if err := awaitLifecycle(t, stopped); err != nil || m.State() != StateStopped || e.Status() != "stopped" {
				t.Fatalf("Stop incomplete: %v manager=%s engine=%s", err, m.State(), e.Status())
			}
			if phase == "snapshot" && starts.Load() != 0 {
				t.Fatal("cancelled snapshot still launched engine")
			}
			if phase == "start" && p.setCalls.Load() != 0 {
				t.Fatal("cancelled Start still changed proxy")
			}
			if phase != "probe" && phase != "reload_probe" && probes.Load() != 0 {
				t.Fatal("cancelled phase still launched protected probe")
			}
			if p.restoreCalls.Load() != 1 {
				t.Fatalf("proxy snapshot not restored exactly once: %d", p.restoreCalls.Load())
			}
		})
	}
}

func TestManagerStopDuringSwitchRejectsLatePass(t *testing.T) {
	m, e := newSelectorManager(t, passProbe())
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	probe := probeFunc(func(context.Context) *ProbeReport {
		close(entered)
		<-release
		return &ProbeReport{AllPassed: true}
	})
	rollback := passProbe()
	switched := make(chan error, 1)
	go func() { switched <- m.SwitchChannel(context.Background(), "proxy", "chan-b", probe, rollback) }()
	awaitLifecycle(t, entered)
	cancelled := observeOperationCancel(m)
	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(context.Background()) }()
	awaitLifecycle(t, cancelled)
	unblock()
	if err := awaitLifecycle(t, switched); !errors.Is(err, context.Canceled) || errors.Is(err, ErrSwitchRolledBack) {
		t.Fatalf("cancelled switch published success/verified rollback: %v", err)
	}
	if rollback.calls != 0 {
		t.Fatal("Stop was followed by another rollback probe")
	}
	if err := awaitLifecycle(t, stopped); err != nil || m.State() != StateStopped || e.Status() != "stopped" {
		t.Fatalf("Stop did not retire the switched engine: %v", err)
	}
}

func TestManagerConcurrentStopSharesCleanupOwnership(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	b := &faultBox{close: func() error { close(entered); <-release; return nil }}
	p := &faultProxy{}
	m := NewManager()
	m.SetEngine(engineWithBox(b))
	m.SetSystemProxy(p)
	m.SetProbe(passProbe())
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 2)
	go func() { stopped <- m.Stop(context.Background()) }()
	awaitLifecycle(t, entered)
	if err := m.Start(context.Background()); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Start admitted during cleanup: %v", err)
	}
	cancelled := observeOperationCancel(m)
	go func() { stopped <- m.Stop(context.Background()) }()
	awaitLifecycle(t, cancelled)
	unblock()
	for i := 0; i < 2; i++ {
		if err := awaitLifecycle(t, stopped); err != nil {
			t.Fatal(err)
		}
	}
	if b.closeCalls.Load() != 1 || p.restoreCalls.Load() != 1 || m.State() != StateStopped {
		t.Fatal("concurrent Stop duplicated cleanup or failed to converge")
	}
}
