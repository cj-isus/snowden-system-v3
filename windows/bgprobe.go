package main

// bgprobe.go — фоновые дешёвые probe неактивных каналов (PLAN Phase D1;
// plan_gpt §41 «бюджет: не чаще сторожа»). Зачем: после fail-closed/BLOCKED
// реестр здоровья кандидатов пуст до их реального использования (осознанный
// дефолт V2-061) — C2-score тогда определяется только независимостью, и
// failover «вслепую» перебирает кандидатов, среди которых могут быть давно
// мёртвые. Фоновый TCP-хендшейк к origin:port даёт бюджетную картину
// «скорее жив / скорее мёртв» заранее.
//
// Честные границы (AGENTS §2.1/§3.3):
//   - probe идёт ВНЕ туннеля (direct TCP к origin:port из дескриптора) — это
//     бюджетная проверка доступности origin, НЕ защищённый путь. Успех фона
//     фактов в health НЕ пишет (открытый порт ≠ живой защищённый канал);
//   - отказ подтверждённой серией (2 подряд — анти-мерцание V2-037) пишет
//     FAIL-факт с LastSrc=background: данные для ПОРЯДКА кандидатов (C2,
//     V2-061), не для решений. Решение по-прежнему принимает только probe
//     на переключении (failoverPolicy → Manager, инвариант V2-037 не тронут);
//   - QUIC (HY2) не пробуется: анти-probe транспорт МОЛЧИТ на мусорные UDP —
//     «тишина» никогда не доказательство (урок V2-042); канал честно
//     пропускается, бюджет отмечается;
//   - только enabled live-verified НЕактивные каналы (configured не
//     провоцируем; активный канал проверяет сторож защищённым probe);
//   - бюджет: канал не чаще bgChanMinInterval; отметки и серии скоупятся
//     сетевой эпохой (факт старой эпохи в новую не попадает — fail-closed);
//   - TUN-режим: физическая сеть не видна (V2-054a) — цикл молчит, действует
//     сторож;
//   - секретов нет: dial по origin:port дескриптора, ничего больше.

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// bgChanMinInterval — минимальный интервал между фоновыми проверками ОДНОГО
// канала (бюджет plan_gpt §41: реже сторожа, который тикает раз в 30с).
const bgChanMinInterval = 10 * time.Minute

// bgDialTimeout — бюджет одного dial; таймаут трактуем как port_blackholed
// (порт-эвристика THEORY §2.4a: тишина на порту ≠ «сервер умер»).
const bgDialTimeout = 5 * time.Second

// bgStartDelay — пауза перед первым раундом после старта приложения:
// стартовая волна (startCore + probe + сторож) важнее, сеть не нагружаем.
const bgStartDelay = 2 * time.Minute

// bgRoundInterval — период пробуждения цикла (выбор созревших таргетов).
const bgRoundInterval = time.Minute

// reBgRefused — форма активного отказа соединения. Шире словарной
// reConnRefused: Windows-диалер пишет «actively refused it» без слова
// «connection» (форма та же — цель отвергла, AGENTS §3.3 ServerUnreachable).
var reBgRefused = regexp.MustCompile(`refused`)

// bgTarget — один фоновый probe: TCP-хендшейк к origin:port канала.
type bgTarget struct {
	ChannelID string
	Host      string
	Port      string
	QUIC      bool // анти-probe транспорт: dial не выполняется (V2-042), только бюджет
}

// bgRegistry — бюджет проверок (эпоха → канал → отметка) и серии отказов.
// last скоупится эпохой, как chanHealth: отметка старой эпохи недоступна.
// lastOK — последний фоновый ответ канала (слабое доказательство живости
// для D2-recovery: открытый порт ≠ защищённый путь, но «цель отвечает»).
type bgRegistry struct {
	mu          sync.Mutex
	last        map[int]map[string]time.Time // бюджет: последняя проверка
	lastOK      map[int]map[string]time.Time // последний ответ (слабое доказательство живости)
	fails       map[int]map[string]int       // подтверждённых отказов в эпохе (D2-история)
	lastFail    map[int]map[string]time.Time // последний подтверждённый отказ
	consec      map[string]int               // id → серия отказов подряд (анти-мерцание)
	consecEpoch int                          // серия не переходит сетевую эпоху
	started     bool
	stop        chan struct{}
	stopOnce    sync.Once
}

var bgReg = &bgRegistry{
	last:     map[int]map[string]time.Time{},
	lastOK:   map[int]map[string]time.Time{},
	fails:    map[int]map[string]int{},
	lastFail: map[int]map[string]time.Time{},
	consec:   map[string]int{},
}

// bgMarkDone — отметить проверку канала в ЕГО эпоху (dial шёл в epoch).
func bgMarkDone(epoch int, id string, now time.Time) {
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	m, ok := bgReg.last[epoch]
	if !ok {
		m = map[string]time.Time{}
		bgReg.last[epoch] = m
	}
	m[id] = now
}

