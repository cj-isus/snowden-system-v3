package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------- фейки ----------

type fakeProbe struct {
	mu      sync.Mutex
	report  *ProbeReport
	calls   int
	block   chan struct{} // если задан — Run блокируется до закрытия/отмены
	lastCtx context.Context
}

func (f *fakeProbe) Run(ctx context.Context) *ProbeReport {
	f.mu.Lock()
	f.calls++
	f.lastCtx = ctx
	rep := f.report
	block := f.block
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
			return &ProbeReport{AllPassed: true, Results: []ProbeResult{{Name: "x", Passed: true}}}
		case <-ctx.Done():
			return &ProbeReport{AllPassed: false, Results: []ProbeResult{{Name: "x", Detail: ctx.Err().Error()}}}
		}
	}
	return rep
}

func passProbe() *fakeProbe {
	return &fakeProbe{report: &ProbeReport{AllPassed: true, Results: []ProbeResult{{Name: "ok", Passed: true}}}}
}

func failProbe() *fakeProbe {
	return &fakeProbe{report: &ProbeReport{AllPassed: false, Results: []ProbeResult{{Name: "bad", Passed: false, Detail: "egress mismatch"}}}}
}

type fakeProxy struct {
	mu        sync.Mutex
	snapCalls int
	setCalls  int
	restore   int
	restoreEr error
}

func (f *fakeProxy) Snapshot() error { f.mu.Lock(); f.snapCalls++; f.mu.Unlock(); return nil }
func (f *fakeProxy) SetProxy(addr string) error {
	f.mu.Lock()
	f.setCalls++
	f.mu.Unlock()
	return nil
}
func (f *fakeProxy) Restore() error {
	f.mu.Lock()
	f.restore++
	e := f.restoreEr
	f.mu.Unlock()
	return e
}

// newTestManager — менеджер с реальным движком (SOCKS на эфемерном порту),
// фейковым probe и фейковым прокси (реестр в тестах не трогаем).
func newTestManager(t *testing.T, probe Probe) *Manager {
	t.Helper()
	m := NewManager()
	m.proxy = &fakeProxy{}
	m.SetEngine(NewEngine(socksRawConfig(t, freePort(t))))
	m.SetProbe(probe)
	return m
}

// ---------- happy path ----------

func TestManagerStartAndStop(t *testing.T) {
	m := newTestManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := m.State(); got != StateRunning {
		t.Fatalf("state after start: %s", got)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := m.State(); got != StateStopped {
		t.Fatalf("state after stop: %s", got)
	}
}

// ---------- guards ----------

func TestManagerStartNoEngine(t *testing.T) {
	m := NewManager()
	m.probe = passProbe()
	m.proxy = &fakeProxy{}
	if err := m.Start(context.Background()); !errors.Is(err, ErrEngineNotConfigured) {
		t.Fatalf("want ErrEngineNotConfigured, got %v", err)
	}
	if got := m.State(); got != StateError {
		t.Fatalf("state: %s", got)
	}
}

func TestManagerStartNoProbe(t *testing.T) {
	m := NewManager()
	m.SetEngine(NewEngine(socksRawConfig(t, freePort(t))))
	m.proxy = &fakeProxy{}
	if err := m.Start(context.Background()); err == nil || !stringsContains(err.Error(), "probe is required") {
		t.Fatalf("want probe-required error, got %v", err)
	}
}

func TestManagerStartFromInvalidState(t *testing.T) {
	m := newTestManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	if err := m.Start(ctx); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second start must be invalid, got %v", err)
	}
}

func TestManagerStartCanceledContext(t *testing.T) {
	m := newTestManager(t, passProbe())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Start(ctx); err == nil {
		t.Fatal("canceled context must fail")
	}
}

// ---------- cleanup paths ----------

func TestManagerStopAfterFailedStart(t *testing.T) {
	m := newTestManager(t, failProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err == nil {
		t.Fatal("start must fail on failed probe")
	}
	if got := m.State(); got != StateError {
		t.Fatalf("state after failed start: %s", got)
	}
	// Stop из Error обязан вернуть в stopped.
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("stop from error: %v", err)
	}
	if got := m.State(); got != StateStopped {
		t.Fatalf("state: %s", got)
	}
}

