package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/config"
)

// State — жизненный цикл менеджера (1:1 с контрактом UI, PLAN §4.1).
type State string

const (
	StateStopped   State = "stopped"
	StateStarting  State = "starting"
	StateRunning   State = "running"
	StateReloading State = "reloading"
	StateStopping  State = "stopping"
	StateError     State = "error"
)

var ErrInvalidState = errors.New("invalid manager state")

// lifecycleTimeout — бюджет ОПЕРАЦИИ (Start/Reload/Stop), не время жизни
// движка: после успешного Start контекст операции закрывается, а туннель
// продолжает работать (урок P0-2026-09-08 «kill after Start»).
const lifecycleTimeout = 30 * time.Second

// Probe — защищённая проверка перед переходом в running (FR-001).
type Probe interface {
	Run(ctx context.Context) *ProbeReport
}

// systemProxy — системный прокси Windows (snapshot/set/restore).
type systemProxy interface {
	Snapshot() error
	SetProxy(string) error
	Restore() error
}

type lifecycleOperation struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager — сериализованный жизненный цикл. Все переходы под mu; активная
// операция ровно одна (active != nil ⇒ переход идёт).
type Manager struct {
	mu          sync.RWMutex
	generation  uint64 // increments when an operation takes lifecycle ownership
	active      *lifecycleOperation
	state       State
	engine      *Engine
	proxy       systemProxy
	proxyActive bool
	probe       Probe
}

func NewManager() *Manager {
	return &Manager{state: StateStopped, proxy: &ProxyManager{}}
}

// SetEngine/SetProbe — только из состояния stopped (композиция до старта).
func (m *Manager) SetEngine(e *Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped {
		m.engine = e
	}
}

func (m *Manager) SetProbe(p Probe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped {
		m.probe = p
	}
}

// SetSystemProxy подменяет реализацию системного прокси (точка внедрения
// для тестов и CLI-режима без управления реестром). Только до старта.
func (m *Manager) SetSystemProxy(p systemProxy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped {
		m.proxy = p
	}
}

func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) restoreProxy(proxy systemProxy) error {
	if proxy == nil {
		return nil
	}
	m.mu.RLock()
	active := m.proxyActive
	m.mu.RUnlock()
	if !active {
		return nil
	}
	if err := cleanupCall("restore system proxy", proxy.Restore); err != nil {
		return fmt.Errorf("restore system proxy: %w", err)
	}
	m.mu.Lock()
	m.proxyActive = false
	m.mu.Unlock()
	return nil
}

func (m *Manager) finish(op *lifecycleOperation, state State) {
	op.cancel()
	m.mu.Lock()
	if m.active == op {
		m.active = nil
		m.state = state
		close(op.done)
	}
	m.mu.Unlock()
}

// Start: engine → system proxy → probe. Любая ошибка = полный cleanup +
// StateError. Паника перехватывается: cleanup обязателен при любом исходе.
func (m *Manager) Start(ctx context.Context) (err error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.state != StateStopped || m.active != nil {
		m.mu.Unlock()
		cancel()
		return ErrInvalidState
	}
	m.active = op
	m.generation++
	m.state = StateStarting
	engine := m.engine
	probe := m.probe
	proxy := m.proxy
	m.mu.Unlock()

	finalState := StateError
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			err = errors.Join(fmt.Errorf("start panic: %v", recovered), m.cleanup(engine, proxy))
		}
		m.finish(op, finalState)
	}()

	fail := func(err error, engineStarted bool) error {
		if engineStarted && engine != nil {
			return errors.Join(err, m.cleanup(engine, proxy))
		}
		return errors.Join(err, m.restoreProxy(proxy))
	}

	if engine == nil {
		return fail(ErrEngineNotConfigured, false)
	}
	if probe == nil {
		return fail(errors.New("protected probe is required"), false)
	}
	if proxy == nil {
		return fail(errors.New("system proxy is not configured"), false)
	}
	if err := proxy.Snapshot(); err != nil {
		return fail(fmt.Errorf("snapshot system proxy: %w", err), false)
	}
	m.mu.Lock()
	m.proxyActive = true
	m.mu.Unlock()
	if err := contextError(operationCtx); err != nil {
		return fail(err, false)
	}
	if err := engine.Start(operationCtx); err != nil {
		return fail(fmt.Errorf("engine start: %w", err), false)
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err, true)
	}
	if err := proxy.SetProxy("127.0.0.1:1080"); err != nil {
		return fail(fmt.Errorf("set system proxy: %w", err), true)
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err, true)
	}

	report := probe.Run(operationCtx)
	if !probePassed(report) {
		detail := "no report"
		if report != nil {
			detail = report.Summary()
		}
		return fail(fmt.Errorf("protected probe failed: %s", detail), true)
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err, true)
	}

	finalState = StateRunning
	return nil
}

