package main

// failover.go — A4: автоматическое переключение каналов по факту деградации
// защищённого пути (PLAN §5 Phase A4, §4.1). V2-037: семантика переработана.
// Правила:
//   - решение принимает ТОЛЬКО этот policy; применение — через
//     switchChannelCore (бесшовно, селектором) с fallback на Manager.Reload;
//   - кандидаты — только enabled live-verified каналы из дескрипторов
//     (FR-002/FR-007; HY2-configured НЕ автопереключается — только явный выбор);
//   - прямой fallback запрещён (FR-001); кандидаты исчерпаны → BLOCKED
//     (fail-closed, tun/SOCKS выключаются), не тишина и не direct;
//   - «подтверждённый отказ» = серия сторожа (тик + fast-репробы провалились
//     подряд, см. watchCore): краткие сетевые мерцания (Wi-Fi⇄LTE, CGNAT)
//     НЕ роняют туннель и НЕ переключают канал — раньше первый же провал
//     тика останавливал VPN целиком (аудит V2-037);
//   - circuit breaker по AGENTS §3.3: попытка открывается только после
//     подтверждённой серии; после неудачной попытки — cooldown 30/60с;
//     success → closed. Состояние в памяти процесса.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// reloadCooldown — минимальная пауза между автоматическими попытками:
// переключение + probe занимают секунды, а деградация может быть
// кратковременной; шторм переключений не лечит сеть, но рвёт сессии.
const reloadCooldown = 30 * time.Second

// failoverMaxSwitches — подряд за один инцидент без успешного probe.
const failoverMaxSwitches = 2

// failoverPolicy — состояние автопереключения (AGENTS §3.3: closed/open/
// half-open). Собственный мьютекс сериализует только автопопытки и не касается
// пользовательских Start/Stop/SelectChannel — их границы enforced Manager'ом.
type failoverPolicy struct {
	mu                sync.Mutex
	state             string // closed | open | half-open
	switches          int    // автоматических переключений за текущий инцидент
	lastAttempt       time.Time
	lastError         string
	attempting        bool   // идёт автоматическая попытка (half-open проба)
	attemptGeneration uint64 // never reset: late completion cannot finish a successor
	recentEpoch       int
	recentAttempts    map[string]time.Time // failed work, never channel-health evidence
}

// cooldownFor — обратный экспоненциальный backoff: 30с → 60с → 120с (cap).
func cooldownFor(switches int) time.Duration {
	shift := switches
	if shift > 2 {
		shift = 2
	}
	return reloadCooldown << uint(shift)
}

// beginAttemptIfDue — вход в попытку ПОСЛЕ подтверждённой серии отказов
// (watchCore уже отфильтровал мерцания fast-репробами, V2-037). false =
// гонка проиграна или cooldown ещё не истёк (следующая серия подождёт).
func (p *failoverPolicy) beginAttemptIfDue(now time.Time) bool {
	return p.beginScopedAttemptIfDue(now) != 0
}

// Register and return the original token under the same policy lock. A late
// currentAttempt read could otherwise capture a replacement after hardReset.
func (p *failoverPolicy) beginScopedAttemptIfDue(now time.Time) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attempting {
		return 0
	}
	if p.state == "open" && now.Before(p.lastAttempt.Add(cooldownFor(p.switches))) {
		return 0
	}
	if p.attemptGeneration == ^uint64(0) {
		return 0
	}
	p.attemptGeneration++
	p.attempting = true
	p.state = "half-open"
	p.lastAttempt = now
	return p.attemptGeneration
}

// endAttempt — исход попытки. success: все счётчики сброшены (→ closed).
// failure: попытка израсходована (switches++ — иначе вечный retry каждые
// 30с при постоянно отказывающем канале); true = лимит исчерпан → BLOCKED
// (fail-closed).
func (p *failoverPolicy) endAttempt(success bool, to string) (blocked bool) {
	return p.endAttemptWithLimit(success, to, failoverMaxSwitches)
}

func (p *failoverPolicy) endAttemptWithLimit(success bool, to string, limit int) (blocked bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.endAttemptLocked(success, to, limit)
}

