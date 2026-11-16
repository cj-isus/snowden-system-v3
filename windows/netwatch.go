package main

// netwatch.go — наблюдение за сменой сети (V2-054).
//
// Один poller доставляет debounced observations. Один отдельный original task
// объединяет события в один pending slot, не блокируя poller сетевым IPC.
// Down запрашивает паузу DNS; up/change — retirement старого DNS-поколения,
// resume и существующую protected reprobe/preference/retry реакцию. Успешный
// handoff сохраняет established SDK transports. Ошибка владения идёт через
// existing failCore, без отрицательного health-факта о канале.
// Fingerprint и control ACK не являются аттестацией системного пути; текущие
// latency/CPU/native/OS свойства этой source-реализации не проверены.
//
// Границы (честно):
//   - TUN-режим: весь трафик (и наш connect) маршрутизируется в snowden0 —
//     физическую смену сети fingerprint не видит. Наблюдатель в TUN ОТКЛЮЧЁН
//     (tunRequested-гвард), там действует 30с-стороже;
//   - событие дебаунсится netwatchDebounce: flapping Wi-Fi⇄LTE даёт одно
//     событие (последний переход), с потолком ожидания;
//   - ошибка/неоднозначность сбора = unknown, не down. Unknown/TUN gap
//     отменяет недоставленный debounce, сохраняя последний доставленный факт;
//   - down требует полного непустого снимка non-loopback адаптеров, в котором
//     нет ни одного поднятого link. Это локальный факт, не Internet/health proof.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
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

// netFingerprint — наблюдение локального route-selected IPv4/IPv6 интерфейса.
// Zero = unknown. allLinksDown допускается только после полного непустого
// снимка всех non-loopback links, включая виртуальные и наш TUN: IPv4 route
// lookup error сам по себе не означает отсутствие IPv6 или другого VPN.
// Наблюдение маршрута не аттестует physical path, Internet или protected egress.
type netFingerprint struct {
	ifaceIndex   int
	ip           string
	alias        string
	luid         uint64 // native route/adapter identity; not a WFP ownership receipt
	netClass     string // class from this snapshot, never a second unrelated MIB read
	allLinksDown bool
}

func (f netFingerprint) present() bool {
	return !f.allLinksDown && f.ifaceIndex > 0 && f.ip != ""
}

func (f netFingerprint) observed() bool { return f.present() || f.allLinksDown }

func (f netFingerprint) equal(o netFingerprint) bool {
	return f.ifaceIndex == o.ifaceIndex && f.ip == o.ip && f.alias == o.alias && f.luid == o.luid && f.netClass == o.netClass && f.allLinksDown == o.allLinksDown
}

// classifyTransition сравнивает последний доставленный факт и новый снимок.
// Unknown не отзывает путь и не означает восстановление после down.
// Начальный confirmed-down тоже доставляется: zero baseline не является up.
func classifyTransition(prev, fp netFingerprint) string {
	switch {
	case !fp.observed():
		return ""
	case fp.present() && !prev.present():
		return "up"
	case fp.allLinksDown && !prev.allLinksDown:
		return "down"
	case fp.present() && prev.present() && !fp.equal(prev):
		return "change"
	default:
		return ""
	}
}

// Class is captured with the native route/LUID snapshot. Consumers do no MIB
// or process work and cannot attach another adapter's later class to this event.
func netClassOf(fp netFingerprint) string { return fp.netClass }

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
// живут только в poller run (блокировок не требуют). startNetWatch создаёт
// ровно один poller и один original DNS-handoff task; оба владельца сохранены.
type netWatcher struct {
	mu            sync.Mutex
	started       bool
	stop          chan struct{}
	stopOnce      sync.Once
	last          netFingerprint // последний ДОСТАВЛЕННЫЙ observed факт, включая down
	lastAt        time.Time      // момент доставки последнего перехода
	app           *App
	dns           *serviceDNSHandoff
	retiredDNS    *serviceDNSHandoff // latest fully retired predecessor, bounded to one
	poller        *networkPollerOwner
	retiredPoller *networkPollerOwner // latest fully retired producer, bounded to one
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
	if a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() {
		netWatch.mu.Unlock()
		return
	}
	netWatch.started = true
	netWatch.stop = make(chan struct{})
	netWatch.app = a
	netWatch.poller = newNetworkPollerOwner(a)
	netWatch.dns = newServiceDNSHandoff(a, netWatch.poller)
	dns, poller := netWatch.dns, netWatch.poller
	netWatch.mu.Unlock()
	go dns.run()
	go poller.run(netWatch)
}

