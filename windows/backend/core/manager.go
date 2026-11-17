package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	bodyReturned    bool              // set only after the original runBody callback returns
	cleanupResult   *lifecycleCleanup // original cleanup sequence; owner goroutine only
	cancel          context.CancelFunc
	done            chan struct{}
	stopRequested   bool                  // guarded by Manager.mu; distinct from our own context retirement
	lifetime        *operationLifetime    // original sampler; installed only by this operation's caller
	runningLifetime *lifetimeWatch        // original previous running task, captured under Manager.mu
	terminalTrust   *lifecycleTrustBudget // original terminal sample, never refreshed under Manager.mu
	terminalContext *lifecycleCompletion  // original callback-free completion metadata/guard
}

// Manager — сериализованный жизненный цикл. Все переходы под mu; активная
// операция ровно одна (active != nil ⇒ переход идёт).
type Manager struct {
	mu                  sync.RWMutex
	identity            uint64
	generation          uint64 // increments when an operation takes lifecycle ownership
	active              *lifecycleOperation
	state               State
	engine              *Engine
	proxy               systemProxy
	proxyActive         bool
	proxyRestoreOutcome string
	proxyConflict       bool // deny-only; a retired foreign conflict requires a fresh Manager
	proxyRecovery       *managerProxyRecovery
	probe               Probe
	lastReport          *ProbeReport
	lifetimeWatch       *lifetimeWatch
	lifetimeSignal      chan<- struct{} // immutable coalesced notification, not an App callback
	lifetimeErr         error
}

var managerSequence atomic.Uint64

// Owner identifies lifecycle ownership, including a rollback/rejected switch
// operation that retained the same config. It is not network-health evidence.
func (m *Manager) Owner() (uint64, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.identity, m.generation
}

// LastProbeReport returns an isolated historical observation. Its SessionID,
// NetworkEpoch and current Manager state must match before live publication.
func (m *Manager) LastProbeReport() *ProbeReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastReport.Clone()
}

func (m *Manager) rememberProbe(report *ProbeReport) {
	m.mu.Lock()
	m.lastReport = report.Clone()
	m.mu.Unlock()
}

func NewManager() *Manager {
	return &Manager{identity: managerSequence.Add(1), state: StateStopped, proxy: &ProxyManager{}}
}

// SetEngine/SetProbe — только из состояния stopped (композиция до старта).
func (m *Manager) SetEngine(e *Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped && m.active == nil && !m.proxyActive {
		m.engine = e
	}
}

func (m *Manager) SetProbe(p Probe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped && m.active == nil && !m.proxyActive {
		m.probe = p
	}
}

// SetSystemProxy подменяет реализацию системного прокси (точка внедрения
// для тестов и CLI-режима без управления реестром). Только до старта.
func (m *Manager) SetSystemProxy(p systemProxy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped && m.active == nil && !m.proxyActive {
		m.proxy = p
	}
}

func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// A completed worker is factual loss of engine ownership, even before the
	// app watchdog performs teardown. Reading state does not change lifecycle.
	if m.state == StateRunning && m.engine != nil && m.engine.Status() != "running" {
		return StateError
	}
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
	var receipt *proxyRestoreReceipt
	native, isNative := proxy.(*ProxyManager)
	err := cleanupCall("restore system proxy", func() error {
		if isNative {
			var restoreErr error
			receipt, restoreErr = native.restoreOriginal()
			return restoreErr
		}
		return proxy.Restore()
	})
	if err != nil && isNative && errors.Is(err, ErrProxyOwnershipConflict) {
		if release, known := receipt.guardForeign(native); known {
			defer release()
			m.mu.Lock()
			defer m.mu.Unlock()
			current, same := m.proxy.(*ProxyManager)
			if !same || current != native || !m.proxyActive {
				return ErrEngineCleanupFailed
			}
			receipt.consumed = true
			m.proxyActive, m.proxyRestoreOutcome = false, proxyForeignPreserved
			if m.proxyRecovery != nil && m.proxyRecovery.proxy == proxy {
				m.proxyRecovery.finished = true
			}
			m.proxyConflict = true
			return nil // exact original HKEY retired; no unrelated cleanup error is suppressed
		}
	}
	if err != nil {
		return fmt.Errorf("restore system proxy: %w", err)
	}
	m.mu.Lock()
	m.proxyActive = false
	if m.proxyRecovery != nil && m.proxyRecovery.proxy == proxy {
		m.proxyRecovery.finished = true
	}
	if receipt != nil {
		m.proxyRestoreOutcome = receipt.outcome
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) finish(op *lifecycleOperation, state State) {
	m.mu.Lock()
	m.finishLocked(op, state)
	m.mu.Unlock()
	if op != nil {
		op.cancel()
	}
}

