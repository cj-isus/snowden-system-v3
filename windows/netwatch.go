package main

// netwatch.go — наблюдение за сменой сети (V2-054).
//
// Проблема: Wi-Fi⇄LTE-переходы и волны деградации мобильного CGNAT ломают
// ЗАЩИЩЁННЫЙ ПУТЬ молча (V2-026: return-путь релеированных потоков тонет при
// живом TLS к CF edge). Реакции сегодня две, обе медленные: сторож тикает
// раз в 30с, авто-ретрай после fail-closed ждёт backoff 30с→10м независимо
// от того, что сеть уже сменилась (смена сети = смена return-пути = смена
// шансов). Между тиками владелец сидит на мёртвом туннеле или без VPN,
// хотя путь мог восстановиться через секунды после смены сети.
//
// Механика: горутина опрашивает fingerprint маршрута по умолчанию каждые
// netwatchPoll и доставляет СОБЫТИЯ (up/down/change) по одному на переход:
//   - up/change: туннель работает — сеть под ним сменилась, старый путь
//     мог умереть → внеочередной probe сейчас, не через 30с (probe не
//     прошёл → та же семантика сторожа: fastReprobe → tryFailover →
//     BLOCKED); fail-closed (BLOCKED/error) — немедленный авто-ретрай
//     (wakeFailClosedRetry, V2-054: begin-гвард, FR-001 не ослаблен).
//   - down: выключение Wi-Fi/выдёргивание кабеля. Туннель не трогаем:
//     dying-gasp ничего не чинит, а Stop рвёт пользовательские сессии —
//     при возврате сети up/change отработают. Только журнал-факт.
//
// Fingerprint — БЕЗ PowerShell (2с-поллинг с спавном процесса = самоддо
// CPU): UDP-connect на IP-литерал 9.9.9.9:53 НЕ отправляет пакетов, но
// выполняет route-lookup — LocalAddr = IPv4 интерфейса, которым ОС идёт
// в дефолт. Интерфейс и имя — из net.Interfaces(). Стоимость — микросекунды.
//
// Границы (честно):
//   - TUN-режим: весь трафик (и наш connect) маршрутизируется в snowden0 —
//     физическую смену сети fingerprint не видит. Наблюдатель в TUN ОТКЛЮЧЁН
//     (tunRequested-гвард), там действует 30с-стороже;
//   - событие дебаунсится netwatchDebounce: flapping Wi-Fi⇄LTE даёт одно
//     событие (последний переход), с потолком ожидания;
//   - ошибка сбора = «сети нет»; переходы считаются только между двумя
//     УСПЕШНЫМИ сборками (шум сборщика не генерирует ложный down).

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// netwatchPoll / netwatchDebounce — период опроса и пауза после последнего
// перехода перед доставкой (поглощает flapping Wi-Fi⇄LTE). Переменные (не
// const): юнит-тесты сжимают тайминги. Потолок ожидания — netwatchDebounceCap.
var (
	netwatchPoll        = 2 * time.Second
	netwatchDebounce    = 5 * time.Second
	netwatchDebounceCap = 35 * time.Second
)

// tunAdapterName — имя TUN-адаптера из рендера (WithTUN interface_name).
// Fingerprint этого адаптера = «сигнала нет» (см. шапку).
const tunAdapterName = "snowden0"

// netFingerprint — идентичность сети по факту ОС: индекс интерфейса
// маршрута по умолчанию, его IPv4 и имя адаптера. Нулевое значение =
// сети нет / факт недоступен / TUN-маршрут (неразличимо и не важно:
// переходы считаются только между двумя успешными «физическими» фактами).
type netFingerprint struct {
	ifaceIndex int
	ip         string
	alias      string
}

func (f netFingerprint) present() bool { return f.ifaceIndex != 0 || f.ip != "" }

func (f netFingerprint) equal(o netFingerprint) bool {
	return f.ifaceIndex == o.ifaceIndex && f.ip == o.ip && f.alias == o.alias
}

// classifyTransition — класс перехода между двумя успешными фактами сети
// (чистая функция для юнит-тестов): "" = нет перехода, "up" = сеть появилась,
// "down" = пропала, "change" = сменилась.
func classifyTransition(prev, fp netFingerprint) string {
	switch {
	case fp.present() && !prev.present():
		return "up"
	case !fp.present() && prev.present():
		return "down"
	case fp.present() && prev.present() && !fp.equal(prev):
		return "change"
	default:
		return ""
	}
}

