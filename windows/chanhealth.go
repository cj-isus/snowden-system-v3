package main

// chanhealth.go — ChannelHealth + EvidenceAge (PLAN §4 Phase A0; plan_gpt
// §29/§30/§55). Наблюдаемое здоровье канала ВНУТРИ сетевой эпохи:
//
//   - смена сети (netwatch up/change → onNetEvent) открывает новую эпоху —
//     старые health-факты устаревают целиком (return-путь сменился: канал,
//     живший в E41, ничего не обещает в E42 — доказано V2-026);
//   - probe-исходы сторожа/reprobe (успех/подтверждённый отказ) кормят
//     запись АКТИВНОГО канала текущей эпохи;
//   - EvidenceAge — свежесть: fail-closed гейт читает «здоровье» только
//     если факт не старше evidenceFresh и не из предыдущей эпохи.
//
// Правила честности (AGENTS §3.3/§3.4):
//   - health = история наблюдений, НЕ доказательство текущей доступности
//     (доказывает только живой probe); ChannelMemory-семантика AGENTS §3.3;
//   - секретов нет — только id канала, счётчики и метки времени;
//   - состояние в памяти процесса (как failoverPolicy): персистентность
//     без нормализации/дебаунса порождала бы «знание» из чужой эпохи —
//     сознательно отложено до B2 (envelope-driven).
//
// Все публичные методы потокобезопасны (вызовы из сторожа/старта/биндингов).