// bgLastFor — отметка канала в эпохе (zero = в этой эпохе не проверялся).
func bgLastFor(epoch int, id string) time.Time {
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	return bgReg.last[epoch][id]
}

// nextBgAt — момент созревания следующей проверки (zero last = можно сейчас).
func nextBgAt(last time.Time) time.Time {
	if last.IsZero() {
		return time.Time{}
	}
	return last.Add(bgChanMinInterval)
}

// confirmFail — инкремент серии отказов канала; true = серия подтверждена
// (второй подряд) — можно писать факт. Подтверждённый отказ также входит
// в D2-историю канала (fails/lastFail — слабые факты для quarantine.go).
// Серия не переживает смену эпохи: половина серии из старой сети не
// подтверждает отказ в новой.
func (r *bgRegistry) confirmFail(epoch int, id string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.consecEpoch != epoch {
		r.consec = map[string]int{}
		r.consecEpoch = epoch
	}
	r.consec[id]++
	if r.consec[id] < 2 {
		return false
	}
	m, ok := r.fails[epoch]
	if !ok {
		m = map[string]int{}
		r.fails[epoch] = m
	}
	m[id]++
	fm, ok := r.lastFail[epoch]
	if !ok {
		fm = map[string]time.Time{}
		r.lastFail[epoch] = fm
	}
	fm[id] = now
	return true
}

// resetConsec — успех любого фонового dial'а гасит ВСЕ серии: сеть только
// что доказала, что TCP-хендшейки проходят, поэтому неподтверждённые отказы
// могли быть transient-флапом локального стека — подтверждение (2 подряд)
// начинается заново. Одиночный отказ снова не подтверждён (семантика V2-037:
// мерцание не пишет факты); уже записанный FAIL-факт не отзывается.
func (r *bgRegistry) resetConsec(epoch int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consec = map[string]int{}
	r.consecEpoch = epoch
}

// bgTargets — таргеты раунда: enabled live-verified НЕактивные каналы из
// дескрипторов (FR-002: данные, не хардкод). QUIC-каналы включаются с флагом:
// бюджет отмечается, dial не выполняется (анти-probe молчит — V2-042).
func bgTargets(descs []render.ChannelDescriptor, activeID string) []bgTarget {
	out := make([]bgTarget, 0, len(descs))
	for _, ch := range descs {
		if !ch.Enabled || ch.ValidationStatus != "live-verified" || ch.ID == activeID {
			continue
		}
		out = append(out, bgTarget{
			ChannelID: ch.ID,
			Host:      ch.OriginServer,
			Port:      strconv.FormatUint(uint64(ch.Port), 10),
			QUIC:      ch.Protocol == "hysteria2" || ch.Transport == "quic",
		})
	}
	return out
}

// classifyBgError — класс отказа dial по словарю failclass. Формы строк
// диалера ОС, не отчёта probe: refused (включая Windows «actively refused»)
// = server_unreachable (цель отвергла — не сеть, AGENTS §3.3); таймаут
// (строкой или net.Error.Timeout) = port_blackholed (тишина на порту);
// прочее — честный unknown.
func classifyBgError(err error) string {
	if err == nil {
		return ""
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return fcPortBlackholed
	}
	d := strings.ToLower(err.Error())
	switch {
	case reBgRefused.MatchString(d):
		return fcServerUnreach
	case reTimeout.MatchString(d):
		return fcPortBlackholed
	default:
		return fcUnknown
	}
}

// bgProbeChannel — одна дешёвая проверка канала. Успех: серия гасится,
// бюджет отмечается, фактов в health НЕТ (TCP-хендшейк ≠ защищённый путь).
// Отказ: класс из словаря; подтверждённая серия (2-й подряд) пишет
// FAIL-факт LastSrc=background через chanHealth.observeBg.
func (a *App) bgProbeChannel(t *bgTarget) {
	epoch := currentChannelEpoch()
	defer bgMarkDone(epoch, t.ChannelID, time.Now())
	if t.QUIC {
		// HY2 молчит на мусорные UDP: пробы нет, бюджет есть (V2-042).
		return
	}
	addr := net.JoinHostPort(t.Host, t.Port)
	conn, err := net.DialTimeout("tcp", addr, bgDialTimeout)
	if err == nil {
		conn.Close()
		bgReg.resetConsec(epoch)
		bgReg.markOK(epoch, t.ChannelID, time.Now())
		return
	}
	cls := classifyBgError(err)
	if bgReg.confirmFail(epoch, t.ChannelID, time.Now()) {
		chanHealth.observeBg(epoch, t.ChannelID, cls, time.Now())
		a.appendLog("warn", "фон: "+t.ChannelID+" не отвечает ("+addr+"): "+cls+
			" — фоновый факт для порядка кандидатов, не решение (решает probe на переключении)")
	}
}

// bgStopCh — стоп-канал цикла (nil в тестах: select с default безопасен).
func (a *App) bgStopCh() <-chan struct{} {
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	return bgReg.stop
}