// netClassOf — класс сети адаптера из факта ОС: «mobile» для
// WWAN (cellular = нестабильный return-путь CGNAT, V2-026), «wifi» для
// 802.11, «ethernet» для 802.3, иначе «». Сборщик факта — мгновенный
// системный вызов MIB (адаптерNetClassByIndex, infobindings_windows.go),
// НЕ PowerShell: fire() живёт на цикле опроса наблюдателя — процесс-спавн
// (0.5–2с) на каждый переход остановил бы поллинг (поймано тайминг-тестами).
func netClassOf(fp netFingerprint) string { return adapterNetClassByIndex(fp.ifaceIndex) }

// IANA ifType (MibIfRow2.Type) и NDIS media (MibIfRow2.MediaType) — значения
// из ifdef.h/netioapi.h. Порядок правил: Type надёжнее (для Wi-Fi=71 и
// WWAN=243/244 стабилен), MediaType — фолбэк для Type=other (виртуальные и
// прочие адаптеры, где вендор отдаёт корректный media).
const (
	ifTypeEthernetCsmacd   = 6
	ifTypeSoftwareLoopback = 24
	ifTypeTunnel           = 131
	ifTypeIeee80211        = 71
	ifTypeWwanpp           = 243
	ifTypeWwanpp2          = 244
	ndisMedium802_3        = 0
	ndisMediumWirelessWan  = 9
	ndisMediumNative802_11 = 16
)

// classifyNetClass — единая точка классификации адаптера (общая с
// transportpref.go через netClassOf/currentNetClass). Чистая функция —
// покрыта юнит-тестами без сети и без ОС.
func classifyNetClass(ifType, mediaType uint32) string {
	switch ifType {
	case ifTypeIeee80211:
		return "wifi"
	case ifTypeEthernetCsmacd:
		return "ethernet"
	case ifTypeWwanpp, ifTypeWwanpp2:
		return "mobile"
	}
	switch mediaType {
	case ndisMediumNative802_11:
		return "wifi"
	case ndisMediumWirelessWan:
		return "mobile"
	case ndisMedium802_3:
		// ifType=other + media 802.3: физический Ethernet — но не для
		// loopback/tunnel-адаптеров (TUN других VPN и wintun отдают media
		// 802.3/loopback; классифицировать их «ethernet» = ложный факт).
		switch ifType {
		case ifTypeSoftwareLoopback, ifTypeTunnel:
			return ""
		}
		return "ethernet"
	}
	return ""
}

// netWatcher — состояние наблюдателя. w.last/w.lastAt под mu; pending-поля
// живут только в горутине run (блокировок не требуют). Горутина одна на
// процесс (startNetWatch guard'ится started).
type netWatcher struct {
	mu       sync.Mutex
	started  bool
	stop     chan struct{}
	stopOnce sync.Once
	last     netFingerprint // последний непустой «физический» факт
	lastAt   time.Time      // момент последнего перехода (для журнала/тестов)
	app      *App
}

// netWatch — единственный экземпляр на приложение.
var netWatch = &netWatcher{}

// startNetWatch — запустить наблюдателя ровно один раз (из app.startup).
func (a *App) startNetWatch() {
	netWatch.mu.Lock()
	if netWatch.started {
		netWatch.mu.Unlock()
		return
	}
	netWatch.started = true
	netWatch.stop = make(chan struct{})
	netWatch.app = a
	netWatch.mu.Unlock()
	go netWatch.run(netWatch.stop)
}

// stopNetWatch — остановить горутину наблюдателя (закрытие окна, перезапуск
// режима). Повторный вызов безопасен (sync.Once).
func (a *App) stopNetWatch() {
	netWatch.mu.Lock()
	ch := netWatch.stop
	netWatch.mu.Unlock()
	if ch != nil {
		netWatch.stopOnce.Do(func() { close(ch) })
	}
}