func (p *failoverPolicy) endAttemptLocked(success bool, to string, limit int) bool {
	p.attempting = false
	if success {
		p.state = "closed"
		p.switches = 0
		p.lastError = ""
		return false
	}
	p.switches++
	p.lastError = "переключение на " + to + " не прошло probe"
	if limit < 1 {
		limit = 1
	}
	p.state = "open" // exhausted work also keeps a cooldown if a live path was retained
	if p.switches >= limit {
		return true
	}
	return false
}

// snapshot — снимок для UI/журнала (без секретов, только факты).
func (p *failoverPolicy) snapshot() (state string, switches int, lastError string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state, p.switches, p.lastError
}

// nextAttemptIn — секунд до ближайшей автопопытки (для UI-карточки
// «Автозащита»). closed = сторож тикает по watchdogInterval; open = ожидает
// конец cooldown; half-open = попытка уже идёт (0). Производная от фактов
// policy, не выдумка.
func (p *failoverPolicy) nextAttemptIn(now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.state {
	case "half-open":
		return 0
	case "open":
		d := cooldownFor(p.switches) - now.Sub(p.lastAttempt)
		if d < 0 {
			return 0
		}
		return int((d + time.Second - 1) / time.Second)
	default:
		return int(watchdogInterval / time.Second)
	}
}

// hardReset — успешный тик сторожа или явный пользовательский Start: прошлый
// инцидент не должен копиться.
func (p *failoverPolicy) hardReset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = "closed"
	p.switches = 0
	p.lastError = ""
	p.attempting = false
}

// failoverCandidates — порядок попыток: прочие селекторные каналы, затем
// остальные enabled live-verified (HY2-configured не входит — FR-007).
// Текущий канал исключается.
func failoverCandidates(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID string) []string {
	var others, verified []string
	seen := map[string]bool{}
	for _, ch := range render.Selectable(descriptors) {
		if ch.ID == currentID || seen[ch.ID] {
			continue
		}
		seen[ch.ID] = true
		if ch.ValidationStatus != "live-verified" {
			continue
		}
		if inSelector[ch.ID] {
			others = append(others, ch.ID)
		} else {
			verified = append(verified, ch.ID)
		}
	}
	return append(others, verified...)
}

// switchChannelCore — переключение на channelID: и для кнопки UI, и для
// автопереключения сторожа. Два уровня (V2-037):
//  1. БЕСШОВНО (основной): оба канала уже работают в селекторе бокса —
//     Manager.SwitchChannel меняет выбор БЕЗ рестарта движка (соединения
//     пользователей не рвутся); probe нового канала обязателен; провал =
//     откат селектором на прежний канал (туннель жив) или fail-closed stop,
//     если мёртв и прежний;
//  2. RELOAD (fallback): селектор недоступен в живом конфиге (структурно
//     новый канал) — прежний путь render → двойная валидация → Reload → probe.
func (a *App) switchChannelCore(channelID string) error {
	return a.switchChannelCoreFor(channelID, nil)
}

var errSessionSuperseded = errors.New("session superseded")

func (a *App) switchChannelCoreFor(channelID string, expected *render.ClientConfig, observed ...*core.ProbeReport) error {
	a.mu.Lock()
	id := a.operationID // registration before waiting; never adopt a later Stop/Start
	a.mu.Unlock()
	return a.switchChannelCoreIntent(id, channelID, expected, observed...)
}

func (a *App) switchChannelCoreIntent(id uint64, channelID string, expected *render.ClientConfig, observed ...*core.ProbeReport) error {
	outcome := a.newLifecycleOutcome(id, "переключение канала")
	return a.switchChannelCoreOutcome(outcome, channelID, expected, observed...)
}

func (a *App) switchChannelCoreRegistered(registration intentRegistration, channelID string) error {
	outcome := &appLifecycleOutcome{app: a, registration: registration, stage: "переключение канала"}
	return a.switchChannelCoreOutcome(outcome, channelID, nil)
}