// Stop: деструктивная очистка — отменяет активную операцию (если есть),
// останавливает движок и восстанавливает прокси ДАЖЕ при отменённом
// caller-контексте. Ожидание операции ограничено собственным таймаутом;
// синхронный third-party Close пока не имеет hard bound (OFF-02).
func (m *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()

	for {
		m.mu.Lock()
		if m.state == StateStopped && m.active == nil {
			m.mu.Unlock()
			return ctx.Err()
		}
		switch m.state {
		case StateRunning, StateError, StateStarting, StateReloading, StateStopping:
		default:
			m.mu.Unlock()
			return ErrInvalidState
		}
		if m.active != nil {
			op := m.active
			op.cancel()
			done := op.done
			m.mu.Unlock()
			select {
			case <-done:
				continue // операция завершилась — повторяем заход на очистку
			case <-waitCtx.Done():
				return waitCtx.Err()
			}
		}

		op := &lifecycleOperation{cancel: func() {}, done: make(chan struct{})}
		m.active = op
		m.generation++
		m.state = StateStopping
		engine := m.engine
		proxy := m.proxy
		m.mu.Unlock()

		cleanupErr := m.cleanup(engine, proxy)
		if cleanupErr != nil {
			m.finish(op, StateError)
		} else {
			m.finish(op, StateStopped)
		}
		return errors.Join(ctx.Err(), cleanupErr)
	}
}

// ReloadOption — параметр операции Reload.
type ReloadOption func(*reloadOpts)

type reloadOpts struct {
	probe Probe
}

// WithReloadProbe — probe для обязательной послепереключательной проверки
// НОВОГО конфига. Нужен когда новый канал живёт на другом сервере с другим
// expected egress (B1/V2-032): стартовый probe содержит ожидание стартового
// канала и после переключения сравнивал бы egress нового VPS с IP старого.
func WithReloadProbe(p Probe) ReloadOption {
	return func(o *reloadOpts) { o.probe = p }
}

// Preparation has no engine/proxy ownership. Callbacks run outside m.mu, so
// State/Stop remain available. A panic or cancellation cannot tear down the
// active session. The caller checks generation before committing the result.
func prepareReload(ctx context.Context, rawJSON []byte, opts []ReloadOption) (raw []byte, ro reloadOpts, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Join(err, contextError(ctx), fmt.Errorf("reload preparation panic: %v", recovered))
		}
		if err != nil {
			err = errors.Join(ErrReloadRejected, err)
		}
	}()
	if err := contextError(ctx); err != nil {
		return nil, ro, err
	}
	validated, err := config.Parse(rawJSON)
	if err != nil {
		return nil, ro, fmt.Errorf("validate reload config: %w", err)
	}
	raw, err = json.Marshal(validated)
	if err != nil {
		return nil, ro, fmt.Errorf("normalize reload config: %w", err)
	}
	for i, fn := range opts {
		if err := contextError(ctx); err != nil {
			return nil, ro, err
		}
		if fn == nil {
			return nil, ro, fmt.Errorf("reload option %d is nil", i)
		}
		fn(&ro)
	}
	return raw, ro, contextError(ctx)
}

// ErrSwitchRolledBack — переключение канала не прошло probe, но защищённый
// путь на ПРЕЖНЕМ канале подтверждён: движок жив, состояние Manager — running.
// Вызывающий НЕ должен ронять туннель (failCore) — это не отказ канала,
// это отказ кандидата. Старая семантика Reload рвала рабочий туннель при
// неудачном переключении на мёртвый канал (V2-037).
var ErrSwitchRolledBack = errors.New("channel switch failed probe: rolled back to previous channel")

// VerifiedRollbackError carries the fresh evidence, not just an error label.
// Restarted distinguishes a new lifetime of the old configuration from a live
// selector rollback; observers and metric baselines must follow that lifetime.
type VerifiedRollbackError struct {
	Cause     error
	Report    *ProbeReport
	Restarted bool
}

