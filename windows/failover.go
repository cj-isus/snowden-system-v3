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
	mu          sync.Mutex
	state       string // closed | open | half-open
	switches    int    // автоматических переключений за текущий инцидент
	lastAttempt time.Time
	lastError   string
	attempting  bool // идёт автоматическая попытка (half-open проба)
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
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attempting {
		return false
	}
	if p.state == "open" && now.Before(p.lastAttempt.Add(cooldownFor(p.switches))) {
		return false
	}
	p.attempting = true
	p.state = "half-open"
	p.lastAttempt = now
	return true
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
	if p.switches >= limit {
		return true
	}
	p.state = "open" // cooldown перед следующей попыткой
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
	for _, ch := range descriptors {
		if !ch.Enabled || ch.ID == currentID || seen[ch.ID] {
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

func (a *App) switchChannelCoreFor(channelID string, expected *render.ClientConfig) error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if expected != nil && !a.sessionCurrent(expected) {
		return errSessionSuperseded
	}
	a.mu.Lock()
	id := a.operationID
	a.mu.Unlock()
	ctx, cancel, ok := a.beginOperation(id)
	if !ok {
		return context.Canceled
	}
	defer a.endOperation(cancel)
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
	a.mu.Lock()
	if a.operationID != id || !a.protectionDesired || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.State = "reloading"
	a.pushStateLocked()
	a.mu.Unlock()

	// Уровень 1: бесшовное переключение селектором. tag канала в живом
	// конфиге == его ID (рендер строит теги из ID).
	seamlessErr := core.ErrSelectorSwitchUnavailable
	if selectorOnlyChange(oldCC.Raw, cc.Raw) {
		seamlessErr = m.SwitchChannel(ctx, "proxy", channelID,
			a.protectedProbe(cc), a.protectedProbe(oldCC))
	}
	if ctx.Err() != nil || !a.operationCurrent(id) {
		return context.Canceled // Stop owns teardown/publication.
	}
	reloaded := false
	if errors.Is(seamlessErr, core.ErrSelectorSwitchUnavailable) {
		// Уровень 2: структурный fallback через Reload.
		reloaded = true
		seamlessErr = m.Reload(ctx, cc.Raw, core.WithReloadProbe(a.protectedProbe(cc)))
	}
	if seamlessErr != nil {
		return a.finishChannelSwitchError(ctx, id, channelID, m, oldCC, seamlessErr)
	}

	if ctx.Err() != nil || !a.operationCurrent(id) {
		return context.Canceled
	}
	runtime_.mu.Lock()
	runtime_.cc = cc
	if reloaded {
		runtime_.metrics = metricsForConfig(cc.Raw)
	}
	runtime_.mu.Unlock()
	a.mu.Lock()
	if a.operationID != id || !a.protectionDesired {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.ActiveID = channelID
	a.state.Active = channelViewOf(cc, channelID)
	a.state.State = "running"
	a.pushStateLocked()
	a.mu.Unlock()
	a.appendLog("info", "канал переключён: "+channelID)
	a.pushState()
	return nil
}

func verifiedRollback(err error) (*core.VerifiedRollbackError, bool) {
	var rollback *core.VerifiedRollbackError
	ok := errors.As(err, &rollback) && rollback != nil && rollback.Report != nil && rollback.Report.AllPassed && !rollback.Report.LeakConfirmed
	return rollback, ok
}

// Both switch mechanisms publish the same outcomes. Caller holds operationMu.
// A restarted old configuration retires the old observer/metrics lifetime.
func (a *App) finishChannelSwitchError(ctx context.Context, id uint64, channelID string, m *core.Manager, oldCC *render.ClientConfig, err error) error {
	if ctx.Err() != nil || !a.operationCurrent(id) {
		return context.Canceled
	}
	if !a.sessionCurrent(oldCC) {
		return errSessionSuperseded
	}
	rollback, verified := verifiedRollback(err)
	rejected := errors.Is(err, core.ErrReloadRejected)
	if !verified && !rejected || m.State() != core.StateRunning {
		a.failCore("переключение на "+channelID, err)
		return err
	}
	if verified && rollback.Restarted {
		// Same configuration, new engine lifetime: stale watchdog/network/probe
		// responses must not affect it, and rate baselines start from scratch.
		restored := *oldCC
		restored.Raw = append([]byte(nil), oldCC.Raw...)
		runtime_.mu.Lock()
		runtime_.cc = &restored
		runtime_.metrics = metricsForConfig(restored.Raw)
		runtime_.mu.Unlock()
	}
	a.mu.Lock()
	if a.operationID != id || !a.protectionDesired || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.State = "running"
	a.state.Error = ""
	if verified {
		a.state.CoreReady = true
		a.state.ProbeRunning = false
		a.state.Probe = toView(rollback.Report)
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
	}
	a.pushStateLocked()
	a.mu.Unlock()
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

// tryFailoverForClass keeps the generic health/independence score, then
// applies the observable countermeasure required by the failure class.
func (a *App) tryFailoverForClass(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID, failureClass string, expected ...*render.ClientConfig) (switched, blocked bool) {
	if len(expected) > 0 && !a.sessionCurrent(expected[0]) {
		return false, false
	}
	candidates := failoverCandidates(descriptors, inSelector, currentID)
	if len(candidates) == 0 {
		a.appendLog("warn", "failover: кандидатов нет (нужен второй validated канал) — BLOCKED")
		return false, true
	}
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	if cc == nil {
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
	scored, scoreLog := a.scoredCandidates(reachableIDs, descriptors, currentID)
	scored = orderCandidatesForFailure(scored, descriptors, currentID, failureClass)
	target := scored[0]
	if scoreLog != "" {
		a.appendLog("info", "failover: "+scoreLog)
	}
	if !a.fpol.beginAttemptIfDue(time.Now()) {
		return false, false // попытка уже идёт или cooldown
	}
	a.sm.markSwitching(time.Now()) // C1 (V2-060): попытка начата → SWITCHING (§25)
	a.appendLog("warn", "failover: переключение "+currentID+" → "+target)
	a.mu.Lock()
	operationID := a.operationID
	a.mu.Unlock()
	return a.finishFailoverAttempt(operationID, target, len(reachableIDs), a.switchChannelCoreFor(target, cc))
}

func (a *App) finishFailoverAttempt(operationID uint64, target string, candidates int, err error) (switched, blocked bool) {
	// Serialize observer publication with Stop/new-session completion as well
	// as the channel change itself. A late recovery cannot overwrite stopped.
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if !a.operationCurrent(operationID) {
		a.mu.Lock()
		desired := a.protectionDesired
		a.mu.Unlock()
		if !desired {
			a.fpol.endAttempt(true, "")
		}
		return false, false
	}
	if err != nil {
		if errors.Is(err, errSessionSuperseded) || errors.Is(err, context.Canceled) {
			a.fpol.endAttempt(true, "") // obsolete observation is not channel failure
			return false, false
		}
		a.sm.markSwitchFailed(time.Now()) // C1: попытка не удалась → DEGRADED (rollback на живой канал / cooldown, §26)
		if !errors.Is(err, core.ErrReloadRejected) {
			a.recordCandidateFailure(target, time.Now())
		}
		a.appendLog("warn", "failover: "+err.Error())
		if rollback, verified := verifiedRollback(err); verified {
			// The old path has fresh proof. Exhausting bad candidates cannot
			// turn that successful recovery into BLOCKED for the working path.
			a.fpol.endAttempt(true, "")
			a.sm.markTick(true, rollback.Report.Summary(), time.Now())
			return false, false
		}
		a.logWaveIfAny() // D3 (V2-065): несколько отказов подряд = одна волна сети, не N инцидентов
		return false, a.fpol.endAttemptWithLimit(false, target, candidates)
	}
	a.sm.markSwitched(time.Now()) // C1: новый канал прошёл probe → HEALTHY (§22)
	a.fpol.endAttempt(true, target)
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

// recordCandidateFailure — ФАКТ отказа кандидата, провалившего переключение
// (V2-075, аудит D2). Без него входные данные scoring не меняются, следующий
// инцидент снова берёт первым того же кандидата, сжигает вторую попытку и
// приводит к BLOCKED (fail-closed Stop) при живом соседнем канале.
//
// Класс честный: защищённый путь кандидата не подтвердился probe'ом
// переключения — protected_channel_unavailable, источник srcProbe (сильный
// счётчик D2: quarantine.go ProbeFails > 0 → QUARANTINED). Слабая фоновая
// семантика (observeBg) этим не подменяется.
func (a *App) recordCandidateFailure(channelID string, now time.Time) {
	if channelID == "" {
		return
	}
	chanHealth.observeResult(currentChannelEpoch(), channelID, false, fcChannelUnavail, now)
}