// Start: engine → protected probe → system proxy. Любая ошибка = полный cleanup +
// StateError. Паника перехватывается: cleanup обязателен при любом исходе.
func (m *Manager) Start(ctx context.Context) (err error) {
	defer m.retainProxyConflict(&err)
	if err := contextError(ctx); err != nil {
		return err
	}
	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.proxyConflict {
		m.mu.Unlock()
		cancel()
		return ErrProxyOwnershipConflict
	}
	if m.state != StateStopped || m.active != nil {
		m.mu.Unlock()
		cancel()
		return ErrInvalidState
	}
	m.active = op
	op.runningLifetime = m.cancelLifetimeLocked()
	m.lifetimeErr = nil
	m.generation++
	m.state = StateStarting
	engine := m.engine
	probe := m.probe
	proxy := m.proxy
	m.mu.Unlock()

	finalState := StateError
	defer m.finalizeLifecycle(&err, &finalState, &operationCtx, op, "start", engine, proxy, probe)

	return op.runBody(func() error {
		if err := m.retireRunningLifetime(operationCtx, op, engine, proxy, false); err != nil {
			return err
		}

		fail := func(err error, engineStarted bool) error {
			if engineStarted && engine != nil {
				return errors.Join(err, m.cleanup(engine, proxy, probe))
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
		if _, err := lifetimeRemaining(probe); err != nil {
			return fail(err, false)
		}
		operationCtx, op.lifetime = observeOperationLifetime(operationCtx, probe)
		if err := validateLifecycleSource(operationCtx, probe); err != nil {
			return fail(err, false)
		}
		m.mu.Lock()
		m.proxyActive = true
		m.mu.Unlock()
		// Snapshot may acquire a native handle before a later read fails. Register
		// cleanup first; Snapshot alone grants no registry restore write authority.
		if err := proxy.Snapshot(); err != nil {
			return fail(fmt.Errorf("snapshot system proxy: %w", err), false)
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err, false)
		}
		if err := prepareEngineChange(operationCtx, probe); err != nil {
			return fail(err, true)
		}
		if err := validateLifecycleSource(operationCtx, probe); err != nil {
			return fail(err, true)
		}
		if err := engine.Start(operationCtx); err != nil {
			return fail(fmt.Errorf("engine start: %w", err), false)
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err, true)
		}
		report := runLifecycleProbe(operationCtx, probe)
		m.rememberProbe(report)
		if !lifecycleProbePassed(operationCtx, engine, probe, report) {
			detail := "no report"
			if report != nil {
				detail = report.Summary()
			}
			return fail(errors.Join(reportLifecycleError(report), fmt.Errorf("protected probe failed: %s", detail)), true)
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err, true)
		}
		if engine.Status() != "running" {
			return fail(ErrEngineWorkerUnavailable, true)
		}

		if _, err := lifetimeRemaining(probe); err != nil {
			return fail(err, true)
		}
		if err := validateLifecycleSource(operationCtx, probe); err != nil {
			return fail(err, true)
		}
		if err := proxy.SetProxy("127.0.0.1:1080"); err != nil {
			return fail(fmt.Errorf("set system proxy: %w", err), true)
		}
		// A blocked native notification/write may outlive the preceding checks.
		if err := contextError(operationCtx); err != nil {
			return fail(err, true)
		}
		if engine.Status() != "running" {
			return fail(ErrEngineWorkerUnavailable, true)
		}
		if _, err := lifetimeRemaining(probe); err != nil {
			return fail(err, true)
		}
		if err := m.commitEngineForwarding(operationCtx, engine, probe, report); err != nil {
			return fail(err, true)
		}
		finalState = StateRunning
		return nil
	})
}