import (
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Источники probe-фактов (healthState.LastSrc; честное происхождение
// наблюдения — AGENTS §3.3): активный probe сторожа/старта или фоновая
// дешёвая проверка неактивного канала (D1, bgprobe.go).
const (
	srcProbe      = "probe"      // сторож/reprobe/старт-гейт активного канала
	srcBackground = "background" // фоновый TCP-хендшейк неактивного канала (D1)
)

// evidenceFreshMax — максимальная свежесть probe-факта для «здоров».
// Сторож тикает раз в 30с: живой последний тик = ≤60с; после этого канал
// не «болен», он «без свежего evidence» (честный unknown, не fail).
const evidenceFreshMax = 2 * time.Minute

// healthState — состояние канала внутри текущей эпохи.
type healthState struct {
	OK          int       // успешных probe-исходов в эпохе
	FAIL        int       // подтверждённых отказов в эпохе (серии сторожа/репроб, не мерцания)
	LastOKAt    time.Time // последний успешный probe (RFC3339 в UI)
	LastFailAt  time.Time
	LastClass   string // класс последнего отказа (failclass.go, "" = не было)
	LastSrc     string // источник последнего факта: srcProbe | srcBackground ("" = нет)
	LastOutcome string // ok | fail; порядок событий надёжнее равных wall-clock timestamp
	ProbeFails  int    // отказов, подтверждённых ЗАЩИЩЁННЫМ probe (сильный счётчик D2; фон не увеличивает)
}

// verdict — наблюдаемый итог по записи (единая точка: UI-снимок и C2-scoring
// обязаны читать один и тот же вердикт):
//   - записей нет → "no_data" (честно: канал не проверялся в этой эпохе);
//   - последний факт — подтверждённый отказ → "failing";
//   - иначе, если последний успех свежее evidenceFreshMax → "healthy";
//   - успех был, но протух (или эпоха не открыта) → "no_data" (EvidenceAge:
//     протухший факт — не fail).
func (st *healthState) verdict(now time.Time) string {
	switch {
	case st.FAIL > 0 && (st.LastOutcome == "fail" || st.OK == 0 || st.LastFailAt.After(st.LastOKAt)):
		return "failing"
	case !st.LastOKAt.IsZero() && now.Sub(st.LastOKAt) <= evidenceFreshMax:
		return "healthy"
	default:
		return "no_data"
	}
}

// ChannelHealth — агрегат по всем каналам + номер эпохи, в которой он собран.
type ChannelHealth struct {
	Epoch   int                  `json:"epoch"`   // 0 = факта сети не было
	Started string               `json:"started"` // момент начала эпохи (RFC3339, "" = нет)
	NetCls  string               `json:"netClass"`
	Now     string               `json:"now"`
	Chans   []ChannelHealthEntry `json:"channels"`
} // ChannelHealthEntry — строка по одному каналу (UI: честные «нет данных»).
type ChannelHealthEntry struct {
	ID         string `json:"id"`
	Transport  string `json:"transport"` // из дескриптора (FR-002: данные, не хардкод); "" = не найден
	OK         int    `json:"ok"`
	FAIL       int    `json:"fail"`
	LastOKAt   string `json:"lastOkAt"`
	LastFailAt string `json:"lastFailAt"`
	LastClass  string `json:"lastClass"`
	LastSrc    string `json:"lastSrc"` // srcProbe | srcBackground ("" = не было факта)
	// D2 (V2-064): производное состояние восстановления канала — выводится
	// при снятии снапшота (quarantine.go), нигде не хранится.
	D2State string `json:"d2State"` // HEALTHY | PROBATION | RECOVERING | QUARANTINED
	D2Note  string `json:"d2Note"`  // человекочитаемое основание
	// Verdict — наблюдаемый итог: "no_data" | "healthy" | "failing".
	// healthy ≠ гарантия: это «живой probe был недавно» (EvidenceAge).
	Verdict string `json:"verdict"`
}

// channelHealth — реестр: эпоха → (канал → healthState). Ключ эпохи —
// номер: записи прошлых эпох недоступны для вердиктов (fail-closed к no_data).
// events — кольцо подтверждённых отказов эпохи (D3-кластеризация, V2-065):
// те же события, что кормят FAIL, но с временем каждого (healthState хранит
// только последний); хоронится вместе с эпохой (resetLocked).
type channelHealth struct {
	mu     sync.Mutex
	epoch  int
	items  map[string]*healthState
	events []failEvent
}

var chanHealth = &channelHealth{items: map[string]*healthState{}}

// observeResult — исход probe по каналу. Вызывается:
//   - watchCore/netChangeReprobe: успех тика → ok=true;
//   - watchCore: подтверждённый отказ (после fastReprobe) → ok=false, класс;
//   - startCore/SelectChannel: успешный старт-гейт → ok=true.
//
// Мерцания (восстановившаяся fastReprobe-серия) не кормят FAIL — они
// поглощаются до «подтверждённого отказа» (семантика V2-037).
func (h *channelHealth) observeResult(epochNum int, channelID string, ok bool, failClass string, now time.Time) {
	h.observeSrc(epochNum, channelID, ok, failClass, now, srcProbe)
}

// observeBg — фоновый факт неактивного канала (D1, bgprobe.go): ТОЛЬКО
// подтверждённые отказы (серия 2 подряд — анти-мерцание там же), источник
// srcBackground. Успехи фона фактов не пишут (открытый TCP-порт ≠ защищённый
// путь); данные нужны для ПОРЯДКА кандидатов failover (C2), решения — по-прежнему
// только probe на переключении (V2-037, инвариант не тронут).
func (h *channelHealth) observeBg(epochNum int, channelID, failClass string, now time.Time) {
	h.observeSrc(epochNum, channelID, false, failClass, now, srcBackground)
}

// observeSrc — общий вход записи факта с явным источником (srcProbe/
// srcBackground); вердикт у всех источников общий (единая точка UI/scoring).
// Отказ от защищённого probe дополнительно увеличивает сильный счётчик
// ProbeFails (D2: сильные факты vs слабые фоновые — quarantine.go).
func (h *channelHealth) observeSrc(epochNum int, channelID string, ok bool, failClass string, now time.Time, src string) {
	if channelID == "" {
		return
	}
	_, _, netClass := netEpoch.current()
	factsCache.mu.Lock()
	isp := factsCache.facts.ISP
	factsCache.mu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.epoch != epochNum {
		h.resetLocked(epochNum) // первая запись новой эпохи открывает чистый реестр
	}
	st := h.items[channelID]
	if st == nil {
		st = &healthState{}
		h.items[channelID] = st
	}
	if ok {
		st.OK++
		st.LastOKAt = now
		st.LastOutcome = "ok"
	} else {
		st.FAIL++
		st.LastFailAt = now
		st.LastOutcome = "fail"
		st.LastClass = failClass
		if src == srcProbe {
			st.ProbeFails++ // сильный счётчик D2: только защищённый probe
		}
		h.events = append(h.events, failEvent{ID: channelID, At: now, Class: failClass, Src: src, NetClass: netClass, ISP: isp})
		if len(h.events) > incidentRingCap { // кольцо: новейшие остаются
			h.events = h.events[len(h.events)-incidentRingCap:]
		}
	}
	st.LastSrc = src
}

// failureEvents — КОПИЯ кольца подтверждённых отказов эпохи (для D3-
// кластеризации; эпоха не совпала = событий нет — fail-closed).
func (h *channelHealth) failureEvents(epochNum int) []failEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.epoch != epochNum {
		return nil
	}
	out := make([]failEvent, len(h.events))
	copy(out, h.events)
	return out
}

// stateOf — потокобезопасная КОПИЯ записи канала (ok=false = записи нет;
// честный no_data для scoring, отсутствие данных не выдумывается).
func (h *channelHealth) stateOf(epochNum int, channelID string) (healthState, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.epoch != epochNum {
		return healthState{}, false // факт из другой эпохи недоступен (fail-closed к no_data)
	}
	st, ok := h.items[channelID]
	if !ok {
		return healthState{}, false
	}
	return *st, true
}

// resetLocked — новая эпоха: предыдущие факты устарели (не «переносятся»),
// вместе с ними умирает и кольцо отказов D3.
func (h *channelHealth) resetLocked(epochNum int) {
	h.epoch = epochNum
	h.items = map[string]*healthState{}
	h.events = nil
}

// snapshot — текущий агрегат для UI/гейта (вердикты — healthState.verdict).
func (h *channelHealth) snapshot(now time.Time) ChannelHealth {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := ChannelHealth{Epoch: h.epoch, Chans: []ChannelHealthEntry{}}
	if en, started, cls := netEpoch.current(); en > 0 {
		// Реестр мог не открыться событием эпохи (эпоха есть, наблюдений нет).
		if h.epoch != en {
			out.Epoch = en
		}
		if !started.IsZero() {
			out.Started = started.Format(time.RFC3339)
		}
		out.NetCls = cls
	}
	out.Now = now.Format(time.RFC3339)
	for id, st := range h.items {
		e := ChannelHealthEntry{
			ID: id, OK: st.OK, FAIL: st.FAIL,
			LastOKAt:   st.LastOKAt.Format(time.RFC3339),
			LastFailAt: st.LastFailAt.Format(time.RFC3339),
			LastClass:  st.LastClass,
			LastSrc:    st.LastSrc,
		}
		e.Verdict = st.verdict(now)
		d2 := deriveChannelRecoveryState(id, h.epoch, st, true, now)
		e.D2State = d2.State
		e.D2Note = d2.Note
		out.Chans = append(out.Chans, e)
	}
	return out
}

// currentChannelEpoch — номер эпохи для observeResult (0 = эпоха не открыта:
// записи всё равно лягут в реестр 0 и умрут при первом bump — приемлемо).
func currentChannelEpoch() int {
	n, _, _ := netEpoch.current()
	return n
}

// recordProbeOutcome — мост из lifecycle: исход probe активного канала.
// classForFail вычисляется лениво (failclass.go) только на отказ.
func (a *App) recordProbeOutcome(ok bool, report *core.ProbeReport) {
	if ok || report == nil {
		if ok {
			chanHealth.observeResult(currentChannelEpoch(), a.activeChannelIDForHealth(), true, "", time.Now())
		}
		return
	}
	// Порт канала — основной признак классификатора (V2-068, аудит D1);
	// порт probe-цели — только фолбэк. currentRuntimeConfig() может быть nil
	// (ядро уже остановлено при разборе отчёта) — раньше это был бы nil-deref.
	cc := currentRuntimeConfig()
	channelPort, probePort := 0, 0
	if cc != nil {
		channelPort, probePort = cc.TunnelPort, probeTunnelPort(cc.ProbeTargets)
	}
	fc := ClassifyFailure(report, channelPort, probePort)
	chanHealth.observeResult(currentChannelEpoch(), a.activeChannelIDForHealth(), false, fc.Code, time.Now())
}

// activeChannelIDForHealth — id активного канала ("" = нет данных).
func (a *App) activeChannelIDForHealth() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state.ActiveID
}