// run — цикл наблюдателя. Доставка НЕ блокирует опрос: переход создаёт
// отложенное событие (pending + fireAt); каждый новый переход перезаписывает
// pending и отодвигает fireAt (коалесинг flapping); при выходе за потолок —
// доставляем как есть. Доставка происходит на очередном тике — задержка
// ≤ debounce+poll, для реакции «вне очереди» это несущественно.
func (w *netWatcher) run(stop <-chan struct{}) {
	ticker := time.NewTicker(netwatchPoll)
	defer ticker.Stop()
	var (
		pendingKind string
		pendingFP   netFingerprint
		pendingAt   time.Time // момент первого отложенного перехода (потолок)
		fireAt      time.Time
	)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		// TUN-режим: физическую сеть не видно (весь трафик в snowden0) —
		// наблюдатель молчит, действует 30с-стороже.
		if tunRequested() {
			continue
		}
		fp := currentNetFingerprint()
		w.mu.Lock()
		prev := w.last
		kind := classifyTransition(prev, fp)
		if kind == "down" {
			// Сеть пропала: факт обнуляется, иначе возврат в ТУ ЖЕ сеть
			// не будет отличён от «всё ещё старая» и up не сработает.
			w.last = netFingerprint{}
		}
		if kind != "" {
			if fp.present() {
				w.last = fp
			}
			w.lastAt = time.Now()
			if pendingKind == "" {
				pendingAt = time.Now()
			}
			pendingKind, pendingFP = kind, fp
			fireAt = time.Now().Add(netwatchDebounce)
		}
		w.mu.Unlock()

		if pendingKind != "" && (time.Now().After(fireAt) || time.Since(pendingAt) > netwatchDebounceCap) {
			kind2, fp2 := pendingKind, pendingFP
			pendingKind, fireAt, pendingAt = "", time.Time{}, time.Time{}
			w.fire(kind2, fp2)
		}
	}
}

// fire — фактическая доставка: журнал-факт + реакция. Тесты зовут напрямую
// с app=nil (тишина).
func (w *netWatcher) fire(kind string, fp netFingerprint) {
	w.mu.Lock()
	cls := netClassOf(w.last)
	app := w.app
	w.mu.Unlock()
	// V2-057 (A0): up/change открывают новую сетевую эпоху (health-факты
	// прошлой эпохи устаревают); down не бампит (сети нет — не смена). До
	// ветвления app: событие должно бампнуть эпоху даже без живого App.
	onNetEvent(kind, cls, time.Now())
	if app == nil {
		return
	}
	switch kind {
	case "down":
		app.appendLog("warn", "сеть: подключение потеряно — наблюдаю за возвратом (туннель не трогаю)")
	case "up":
		app.appendLog("info", "сеть: подключение появилось ("+classLabel(cls)+") — проверка защищённого пути вне очереди")
		go app.onNetPathChanged()
	case "change":
		app.appendLog("info", "сеть: смена подключения → "+classLabel(cls)+" — проверка защищённого пути вне очереди")
		go app.onNetPathChanged()
	}
}

// classLabel — человекочитаемый класс для журнала (факт, не приукрашивание).
func classLabel(cls string) string {
	switch cls {
	case "mobile":
		return "мобильная сеть"
	case "wifi":
		return "Wi-Fi"
	case "ethernet":
		return "Ethernet"
	default:
		return "класс сети неизвестен"
	}
}

// ---------- реакции приложения -------------------------------------------------

// onNetPathChanged — реакция на смену сети: живой туннель — внеочередной
// probe; fail-closed — немедленный авто-ретрай. Повторный вызов при идущем
// старте безопасен (startBusy/begin-гварды). Точки подмены для тестов:
// netWatchProbeStart/netWatchWakeRetry.
var netWatchProbeStart = func(a *App) { go a.netChangeReprobe() }
var netWatchWakeRetry = func(a *App) { a.wakeFailClosedRetry() }

func (a *App) onNetPathChanged() {
	if a.closing.Load() {
		return
	}
	runtime_.mu.Lock()
	running := runtime_.manager != nil && runtime_.cc != nil
	busy := runtime_.startBusy
	runtime_.mu.Unlock()

	if running && !busy {
		// Туннель жив, но сеть под ним сменилась: старый путь мог умереть
		// (V2-026), сторож узнал бы об этом только через ≤30с.
		netWatchProbeStart(a)
		return
	}
	if running || busy {
		return // старт уже идёт — probe будет в его составе
	}
	// Fail-closed (error/BLOCKED): смена сети = смена шансов; ждать остаток
	// backoff на старой сети бессмысленно. Немедленная попытка (то же
	// расписание, тот же счётчик эпизода — exactly-once begin-гвардом).
	a.mu.Lock()
	fc := a.failClosedState()
	a.mu.Unlock()
	if !fc {
		return
	}
	netWatchWakeRetry(a)
}

