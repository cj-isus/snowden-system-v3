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
	if err := proxy.Restore(); err != nil {
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
	m.state = StateStarting
	engine := m.engine
	probe := m.probe
	proxy := m.proxy
	m.mu.Unlock()

	finalState := StateError
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			cleanupErr := error(nil)
			if engine != nil {
				cleanupErr = engine.Stop()
			}
			cleanupErr = errors.Join(cleanupErr, m.restoreProxy(proxy))
			err = errors.Join(fmt.Errorf("start panic: %v", recovered), cleanupErr)
		}
		m.finish(op, finalState)
	}()

	fail := func(err error, engineStarted bool) error {
		var cleanupErr error
		if engineStarted && engine != nil {
			cleanupErr = engine.Stop()
		}
		cleanupErr = errors.Join(cleanupErr, m.restoreProxy(proxy))
		return errors.Join(err, cleanupErr)
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

	report := probe.Run(operationCtx)
	if report == nil || !report.AllPassed {
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
// caller-контексте (очистка ограничена собственным таймаутом).
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
		m.state = StateStopping
		engine := m.engine
		proxy := m.proxy
		m.mu.Unlock()

		var stopErr error
		if engine != nil {
			stopErr = engine.Stop()
		}
		restoreErr := m.restoreProxy(proxy)

		if stopErr != nil || restoreErr != nil {
			m.finish(op, StateError)
		} else {
			m.finish(op, StateStopped)
		}
		return errors.Join(ctx.Err(), stopErr, restoreErr)
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

// ErrSwitchRolledBack — переключение канала не прошло probe, но защищённый
// путь на ПРЕЖНЕМ канале подтверждён: движок жив, состояние Manager — running.
// Вызывающий НЕ должен ронять туннель (failCore) — это не отказ канала,
// это отказ кандидата. Старая семантика Reload рвала рабочий туннель при
// неудачном переключении на мёртвый канал (V2-037).
var ErrSwitchRolledBack = errors.New("channel switch failed probe: rolled back to previous channel")

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
func (m *Manager) SwitchChannel(ctx context.Context, selectorTag, tag string, newProbe, rollbackProbe Probe) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil
	m.mu.RUnlock()
	if !validState {
		return ErrInvalidState
	}

	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.state != StateRunning || m.active != nil {
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
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			_ = engine.Stop()
			_ = m.restoreProxy(proxy)
			cancel()
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
			_ = m.restoreProxy(proxy)
			return err
		}
		return err
	}
	if previous == tag {
		cancel()
		return nil // уже активен
	}

	report := newProbe.Run(operationCtx)
	if report != nil && report.AllPassed {
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
		stopErr := engine.Stop()
		restoreErr := m.restoreProxy(proxy)
		cancel()
		return errors.Join(
			fmt.Errorf("switch to %q failed probe (%s) and rollback failed: %w", tag, detail, rbErr),
			stopErr, restoreErr)
	}
	if rollbackProbe != nil {
		if rb := rollbackProbe.Run(operationCtx); rb != nil && rb.AllPassed {
			cancel()
			return fmt.Errorf("%w: %s (откат на %q подтверждён probe)", ErrSwitchRolledBack, detail, previous)
		}
	}
	// Прежний путь тоже мёртв (или rollbackProbe не задан — fail-closed):
	// полный teardown, как у Reload при неудачном probe.
	finalState = StateError
	stopErr := engine.Stop()
	restoreErr := m.restoreProxy(proxy)
	cancel()
	return errors.Join(
		fmt.Errorf("switch to %q failed probe (%s); previous channel %q not confirmed either", tag, detail, previous),
		stopErr, restoreErr)
}

// Reload: только из running. Новый конфиг валидируется (дважды: строгий
// парсер + sing-box) ДО закрытия старого бокса; после подмены — обязательный
// probe; неудача ⇒ cleanup и StateError (fail-closed: старый туннель уже
// нельзя считать доверенным, если probe нового не прошёл).
func (m *Manager) Reload(ctx context.Context, rawJSON []byte, opts ...ReloadOption) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil
	m.mu.RUnlock()
	if !validState {
		return ErrInvalidState
	}

	validated, err := config.Parse(rawJSON)
	if err != nil {
		return fmt.Errorf("validate reload config: %w", err)
	}
	normalizedJSON, err := json.Marshal(validated)
	if err != nil {
		return fmt.Errorf("normalize reload config: %w", err)
	}

	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.state != StateRunning || m.active != nil {
		m.mu.Unlock()
		cancel()
		return ErrInvalidState
	}
	engine := m.engine
	probe := m.probe
	proxy := m.proxy
	ro := reloadOpts{}
	for _, fn := range opts {
		fn(&ro)
	}
	if ro.probe != nil {
		probe = ro.probe // ожидание egress следует за выбранным каналом (V2-032)
	}
	if engine == nil {
		m.mu.Unlock()
		cancel()
		return ErrEngineNotConfigured
	}
	m.active = op
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer func() {
		if recovered := recover(); recovered != nil {
			finalState = StateError
			_ = engine.Stop()
			_ = m.restoreProxy(proxy)
			err = fmt.Errorf("reload panic: %v", recovered)
		}
		m.finish(op, finalState)
	}()

	fail := func(err error) error {
		finalState = StateError
		cleanupErr := engine.Stop()
		cleanupErr = errors.Join(cleanupErr, m.restoreProxy(proxy))
		return errors.Join(err, cleanupErr)
	}

	if probe == nil {
		return fail(errors.New("protected probe is required"))
	}
	if err := engine.Reload(operationCtx, normalizedJSON); err != nil {
		// Движок мог откатиться сам (валидный sing-box-парс, но неудачный
		// старт нового бокса) — тогда канал жив и мы НЕ рвём состояние.
		if engine.Status() == "stopped" {
			return fail(err)
		}
		return err
	}
	if err := contextError(operationCtx); err != nil {
		return fail(err)
	}
	report := probe.Run(operationCtx)
	if report == nil || !report.AllPassed {
		detail := "no report"
		if report != nil {
			detail = report.Summary()
		}
		return fail(fmt.Errorf("protected probe failed: %s", detail))
	}
	return nil
}