// GetChannelHealth — биндинг UI: агрегат по каналам текущей эпохи.
// Строки — по ВСЕМ enabled каналам из дескрипторов (FR-002: карточки из
// данных): канал без записей в этой эпохе = честная строка no_data, а не
// отсутствие строки (отсутствие данных — тоже факт, AGENTS §3.6).
func (a *App) GetChannelHealth() ChannelHealth {
	snap := chanHealth.snapshot(time.Now())
	descs, err := render.LoadDescriptors()
	if err != nil {
		return snap // дескрипторы нечитаемы: отдаём то, что есть (без выдуманных строк)
	}
	byID := make(map[string]ChannelHealthEntry, len(snap.Chans))
	for _, e := range snap.Chans {
		byID[e.ID] = e
	}
	out := make([]ChannelHealthEntry, 0, len(descs))
	for _, ch := range descs {
		if !ch.Enabled {
			continue
		}
		e, ok := byID[ch.ID]
		if !ok {
			e = ChannelHealthEntry{ID: ch.ID, Verdict: "no_data"}
		}
		e.Transport = ch.Protocol + "+" + ch.Transport
		if e.D2State == "" {
			// Канал без записей: D2-состояние выводится из слабых фоновых
			// фактов (D1) — честно QUARANTINED при открытой серии, HEALTHY
			// без фактов против (не «болен», просто не проверялся).
			d2 := deriveChannelRecoveryState(ch.ID, snap.Epoch, nil, false, time.Now())
			e.D2State = d2.State
			e.D2Note = d2.Note
		}
		out = append(out, e)
	}
	snap.Chans = out
	return snap
}