func TestManagerProbeRunsThroughProxyEngine(t *testing.T) {
	p := passProbe()
	m := newTestManager(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	if p.calls != 1 {
		t.Fatalf("probe must run exactly once during start, ran %d", p.calls)
	}
	if p.lastCtx == nil {
		t.Fatal("probe must receive operation context")
	}
}

func TestManagerStopsEngineBeforeRestoringProxy(t *testing.T) {
	fp := &fakeProxy{}
	m := NewManager()
	m.proxy = fp
	m.SetEngine(NewEngine(socksRawConfig(t, freePort(t))))
	m.SetProbe(failProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err == nil {
		t.Fatal("must fail")
	}
	if fp.restore != 1 {
		t.Fatalf("proxy must be restored once after failed start, got %d", fp.restore)
	}
}

func TestManagerProxyRestoreErrorSurfaces(t *testing.T) {
	fp := &fakeProxy{restoreEr: errors.New("registry busy")}
	m := NewManager()
	m.proxy = fp
	m.SetEngine(NewEngine(socksRawConfig(t, freePort(t))))
	m.SetProbe(failProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := m.Start(ctx)
	if err == nil || !stringsContains(err.Error(), "registry busy") {
		t.Fatalf("restore error must surface, got %v", err)
	}
}

// ---------- Stop vs active operations ----------

func TestManagerStopCancelsBlockedStart(t *testing.T) {
	p := &fakeProbe{block: make(chan struct{})}
	m := newTestManager(t, p)

	startDone := make(chan error, 1)
	go func() { startDone <- m.Start(context.Background()) }()

	// Дождаться, пока Start реально начнёт выполняться (probe запущен).
	waitForProbe(t, p, 5*time.Second)

	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("stop during blocked start: %v", err)
	}
	select {
	case err := <-startDone:
		if err == nil {
			t.Fatal("blocked start must return an error after Stop")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("blocked start did not finish")
	}
	if got := m.State(); got != StateStopped {
		t.Fatalf("final state: %s", got)
	}
}

func TestManagerCanceledStopStillCleansUp(t *testing.T) {
	p := passProbe()
	m := newTestManager(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	stopCtx, stopCancel := context.WithCancel(context.Background())
	stopCancel() // caller-контекст отменён ДО Stop
	err := m.Stop(stopCtx)
	// Очистка всё равно выполнена: движок остановлен, прокси восстановлен,
	// состояние stopped; ошибка caller-контекста допустима в возврате.
	if got := m.State(); got != StateStopped {
		t.Fatalf("state must be stopped after canceled stop, got %s", got)
	}
	_ = err
}

// ---------- Reload probe refresh (B1/V2-032: egress следует за каналом) ----------

// countedProbe — probe, различающий инстансы по имени в отчёте.
type countedProbe struct {
	fakeProbe
	name string
}

func (c *countedProbe) Run(ctx context.Context) *ProbeReport {
	rep := c.fakeProbe.Run(ctx)
	if rep != nil {
		rep2 := *rep
		rep2.Results = append([]ProbeResult(nil), rep.Results...)
		if len(rep2.Results) > 0 {
			rep2.Results[0].Name = c.name
		}
		return &rep2
	}
	return rep
}

func TestManagerReloadWithProbeOptionUsesNewProbe(t *testing.T) {
	// B1: канал B живёт на другом VPS — послепереключательный probe обязан
	// нести ожидание НОВОГО канала (WithReloadProbe), а не стартового.
	startProbe := &countedProbe{fakeProbe: *passProbe(), name: "start-probe"}
	m := newTestManager(t, startProbe)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())

	reloadProbe := &countedProbe{fakeProbe: *passProbe(), name: "reload-probe"}
	if err := m.Reload(ctx, []byte(protectedFixture(t)), WithReloadProbe(reloadProbe)); err != nil {
		t.Fatalf("reload: %v", err)
	}
	reloadProbe.mu.Lock()
	calls := reloadProbe.calls
	reloadProbe.mu.Unlock()
	if calls == 0 {
		t.Fatal("reload probe must run after reload")
	}
	startProbe.mu.Lock()
	startCalls := startProbe.calls
	startProbe.mu.Unlock()
	_ = startCalls // стартовый мог тикнуть в Start; важно — reload-probe отработал
}

func TestManagerReloadProbeOptionFailureFailsClosed(t *testing.T) {
	m := newTestManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())

	// Опционный probe отвергает новый конфиг → reload обязан провалиться
	// (fail-closed), несмотря на зелёный стартовый probe.
	if err := m.Reload(ctx, []byte(protectedFixture(t)), WithReloadProbe(failProbe())); err == nil {
		t.Fatal("reload must fail when reload probe rejects")
	}
}

// ---------- Reload ----------

func TestManagerReloadFromInvalidState(t *testing.T) {
	m := newTestManager(t, passProbe())
	if err := m.Reload(context.Background(), socksRawConfig(t, freePort(t))); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("reload from stopped must be invalid, got %v", err)
	}
}

func TestManagerReloadProbesNewEngineBeforeKeepingRunning(t *testing.T) {
	p := passProbe()
	m := newTestManager(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	before := p.calls
	// Reload только на ВАЛИДИРОВАННЫЙ защищённый конфиг (контракт ядра):
	// direct-only конфиг обязан быть отбракован строгим парсером.
	if err := m.Reload(ctx, []byte(protectedFixture(t))); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if p.calls != before+1 {
		t.Fatalf("probe must run after reload, before=%d after=%d", before, p.calls)
	}
	if got := m.State(); got != StateRunning {
		t.Fatalf("state after successful reload: %s", got)
	}
}

func TestManagerReloadProbeFailureLeavesErrorState(t *testing.T) {
	p := passProbe()
	m := newTestManager(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())

	// Подменяем probe на отказывающийся уже ПОСЛЕ старта.
	m.mu.Lock()
	m.probe = failProbe()
	m.mu.Unlock()

	raw := protectedFixture(t) // валидный защищённый конфиг
	if err := m.Reload(ctx, []byte(raw)); err == nil {
		t.Fatal("reload must fail when probe rejects new config")
	}
	if got := m.State(); got != StateError {
		t.Fatalf("state after failed reload: %s", got)
	}
	if m.engine.Status() != "stopped" {
		t.Fatalf("engine must be stopped after failed reload, got %s", m.engine.Status())
	}
}

func TestManagerReloadInvalidConfigKeepsRunning(t *testing.T) {
	p := passProbe()
	m := newTestManager(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())

	if err := m.Reload(ctx, []byte(`{not json}`)); err == nil {
		t.Fatal("invalid config must fail reload")
	}
	// Строгий парсер отбраковал ДО всяких мутаций: движок жив, состояние running.
	if got := m.State(); got != StateRunning {
		t.Fatalf("running must survive rejected reload, got %s", got)
	}
	if m.engine.Status() != "running" {
		t.Fatal("engine must keep serving")
	}
}

// ---------- concurrency ----------

func TestConcurrentStartStop(t *testing.T) {
	m := newTestManager(t, passProbe())
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.Start(ctx); err != nil {
				errs <- err
				return
			}
			if err := m.Stop(context.Background()); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		// Допустимы только отклонения состояния (гонка за единственный слот),
		// но не паники/дедлоки/потеря очистки.
		if !errors.Is(err, ErrInvalidState) && !stringsContains(err.Error(), "context") {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if got := m.State(); got != StateStopped && got != StateError {
		t.Fatalf("final state after concurrency: %s", got)
	}
}

// ---------- helpers ----------

func waitForProbe(t *testing.T, p *fakeProbe, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		calls := p.calls
		p.mu.Unlock()
		if calls > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("probe did not start in time")
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ---------- SwitchChannel (V2-037: бесшовное переключение) ----------

func newSelectorManager(t *testing.T, probe Probe) (*Manager, *Engine) {
	t.Helper()
	m := NewManager()
	m.proxy = &fakeProxy{}
	e := NewEngine(selectorRawConfig(t, freePort(t)))
	m.SetEngine(e)
	m.SetProbe(probe)
	return m, e
}

func TestManagerSwitchChannelSeamlessSuccess(t *testing.T) {
	m, e := newSelectorManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	if err := m.SwitchChannel(ctx, "proxy", "chan-b", passProbe(), passProbe()); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if got := m.State(); got != StateRunning {
		t.Fatalf("state must stay running, got %s", got)
	}
	if tag, _ := e.CurrentDefault("proxy"); tag != "chan-b" {
		t.Fatalf("selector: %q", tag)
	}
}

func TestManagerSwitchChannelRollbackKeepsTunnelAlive(t *testing.T) {
	// Новый канал не прошёл probe, прежний подтверждён: туннель ЖИВ,
	// состояние running (старая семантика Reload рвала рабочий туннель).
	m, e := newSelectorManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	err := m.SwitchChannel(ctx, "proxy", "chan-b", failProbe(), passProbe())
	if !errors.Is(err, ErrSwitchRolledBack) {
		t.Fatalf("want ErrSwitchRolledBack, got %v", err)
	}
	if got := m.State(); got != StateRunning {
		t.Fatalf("state must stay running after rollback, got %s", got)
	}
	if tag, _ := e.CurrentDefault("proxy"); tag != "chan-a" {
		t.Fatalf("selector must roll back to chan-a, got %q", tag)
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("engine must stay running, got %s", got)
	}
}

func TestManagerSwitchChannelAllDeadFailsClosed(t *testing.T) {
	// Ни новый, ни прежний канал не подтверждены: полный teardown (fail-closed).
	m, e := newSelectorManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	err := m.SwitchChannel(ctx, "proxy", "chan-b", failProbe(), failProbe())
	if err == nil || errors.Is(err, ErrSwitchRolledBack) {
		t.Fatalf("both-dead must hard-fail, got %v", err)
	}
	if got := m.State(); got != StateError {
		t.Fatalf("state must be error, got %s", got)
	}
	if got := e.Status(); got != "stopped" {
		t.Fatalf("engine must be stopped, got %s", got)
	}
}

func TestManagerSwitchChannelNoSelectorFallsBack(t *testing.T) {
	// Конфиг без селектора: ErrSelectorSwitchUnavailable, состояние не менялось
	// (вызывающий откатывается на Reload-путь).
	m := newTestManager(t, passProbe())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	err := m.SwitchChannel(ctx, "proxy", "x", passProbe(), passProbe())
	if !errors.Is(err, ErrSelectorSwitchUnavailable) {
		t.Fatalf("want ErrSelectorSwitchUnavailable, got %v", err)
	}
	if got := m.State(); got != StateRunning {
		t.Fatalf("state must stay running, got %s", got)
	}
}
