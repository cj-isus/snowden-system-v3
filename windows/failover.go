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
	"context"
	"errors"
	"fmt"
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
	if p.switches >= failoverMaxSwitches {
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
	runtime_.mu.Lock()
	m := runtime_.manager
	oldCC := runtime_.cc
	runtime_.mu.Unlock()
	if m == nil || m.State() != "running" || oldCC == nil {
		return fmt.Errorf("переключение канала: VPN не запущен")
	}
	cc, err := a.renderAndPrepare(channelID)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.state.State = "reloading"
	a.pushStateLocked()
	a.mu.Unlock()

	// Уровень 1: бесшовное переключение селектором. tag канала в живом
	// конфиге == его ID (рендер строит теги из ID).
	seamlessErr := m.SwitchChannel(context.Background(), "proxy", channelID,
		newAppProbe(cc), newAppProbe(oldCC))
	switch {
	case seamlessErr == nil:
		// success ниже.
	case errors.Is(seamlessErr, core.ErrSwitchRolledBack):
		// Кандидат не прошёл probe, прежний канал жив и подтверждён:
		// туннель НЕ роняем (аудит V2-037: старое поведение рвало рабочий
		// туннель при неудачном переключении на мёртвый канал).
		a.mu.Lock()
		a.state.State = "running"
		a.pushStateLocked()
		a.mu.Unlock()
		a.appendLog("warn", "переключение на "+channelID+" не прошло probe — туннель жив на прежнем канале: "+seamlessErr.Error())
		return seamlessErr
	case errors.Is(seamlessErr, core.ErrSelectorSwitchUnavailable):
		// Уровень 2: структурный fallback через Reload.
		if err := m.Reload(context.Background(), cc.Raw, core.WithReloadProbe(newAppProbe(cc))); err != nil {
			a.failCore("переключение на "+channelID, err)
			return err
		}
	default:
		// Fail-closed внутри SwitchChannel (оба канала мертвы): движок уже
		// остановлен менеджером — честный error.
		a.mu.Lock()
		a.state.State = "error"
		a.state.Error = "переключение на " + channelID + ": " + seamlessErr.Error()
		a.pushStateLocked()
		a.mu.Unlock()
		a.appendLog("error", a.state.Error)
		return seamlessErr
	}

	runtime_.mu.Lock()
	runtime_.cc = cc
	runtime_.mu.Unlock()
	a.mu.Lock()
	a.state.ActiveID = channelID
	a.state.Active = channelViewOf(cc, channelID)
	a.state.State = "running"
	a.pushStateLocked()
	a.mu.Unlock()
	a.appendLog("info", "канал переключён: "+channelID)
	a.pushState()
	return nil
}

// tryFailover — одна автоматическая попытка: candidates → первый, чей тег
// реально в живом селекторе; переключение через switchChannelCore (бесшовно,
// с fallback). Итог: (переключились, BLOCKED-все-исчерпаны).
func (a *App) tryFailover(descriptors []render.ChannelDescriptor, inSelector map[string]bool, currentID string) (switched, blocked bool) {
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
	var target string
	for _, cand := range candidates {
		if chID, ok := cc.ChannelIDs[cand]; ok && chID != "" {
			target = cand
			break
		}
	}
	if target == "" {
		a.appendLog("warn", "failover: кандидат(ы) вне живого селектора — автопереключение невозможно, BLOCKED")
		return false, true
	}
	if !a.fpol.beginAttemptIfDue(time.Now()) {
		return false, false // попытка уже идёт или cooldown
	}
	a.appendLog("warn", "failover: переключение "+currentID+" → "+target)
	if err := a.switchChannelCore(target); err != nil {
		a.appendLog("warn", "failover: "+err.Error())
		return false, a.fpol.endAttempt(false, target)
	}
	a.fpol.endAttempt(true, target)
	a.appendLog("info", "failover: канал переключён на "+target)
	return true, false
}
