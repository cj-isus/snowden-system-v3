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
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

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
	mu                sync.Mutex
	box               boxInstance
	rawJSON           []byte
	runCtx            context.Context
	runCancel         context.CancelFunc
	cleanupErr        error
	workerCustody     *workerIngressCustody // only serveEngineWorker supplies this negative capability
	isolated          bool
	process           *engineProcess
	startPending      *engineStartOperation   // exact pre-native start reservation; Stop revokes it
	reloadPending     *engineReloadOperation  // original replacement owner across no-process/rollback gaps
	forwardingPermit  *engineForwardingPermit // private original-worker receipt, never decoded from public reports
	forwardingConsume *engineForwardingPermit // exact in-flight metadata handoff; any shared invalidation revokes it
	nativePermit      *engineNativeTUNPermit  // separate prepared native lane; no ordinary forwarding authority
	tunScope          *engineTUNScope         // original target process owns its duplicated actor handle
	// Optional per-engine factory for deterministic lifecycle fault injection.
	// Production uses newSingBox; it is not a shared/global test hook.
	newBox func(context.Context, []byte) (boxInstance, error)
}

// NewEngine embeds sing-box for the isolated worker and local harness only.
// It has cooperative cancellation; application composition uses NewIsolatedEngine.
func NewEngine(rawJSON []byte) *Engine {
	return &Engine{rawJSON: append([]byte(nil), rawJSON...)}
}

// NewIsolatedEngine is the Windows application entry. Only the worker/local
// harness uses NewEngine to embed sing-box. Isolation failure has no fallback.
func NewIsolatedEngine(rawJSON []byte) *Engine {
	return &Engine{rawJSON: append([]byte(nil), rawJSON...), isolated: true}
}

// Stop останавливает бокс (идемпотентно) и закрывает контекст времени жизни.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closeBoxLocked()
}

// Caller holds mu. Never forget an owner whose cleanup did not succeed.
func (e *Engine) closeBoxLocked() (result error) {
	e.startPending = nil // revoke preparation even before any process was created
	e.reloadPending = nil
	return e.closeBoxResourcesLocked()
}