func (e *VerifiedRollbackError) Error() string {
	if e.Cause == nil {
		return ErrSwitchRolledBack.Error()
	}
	return fmt.Sprintf("%s: %v", ErrSwitchRolledBack, e.Cause)
}
func (e *VerifiedRollbackError) Unwrap() error        { return e.Cause }
func (e *VerifiedRollbackError) Is(target error) bool { return target == ErrSwitchRolledBack }

func probePassed(report *ProbeReport) bool {
	return report != nil && report.AllPassed && !report.LeakConfirmed
}

// SwitchChannel — бесшовное переключение активного outbound селектора на
// живом движке (V2-037): без Stop/Reload бокса, соединения не рвутся.
//   - newProbe     — probe с ожиданием egress НОВОГО канала (обязателен);
//   - rollbackProbe — probe с ожиданием egress ТЕКУЩЕГО канала (для
//     подтверждения отката).
//
// Исходы:
//
//	nil                        — переключение прошло probe, селектор на новом теге;
//	ErrSwitchRolledBack        — новый канал не прошёл probe; откат на прежний
//	                             тег тем же API, прежний канал подтверждён probe;
//	ошибка с ErrSelectorSwitchUnavailable — seamless невозможен (нет селектора
//	                             в живом конфиге); состояние не менялось,
//	                             вызывающий откатывается на Reload;
//	прочая ошибка              — fail-closed: движок остановлен, прокси
//	                             восстановлен, состояние error.
func (m *Manager) SwitchChannel(ctx context.Context, selectorTag, tag string, newProbe, rollbackProbe Probe) (err error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil
	generation := m.generation
	m.mu.RUnlock()
	if !validState {
		return ErrInvalidState
	}

	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.state != StateRunning || m.active != nil || m.generation != generation {
		m.mu.Unlock()
		cancel()
		return ErrInvalidState
	}
	engine := m.engine
	proxy := m.proxy
	if engine == nil {
		m.mu.Unlock()
		cancel()
		return ErrEngineNotConfigured
	}
	m.active = op
	m.generation++
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			err = errors.Join(fmt.Errorf("switch panic: %v", recovered), m.cleanup(engine, proxy))
		}
		m.finish(op, finalState)
	}()

	previous, err := engine.SwitchDefault(selectorTag, tag)
	if err != nil {
		// Seamless невозможен ИЛИ движок не работает: состояние не менялось
		// (или движок мёртв — ниже выясним probe'ом). Ошибка переключения
		// сама по себе не роняет туннель.
		if engine.Status() != "running" {
			finalState = StateError
			return errors.Join(err, m.restoreProxy(proxy))
		}
		return err
	}
	report := newProbe.Run(operationCtx)
	if err := contextError(operationCtx); err != nil {
		finalState = StateError
		return errors.Join(err, m.cleanup(engine, proxy))
	}
	if report != nil && report.LeakConfirmed {
		finalState = StateError
		return errors.Join(errors.New("switch protected probe confirmed a leak"), m.cleanup(engine, proxy))
	}
	if probePassed(report) {
		m.mu.Lock()
		m.probe = newProbe
		m.mu.Unlock()
		cancel()
		return nil
	}
	detail := "no report"
	if report != nil {
		detail = report.Summary()
	}

	// Новый канал не подтверждён: откат на прежний тег тем же API (без
	// рестарта) и повторное подтверждение прежнего пути.
	if _, rbErr := engine.SwitchDefault(selectorTag, previous); rbErr != nil {
		finalState = StateError
		cleanupErr := m.cleanup(engine, proxy)
		cancel()
		return errors.Join(
			fmt.Errorf("switch to %q failed probe (%s) and rollback failed: %w", tag, detail, rbErr),
			cleanupErr)
	}
	if rollbackProbe != nil {
		if rb := rollbackProbe.Run(operationCtx); probePassed(rb) && contextError(operationCtx) == nil {
			cancel()
			return &VerifiedRollbackError{Cause: fmt.Errorf("%s (previous channel %q)", detail, previous), Report: rb}
		}
	}
	// Прежний путь тоже мёртв (или rollbackProbe не задан — fail-closed):
	// полный teardown, как у Reload при неудачном probe.
	finalState = StateError
	cleanupErr := m.cleanup(engine, proxy)
	cancel()
	return errors.Join(
		fmt.Errorf("switch to %q failed probe (%s); previous channel %q not confirmed either", tag, detail, previous),
		cleanupErr)
}