func (a *App) switchChannelCoreOutcome(outcome *appLifecycleOutcome, channelID string, expected *render.ClientConfig, observed ...*core.ProbeReport) (resultErr error) {
	defer outcome.finish(&resultErr)
	resultErr = a.switchChannelCoreBody(outcome.registration.intent, channelID, expected, outcome, observed...)
	outcome.returned = true
	return resultErr
}

func (a *App) switchChannelCoreBody(id uint64, channelID string, expected *render.ClientConfig, outcome *appLifecycleOutcome, observed ...*core.ProbeReport) (resultErr error) {
	if !outcome.acquire() {
		return core.ErrEngineCleanupFailed
	}
	if !outcome.originalCurrent() {
		return context.Canceled
	}
	if expected != nil && !a.sessionCurrent(expected) {
		return errSessionSuperseded
	}
	ctx, cancel, preparationErr := a.beginOperationContext(id, outcome.registration.source)
	if preparationErr != nil {
		outcome.rejectPreparation(preparationErr)
		return preparationErr
	}
	defer a.endOperation(cancel)
	// Keep the actual Manager/cleanup/trust error when the network cancels
	// this frame. A current network cancellation has no separate user Stop.
	defer func() { a.finishCanceledLifecycleCause(ctx, id, resultErr) }()
	if len(observed) > 0 && (len(observed) != 1 || observed[0] == nil || observed[0].IntentGeneration != id || !a.currentProbeReport(expected, observed[0])) {
		return errSessionSuperseded
	}
	phase := observationApplied
	if expected == nil && len(observed) == 0 {
		phase = observationStarted // explicit selection ends the old incident
	}
	runtime_.mu.Lock()
	m := runtime_.manager
	oldCC := runtime_.cc
	oldMetrics := runtime_.metrics
	runtime_.mu.Unlock()
	if m == nil || m.State() != "running" || oldCC == nil {
		return fmt.Errorf("переключение канала: VPN не запущен")
	}
	cc, err := a.renderAndPrepareContext(ctx, channelID, oldMetrics)
	if err != nil {
		return err
	}
	if err := a.retainSessionClock(cc); err != nil {
		return err // candidate was never applied; retain the current owner
	}
	a.mu.Lock()
	if a.operationID != id || !a.protectionDesired || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.State = "reloading"
	a.mu.Unlock()
	a.pushState()

	// Уровень 1: бесшовное переключение селектором. tag канала в живом
	// конфиге == его ID (рендер строит теги из ID).
	seamlessErr := core.ErrSelectorSwitchUnavailable
	candidateValidity := cc.Validity
	if selectorOnlyChange(oldCC.Raw, cc.Raw) {
		// Selector changes can leave existing streams on their old transport.
		// Retain their trust deadlines until a real worker replacement.
		cc.Validity = cc.Validity.Constrain(oldCC.Validity)
		rollbackCC := oldCC.Clone()
		rollbackCC.Validity = cc.Validity
		seamlessErr = m.SwitchChannel(ctx, "proxy", channelID,
			a.protectedOperationProbe(ctx, cc, m), a.protectedOperationProbe(ctx, rollbackCC, m))
		if _, verified := verifiedRollback(seamlessErr); verified && m.State() == core.StateRunning {
			runtime_.mu.Lock()
			runtime_.cc = rollbackCC
			runtime_.mu.Unlock()
			oldCC = rollbackCC
		}
	}
	if ctx.Err() != nil {
		return core.InvalidateLifecycleOutcome(seamlessErr, ctx.Err())
	}
	if !a.operationCurrent(id) {
		return core.InvalidateLifecycleOutcome(seamlessErr, context.Canceled) // a newer intent owns teardown
	}
	reloaded := false
	if errors.Is(seamlessErr, core.ErrSelectorSwitchUnavailable) {
		// Уровень 2: структурный fallback через Reload.
		reloaded = true
		cc.Validity = candidateValidity // the fresh worker retires old streams
		seamlessErr = m.Reload(ctx, cc.Raw, core.WithReloadProbe(a.protectedOperationProbe(ctx, cc, m)))
	}
	if seamlessErr != nil {
		return a.finishChannelSwitchErrorForPhase(ctx, id, channelID, m, oldCC, seamlessErr, phase)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !a.operationCurrent(id) {
		return context.Canceled
	}
	cc.OwnerID, cc.OwnerGeneration = m.Owner()
	runtime_.mu.Lock()
	runtime_.cc = cc
	if reloaded {
		runtime_.metrics = metricsForConfig(cc.Raw)
	}
	runtime_.mu.Unlock()
	if !a.publishObservationContextLocked(ctx, cc, m.LastProbeReport(), phase, "") {
		if a.operationCurrent(id) {
			a.failCoreForIntent(id, "результат переключения утратил привязку к текущей сессии/сети", errSessionSuperseded)
		}
		return errSessionSuperseded
	}
	a.appendLog("info", "канал переключён: "+channelID)
	a.pushState()
	return nil
}

func verifiedRollback(err error) (*core.VerifiedRollbackError, bool) {
	var rollback *core.VerifiedRollbackError
	ok := errors.As(err, &rollback) && rollback != nil && rollback.Report != nil && rollback.Report.AllPassed && !rollback.Report.LeakConfirmed && !rollback.Report.ProtectionUnavailable && !rollback.Report.CleanupUnconfirmed && !rollback.Report.TrustUnavailable && !rollback.Report.Stale
	return rollback, ok
}

// Both switch mechanisms publish the same outcomes. Caller holds operationMu.
// A restarted old configuration retires the old observer/metrics lifetime.
func (a *App) finishChannelSwitchError(ctx context.Context, id uint64, channelID string, m *core.Manager, oldCC *render.ClientConfig, err error, observed ...*core.ProbeReport) error {
	phase := observationApplied
	if len(observed) == 0 {
		phase = observationStarted
	}
	return a.finishChannelSwitchErrorForPhase(ctx, id, channelID, m, oldCC, err, phase)
}

func (a *App) finishChannelSwitchErrorForPhase(ctx context.Context, id uint64, channelID string, m *core.Manager, oldCC *render.ClientConfig, err error, phase observationPhase) error {
	if ctx.Err() != nil {
		return core.InvalidateLifecycleOutcome(err, ctx.Err())
	}
	if !a.operationCurrent(id) {
		return core.InvalidateLifecycleOutcome(err, context.Canceled)
	}
	if !a.sessionCurrent(oldCC) {
		return core.InvalidateLifecycleOutcome(err, errSessionSuperseded)
	}
	rollback, verified := verifiedRollback(err)
	rejected := errors.Is(err, core.ErrReloadRejected)
	if channelRetryBlocked(err) || !verified && !rejected || m.State() != core.StateRunning {
		// Independent custody failure invalidates any joined positive rollback
		// receipt; it is never permission to retain or try another channel.
		err = core.InvalidateLifecycleOutcome(err, nil)
		a.failCoreForIntent(id, "переключение на "+channelID, err)
		return err
	}
	owner, generation := m.Owner()
	applied := oldCC
	if verified && rollback.Restarted || oldCC.OwnerID != owner || oldCC.OwnerGeneration != generation {
		// Same configuration, new engine lifetime: stale watchdog/network/probe
		// responses must not affect it, and rate baselines start from scratch.
		restored := oldCC.Clone()
		applied = restored
		restored.OwnerID, restored.OwnerGeneration = owner, generation
		runtime_.mu.Lock()
		runtime_.cc = restored
		if verified && rollback.Restarted {
			runtime_.metrics = metricsForConfig(restored.Raw)
		}
		runtime_.mu.Unlock()
	}
	if verified {
		if !a.publishObservationContextLocked(ctx, applied, rollback.Report, phase, "") {
			if a.operationCurrent(id) {
				a.failCoreForIntent(id, "rollback утратил привязку к текущей сессии/сети", errSessionSuperseded)
			}
			return core.InvalidateLifecycleOutcome(err, errSessionSuperseded)
		}
	} else {
		// Rejection retained the runtime but produced no new path proof. Do not
		// refresh probe time, health or HEALTHY from an old UI/report projection.
		if !a.publishRetainedLifecycleLocked(ctx, applied, m, id) {
			if a.operationCurrent(id) {
				a.failCoreForIntent(id, "сохранённая сессия утратила привязку к текущей операции/сети", errSessionSuperseded)
			}
			return core.InvalidateLifecycleOutcome(err, errSessionSuperseded)
		}
	}
	if verified {
		a.appendLog("warn", "канал "+channelID+" не применён; прежний защищённый путь повторно подтверждён: "+err.Error())
	} else {
		a.appendLog("warn", "канал "+channelID+" отклонён до замены ядра: "+err.Error())
	}
	return err
}

// Seamless only changes the live selector. Any other config difference must
// actually be loaded, including DNS, credentials, routes and controller.
func selectorOnlyChange(oldRaw, nextRaw []byte) bool {
	normalize := func(raw []byte) ([]byte, error) {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		var outs []map[string]json.RawMessage
		if err := json.Unmarshal(doc["outbounds"], &outs); err != nil {
			return nil, err
		}
		found := false
		for _, out := range outs {
			if string(out["tag"]) == `"proxy"` && string(out["type"]) == `"selector"` {
				delete(out, "default")
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("no protected selector")
		}
		var err error
		doc["outbounds"], err = json.Marshal(outs)
		if err != nil {
			return nil, err
		}
		return json.Marshal(doc)
	}
	old, err := normalize(oldRaw)
	if err != nil {
		return false
	}
	next, err := normalize(nextRaw)
	return err == nil && bytes.Equal(old, next)
}

// tryFailover — одна автоматическая попытка: candidates → первый, чей тег
// реально в живом селекторе; переключение через switchChannelCore (бесшовно,
// с fallback). Итог: (переключились, BLOCKED-все-исчерпаны).
func (a *App) tryFailover(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID string) (switched, blocked bool) {
	return a.tryFailoverForClass(descriptors, inSelector, currentID, "")
}

// Compatibility entry for existing local fixtures. Runtime watchdog/network
// observers use tryObservedFailover with captured intent/epoch/report evidence.

// tryFailoverForClass keeps the generic health/independence score, then
// applies the observable countermeasure required by the failure class.
func (a *App) tryFailoverForClass(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID, failureClass string, expected ...*render.ClientConfig) (switched, blocked bool) {
	return a.tryFailoverWithObservation(descriptors, inSelector, currentID, failureClass, nil, expected...)
}

func (a *App) tryObservedFailover(cc *render.ClientConfig, report *core.ProbeReport, failureClass string) (switched, blocked bool) {
	if cc == nil || report == nil || report.AllPassed || unsafeObservation(report) || !a.currentProbeReport(cc, report) {
		return false, false
	}
	// Recovery may try another accepted candidate after unavailable measurements;
	// its work budget/hints do not establish transport failure or a censor cause.
	failureClass = ClassifyFailure(report, 0, 0).Code
	return a.tryFailoverWithObservation(cc.Descriptors, nil, cc.SelectorDefault, failureClass, report, cc)
}

func (a *App) tryFailoverWithObservation(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID, failureClass string, report *core.ProbeReport, expected ...*render.ClientConfig) (switched, blocked bool) {
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	if cc == nil {
		return false, true
	}
	if len(expected) > 0 && (cc != expected[0] || !a.sessionCurrent(expected[0])) {
		return false, false
	}
	// Rank/eligibility and profile provenance must describe the same applied
	// snapshot. A later metadata/UI read is not this engine's configuration.
	if len(cc.Descriptors) == 0 {
		return false, false // unknown applied set, not evidence all paths failed
	}
	descriptors, currentID = cc.Descriptors, cc.SelectorDefault
	inSelector = make(map[string]bool, len(cc.SelectorTags))
	for _, tag := range cc.SelectorTags {
		inSelector[tag] = true
	}
	candidates := failoverCandidates(descriptors, inSelector, currentID)
	if len(candidates) == 0 {
		a.appendLog("warn", "failover: кандидатов нет (нужен второй validated канал) — BLOCKED")
		return false, true
	}

	// Кандидат вне живого селектора требует полного Reload — это осознанное
	// решение о структуре конфига; автопереключение берёт только живые теги
	// (Reload всё равно доступен как fallback switchChannelCore, но выбор
	// кандидата должен быть доказанным).
	var reachableIDs []string
	for _, cand := range candidates {
		if chID, ok := cc.ChannelIDs[cand]; ok && chID != "" {
			reachableIDs = append(reachableIDs, cand)
		}
	}
	if len(reachableIDs) == 0 {
		a.appendLog("warn", "failover: кандидат(ы) вне живого селектора — автопереключение невозможно, BLOCKED")
		return false, true
	}
	// C2 (V2-061): пересортировка ДОСТИЖИМЫХ кандидатов по score против
	// деградировавшего канала (plan_gpt §28: независимость с наибольшим
	// весом — «минимально коррелированный survivor»). Список кандидатов
	// не расширяется и не сокращается — только порядок; при пустой эпохе
	// и равной независимости порядок = прежний (failoverCandidates, V2-027).
	scored, scoreLog := a.scoredCandidates(reachableIDs, descriptors, currentID, cc.Profile)
	scored = orderCandidatesForFailure(scored, descriptors, currentID, failureClass)
	attemptEpoch := currentChannelEpoch()
	scored = a.fpol.orderByRecentAttempts(scored, attemptEpoch, time.Now())
	target := scored[0]
	if scoreLog != "" {
		a.appendLog("info", "failover: "+scoreLog)
	}
	if report != nil {
		attempt, ok := a.reserveObservedSwitch(cc, report, failureClass)
		if !ok {
			return false, false
		}
		err := a.switchChannelCoreIntent(attempt.intent, target, cc, report)
		return a.finishObservedSwitch(attempt, target, len(reachableIDs), err)
	}
	a.mu.Lock()
	operationID := a.operationID
	a.mu.Unlock()
	attemptPath := netEpoch.pathSource()
	attempt := a.fpol.beginScopedAttemptIfDue(time.Now())
	if attempt == 0 {
		return false, false // попытка уже идёт или cooldown
	}
	a.sm.markSwitching(time.Now()) // C1 (V2-060): попытка начата → SWITCHING (§25)
	a.appendLog("warn", "failover: переключение "+currentID+" → "+target)
	return a.finishFailoverAttemptForScope(attempt, operationID, target, len(reachableIDs), a.switchChannelCoreIntent(operationID, target, cc), attemptPath, attemptEpoch)
}

// Existing direct entry point; asynchronous work must pass its original token
// to finishFailoverAttemptForScope, not capture a replacement on completion.
func (a *App) finishFailoverAttempt(operationID uint64, target string, candidates int, err error, observedEpoch ...int) (switched, blocked bool) {
	return a.finishFailoverAttemptForScope(a.fpol.currentAttempt(), operationID, target, candidates, err, nil, observedEpoch...)
}

func (a *App) finishFailoverAttemptForScope(attempt, operationID uint64, target string, candidates int, err error, path *networkPathSource, observedEpoch ...int) (switched, blocked bool) {
	// Serialize observer publication with Stop/new-session completion as well
	// as the channel change itself. A late recovery cannot overwrite stopped.
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if attempt == 0 || a.fpol.currentAttempt() != attempt {
		return false, false
	}
	if !a.operationCurrent(operationID) {
		a.fpol.endScopedAttempt(attempt, true, "", candidates)
		return false, false
	}
	if path != nil && !path.current.Load() || len(observedEpoch) == 1 && observedEpoch[0] != currentChannelEpoch() {
		a.fpol.endScopedAttempt(attempt, true, "", candidates)
		return false, false // stale success and failure both abstain, including epoch0 down
	}
	if err != nil {
		if channelRetryBlocked(err) || trustFailureCode(err) != "" || errors.Is(err, errInstalledImageChanged) {
			a.fpol.deferScopedAttempt(attempt)
			return false, false // local lifecycle failure is not candidate exhaustion or fresh path proof
		}
		if errors.Is(err, errSessionSuperseded) || errors.Is(err, context.Canceled) {
			a.fpol.endScopedAttempt(attempt, true, "", candidates) // obsolete observation is not channel failure
			return false, false
		}
		if rollback, verified := verifiedRollback(err); verified {
			// The old path has fresh proof. Exhausting bad candidates cannot
			// turn that successful recovery into BLOCKED for the working path.
			if accepted, _ := a.fpol.endScopedAttempt(attempt, true, "", candidates); !accepted {
				return false, false
			}
			if !errors.Is(err, core.ErrReloadRejected) {
				a.recordCandidateFailure(target, time.Now(), observedEpoch...)
			}
			a.sm.markTick(true, rollback.Report.Summary(), time.Now())
			a.appendLog("warn", "failover: "+err.Error())
			return false, false
		}
		accepted, exhausted := a.fpol.endScopedAttempt(attempt, false, target, candidates)
		if !accepted {
			return false, false
		}
		a.sm.markSwitchFailed(time.Now())
		if !errors.Is(err, core.ErrReloadRejected) {
			a.recordCandidateFailure(target, time.Now(), observedEpoch...)
		}
		a.appendLog("warn", "failover: "+err.Error())
		a.logWaveIfAny() // D3 (V2-065): несколько отказов подряд = одна волна сети, не N инцидентов
		runtime_.mu.Lock()
		running := runtime_.manager != nil && runtime_.manager.State() == core.StateRunning
		runtime_.mu.Unlock()
		return false, exhausted && !running // exhaustion alone cannot retire a retained live owner
	}
	if accepted, _ := a.fpol.endScopedAttempt(attempt, true, target, candidates); !accepted {
		return false, false
	}
	a.sm.markSwitched(time.Now()) // C1: новый канал прошёл probe → HEALTHY (§22)
	a.appendLog("info", "failover: канал переключён на "+target)
	return true, false
}

// orderCandidatesForFailure is a stable class-aware preference. It only
// reorders validated candidates; the mandatory probe remains the decision
// gate. Unknown and broad classes preserve generic scoring.
func orderCandidatesForFailure(ids []string, descriptors []render.ChannelDescriptor, failedID, failureClass string) []string {
	if len(ids) < 2 || failureClass == "" {
		return ids
	}
	byID := make(map[string]render.ChannelDescriptor, len(descriptors))
	for _, d := range descriptors {
		byID[d.ID] = d
	}
	failed, ok := byID[failedID]
	if !ok {
		return ids
	}
	out := append([]string(nil), ids...)
	sort.SliceStable(out, func(i, j int) bool {
		return countermeasureTier(byID[out[i]], failed, failureClass) < countermeasureTier(byID[out[j]], failed, failureClass)
	})
	return out
}

func countermeasureTier(candidate, failed render.ChannelDescriptor, failureClass string) int {
	layer := func(d render.ChannelDescriptor) string {
		if d.Transport == "quic" {
			return "udp"
		}
		return "tcp"
	}
	portFamily := func(d render.ChannelDescriptor) string {
		if d.Port == 443 {
			return "443"
		}
		for _, p := range d.ServerPorts {
			if p == "443" || len(p) > 4 && p[:4] == "443:" {
				return "443"
			}
		}
		return "other"
	}
	switch failureClass {
	case fcSilentFreeze:
		if layer(candidate) == "udp" {
			return 0
		}
		if candidate.Transport != failed.Transport {
			return 1
		}
	case fcTlsPolicing443:
		if layer(candidate) == "udp" {
			return 0
		}
		if portFamily(candidate) == "other" {
			return 1
		}
	case fcPortBlackholed:
		if portFamily(candidate) != portFamily(failed) {
			return 0
		}
		if layer(candidate) != layer(failed) {
			return 1
		}
	case fcResetInjected:
		if layer(candidate) != layer(failed) {
			return 0
		}
		if candidate.Transport != failed.Transport {
			return 1
		}
	default:
		return 0
	}
	return 2
}

// Failed work may change bounded retry order, never invent a protected probe
// result. An error alone can be preflight, cancellation or measurement outage.
func (a *App) recordCandidateFailure(channelID string, now time.Time, observedEpoch ...int) {
	if channelID == "" || len(observedEpoch) != 1 || observedEpoch[0] != currentChannelEpoch() {
		return
	}
	a.fpol.noteAttemptFailure(channelID, observedEpoch[0], now)
}