// netChangeReprobe — внеочередная проверка живого туннеля после смены сети.
// Один probe сразу; при отказе — та же серия fastReprobe, что у сторожа
// (мерцание не роняет туннель); подтверждённая смерть — немедленный
// tryFailover (кандидаты/cooldown/лимиты — тот же код, что у сторожа) и
// при исчерпании — BLOCKED + авто-ретрай, как в watchCore.
func (a *App) netChangeReprobe() {
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	if !a.sessionCurrent(cc) {
		return
	}
	probeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	report := a.protectedProbe(cc).Run(probeCtx)
	cancel()
	stillOurs := a.sessionCurrent(cc)
	if !stillOurs {
		return
	}
	if report.LeakConfirmed {
		a.blockUnsafePath(cc, "сеть: обнаружена утечка защищённого пути", report, fcLeakConfirmed)
		return
	}
	if report.AllPassed {
		// V2-077 (Этап 8): PASS текущего protected probe — обязательный гейт
		// для переоценки transport preference. Без этого смена класса сети не
		// имеет права трогать работающий туннель: предпочтение — policy hint,
		// не доказательство доступности нового канала.
		a.appendLog("info", "сеть: защищённый путь жив на новой сети — probe-гейт preference PASS")
		a.recordProbeOutcome(true, report) // V2-057 (A0): health новой эпохи
		a.mu.Lock()
		a.state.Probe = toView(report)
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
		a.pushStateLocked()
		a.mu.Unlock()
		a.fpol.hardReset()
		a.sm.markTick(true, "", time.Now()) // C1 (V2-060): путь жив на новой сети → HEALTHY
		if err := a.renewKillSwitchIfTUN(); err != nil {
			a.blockUnsafePath(cc, "WFP: policy не подтверждена после смены сети", report, fcUnknown)
			return
		}
		a.tryPreferredChannelOnNetworkChange(cc)
		return
	}
	a.appendLog("warn", "сеть: туннель не отвечает на новой сети — подтверждаю серией быстрых репроб: "+report.Summary())
	a.sm.markTick(false, report.Summary(), time.Now()) // C1: одиночный провал на новой сети = SUSPECTED (§23)
	recovered, lastReport := a.fastReprobe(context.Background(), cc)
	if !a.sessionCurrent(cc) {
		return
	}
	if lastReport != nil {
		report = lastReport
	}
	if report.LeakConfirmed {
		a.blockUnsafePath(cc, "сеть: обнаружена утечка защищённого пути", report, fcLeakConfirmed)
		return
	}
	if recovered {
		a.appendLog("info", "сеть: путь восстановился (мерцание) — туннель не тронут")
		a.fpol.hardReset()
		a.sm.markRecovered(time.Now()) // C1: серия прервалась успехом → HEALTHY
		a.recordProbeOutcome(true, report)
		a.mu.Lock()
		a.state.Probe = toView(report)
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
		a.pushStateLocked()
		a.mu.Unlock()
		if err := a.renewKillSwitchIfTUN(); err != nil {
			a.blockUnsafePath(cc, "WFP: policy не подтверждена после восстановления сети", report, fcUnknown)
			return
		}
		a.tryPreferredChannelOnNetworkChange(cc)
		return
	}
	summary := ""
	if lastReport != nil {
		summary = lastReport.Summary()
	}
	a.appendLog("error", "сеть: защищённый путь мёртв после смены сети — немедленный failover: "+summary)
	// Немедленный переход к семантике сторожа, без ожидания тика.
	inSelector := map[string]bool{}
	for _, tag := range cc.SelectorTags {
		inSelector[tag] = true
	}
	channels, cerr := render.LoadDescriptors()
	if cerr != nil {
		channels = nil
	}
	fc := ClassifyFailure(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets))
	a.sm.markClassified(fc.Code, report.Summary(), time.Now())
	a.appendLog("error", "сеть: "+classifyFailureLogLine(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets)))
	a.sm.markDegraded(time.Now()) // C1: классификация подтверждена → DEGRADED
	if fc.Code == fcEgressDivergence {
		a.blockUnsafePath(cc, "сеть: egress защищённого пути не совпадает с ожидаемым", report, fc.Code)
		return
	}
	if fc.Code == fcWhitelistMode {
		a.blockUnsafePath(cc, "сеть: признаки allowlist-ограничения; диагноз требует дополнительной проверки", report, fc.Code)
		return
	}
	chanHealth.observeResult(currentChannelEpoch(), a.activeChannelIDForHealth(), false, fc.Code, time.Now())
	switched, blocked := a.tryFailoverForClass(channels, inSelector, cc.SelectorDefault, fc.Code, cc)
	if switched {
		return
	}
	if !blocked {
		runtime_.mu.Lock()
		m := runtime_.manager
		runtime_.mu.Unlock()
		if m != nil && m.State() == "running" {
			return
		}
		// A failed switch may have stopped both paths before the attempt budget
		// is exhausted. Recovery still needs a timer.
		a.failSession(cc, report, "сеть: защищённый путь остановлен при переключении — ожидается автоматическое восстановление", false)
		return
	}
	// Кандидатов нет/исчерпаны: fail-closed как у сторожа (BLOCKED + авторетрай).
	a.failSession(cc, lastReport, "сеть: защищённый путь недоступен на новой сети (все каналы, BLOCKED)", true)
}

