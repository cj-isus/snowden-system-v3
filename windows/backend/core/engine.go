// Package core — ядро VPN: движок sing-box, менеджер lifecycle, защищённый
// probe, TLS preflight, системный прокси.
//
// Контракты (PLAN §3): FR-001 (жизненный цикл с probe-gate), FR-003
// (структурный отчёт probe), FR-004 (сериализация операций, engine-контекст
// ≠ operation-контекст), FR-005 (строгая валидация — см. backend/config).
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"

	"github.com/snowden-system/windows/backend/config"
)

var (
	ErrEngineNotConfigured  = errors.New("sing-box engine is not configured")
	ErrEngineAlreadyRunning = errors.New("sing-box engine is already running")
	ErrEngineNotRunning     = errors.New("sing-box engine is not running")
	// A rejected update did not replace the live instance.
	ErrReloadRejected = errors.New("reload rejected before engine replacement")
	// Restoring configuration is not proof of the old path; only Manager may
	// upgrade this internal result to ErrSwitchRolledBack after a fresh probe.
	errEngineReloadRolledBack = errors.New("previous engine configuration restored")
	// Box.Close cannot retry child cleanup after it closes its done channel.
	// Keep this error sticky: only process isolation/restart can retire the owner.
	ErrEngineCleanupFailed = errors.New("engine cleanup incomplete; process restart required")
	// ErrSelectorSwitchUnavailable — seamless-переключение невозможно в текущем
	// конфиге (нет outbound-группы/тега). Вызывающий обязан откатиться на путь
	// Reload (Engine.Reload), а не считать это отказом канала.
	ErrSelectorSwitchUnavailable = errors.New("selector switch unavailable")
)

// boxInstance is the sing-box boundary used for local lifecycle fault injection.
type boxInstance interface {
	Start() error
	Close() error
	Outbound() adapter.OutboundManager
}

// Engine — владелец экземпляра sing-box. Потокобезопасен; каждый вызов
// проверяет контекст перед входом (FR-004: отмена операции не должна
// оставлять полузапущенный бокс).
//
// Урок P0-2026-09-08 «engine-context kill after Start»: бокс живёт на
// СОБСТВЕННОМ контексте времени жизни (runCtx), а не на контексте операции.
// После успешного Start менеджер закрывает operationCtx — это НЕ должно
// убивать туннель: соединения sing-box привязаны к контексту бокса.
type Engine struct {
	mu         sync.Mutex
	box        boxInstance
	rawJSON    []byte
	runCtx     context.Context
	runCancel  context.CancelFunc
	cleanupErr error
	// Optional per-engine factory for deterministic lifecycle fault injection.
	// Production uses newSingBox; it is not a shared/global test hook.
	newBox func(context.Context, []byte) (boxInstance, error)
}

// NewEngine создаёт движок с нормализованным (провалидированным выше по
// стеку) конфигом. Копия данных — вызывающий может переиспользовать слайс.
func NewEngine(rawJSON []byte) *Engine {
	return &Engine{rawJSON: append([]byte(nil), rawJSON...)}
}

// Start запускает бокс. Повторный запуск без Stop — ошибка.
func (e *Engine) Start(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	if e.box != nil {
		return ErrEngineAlreadyRunning
	}
	if len(e.rawJSON) == 0 {
		return ErrEngineNotConfigured
	}
	// Собственный контекст времени жизни: только Stop/Reload закрывают его.
	return e.startForOperation(ctx)
}

// Stop останавливает бокс (идемпотентно) и закрывает контекст времени жизни.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closeBoxLocked()
}

// Caller holds mu. Never forget an owner whose cleanup did not succeed.
func (e *Engine) closeBoxLocked() error {
	if e.runCancel != nil {
		e.runCancel()
	}
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	if e.box == nil {
		e.runCtx, e.runCancel = nil, nil
		return nil
	}
	if err := cleanupCall("engine close", e.box.Close); err != nil {
		e.cleanupErr = errors.Join(ErrEngineCleanupFailed, err)
		return e.cleanupErr
	}
	e.box = nil
	e.runCtx, e.runCancel = nil, nil
	return nil
}