// stopNetWatch — остановить горутину наблюдателя (закрытие окна, перезапуск
// режима). Повторный вызов безопасен (sync.Once).
func (a *App) stopNetWatch() {
	pollerDone, dnsDone, err := a.cancelNetWatch()
	if err != nil {
		a.appendLog("warn", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	if err := a.joinNetworkPoller(ctx, pollerDone); err != nil {
		a.appendLog("warn", "завершение цикла наблюдения за сетью не подтверждено; исходный владелец сохранён")
	}
	if err := a.joinServiceDNSObserver(ctx, dnsDone); err != nil {
		a.appendLog("warn", "завершение сетевого DNS-обработчика не подтверждено; исходный владелец сохранён")
	}
}

// Cancellation only; the terminal coordinator joins both original tasks with
// its other producers under one budget. Never cancel another App's watcher.
func (a *App) cancelNetWatch() (pollerDone <-chan struct{}, dnsDone <-chan struct{}, result error) {
	netWatch.mu.Lock()
	if netWatch.app != nil && netWatch.app != a {
		netWatch.mu.Unlock()
		return nil, nil, fmt.Errorf("network watcher belongs to another App: %w", errAppProducerPending)
	}
	ch := netWatch.stop
	dns := netWatch.dns
	poller := netWatch.poller
	netWatch.mu.Unlock()
	if poller != nil {
		pollerDone = poller.done
	}
	if dns != nil {
		dnsDone = dns.done
	}
	sequence := &appCleanupSequence{}
	defer sequence.finish(&result)
	defer appCleanupStep("cancel original network DNS task", sequence, func() error { dns.stop(); return nil })
	defer appCleanupStep("cancel original network poller task", sequence, func() error { poller.stop(); return nil })
	defer appCleanupStep("signal original network watcher stop", sequence, func() error {
		if ch != nil {
			netWatch.stopOnce.Do(func() { close(ch) })
		}
		return nil
	})
	defer appCleanupStep("revoke original network DNS source", sequence, func() error {
		if dns != nil {
			dns.mu.Lock()
			defer dns.mu.Unlock()
			dns.source.Revoke()
		}
		return nil
	})
	return pollerDone, dnsDone, nil
}

// run — цикл наблюдателя. Доставка НЕ блокирует опрос: переход создаёт
// отложенное событие (pending + fireAt); каждый новый переход перезаписывает
// pending и отодвигает fireAt (коалесинг flapping). Одинаковые успешные
// samples не продлевают debounce. Cap действует только без unknown/TUN gaps;
// доставка на следующем тике, без обещания hard OS/IPC/latency bound.
func (w *netWatcher) run(ctx context.Context) {
	w.runWithPowerOwner(ctx, nil) // retained direct entry; production passes original owner
}

func (w *netWatcher) runWithPowerOwner(ctx context.Context, owner *networkPollerOwner) {
	if !lifecyclePublicationCurrent(ctx) {
		return
	}
	ticker := time.NewTicker(netwatchPoll)
	defer ticker.Stop()
	var powerWake <-chan struct{}
	if owner != nil {
		powerWake = owner.powerWake
	}
	var (
		pendingKind    string
		pendingFP      netFingerprint
		pendingAt      time.Time // момент первого отложенного перехода (потолок)
		fireAt         time.Time
		continuityLost bool // observed departure from delivered route, not unknown
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-powerWake:
			netEpoch.cancelPowerOperation()
			return // original finalizers retire producer/DNS/runtime; no second monitor
		case <-ticker.C:
		}
		if !lifecyclePublicationCurrent(ctx) {
			return
		}
		// TUN-режим: физическую сеть не видно (весь трафик в snowden0) —
		// наблюдатель молчит, действует 30с-стороже.
		if tunRequested() {
			pendingKind, pendingFP = "", netFingerprint{}
			pendingAt, fireAt = time.Time{}, time.Time{}
			continue
		}
		fp := currentNetFingerprint(ctx)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if tunRequested() || !fp.observed() {
			// Не доставлять старый pending после потери наблюдаемости.
			// Baseline и ранее observed discontinuity сохраняются. Unknown
			// сам не создаёт discontinuity; новая доставка ждёт свежий debounce.
			pendingKind, pendingFP = "", netFingerprint{}
			pendingAt, fireAt = time.Time{}, time.Time{}
			continue
		}
		now := time.Now()
		w.mu.Lock()
		prev := w.last
		w.mu.Unlock()
		kind := classifyTransition(prev, fp)
		if prev.present() && (fp.allLinksDown || fp.present() && !fp.equal(prev)) {
			continuityLost = true
		}
		if kind == "" && continuityLost && fp.present() && prev.present() {
			// A→down/B→A до доставки всё равно отзывает старое DNS/path
			// поколение одним change после стабилизации. Исчезновение
			// observable departure не восстанавливает continuity authority.
			kind = "change"
		}
		if kind == "" {
			// Нет observed departure от baseline и нет pending перехода.
			pendingKind, pendingFP = "", netFingerprint{}
			pendingAt, fireAt = time.Time{}, time.Time{}
			continue
		}
		if pendingKind != kind || !pendingFP.equal(fp) {
			if pendingKind == "" {
				pendingAt = now
			}
			pendingKind, pendingFP = kind, fp
			fireAt = now.Add(netwatchDebounce)
		}

		if !now.Before(fireAt) || now.Sub(pendingAt) >= netwatchDebounceCap {
			kind2, fp2 := pendingKind, pendingFP
			pendingKind, pendingFP = "", netFingerprint{}
			fireAt, pendingAt = time.Time{}, time.Time{}
			continuityLost = false
			w.mu.Lock()
			w.last, w.lastAt = fp2, now
			w.mu.Unlock()
			w.fire(kind2, fp2)
		}
	}
}

// fire — фактическая доставка: журнал-факт + реакция. Тесты зовут напрямую
// с app=nil (тишина).
func (w *netWatcher) fire(kind string, fp netFingerprint) {
	// Class is already part of this event's captured native snapshot.
	cls := netClassOf(fp)
	w.mu.Lock()
	app := w.app
	dns := w.dns
	poller := w.poller
	w.mu.Unlock()
	var change *serviceDNSChange
	if app != nil {
		releasePoller, live := poller.guardRunning(app)
		if !live {
			return
		}
		releasePoller()                                 // no MIB, epoch callbacks, logs or enqueue under the lease
		change = app.captureServiceDNSChange(dns, kind) // before epoch/log/cancellation callbacks
	}
	// V2-057 (A0): up/change открывают новую сетевую эпоху (health-факты
	// прошлой эпохи устаревают); down не бампит (сети нет — не смена). До
	// ветвления app: событие должно бампнуть эпоху даже без живого App.
	event := onNetEvent(kind, cls, time.Now())
	if app == nil {
		return
	}
	releasePoller, live := poller.guardRunning(app)
	if !live {
		return
	}
	poller.observed = true // actual delivered event; private value, not Internet/path proof
	releasePoller()
	switch kind {
	case "down":
		app.appendLog("warn", "сеть: локальные сетевые адаптеры отключены — запрошена приостановка DNS-разрешения серверов")
	case "up":
		app.appendLog("info", "сеть: локальный маршрут появился ("+classLabel(cls)+") — проверка защищённого пути вне очереди")
	case "change":
		app.appendLog("info", "сеть: локальный маршрут изменился ("+classLabel(cls)+") — проверка защищённого пути вне очереди")
	}
	if dns != nil {
		dns.submitChange(change.bindEvent(event))
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
var netWatchProbeStart = func(a *App) { a.netChangeReprobe() }
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
	parent := a.ctx
	netWatch.mu.Lock()
	if netWatch.app == a && netWatch.dns != nil {
		parent = netWatch.dns.ctx
	}
	netWatch.mu.Unlock()
	if parent == nil || parent.Err() != nil || a.closing.Load() {
		return
	}
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	a.netChangeReprobeFor(parent, cc)
}

// Production DNS reactions pass their original observer/event context and
// applied config. This method never resolves a newer actor after queueing.
func (a *App) netChangeReprobeFor(parent context.Context, cc *render.ClientConfig) {
	if !lifecyclePublicationCurrent(parent) {
		return
	}
	if !a.sessionCurrent(cc) {
		return
	}
	probeCtx, cancel := context.WithTimeout(parent, 15*time.Second)
	report := a.protectedProbe(cc).Run(probeCtx)
	cancel()
	stillOurs := a.sessionCurrent(cc)
	if !stillOurs {
		return
	}
	if a.blockUnsafeObservation(cc, report, "сеть") {
		return
	}
	if !lifecyclePublicationCurrent(parent) || !a.currentProbeReport(cc, report) {
		return
	}
	if report.AllPassed {
		// V2-077 (Этап 8): PASS текущего protected probe — обязательный гейт
		// для переоценки transport preference. Без этого смена класса сети не
		// имеет права трогать работающий туннель: предпочтение — policy hint,
		// не доказательство доступности нового канала.
		a.appendLog("info", "сеть: защищённый путь жив на новой сети — probe-гейт preference PASS")
		if err := a.verifySessionProtection(cc, report); err != nil {
			a.blockProtectionFailure(cc, err, "WFP: policy не подтверждена после смены сети")
			return
		}
		if a.publishObservation(cc, report, observationHealthy, "") {
			a.tryPreferredChannelOnNetworkChange(parent, cc, report)
		}
		return
	}
	a.appendLog("warn", "сеть: проверка не подтвердила путь на новой сети — повторяю измерение: "+report.Summary())
	if !a.publishObservation(cc, report, observationSuspected, "") {
		return
	}
	recovered, lastReport := a.fastReprobe(parent, cc)
	if !a.sessionCurrent(cc) {
		return
	}
	if lastReport != nil {
		report = lastReport
	}
	if a.blockUnsafeObservation(cc, report, "сеть") {
		return
	}
	if !lifecyclePublicationCurrent(parent) || !a.currentProbeReport(cc, report) {
		return
	}
	if recovered {
		a.appendLog("info", "сеть: путь восстановился (мерцание) — туннель не тронут")
		if err := a.verifySessionProtection(cc, report); err != nil {
			a.blockProtectionFailure(cc, err, "WFP: policy не подтверждена после восстановления сети")
			return
		}
		if a.publishObservation(cc, report, observationRecovered, "") {
			a.tryPreferredChannelOnNetworkChange(parent, cc, report)
		}
		return
	}
	summary := ""
	if lastReport != nil {
		summary = lastReport.Summary()
	}
	a.appendLog("error", "сеть: серия проверок не подтвердила путь после смены сети — ограниченная попытка восстановления: "+summary)
	// Немедленный переход к семантике сторожа, без ожидания тика.
	fc := ClassifyFailure(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets))
	a.appendLog("error", "сеть: "+classifyFailureLogLine(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets)))
	if !a.publishObservation(cc, report, observationDegraded, fc.Code) {
		return
	}
	switched, blocked := a.tryObservedFailover(cc, report, fc.Code)
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
	a.failSession(cc, report, "сеть: путь не подтверждён; доступные попытки восстановления исчерпаны (BLOCKED)", true)
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
func (a *App) tryPreferredChannelOnNetworkChange(ctx context.Context, current *render.ClientConfig, observed ...*core.ProbeReport) {
	if current == nil || !lifecyclePublicationCurrent(ctx) || a.closing.Load() {
		return
	}
	netClass := currentNetClass(ctx)
	if !lifecyclePublicationCurrent(ctx) {
		return
	}
	if netClass == "" {
		a.appendLog("info", "сеть: класс не определён — transport preference не применяется")
		return
	}
	if len(observed) > 0 && (len(observed) != 1 || !a.currentProbeReport(current, observed[0])) {
		return
	}
	descs := current.Descriptors
	want := preferredChannelForNetworkChange(descs, current.SelectorDefault, netClass)
	if want == "" {
		a.appendLog("info", "сеть: для класса "+classLabel(netClass)+" нет нового live-verified preference — текущий канал не тронут")
		return
	}
	a.appendLog("info", "сеть: probe-гейт PASS, пробую preference "+want+" для "+classLabel(netClass))
	if err := a.switchChannelCoreFor(want, current, observed...); err != nil {
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

// Local Windows route observation, with original caller cancellation and a
// cooperative whole-sample budget. No socket/UDP dial, DNS/HTTP or process.
// Errors/unsupported/forbidden/stale snapshots are unknown, never all-link-down.
// The separate complete non-loopback snapshot supplies explicit local down.
// Neither native route/source/LUID nor its class proves Internet/protected path.
var currentNetFingerprint = netFingerprintOS