// Reload: только из running. Новый конфиг валидируется (дважды: строгий
// парсер + sing-box) ДО закрытия старого бокса; после подмены — обязательный
// probe. Неудача кандидата допускает восстановление старых applied bytes с
// повторным старым probe. Отмена/утечка/ошибка cleanup запрещают такой откат.
func (m *Manager) Reload(ctx context.Context, rawJSON []byte, opts ...ReloadOption) (err error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil
	generation := m.generation
	m.mu.RUnlock()
	if !validState {
		return ErrInvalidState
	}

	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	defer cancel()
	normalizedJSON, ro, err := prepareReload(operationCtx, rawJSON, opts)
	if err != nil {
		return err
	}
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if ctxErr := contextError(operationCtx); ctxErr != nil {
		m.mu.Unlock()
		return errors.Join(ErrReloadRejected, ctxErr)
	}
	if m.state != StateRunning || m.active != nil || m.generation != generation {
		m.mu.Unlock()
		return errors.Join(ErrReloadRejected, fmt.Errorf("reload preparation superseded: %w", ErrInvalidState))
	}
	engine := m.engine
	previousProbe := m.probe
	probe := previousProbe
	proxy := m.proxy
	if ro.probe != nil {
		probe = ro.probe // ожидание egress следует за выбранным каналом (V2-032)
	}
	if engine == nil {
		m.mu.Unlock()
		cancel()
		return ErrEngineNotConfigured
	}
	m.active = op
	m.generation++
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			err = errors.Join(err, fmt.Errorf("reload panic: %v", recovered), m.cleanup(engine, proxy))
		}
		m.finish(op, finalState)
	}()

	fail := func(err error) error {
		finalState = StateError
		return errors.Join(err, m.cleanup(engine, proxy))
	}
	previousRaw := engine.rawSnapshot()
	verifyPrevious := func(cause error, alreadyRestored bool) error {
		// Keep the candidate failure in the named return even if rollback
		// panics before it can return its own result to the caller.
		err = cause
		if err := contextError(operationCtx); err != nil {
			return fail(errors.Join(cause, err))
		}
		if !alreadyRestored {
			if err := engine.Reload(operationCtx, previousRaw); err != nil {
				return fail(errors.Join(cause, fmt.Errorf("restore previous config: %w", err)))
			}
		}
		if err := contextError(operationCtx); err != nil {
			return fail(errors.Join(cause, err))
		}
		if previousProbe == nil {
			return fail(errors.Join(cause, errors.New("previous protected probe is required")))
		}
		report := previousProbe.Run(operationCtx)
		if err := contextError(operationCtx); err != nil {
			return fail(errors.Join(cause, err))
		}
		if !probePassed(report) {
			return fail(errors.Join(cause, fmt.Errorf("previous protected path not confirmed: %s", report.Summary())))
		}
		m.mu.Lock()
		m.probe = previousProbe
		m.mu.Unlock()
		return &VerifiedRollbackError{Cause: cause, Report: report, Restarted: true}
	}

	if probe == nil {
		return fail(errors.New("protected probe is required"))
	}
	if err := engine.Reload(operationCtx, normalizedJSON); err != nil {
		if contextError(operationCtx) != nil || engine.Status() != "running" {
			return fail(err)
		}
		if errors.Is(err, ErrReloadRejected) {
			return err // the original instance was not replaced
		}
		if errors.Is(err, errEngineReloadRolledBack) {
			return verifyPrevious(err, true)
		}
		return fail(err) // an unknown error cannot establish which config is live
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err)
	}
	report := probe.Run(operationCtx)
	if err := contextError(operationCtx); err != nil {
		return fail(err)
	}
	if report != nil && report.LeakConfirmed {
		return fail(errors.New("reload protected probe confirmed a leak"))
	}
	if !probePassed(report) {
		detail := "no report"
		if report != nil {
			detail = report.Summary()
		}
		return verifyPrevious(fmt.Errorf("protected probe failed: %s", detail), false)
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err)
	}
	m.mu.Lock()
	m.probe = probe
	m.mu.Unlock()
	return nil
}