// Reload заменяет конфиг на лету. Новый конфиг парсится sing-box ДО закрытия
// старого бокса: битый апдейт не разрывает живой защищённый канал. Если новый
// бокс не стартовал — предыдущий конфиг восстанавливается (rollback).
func (e *Engine) Reload(ctx context.Context, rawJSON []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(rawJSON) == 0 {
		return errors.Join(ErrReloadRejected, errors.New("reload: empty config"))
	}
	if err := validateOptions(ctx, rawJSON); err != nil {
		return errors.Join(ErrReloadRejected, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	if e.box == nil {
		return ErrEngineNotRunning
	}

	oldJSON := append([]byte(nil), e.rawJSON...)
	if err := e.closeBoxLocked(); err != nil {
		return fmt.Errorf("reload: close old box: %w", err)
	}
	e.rawJSON = append([]byte(nil), rawJSON...)
	if err := e.startForOperation(ctx); err != nil {
		if contextError(ctx) != nil || e.cleanupErr != nil {
			return err // canceled or unretired candidate must never resurrect the old box
		}
		// Rollback: вернуть прежний рабочий конфиг, чтобы защищённый канал жил.
		e.rawJSON = oldJSON
		if restartErr := e.startForOperation(ctx); restartErr != nil {
			return errors.Join(err, restartErr)
		}
		return errors.Join(errEngineReloadRolledBack, fmt.Errorf("reload: %w", err))
	}
	return nil
}

// Copy the applied bytes while Manager owns the lifecycle operation. Never
// rebuild rollback configuration from descriptors or current secret values.
func (e *Engine) rawSnapshot() []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]byte(nil), e.rawJSON...)
}

// Cancellation during Start reaches the box's lifetime context. A successful
// operation detaches the callback: its later timeout cannot kill the tunnel.
// This is cooperative cancellation, not a hard bound on third-party Close.
func (e *Engine) startForOperation(ctx context.Context) (err error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	e.runCtx, e.runCancel = context.WithCancel(context.Background())
	stopCancellation := context.AfterFunc(ctx, e.runCancel)
	defer func() {
		detached := stopCancellation()
		if recovered := recover(); recovered != nil {
			err = errors.Join(err, fmt.Errorf("engine start panic: %v", recovered))
		}
		if !detached || contextError(ctx) != nil {
			err = errors.Join(err, ctx.Err())
		}
		if err != nil {
			err = errors.Join(err, e.closeBoxLocked())
		}
	}()
	return e.startBox(e.runCtx)
}

// Status — "running" | "stopped" | "error" (cleanup has not been proved).
func (e *Engine) Status() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanupErr != nil {
		return "error"
	}
	if e.box != nil {
		return "running"
	}
	return "stopped"
}

// SwitchDefault — переключение активного outbound селектора на живом боксе
// БЕЗ рестарта (V2-037): соединения пользователей не рвутся, слушатели не
// перевешиваются, системный прокси не трогается. Оба канала уже работают в
// боксе как кандидаты селектора — меняется только выбор.
//
// rawJSON синхронизируется следом: он остаётся «последним рабочим конфигом»
// для будущих Start/Reload-откатов, и рассинхрон превращённого состояния
// был бы тихим дефектом при следующем рестарте.
//
// Возвращает тег, активный ДО переключения (для откката вызывающим).
// Ошибки с ErrSelectorSwitchUnavailable = seamless-путь невозможен,
// вызывающий откатывается на Reload.
func (e *Engine) SwitchDefault(selectorTag, tag string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanupErr != nil {
		return "", e.cleanupErr
	}
	if e.box == nil {
		return "", ErrEngineNotRunning
	}
	out, ok := e.box.Outbound().Outbound(selectorTag)
	if !ok {
		return "", fmt.Errorf("%w: outbound %q not found", ErrSelectorSwitchUnavailable, selectorTag)
	}
	sel, isSel := out.(*group.Selector)
	if !isSel {
		return "", fmt.Errorf("%w: %q is not a selector", ErrSelectorSwitchUnavailable, selectorTag)
	}
	previous := sel.Now()
	if previous == tag {
		return previous, nil // уже активен: идемпотентно
	}
	if !sel.SelectOutbound(tag) {
		return previous, fmt.Errorf("%w: outbound %q is not a selector candidate", ErrSelectorSwitchUnavailable, tag)
	}
	if err := e.rewriteSelectorDefault(selectorTag, tag); err != nil {
		// Селектор переключён, но конфиг-хранилище не обновлено — рассинхрон
		// хуже отказа: откатываем выбор и отдаём ошибку (вызывающий → Reload).
		sel.SelectOutbound(previous)
		return previous, fmt.Errorf("sync raw config after switch: %w", err)
	}
	return previous, nil
}