// Caller holds Engine.mu. A registered reload uses this resource close while
// retaining its own metadata reservation; public Stop revokes it above.
func (e *Engine) closeBoxResourcesLocked() (result error) {
	if e.process != nil {
		e.process.serviceDNSPending = nil // Stop/reload close revokes the original DNS slot before callbacks
	}
	e.clearForwardingPermitsLocked()
	if e.isolated {
		return e.stopIsolatedLocked()
	}
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
	closeReturned := false
	defer func() {
		if !closeReturned {
			_ = recover()
			e.cleanupErr = errors.Join(ErrEngineCleanupFailed, e.cleanupErr)
			result = e.cleanupErr // a recovered unwind must not become a nil return
		}
	}()
	closeErr := cleanupCall("engine close", e.box.Close)
	closeReturned = true
	if err := closeErr; err != nil {
		e.cleanupErr = errors.Join(ErrEngineCleanupFailed, err)
		return e.cleanupErr
	}
	e.box = nil
	e.runCtx, e.runCancel = nil, nil
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
	if e.isolated {
		return ErrEngineWorkerUnavailable // isolated startup requires a prepared context/current attempt
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	normalized, err := protectedRuntimeConfig(ctx, e.rawJSON)
	if err != nil {
		return err // before worker/factory/listeners/routes or protocol credentials
	}
	e.rawJSON = normalized // apply the validated graph; do not re-interpret caller aliases/defaults
	lifetime := context.Background()
	if e.workerCustody != nil {
		lifetime = context.WithValue(lifetime, workerIngressCustodyKey{}, e.workerCustody)
	}
	e.runCtx, e.runCancel = context.WithCancel(lifetime)
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
	if e.process != nil {
		if e.process.unusable || e.process.exited() {
			return "error"
		}
		return "running"
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
	return e.SwitchDefaultContext(context.Background(), selectorTag, tag)
}

// CurrentDefault — тег, активный в селекторе сейчас (диагностика/тесты).
func (e *Engine) CurrentDefault(selectorTag string) (string, error) {
	e.mu.Lock()
	if e.cleanupErr != nil {
		err := e.cleanupErr
		e.mu.Unlock()
		return "", err
	}
	if e.isolated {
		e.mu.Unlock()
		if selectorTag != "proxy" {
			return "", ErrSelectorSwitchUnavailable
		}
		identity, err := e.CurrentRuntime(context.Background()) // prepares/cancels outside Engine.mu
		return identity.DefaultOutbound, err
	}
	defer e.mu.Unlock()
	if e.box == nil {
		return "", ErrEngineNotRunning
	}
	out, ok := e.box.Outbound().Outbound(selectorTag)
	if !ok {
		return "", fmt.Errorf("%w: outbound %q not found", ErrSelectorSwitchUnavailable, selectorTag)
	}
	sel, isSel := out.(selectorControl)
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
	raw, err := selectorConfiguration(e.rawJSON, selectorTag, tag)
	if err != nil {
		return err
	}
	return e.publishSelectorConfiguration(raw)
}

// Rewrite the already-owned graph with value-only decoding. Initial protected
// admission performs full validation; selector synchronization never reopens
// certificate files while holding Engine.mu. Legacy raw local graphs keep the
// existing generic JSON fallback, without acquiring forwarding authority.
func selectorConfiguration(raw []byte, selectorTag, tag string) ([]byte, error) {
	if cfg, _, err := runtimeConfiguration(raw); err == nil {
		return marshalSelectorConfiguration(cfg, selectorTag, tag)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("raw config is not a JSON object: %w", err)
	}
	outs, ok := doc["outbounds"].([]any)
	if !ok {
		return nil, errors.New("raw config has no outbounds array")
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
		return nil, fmt.Errorf("selector %q not found in raw config", selectorTag)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func marshalSelectorConfiguration(cfg *config.Config, selectorTag, tag string) ([]byte, error) {
	found := false
	for i := range cfg.Outbounds {
		if cfg.Outbounds[i].Tag == selectorTag && cfg.Outbounds[i].Type == "selector" {
			cfg.Outbounds[i].Default = tag
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("selector %q not found in raw config", selectorTag)
	}
	raw, err := jsonMarshalConfig(cfg)
	if err == nil && (len(raw) == 0 || len(raw) > maxWorkerConfigBytes) {
		return nil, errors.New("selector config exceeds worker bound")
	}
	return raw, err
}

func (e *Engine) publishSelectorConfiguration(raw []byte) error {
	if guarded, ok := e.box.(*admissionBox); ok && guarded.bootstrap != nil {
		subject, err := configurationSubject(raw)
		if err != nil {
			return err
		}
		if err := guarded.bootstrap.bindSource(subject); err != nil {
			return err
		}
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
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	// Windows builds a fresh worker lifetime without a platform adapter.
	// Such an adapter can activate SDK-internal parallel interface dials,
	// whose original resources are outside the owned base-call contract.
	if service.FromContext[adapter.PlatformInterface](ctx) != nil {
		return nil, ErrPreflightScope
	}
	if len(rawJSON) > maxWorkerConfigBytes {
		return nil, ErrPreflightScope
	}
	sdkJSON, cfg, err := sdkRuntimeConfiguration(rawJSON)
	if err != nil {
		return nil, err
	}
	subject, err := configurationSubject(rawJSON)
	if err != nil {
		return nil, err
	}
	// ContextWith mutates its service registry: clone before replacing the
	// inbound adapter so another box/caller cannot inherit this admission owner.
	boxCtx := service.ExtendContext(include.Context(ctx))
	registry := service.FromContext[adapter.InboundRegistry](boxCtx)
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](boxCtx)
	dnsRegistry := service.FromContext[adapter.DNSTransportRegistry](boxCtx)
	if registry == nil || outboundRegistry == nil || dnsRegistry == nil {
		return nil, ErrForwardingNotAdmitted
	}
	admission := newForwardingAdmission()
	admission.ownedDNSPort53 = firstDNSPort53Rule(cfg)
	admission.workerCustody, _ = ctx.Value(workerIngressCustodyKey{}).(*workerIngressCustody)
	bootstrap := newBootstrapWorker(ctx, admission, subject, cfg)
	if err := bootstrap.failure(); err != nil {
		return nil, errors.Join(err, bootstrap.close())
	}
	boxCtx = service.ContextWith[adapter.InboundRegistry](boxCtx, &admissionInboundRegistry{InboundRegistry: registry, admission: admission})
	boxCtx = service.ContextWith[adapter.OutboundRegistry](boxCtx, &admissionOutboundRegistry{OutboundRegistry: outboundRegistry, admission: admission, bootstrap: bootstrap})
	boxCtx = service.ContextWith[adapter.DNSTransportRegistry](boxCtx, &bootstrapDNSRegistry{DNSTransportRegistry: dnsRegistry, worker: bootstrap})
	var options option.Options
	if err := options.UnmarshalJSONContext(boxCtx, sdkJSON); err != nil {
		return nil, errors.Join(err, bootstrap.close())
	}
	b, err := box.New(box.Options{Context: boxCtx, Options: options})
	if err != nil {
		return nil, errors.Join(err, bootstrap.close()) // do not wrap a typed nil *box.Box in boxInstance
	}
	return &admissionBox{Box: b, admission: admission, bootstrap: bootstrap}, nil
}

// validateOptions — парсинг конфига средствами sing-box без старта.
func validateOptions(ctx context.Context, rawJSON []byte) error {
	_, err := validatedRuntimeOptions(ctx, rawJSON)
	return err
}

func validatedRuntimeOptions(ctx context.Context, rawJSON []byte) ([]byte, error) {
	normalized, err := protectedRuntimeConfig(ctx, rawJSON)
	if err != nil {
		return nil, err // rejected candidate cannot close/replace the running owner
	}
	boxCtx := include.Context(ctx)
	sdkJSON, _, err := sdkRuntimeConfiguration(normalized)
	if err != nil {
		return nil, err
	}
	var options option.Options
	if err := options.UnmarshalJSONContext(boxCtx, sdkJSON); err != nil {
		return nil, fmt.Errorf("invalid config for sing-box: %w", err)
	}
	return normalized, contextError(ctx)
}

func protectedRuntimeConfig(ctx context.Context, rawJSON []byte) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if len(rawJSON) > maxWorkerConfigBytes {
		return nil, errors.New("protected runtime config exceeds lifecycle bound")
	}
	validated, err := config.Parse(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("protected runtime config rejected: %w", err)
	}
	normalized, err := jsonMarshalConfig(validated)
	if err != nil || len(normalized) > maxWorkerConfigBytes {
		return nil, errors.New("normalized protected runtime config exceeds lifecycle bound")
	}
	return normalized, contextError(ctx)
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
		if deadline, bounded := ctx.Deadline(); bounded && !time.Now().Before(deadline) {
			return context.DeadlineExceeded // also cover delayed timer delivery before a write
		}
		if !OperationSourcesCurrent(ctx) {
			return context.Canceled // value-only; safe under Engine/Manager locks
		}
		return nil
	}
}