// EpochLabel — короткий идентификатор эпохи для карточек UI.
func (a *App) EpochLabel() string {
	return netEpoch.label()
}

// ChannelIndependenceInfo — пара каналов: число + человекочитаемые общие
// домены (пусто = общих ИЗВЕСТНЫХ доменов нет).
type ChannelIndependenceInfo struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Score      float64 `json:"score"`
	SharedFDs  string  `json:"shared_fdoms"`
	BelowLimit bool    `json:"below_limit"` // score < порога 0.35 (plan_gpt §6)
}

// GetChannelIndependence — матрица независимости текущего набора каналов по
// failure domains (B2, V2-059; plan_gpt §6). Вход — дескрипторы + fd.json;
// выход — детерминированная policy-модель, не физика. UI-контракт по PLAN §4.1.5
// (contract.ts) будет добавлен отдельным шагом C2; биндинг уже честен.
func (a *App) GetChannelIndependence() []ChannelIndependenceInfo {
	chans, err := render.LoadDescriptors()
	if err != nil {
		return nil
	}
	var out []ChannelIndependenceInfo
	for i := 0; i < len(chans); i++ {
		for j := i + 1; j < len(chans); j++ {
			score := render.Independence(chans[i], chans[j])
			out = append(out, ChannelIndependenceInfo{
				A:          chans[i].ID,
				B:          chans[j].ID,
				Score:      score,
				SharedFDs:  render.IndependenceSummary(chans[i], chans[j]),
				BelowLimit: score < render.IndependenceThreshold,
			})
		}
	}
	return out
}