// tryPreferredChannelOnNetworkChange — мягкая переоценка предпочтения после
// успешного protected probe на НОВОЙ сети (V2-077, аудит D10).
//
// Важная семантика:
//  1. этот метод вызывается только после report.AllPassed; при FAIL старый
//     путь идёт по обычной fastReprobe/failover-ветке выше;
//  2. новый transport preference — только кандидат, не решение: switchChannelCore
//     проходит через Manager и обязательный probe нового канала, с rollback на
//     старый живой канал при ошибке;
//  3. отсутствие класса/дескрипторов/предпочтения = no-op, не ошибка и не
//     direct fallback; ручной SelectChannel не переопределяется.
func (a *App) tryPreferredChannelOnNetworkChange(current *render.ClientConfig) {
	if current == nil || a.closing.Load() {
		return
	}
	netClass := currentNetClass()
	if netClass == "" {
		a.appendLog("info", "сеть: класс не определён — transport preference не применяется")
		return
	}
	descs, err := render.LoadDescriptors()
	if err != nil {
		a.appendLog("warn", "сеть: дескрипторы preference недоступны — текущий канал не тронут: "+err.Error())
		return
	}
	want := preferredChannelForNetworkChange(descs, current.SelectorDefault, netClass)
	if want == "" {
		a.appendLog("info", "сеть: для класса "+classLabel(netClass)+" нет нового live-verified preference — текущий канал не тронут")
		return
	}
	a.appendLog("info", "сеть: probe-гейт PASS, пробую preference "+want+" для "+classLabel(netClass))
	if err := a.switchChannelCoreFor(want, current); err != nil {
		// switchChannelCore уже проверил новый канал и откатил живой старый
		// (либо сделал fail-closed, если старый тоже мёртв). Здесь не запускаем
		// второй failover и не меняем policy самостоятельно.
		if _, verified := verifiedRollback(err); verified {
			a.appendLog("warn", "сеть: preference "+want+" не применён; прежний путь повторно подтверждён: "+err.Error())
		} else {
			a.appendLog("warn", "сеть: preference "+want+" не применён: "+err.Error())
		}
		return
	}
	a.appendLog("info", "сеть: preference применён после probe-гейта: "+want)
}

// currentNetFingerprint — факт ОС: каким интерфейсом машина идёт в дефолт
// сейчас (переменная уровня пакета: тесты подменяют без сети; чистый
// оригинал живёт отдельным именем netFingerprintOS для восстановления).
//
// Механика без PowerShell: UDP-connect на IP-литерал НЕ отправляет пакетов,
// но выполняет route-lookup — LocalAddr = IPv4 интерфейса дефолтного
// маршрута. Интерфейс и имя — сопоставлением IP с net.Interfaces().
// Результат «snowden0» (TUN) = нулевой факт: физическую сеть в TUN-режиме
// так не увидеть, а ложные down-переходы на старте/остановке туннеля
// недопустимы.
var currentNetFingerprint = netFingerprintOS

func netFingerprintOS() netFingerprint {
	d := net.Dialer{Timeout: 700 * time.Millisecond}
	conn, err := d.Dial("udp", "9.9.9.9:53")
	if err != nil {
		return netFingerprint{} // маршрута в дефолт нет (или сеть умирает)
	}
	udp, ok := conn.LocalAddr().(*net.UDPAddr)
	conn.Close()
	if !ok || udp.IP == nil {
		return netFingerprint{}
	}
	local := udp.IP.String()
	ifaces, ierr := net.Interfaces()
	for i := range ifaces {
		if ierr != nil {
			break
		}
		if ifaces[i].Name == tunAdapterName {
			continue // TUN-адаптер — не физическая сеть
		}
		addrs, aerr := ifaces[i].Addrs()
		if aerr != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.String() == local {
				return netFingerprint{ifaceIndex: ifaces[i].Index, ip: local, alias: ifaces[i].Name}
			}
		}
	}
	return netFingerprint{}
}