// bgProbeLoop — цикл фоновых проверок: после стартовой паузы каждые
// bgRoundInterval выбираются созревшие таргеты и probe'ятся по одному.
// Стоп-канал проверяется до раунда и между dial'ами — горутина не переживает
// остановку приложения даже до истечения bgStartDelay.
// TUN-режим: физическая сеть не видна (V2-054a) — цикл спит, сторожа достаточно.
func (a *App) bgProbeLoop(stop <-chan struct{}) {
	select {
	case <-stop:
		return
	case <-time.After(bgStartDelay):
	}
	for {
		select {
		case <-stop:
			return
		default:
		}
		if tunRequested() {
			select {
			case <-stop:
				return
			case <-time.After(bgRoundInterval):
			}
			continue
		}
		if err := a.runBgRound(); err != nil {
			a.appendLog("warn", "фон: раунд пропущен: "+err.Error())
		}
		select {
		case <-stop:
			return
		case <-time.After(bgRoundInterval):
		}
	}
}

// runBgRound — один раунд: созревшие по бюджету таргеты probe'ятся подряд.
// Дескрипторы нечитаемы → ошибка (раунд честно пропущен, не «нет каналов»).
func (a *App) runBgRound() error {
	a.mu.Lock()
	active := a.state.ActiveID
	a.mu.Unlock()
	descs, err := bgDescriptorsAt()
	if err != nil {
		return err
	}
	due := bgDueTargets(bgTargets(descs, active))
	for i := range due {
		select {
		case <-a.bgStopCh():
			return nil
		default:
		}
		a.bgProbeChannel(&due[i])
	}
	return nil
}

// bgDueTargets — фильтр бюджета: непроверенный в текущей эпохе канал
// созревает сразу, проверенный — через bgChanMinInterval.
func bgDueTargets(targets []bgTarget) []bgTarget {
	now := time.Now()
	epoch := currentChannelEpoch()
	out := make([]bgTarget, 0, len(targets))
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	for _, tg := range targets {
		if nextBgAt(bgReg.last[epoch][tg.ChannelID]).Before(now) {
			out = append(out, tg)
		}
	}
	return out
}

// bgDescriptorsAt — подменяемая точка загрузки дескрипторов (герметичность
// тестов от AppData/репозитория; по практике V2-037).
var bgDescriptorsAt = render.LoadDescriptors

// startBgProbes — запустить фоновый цикл ровно один раз (из app.startup).
func (a *App) startBgProbes() {
	bgReg.mu.Lock()
	defer bgReg.mu.Unlock()
	if bgReg.started {
		return
	}
	bgReg.started = true
	bgReg.stop = make(chan struct{})
	go a.bgProbeLoop(bgReg.stop)
}

// stopBgProbes — остановить фоновый цикл (закрытие окна). Повтор безопасен.
func (a *App) stopBgProbes() {
	bgReg.mu.Lock()
	ch := bgReg.stop
	bgReg.mu.Unlock()
	if ch != nil {
		bgReg.stopOnce.Do(func() { close(ch) })
	}
}

// markOK — фоновый ответ канала (в ЕГО эпоху) — слабое доказательство живости
// для D2-recovery (quarantine.go).
func (r *bgRegistry) markOK(epoch int, id string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.lastOK[epoch]
	if !ok {
		m = map[string]time.Time{}
		r.lastOK[epoch] = m
	}
	m[id] = now
}

// bgFacts — слабые факты фоновой проверки одного канала в эпохе (для
// D2-recovery; без секретов: счётчики и метки времени).
type bgFacts struct {
	Fails    int       // подтверждённых фоновых отказов в эпохе (D2-история)
	Consec   int       // ТЕКУЩАЯ открытая серия отказов (0 = серия закрыта/нет)
	LastAt   time.Time // последний подтверждённый фоновый отказ
	LastOKAt time.Time // последний фоновый ответ (zero = в этой эпохе не отвечал)
}

// factsOf — копия слабых фактов канала в эпохе (zero = фоновых фактов нет).
func (r *bgRegistry) factsOf(epoch int, id string) bgFacts {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := bgFacts{}
	if r.consecEpoch == epoch {
		out.Consec = r.consec[id]
	}
	out.Fails = r.fails[epoch][id]
	out.LastAt = r.lastFail[epoch][id]
	out.LastOKAt = r.lastOK[epoch][id]
	return out
}

// bgResetForTest — только тесты: чистый бюджет/серии/эпохи/health.
func bgResetForTest() {
	bgReg.mu.Lock()
	bgReg.last = map[int]map[string]time.Time{}
	bgReg.lastOK = map[int]map[string]time.Time{}
	bgReg.fails = map[int]map[string]int{}
	bgReg.lastFail = map[int]map[string]time.Time{}
	bgReg.consec = map[string]int{}
	bgReg.consecEpoch = 0
	bgReg.started = false
	bgReg.stop = nil
	bgReg.stopOnce = sync.Once{}
	bgReg.mu.Unlock()
	netEpoch.resetForTest()
	chanHealth.mu.Lock()
	chanHealth.resetLocked(0)
	chanHealth.mu.Unlock()
}