// Stop: деструктивная очистка — отменяет активную операцию (если есть),
// останавливает движок и восстанавливает прокси ДАЖЕ при отменённом
// caller-контексте. Ожидание операции ограничено собственным таймаутом;
// Application Engine isolates third-party work in an owned process; its Close
// has a separate graceful/reap budget. Embedded local harness stays cooperative.
// This is not an overall OS syscall/proxy/WFP cleanup time guarantee.
func (m *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()

	for {
		m.mu.Lock()
		if m.state == StateStopped && m.active == nil && m.lifetimeWatch == nil && !m.proxyActive {
			m.mu.Unlock()
			return ctx.Err()
		}
		switch m.state {
		case StateStopped, StateRunning, StateError, StateStarting, StateReloading, StateStopping:
		default:
			m.mu.Unlock()
			return ErrInvalidState
		}
		if m.active != nil {
			op := m.active
			op.stopRequested = true
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
		op.runningLifetime = m.cancelLifetimeLocked()
		m.generation++
		m.state = StateStopping
		engine := m.engine
		proxy := m.proxy
		m.mu.Unlock()

		return m.stopOwnedOperation(ctx, waitCtx, op, engine, proxy)
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
			err = errors.Join(err, contextError(ctx), fmt.Errorf("reload preparation panic (%T)", recovered))
		}
		if err != nil {
			err = errors.Join(ErrReloadRejected, err)
		}
	}()
	if err := contextError(ctx); err != nil {
		return nil, ro, err
	}
	if len(rawJSON) > maxWorkerConfigBytes {
		return nil, ro, errors.New("reload config exceeds lifecycle bound")
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
	if err := validateOptions(ctx, raw); err != nil {
		return nil, ro, err
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
	if report == nil || !report.AllPassed || report.LeakConfirmed || report.ProtectionUnavailable || report.CleanupUnconfirmed || report.TrustUnavailable || report.Stale || len(report.Results) == 0 {
		return false
	}
	required := 0
	for _, step := range report.Results {
		step = step.NormalizeDiagnostic()
		if step.Informational() {
			continue
		}
		required++
		if !step.Passed || step.Status != "" && step.Status != "pass" {
			return false
		}
		if step.Observation.Schema != 0 && (step.Observation.Schema != 1 || !step.Observation.BodyComplete || step.Observation.ErrorKind != "") {
			return false
		}
	}
	return required != 0
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
	defer m.retainProxyConflict(&err)
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil && !m.proxyConflict
	generation := m.generation
	m.mu.RUnlock()
	if !validState {
		return ErrInvalidState
	}

	operationCtx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.state != StateRunning || m.active != nil || m.generation != generation || m.proxyConflict {
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
	op.runningLifetime = m.cancelLifetimeLocked()
	m.generation++
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer m.finalizeLifecycle(&err, &finalState, &operationCtx, op, "switch", engine, proxy, newProbe, rollbackProbe)

	return op.runBody(func() error {
		if err := m.retireRunningLifetime(operationCtx, op, engine, proxy, false); err != nil {
			finalState = StateError
			return err
		}

		if _, err := lifetimeRemaining(newProbe); err != nil {
			return errors.Join(ErrReloadRejected, err) // no mutation; rearm previous lifetime
		}
		operationCtx, op.lifetime = observeOperationLifetime(operationCtx, newProbe)
		if err := validateLifecycleSource(operationCtx, newProbe); err != nil {
			return errors.Join(ErrReloadRejected, err) // no selector mutation
		}
		previous, err := engine.SwitchDefaultContext(operationCtx, selectorTag, tag)
		if err != nil {
			// Ordinary preflight rejection may leave the old selector unchanged.
			// An unconfirmed/superseded switch cannot rearm the old state merely
			// because its original or successor process is still running.
			if errors.Is(err, errSwitchStateUnconfirmed) || engine.Status() != "running" {
				finalState = StateError
				return errors.Join(err, m.restoreProxy(proxy))
			}
			return err
		}
		report := runLifecycleProbe(operationCtx, newProbe)
		m.rememberProbe(report)
		if report != nil && report.CleanupUnconfirmed {
			finalState = StateError
			return errors.Join(reportLifecycleError(report), m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if err := contextError(operationCtx); err != nil {
			finalState = StateError
			return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if report != nil && report.LeakConfirmed {
			finalState = StateError
			return errors.Join(errors.New("switch protected probe confirmed a leak"), m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if lifecycleProbePassed(operationCtx, engine, newProbe, report) {
			if _, err := lifetimeRemaining(newProbe); err != nil {
				finalState = StateError
				return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
			}
			if engine.Status() != "running" {
				finalState = StateError
				return errors.Join(ErrEngineWorkerUnavailable, m.cleanup(engine, proxy, newProbe, rollbackProbe))
			}
			if err := m.commitEngineForwarding(operationCtx, engine, newProbe, report); err != nil {
				finalState = StateError
				return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
			}
			m.mu.Lock()
			m.probe = newProbe
			m.mu.Unlock()
			return nil
		}
		detail := "no report"
		if report != nil {
			detail = report.Summary()
		}

		// Новый канал не подтверждён: откат на прежний тег тем же API (без
		// рестарта) и повторное подтверждение прежнего пути.
		// Protection uncertainty is not a successful candidate or a proven leak;
		// restoration must revoke current permits and pass the full old lifecycle gate.
		if _, err := lifetimeRemaining(rollbackProbe); err != nil {
			finalState = StateError
			return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if err := prepareEngineChange(operationCtx, rollbackProbe); err != nil {
			finalState = StateError
			return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if err := validateLifecycleSource(operationCtx, rollbackProbe); err != nil {
			finalState = StateError
			return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
		}
		if _, rbErr := engine.SwitchDefaultContext(operationCtx, selectorTag, previous); rbErr != nil {
			finalState = StateError
			cleanupErr := m.cleanup(engine, proxy, newProbe, rollbackProbe)
			return errors.Join(
				fmt.Errorf("switch to %q failed probe (%s) and rollback failed: %w", tag, detail, rbErr),
				cleanupErr)
		}
		if rollbackProbe != nil {
			rb := runLifecycleProbe(operationCtx, rollbackProbe)
			m.rememberProbe(rb)
			if rb != nil && rb.CleanupUnconfirmed {
				finalState = StateError
				return errors.Join(reportLifecycleError(rb), m.cleanup(engine, proxy, newProbe, rollbackProbe))
			}
			if lifecycleProbePassed(operationCtx, engine, rollbackProbe, rb) && contextError(operationCtx) == nil && engine.Status() == "running" {
				if _, err := lifetimeRemaining(rollbackProbe); err != nil {
					finalState = StateError
					return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
				}
				if err := m.commitEngineForwarding(operationCtx, engine, rollbackProbe, rb); err != nil {
					finalState = StateError
					return errors.Join(err, m.cleanup(engine, proxy, newProbe, rollbackProbe))
				}
				m.mu.Lock()
				m.probe = rollbackProbe // retains constraints of any selector-lived streams
				m.mu.Unlock()
				rb = m.LastProbeReport() // includes the completed native phase when present
				return &VerifiedRollbackError{Cause: fmt.Errorf("%s (previous channel %q)", detail, previous), Report: rb}
			}
		}
		// Прежний путь тоже мёртв (или rollbackProbe не задан — fail-closed):
		// полный teardown, как у Reload при неудачном probe.
		finalState = StateError
		cleanupErr := m.cleanup(engine, proxy, newProbe, rollbackProbe)
		return errors.Join(
			fmt.Errorf("switch to %q failed probe (%s); previous channel %q not confirmed either", tag, detail, previous),
			cleanupErr)
	})
}

// Reload: только из running. Новый конфиг валидируется (дважды: строгий
// парсер + sing-box) ДО закрытия старого бокса; после подмены — обязательный
// probe. Неудача кандидата допускает восстановление старых applied bytes с
// повторным старым probe. Отмена/утечка/ошибка cleanup запрещают такой откат.
func (m *Manager) Reload(ctx context.Context, rawJSON []byte, opts ...ReloadOption) (err error) {
	defer m.retainProxyConflict(&err)
	if err := contextError(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	validState := m.state == StateRunning && m.active == nil && !m.proxyConflict
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
	if m.state != StateRunning || m.active != nil || m.generation != generation || m.proxyConflict {
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
	op.runningLifetime = m.cancelLifetimeLocked()
	m.generation++
	m.state = StateReloading
	m.mu.Unlock()

	finalState := StateRunning
	defer m.finalizeLifecycle(&err, &finalState, &operationCtx, op, "reload", engine, proxy, probe, previousProbe)

	return op.runBody(func() error {
		if err := m.retireRunningLifetime(operationCtx, op, engine, proxy, false); err != nil {
			finalState = StateError
			return err
		}

		fail := func(err error) error {
			finalState = StateError
			return errors.Join(err, m.cleanup(engine, proxy, probe, previousProbe))
		}
		previousRaw := engine.rawSnapshot()
		verifyPrevious := func(cause error, alreadyRestored, restarted bool) error {
			// Keep the candidate failure in the named return even if rollback
			// panics before it can return its own result to the caller.
			err = cause
			if err := contextError(operationCtx); err != nil {
				return fail(errors.Join(cause, err))
			}
			if !alreadyRestored {
				if _, err := lifetimeRemaining(previousProbe); err != nil {
					return fail(errors.Join(cause, err))
				}
				if err := prepareEngineChange(operationCtx, previousProbe); err != nil {
					return fail(errors.Join(cause, err))
				}
				if err := validateLifecycleSource(operationCtx, previousProbe); err != nil {
					return fail(errors.Join(cause, err))
				}
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
			report := runLifecycleProbe(operationCtx, previousProbe)
			m.rememberProbe(report)
			if err := contextError(operationCtx); err != nil {
				return fail(errors.Join(cause, err))
			}
			if !lifecycleProbePassed(operationCtx, engine, previousProbe, report) {
				return fail(errors.Join(cause, reportLifecycleError(report), fmt.Errorf("previous protected path not confirmed: %s", report.Summary())))
			}
			if engine.Status() != "running" {
				return fail(errors.Join(cause, ErrEngineWorkerUnavailable))
			}
			if _, err := lifetimeRemaining(previousProbe); err != nil {
				return fail(errors.Join(cause, err))
			}
			if err := m.commitEngineForwarding(operationCtx, engine, previousProbe, report); err != nil {
				return fail(errors.Join(cause, err))
			}
			m.mu.Lock()
			m.probe = previousProbe
			m.mu.Unlock()
			report = m.LastProbeReport()
			return &VerifiedRollbackError{Cause: cause, Report: report, Restarted: restarted}
		}

		if probe == nil {
			return fail(errors.New("protected probe is required"))
		}
		if _, err := lifetimeRemaining(probe); err != nil {
			return errors.Join(ErrReloadRejected, err)
		}
		operationCtx, op.lifetime = observeOperationLifetime(operationCtx, probe)
		if err := prepareEngineChange(operationCtx, probe); err != nil {
			return fail(err)
		}
		if err := validateLifecycleSource(operationCtx, probe); err != nil {
			return fail(err)
		}
		if err := engine.Reload(operationCtx, normalizedJSON); err != nil {
			if contextError(operationCtx) != nil || engine.Status() != "running" {
				return fail(err)
			}
			if errors.Is(err, ErrReloadRejected) {
				if _, guarded := probe.(LifecycleProbe); guarded {
					return verifyPrevious(err, true, false) // restore revoked permits and fresh proof
				}
				return err // unguarded original instance was not replaced
			}
			if errors.Is(err, errEngineReloadRolledBack) {
				return verifyPrevious(err, true, true)
			}
			return fail(err) // an unknown error cannot establish which config is live
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err)
		}
		report := runLifecycleProbe(operationCtx, probe)
		m.rememberProbe(report)
		if report != nil && report.CleanupUnconfirmed {
			return fail(reportLifecycleError(report)) // do not start rollback over uncertain helper custody
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err)
		}
		if report != nil && report.LeakConfirmed {
			return fail(errors.New("reload protected probe confirmed a leak"))
		}
		if !lifecycleProbePassed(operationCtx, engine, probe, report) {
			detail := "no report"
			if report != nil {
				detail = report.Summary()
			}
			return verifyPrevious(errors.Join(reportLifecycleError(report), fmt.Errorf("protected probe failed: %s", detail)), false, true)
		}
		if err := contextError(operationCtx); err != nil {
			return fail(err)
		}
		if engine.Status() != "running" {
			return fail(ErrEngineWorkerUnavailable)
		}
		if _, err := lifetimeRemaining(probe); err != nil {
			return fail(err)
		}
		if err := m.commitEngineForwarding(operationCtx, engine, probe, report); err != nil {
			return fail(err)
		}
		m.mu.Lock()
		m.probe = probe
		m.mu.Unlock()
		return nil
	})
}