// CurrentDefault — тег, активный в селекторе сейчас (диагностика/тесты).
func (e *Engine) CurrentDefault(selectorTag string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanupErr != nil {
		return "", e.cleanupErr
	}
	if e.box == nil {
		return "", ErrEngineNotRunning
	}
	out, ok := e.box.Outbound().Outbound(selectorTag)
	if !ok {
		return "", fmt.Errorf("%w: outbound %q not found", ErrSelectorSwitchUnavailable, selectorTag)
	}
	sel, isSel := out.(*group.Selector)
	if !isSel {
		return "", fmt.Errorf("%w: %q is not a selector", ErrSelectorSwitchUnavailable, selectorTag)
	}
	return sel.Now(), nil
}

// rewriteSelectorDefault — точечная правка default селектора в rawJSON.
// Основной путь — типизированный round-trip (конфиг рендера всегда проходит
// config.Parse, round-trip байт-эквивалентен по семантике); если конфиг
// строгому парсеру не подчиняется (тесты/сырые конфиги), правится generic-
// представление. Оба пути дают на выходе валидный JSON с новым default.
func (e *Engine) rewriteSelectorDefault(selectorTag, tag string) error {
	if cfg, err := config.Parse(e.rawJSON); err == nil {
		found := false
		for i := range cfg.Outbounds {
			if cfg.Outbounds[i].Tag == selectorTag && cfg.Outbounds[i].Type == "selector" {
				cfg.Outbounds[i].Default = tag
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("selector %q not found in raw config", selectorTag)
		}
		raw, err := jsonMarshalConfig(cfg)
		if err != nil {
			return err
		}
		e.rawJSON = raw
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(e.rawJSON, &doc); err != nil {
		return fmt.Errorf("raw config is not a JSON object: %w", err)
	}
	outs, ok := doc["outbounds"].([]any)
	if !ok {
		return errors.New("raw config has no outbounds array")
	}
	found := false
	for _, o := range outs {
		m, ok := o.(map[string]any)
		if !ok || m["tag"] != selectorTag {
			continue
		}
		m["default"] = tag
		found = true
	}
	if !found {
		return fmt.Errorf("selector %q not found in raw config", selectorTag)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	e.rawJSON = raw
	return nil
}

func (e *Engine) startBox(ctx context.Context) error {
	factory := e.newBox
	if factory == nil {
		factory = newSingBox
	}
	b, err := factory(ctx, e.rawJSON)
	// Retain the box before Start: a partial Start or panic still owns resources.
	e.box = b
	if err != nil {
		return err
	}
	if b == nil {
		return errors.New("engine factory returned no instance")
	}
	return b.Start()
}

func newSingBox(ctx context.Context, rawJSON []byte) (boxInstance, error) {
	boxCtx := include.Context(ctx)
	var options option.Options
	if err := options.UnmarshalJSONContext(boxCtx, rawJSON); err != nil {
		return nil, err
	}
	b, err := box.New(box.Options{Context: boxCtx, Options: options})
	if err != nil {
		return nil, err // do not wrap a typed nil *box.Box in boxInstance
	}
	return b, nil
}

// validateOptions — парсинг конфига средствами sing-box без старта.
func validateOptions(ctx context.Context, rawJSON []byte) error {
	boxCtx := include.Context(ctx)
	var options option.Options
	if err := options.UnmarshalJSONContext(boxCtx, rawJSON); err != nil {
		return fmt.Errorf("invalid config for sing-box: %w", err)
	}
	return nil
}

func jsonMarshalConfig(cfg *config.Config) ([]byte, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return raw, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
