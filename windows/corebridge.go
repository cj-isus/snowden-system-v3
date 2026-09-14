package main

// corebridge.go — подключение нового ядра (backend/core + render) к UI.
// Правила: только фактические состояния; секреты в UI/логи не попадают;
// probe-отчёт структурен; операции асинхронны и не блокируют UI.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

// vaultSecrets — адаптер хранилища к интерфейсу SecretsSource рендера.
type vaultSecrets struct {
	vault *secretvault.Manager
	pins  map[string]string
	mu    sync.Mutex
}

func (v *vaultSecrets) Get(ref string) (string, error) {
	metas, err := v.vault.List()
	if err != nil {
		return "", err
	}
	for _, m := range metas {
		if m.Kind == ref {
			return v.vault.Value(m.ID)
		}
	}
	return "", fmt.Errorf("секрет %q не найден в хранилище", ref)
}

func (v *vaultSecrets) PinCertPath(channelID string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.pins[channelID]
}

func (v *vaultSecrets) setPin(channelID, path string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.pins == nil {
		v.pins = map[string]string{}
	}
	v.pins[channelID] = path
}

// coreRuntime — владелец ядра в приложении.
type coreRuntime struct {
	mu        sync.Mutex
	manager   *core.Manager
	engine    *core.Engine
	cc        *render.ClientConfig
	startBusy bool
	watchOn   bool // сторожевой тикер запущен
}

// watchdogInterval — период контроля защищённого пути. Умеренный тик: probe
// лёгкий (2 HTTPS-запроса), но каждый занимает SOCKS-инбаунд; 30с — компромисс
// «заметить отказ быстрее пользователя, не устраивая самоддо».
const watchdogInterval = 30 * time.Second

// ensureWatchdog — ровно один сторож на процесс. Раньше стартовали его из
// ДВУХ мест (startup и успешный startCore) с независимыми guard'ами: при
// --auto-connect оба успевали до первого флага → два тикера → два probe каждые
// 30с (самоддо SOCKS-инбаунда и двойные failover-решения). Единая точка входа.
func (a *App) ensureWatchdog() {
	runtime_.mu.Lock()
	firstWatch := !runtime_.watchOn
	runtime_.watchOn = true
	runtime_.mu.Unlock()
	if firstWatch {
		go a.watchCore(context.Background())
	}
}

var runtime_ = &coreRuntime{}

// renderAndPrepare — сборка конфига из дескрипторов + секретов и подготовка
// ядра к старту. Вызывается перед Start (или из Reload); channelID —
// желаемый default селектора ("" = первый live-verified).
//
// V2-037: перед рендером выполняется preflight-резолв хостнеймов vless+ws
// каналов (мультистратегийный, с кэшем) — рендер получает IP-диал
// (server=IP, SNI/Host=домен), и старт канала перестаёт зависеть от
// bootstrap-DNS sing-box.
func (a *App) renderAndPrepare(channelID string) (*render.ClientConfig, error) {
	src := &vaultSecrets{vault: a.vault}
	channels, err := render.LoadDescriptors()
	if err != nil {
		return nil, err
	}
	// Пины HY2: по каналу из generic-каталога (FR-002), legacy-файл — только
	// для исторического channel-a-hy2 (V2-037: раньше хардкод ID ломал
	// envelope-доставку новых HY2-каналов).
	for _, ch := range channels {
		if ch.Protocol != "hysteria2" {
			continue
		}
		if pin := pinForChannel(ch.ID); pin != "" {
			src.setPin(ch.ID, pin)
		}
	}

	// Preflight-резолв: только vless+ws (их адрес — домен за CDN); HY2 и
	// REALITY диалят origin по IP из дескриптора.
	overrides := map[string]string{}
	resolver := &core.PreflightResolver{
		CachePath: filepath.Join(appDataDir(), "resolve-cache.json"),
		Logf: func(format string, args ...any) {
			a.appendLog("info", fmt.Sprintf(format, args...))
		},
	}
	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 6*time.Second)
	for _, ch := range channels {
		if !ch.Enabled || ch.Protocol != "vless" || ch.Transport != "ws" {
			continue
		}
		if net.ParseIP(ch.Hostname) != nil {
			continue
		}
		ip, _, rerr := resolver.Resolve(resolveCtx, ch.Hostname)
		if rerr != nil {
			a.appendLog("warn", "preflight: "+ch.Hostname+" не резолвится ("+rerr.Error()+") — dial по имени (bootstrap-DNS)")
			continue
		}
		overrides[ch.Hostname] = ip
	}
	resolveCancel()

	withTUN := tunRequested()
	// Валидация запроса ДО рендера: несуществующий/не включённый канал —
	// ошибка, а не тихий откат к первому (fail-closed, не угадывание).
	if channelID != "" {
		known := false
		for _, ch := range channels {
			if ch.ID == channelID && ch.Enabled {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("канал %q не найден или выключен", channelID)
		}
	}
	var opts []render.RenderOption
	if channelID != "" {
		opts = append(opts, render.WithDefaultChannel(channelID))
	}
	if withTUN {
		opts = append(opts, render.WithTUN())
	}
	if len(overrides) > 0 {
		opts = append(opts, render.WithDialOverrides(overrides))
	}
	// Лог ядра в файл (V2-046): усекаем на КАЖДОМ рендере — файл per-session,
	// строки прошлых запусков не должны смешиваться с текущим каналом.
	enginePath := engineFileLogPath()
	_ = os.MkdirAll(logsDir(), 0o700)
	if f, terr := os.OpenFile(enginePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); terr == nil {
		f.Close()
	}
	opts = append(opts, render.WithEngineLogPath(enginePath))
	cc, err := render.RenderFrom(channels, src, opts...)
	if err != nil {
		return nil, err
	}
	return cc, nil
}

// pinForChannel — путь к pin-PEM канала: generic-каталог pins/<id>.pem в
// AppData рядом с хранилищем; для channel-a-hy2 — исторические расположения.
func pinForChannel(channelID string) string {
	if p := filepath.Join(vaultDir(), "pins", channelID+".pem"); fileExists(p) {
		return p
	}
	if channelID == "channel-a-hy2" {
		for _, p := range legacyHy2PinPaths() {
			if fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func legacyHy2PinPaths() []string {
	paths := []string{filepath.Join(vaultDir(), "hy2_pin.pem")}
	if exe, err := osExecutable(); err == nil {
		paths = append(paths, filepath.Join(exeDir(exe), "secrets", "hy2_pin.pem"))
	}
	return append(paths, filepath.Join("secrets", "hy2_pin.pem"))
}

// ---------- контракты UI: реальные Start/Stop/RunProbe ----------

// startCore — фактический запуск VPN (без прокси-простоты: полный путь
// engine → system proxy → protected probe). Горутина: UI не блокируется.
func (a *App) startCore() {
	runtime_.mu.Lock()
	if runtime_.startBusy {
		runtime_.mu.Unlock()
		return
	}
	runtime_.startBusy = true
	runtime_.mu.Unlock()
	defer func() {
		runtime_.mu.Lock()
		runtime_.startBusy = false
		runtime_.mu.Unlock()
	}()

	a.mu.Lock()
	a.state.State = "starting"
	a.state.Error = ""
	a.state.BlockedReason = "" // прошлый BLOCKED (например, исчерпан failover) не висит над новым запуском
	a.pushStateLocked()
	a.mu.Unlock()

	cc, err := a.renderAndPrepare("")
	if err != nil {
		a.failCore("рендер конфига", err)
		return
	}
	runtime_.mu.Lock()
	runtime_.cc = cc
	runtime_.mu.Unlock()

	if !a.startWithProbe(cc) {
		a.appendLog("warn", "запуск: protected probe на "+cc.SelectorDefault+" не прошёл")
		// Failover-on-start (V2-040): провал probe на первом канале не является
		// приговором всей защите — сеть могла отрезать именно этот транспорт
		// (V2-039b: CDN-путь тонет, QUIC-origin жив). Перебираем прочие
		// selectable каналы (тот же порядок, что у сторожевого failover),
		// каждый — свежим рендером и обязательным probe. Ни один не прошёл —
		// честный error (fail-closed, FR-001: никогда direct).
		channels, lerr := render.LoadDescriptors()
		if lerr != nil {
			a.failCore("запуск", fmt.Errorf("protected probe failed и дескрипторы нечитаемы: %w", lerr))
			return
		}
		others := failoverCandidates(channels, map[string]bool{}, cc.SelectorDefault)
		if len(others) == 0 {
			a.failCore("запуск", fmt.Errorf("protected probe failed на канале %s; других validated каналов нет (fail-closed)", cc.SelectorDefault))
			return
		}
		a.appendLog("warn", "запуск: probe канала "+cc.SelectorDefault+" не прошёл — перебираю альтернативные каналы (failover-on-start)")
		started := false
		for _, cand := range others {
			candCC, ok := a.startChannelWithProbe(cand)
			if !ok {
				continue
			}
			cc = candCC
			started = true
			break
		}
		if !started {
			// Последний рубеж (V2-045): одна повторная серия по всем кандидатам.
			// Волны деградации (V2-026/039 класс) длятся минуты и бьют по всем
			// каналам ОДНОВРЕМЕННО, но не непрерывно: за 2–3 мин окна хотя бы один
			// канал обычно отпускает. Дешёвый ретрай резко снижает «встал без VPN»
			// без ослабления FR-001 (probe обязателен на каждой попытке).
			a.appendLog("warn", "запуск: все кандидаты не прошли с первого раза — повторная серия через 20с (волны деградации бьют не непрерывно)")
			time.Sleep(20 * time.Second)
			for _, cand := range others {
				candCC, ok := a.startChannelWithProbe(cand)
				if !ok {
					continue
				}
				cc = candCC
				started = true
				break
			}
		}
		if !started {
			a.failCore("запуск", fmt.Errorf("все каналы не прошли probe (%s и %v), включая повторную серию — fail-closed (FR-001)", cc.SelectorDefault, others))
			return
		}
		a.appendLog("info", "запуск: канал "+cc.SelectorDefault+" прошёл probe (автоперебор)")
	}
	// Сторож — один на процесс (не на Start): повторный Start после Stop
	// не должен плодить тикеры. Явный Start также сбрасывает состояние
	// failover: прошлый инцидент не должен копиться в новом запуске.
	a.ensureWatchdog()
	a.fpol.hardReset()

	a.mu.Lock()
	a.state.State = "running"
	if tunRequested() {
		a.state.ProxyMode = "tun"
	} else {
		a.state.ProxyMode = "socks"
	}
	a.state.ActiveID = cc.SelectorDefault
	a.state.CoreReady = true
	a.state.CoreBlockMsg = ""
	a.state.Active = channelViewOf(cc, cc.SelectorDefault)
	a.mu.Unlock()

	// Kill switch — ТОЛЬКО TUN (V2-038, аудит п.5): в SOCKS смерть движка
	// означает повисший браузер (fail-closed по построению), в TUN — молча
	// прямой трафик мимо туннеля. Применяется ПОСЛЕ успешного probe (политика
	// разрешает наш exe — движок in-process); startup-очистка снимет её при
	// следующем старте, даже если этот процесс умрёт (dead-man + state-файл).
	// Вне a.mu: netsh — секунды, UI-мьютекс держать нельзя (урок V2-028).
	if tunRequested() {
		exe, eerr := osExecutable()
		switch {
		case eerr != nil:
			a.appendLog("warn", "kill switch: путь exe не определён — блок не применён (fail-open, честно): "+eerr.Error())
		default:
			if kerr := applyKillSwitch(osRunner{}, exe, killSwitchExpireDefault); kerr != nil {
				a.appendLog("warn", "kill switch: не применён (fail-open, честно): "+kerr.Error())
			} else {
				a.appendLog("info", "kill switch: блок-политика активна (TUN): прямой трафик вне туннеля заблокирован")
			}
		}
	}
	a.appendLog("info", "запуск: protected probe пройден — канал "+cc.SelectorDefault)
	a.appendLog("info", "VPN запущен: селектор "+fmt.Sprint(cc.SelectorTags)+", ожидаемый egress "+cc.ExpectedEgress)
	a.pushState()
}

// startWithProbe — Start текущего cc через Manager; провал = остановленный
// менеджер и false. Единая точка для startCore и перебора каналов.
func (a *App) startWithProbe(cc *render.ClientConfig) bool {
	engine := core.NewEngine(cc.Raw)
	manager := core.NewManager()
	manager.SetEngine(engine)
	manager.SetProbe(newAppProbe(cc))
	// TUN-режим: маршрутизацию делает адаптер snowden0 на IP-уровне, системный
	// прокси-реестр не трогаем (он же не действует на весь трафик — зачем).
	if tunRequested() {
		manager.SetSystemProxy(noopProxy{})
	}

	runtime_.mu.Lock()
	runtime_.manager = manager
	runtime_.engine = engine
	runtime_.mu.Unlock()

	if err := manager.Start(context.Background()); err != nil {
		_ = manager.Stop(context.Background())
		return false
	}
	return true
}

// startChannelWithProbe — полный путь «рендер под канал → Start с probe».
// Используется failover-on-start: каждый кандидат получает свежий рендер
// (expected egress/probe-цели следуют за каналом, V2-032).
func (a *App) startChannelWithProbe(channelID string) (*render.ClientConfig, bool) {
	cc, err := a.renderAndPrepare(channelID)
	if err != nil {
		a.appendLog("warn", "старт канала "+channelID+": рендер не собран: "+err.Error())
		return nil, false
	}
	runtime_.mu.Lock()
	runtime_.cc = cc
	runtime_.mu.Unlock()
	return cc, a.startWithProbe(cc)
}

// failCore — честный перевод в error с человеческой причиной.
// Живые manager/engine/конфиг ОБЯЗАТЕЛЬНО останавливаются: «error» с живым
// движком = процесс-зомби, держащий SOCKS:1080 и защищённые сокеты (следующий
// Start упал бы на занятом порту, а системный прокси остался бы на полу-мертвый
// инбаунд). Учитель — фиксированный rollback в Engine.Reload.
func (a *App) failCore(stage string, err error) {
	runtime_.mu.Lock()
	m := runtime_.manager
	runtime_.mu.Unlock()
	if m != nil {
		if serr := a.stopCore(); serr != nil {
			a.appendLog("warn", "cleanup после ошибки: "+serr.Error())
		}
	}
	a.mu.Lock()
	a.state.State = "error"
	a.state.Error = stage + ": " + err.Error()
	a.state.ProxyMode = ""
	a.state.Active = nil
	a.state.ActiveID = ""
	a.mu.Unlock()
	a.appendLog("error", a.state.Error)
	a.pushState()
}

// fastReprobe* — параметры подтверждения деградации (V2-037): первый провал
// тика НЕ останавливает туннель; подряд fastReprobeCount репроб с паузой
// fastReprobeGap должны тоже провалиться, прежде чем отказ считается
// подтверждённым. Поглощает Wi-Fi⇄LTE-переходы и CGNAT-мерцания (V2-026),
// которые раньше роняли VPN целиком на первом же тике.
const (
	fastReprobeCount = 2
	fastReprobeGap   = 5 * time.Second
)

// fastReprobe — серия быстрых репроб после провала тика. true = путь
// восстановился (мерцание), туннель жив. Возвращает последний отчёт.
func (a *App) fastReprobe(ctx context.Context, cc *render.ClientConfig) (recovered bool, report *core.ProbeReport) {
	for i := 0; i < fastReprobeCount; i++ {
		select {
		case <-ctx.Done():
			return false, nil
		case <-time.After(fastReprobeGap):
		}
		if a.closing.Load() {
			return false, nil
		}
		runtime_.mu.Lock()
		ours := runtime_.cc == cc && runtime_.manager != nil
		runtime_.mu.Unlock()
		if !ours {
			return false, nil // туннель переключили/остановили, пока мы ждали
		}
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		r := newAppProbe(cc).Run(probeCtx)
		cancel()
		if r.AllPassed {
			return true, r
		}
		report = r
	}
	return false, report
}

// watchCore — сторож защищённого пути. Раз в watchdogInterval повторный
// protected probe по живому туннелю (V2-037):
//  1. провал тика → fast-репробы (5с×2): восстановление = мерцание, счётчики
//     сброшены, туннель НЕ трогаем;
//  2. подтверждённый отказ → failoverPolicy: автопереключение на другой
//     validated канал (бесшовно, через switchChannelCore);
//  3. кандидатов нет / лимит попыток исчерпан / оба канала мертвы —
//     fail-closed Stop и BLOCKED. Прямой fallback запрещён (FR-001).
//
// Инцидент 2026-09-09 (сеть отвалилась под живым процессом) и V2-026
// (нестабильный return-путь CGNAT) — классы отказов, которые закрывает
// связка «fast-репробы + бесшовный failover».
func (a *App) watchCore(ctx context.Context) {
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		runtime_.mu.Lock()
		running := runtime_.manager != nil && runtime_.cc != nil
		cc := runtime_.cc
		m := runtime_.manager
		startBusy := runtime_.startBusy
		runtime_.mu.Unlock()
		// Guard (V2-046): тик при идущем Start/переборе каналов — probe по
		// полу-стартованному менеджеру даёт ложную «подтверждённую серию»,
		// поверх которой tryFailover сжигает попытку и портит state. Пропуск
		// тика безопасен: Start сам ставит сторож (ensureWatchdog) и тик
		// придёт на следующем интервале, когда манеджер консистентен.
		if startBusy {
			continue
		}
		if !running || a.closing.Load() {
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		report := newAppProbe(cc).Run(probeCtx)
		cancel()
		// Состояние могло измениться, пока probe шёл (Stop/Reload): выходим,
		// если туннель уже не наш.
		runtime_.mu.Lock()
		stillOurs := runtime_.manager != nil && runtime_.cc == cc
		runtime_.mu.Unlock()
		if !stillOurs {
			continue
		}
		if !report.AllPassed {
			a.appendLog("warn", "сторож: шаг probe не прошёл — подтверждаю серией быстрых репроб: "+report.Summary())
			recovered, lastReport := a.fastReprobe(ctx, cc)
			if lastReport != nil {
				report = lastReport
			}
			if recovered {
				a.appendLog("info", "сторож: путь восстановился (краткое мерцание) — туннель не тронут")
				a.fpol.hardReset()
				a.renewKillSwitchIfTUN()
				a.mu.Lock()
				a.state.Probe = toView(report)
				a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
				a.pushStateLocked()
				a.mu.Unlock()
				continue
			}
			if report.AllPassed {
				continue // серия завершилась успехом на последнем шаге
			}
			// Подтверждённый отказ защищённого пути.
			a.appendLog("error", "сторож: подтверждённый отказ защищённого пути — "+report.Summary())
			inSelector := map[string]bool{}
			for _, tag := range cc.SelectorTags {
				inSelector[tag] = true
			}
			channels, cerr := render.LoadDescriptors()
			if cerr != nil {
				channels = nil
			}
			switched, blocked := a.tryFailover(channels, inSelector, cc.SelectorDefault)
			if switched {
				continue // попытка удалась: следующий тик проверит новый канал
			}
			if blocked {
				a.appendLog("error", "failover: все каналы исчерпаны — остановка (fail-closed, BLOCKED)")
				a.stopCore()
				a.mu.Lock()
				a.state.State = "stopped"
				a.state.BlockedReason = "all_channels_failed"
				a.state.Error = "failover: защищённый путь недоступен на всех validated каналах — туннель остановлен (BLOCKED). Повторите подключение, когда сеть восстановится."
				a.state.Probe = toView(report)
				a.pushStateLocked()
				a.mu.Unlock()
				continue
			}
			// Попытка не удалась, лимит не исчерпан. Два сценария:
			//  а) переключение откатилось бесшовно — туннель ЖИВ на прежнем
			//     канале (Manager.SwitchChannel, V2-037): НЕ останавливаем,
			//     сторож продолжит наблюдение и повторит попытку после cooldown;
			//  б) движок остановлен fail-closed внутри переключения —
			//     фиксируем честный error.
			if m != nil && m.State() == "running" {
				a.appendLog("warn", "failover: попытка не удалась, туннель жив на прежнем канале — повтор после cooldown")
				continue
			}
			a.mu.Lock()
			if a.state.State != "error" { // failCore уже мог записать причину
				a.state.State = "error"
				a.state.Error = "защищённый путь недоступен (сторож): " + report.Summary() + " — повторите подключение"
			}
			a.pushStateLocked()
			a.mu.Unlock()
			continue
		}
		// Успешный тик: обновляем evidence и сбрасываем риск-серию.
		a.mu.Lock()
		a.state.Probe = toView(report)
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
		a.pushStateLocked()
		a.mu.Unlock()
		a.fpol.hardReset()
		a.renewKillSwitchIfTUN()
		a.pushState()
	}
}

// renewKillSwitchIfTUN — продление lease dead-man helper'а на каждый
// успешный тик сторожа (V2-046, закрытие P1). Без этого helper снимал
// политику через killSwitchExpireDefault (90с) посреди живой TUN-сессии:
// лог говорил «активна», а блок уже снят. Тик каждые 30с << lease 90с —
// продление идёт регулярно; при смерти отца последний lease дорабатывает
// срок и снимает политику (dead-man семантика сохранена). Только TUN:
// в SOCKS политика не применяется и продлевать нечего. Ошибки честно в
// журнал (fail-open объявлен), не молча.
func (a *App) renewKillSwitchIfTUN() {
	if !tunRequested() {
		return
	}
	exe, err := osExecutable()
	if err != nil {
		return
	}
	if err := renewKillSwitchLease(exe, killSwitchExpireDefault); err != nil {
		a.appendLog("warn", "kill switch: lease не продлён (fail-open, честно): "+err.Error())
	}
}

// startBusyWait — пауза ожидания завершения стартующей горутины (startCore
// продолжает работать под флагом startBusy). Раньше stopCore при manager==nil
// во время Start просто объявлял «stopped»: предстартовый Stop оставлял гонку —
// startCore продолжал старт и включал прокси/туннель ПОВЕРХ остановки. 100мс
// ниже максимального времени render+validate; Stop всё равно остаётся короткой
// операцией, а затяжной Start сам упрётся в lifecycleTimeout менеджера.
const startBusyWait = 100 * time.Millisecond

// stopCore — фактическая остановка.
func (a *App) stopCore() error {
	// Если идёт Start, ждём его завершения: иначе stopCore увидит manager==nil
	// (или поймает полу-сконструированный движок) и объявит stopped, а горутина
	// startCore продолжит включать прокси/туннель поверх остановки.
	for waited := time.Duration(0); waited < 5*time.Second; waited += startBusyWait {
		runtime_.mu.Lock()
		busy := runtime_.startBusy
		runtime_.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(startBusyWait)
	}
	runtime_.mu.Lock()
	stillBusy := runtime_.startBusy
	m := runtime_.manager
	runtime_.mu.Unlock()
	if stillBusy {
		a.appendLog("warn", "остановка: Start не завершился за 5с — продолжаем с текущим состоянием")
	}
	if m == nil {
		a.mu.Lock()
		a.state.State = "stopped"
		a.pushStateLocked()
		a.mu.Unlock()
		return nil
	}
	a.mu.Lock()
	a.state.State = "stopping"
	a.pushStateLocked()
	a.mu.Unlock()

	// Kill switch (V2-038): снять блок-политику ДО остановки движка — пока
	// туннель ещё маршрутизирует (окно «политика снята, туннель жив» безопасно:
	// туннель продолжает защищать; обратный порядок дал бы секунды прямого
	// трафика между смертью маршрутов и удалением правил). Helper гасим только
	// после успешного снятия — иначе он не нужен: правила уже удалены.
	if tunRequested() {
		if kerr := removeKillSwitch(osRunner{}); kerr != nil {
			a.appendLog("warn", "kill switch: снятие при остановке неполное (expire-helper доведёт): "+kerr.Error())
			// Helper оставляем: он снимет остатки через expire — сеть не останется
			// заблокированной из-за одной неудачной попытки netsh.
		} else {
			stopKillSwitchHelper()
		}
	}

	err := m.Stop(context.Background())

	runtime_.mu.Lock()
	runtime_.manager = nil
	runtime_.engine = nil
	runtime_.cc = nil
	runtime_.mu.Unlock()

	a.mu.Lock()
	if err != nil {
		a.state.State = "error"
		a.state.Error = "остановка: " + err.Error()
	} else {
		a.state.State = "stopped"
		a.state.Error = ""
		a.state.ProxyMode = ""
		a.state.Active = nil
		a.state.ActiveID = ""
		a.state.CoreReady = false
	}
	a.mu.Unlock()
	a.appendLog("info", "VPN остановлен")
	a.pushState()
	return err
}

// probeCore — повторный protected probe по работающему туннелю (UI-кнопка).
func (a *App) probeCore() error {
	runtime_.mu.Lock()
	cc := runtime_.cc
	running := runtime_.manager != nil
	runtime_.mu.Unlock()
	if !running || cc == nil {
		return fmt.Errorf("probe: VPN не запущен")
	}
	a.mu.Lock()
	a.state.ProbeRunning = true
	a.pushState()
	a.mu.Unlock()

	report := newAppProbe(cc).Run(context.Background())

	a.mu.Lock()
	a.state.ProbeRunning = false
	a.state.Probe = toView(report)
	if report.AllPassed {
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
	}
	a.mu.Unlock()
	a.appendLog("info", "probe: "+report.Summary())
	a.pushState()
	if !report.AllPassed {
		return fmt.Errorf("probe: %s", report.Summary())
	}
	return nil
}

// newAppProbe — probe с ожидаемым egress из дескриптора выбранного канала.
// Цели — из дескриптора (V2-037: первая цель self-hosted на нашем VPS,
// вторая — независимое эхо; сторож каждые 30с больше не зависит от
// rate-limit'ов сторонних ipify/ifconfig). Direct-leak семантика следует
// режиму (SOCKS: различие; TUN: совпадение — PLAN §2.3).
func newAppProbe(cc *render.ClientConfig) *core.ProtectedProbe {
	targets := cc.ProbeTargets
	if len(targets) < 2 {
		targets = render.DefaultProbeTargets
	}
	return &core.ProtectedProbe{
		SOCKSAddr:               "127.0.0.1:1080",
		Targets:                 targets,
		Timeout:                 10 * time.Second,
		ExpectedEgress:          cc.ExpectedEgress,
		DirectLeakThroughTunnel: tunRequested(),
	}
}

// toView — маппинг отчёта ядра в UI-контракт.
func toView(r *core.ProbeReport) *ProbeReportView {
	if r == nil {
		return nil
	}
	out := &ProbeReportView{OK: r.AllPassed, Steps: []ProbeStepView{}}
	for _, res := range r.Results {
		status := "pass"
		if !res.Passed {
			status = "fail"
		}
		out.Steps = append(out.Steps, ProbeStepView{
			Name: res.Name, Status: status, Detail: res.Detail,
		})
	}
	return out
}

// SelectChannel — переключение активного канала (A1.4) через Reload:
// новый конфиг собирается с другим default селектора, валидируется дважды
// (строгий парсер + sing-box), затем обязательный protected probe.
// Неудача любого шага = fail-closed (Manager сам откатывает/останавливает).
// Фактический путь — switchChannelCore (failover.go): тот же код использует
// автоматическое переключение сторожа (A4) — расхождений UX-кнопки и
// автопереключения нет по построению.
func (a *App) SelectChannel(channelID string) error {
	return a.switchChannelCore(channelID)
}

// channelViewOf — карточка канала из дескрипторов (FR-002: данные, не хардкод).
// Идентификатор не в дескрипторах = честный nil (не выдуманная карточка).
func channelViewOf(cc *render.ClientConfig, channelID string) *ChannelView {
	if cc == nil {
		return nil
	}
	chID, ok := cc.ChannelIDs[channelID]
	if !ok {
		return nil
	}
	for _, ch := range cc.Descriptors {
		if ch.ID != chID {
			continue
		}
		return &ChannelView{
			ID: ch.ID, Transport: ch.Protocol + "+" + ch.Transport,
			Server: ch.Hostname, Port: int(ch.Port),
			Validation: ch.ValidationStatus, Enabled: true,
		}
	}
	return nil
}

// engineFileLogPath — файл engine-логов sing-box (V2-046): рендер пишет
// log.output = engine.log, sing-box пишет туда же с тегами компонентов
// (inbound/outbound/…), tailer доставляет строки в общий журнал.
const engineFileLogName = "engine.log"

func engineFileLogPath() string {
	return filepath.Join(logsDir(), engineFileLogName)
}

// engineLogMsg — строка engine-лога для доставки в appendLog.
type engineLogMsg struct {
	level string
	text  string
}

// classifyEngineLine — уровень строки engine-лога по префиксу ядра
// (форматтер sing-box: "LEVEL[...]"). debug/trace → 0 (не логируем).
func classifyEngineLine(line string) string {
	switch {
	case strings.HasPrefix(line, "ERROR"), strings.HasPrefix(line, "FATAL"), strings.HasPrefix(line, "PANIC"):
		return "error"
	case strings.HasPrefix(line, "WARN"):
		return "warn"
	case strings.HasPrefix(line, "DEBUG"), strings.HasPrefix(line, "TRACE"):
		return ""
	default:
		return "info"
	}
}

// stopEngineLogTailer — сигнал остановки tailer'у (drain перед exit/restart).
func (a *App) stopEngineLogTailer() {
	if a.engineLogDone != nil {
		select {
		case <-a.engineLogDone:
			// уже закрыт
		default:
			close(a.engineLogDone)
		}
	}
}

// startEngineLogTailer — хвостовой читатель engine.log (V2-046): sing-box
// пишет в файл (log.output), tailer доставляет новые строки в appendLog.
// Файл переживает рестарты движка — на старте доезжаем до конца (только
// новые строки), при ротации (усечение) начинаем с начала.
func (a *App) startEngineLogTailer() {
	go func() {
		path := engineFileLogPath()
		_ = os.MkdirAll(logsDir(), 0o700)
		for {
			select {
			case <-a.engineLogDone:
				return
			default:
			}
			f, err := os.Open(path)
			if err != nil {
				if !a.sleepTill(500 * time.Millisecond) {
					return
				}
				continue
			}
			r := bufio.NewReader(f)
			for {
				line, rerr := r.ReadString('\n')
				if line != "" {
					lv := classifyEngineLine(line)
					if lv != "" {
						a.appendLog(lv, "движок "+strings.TrimRight(line, "\r\n"))
					}
				}
				if rerr != nil {
					break
				}
			}
			f.Close()
			if !a.sleepTill(500 * time.Millisecond) {
				return
			}
		}
	}()
}

// sleepTill — спать d с пробуждением на shutdown (true = проспал, false = закрываемся).
func (a *App) sleepTill(d time.Duration) bool {
	select {
	case <-a.engineLogDone:
		return false
	case <-time.After(d):
		return true
	}
}

// noopProxy — заглушка системного прокси для TUN-режима (см. startCore).
type noopProxy struct{}

func (noopProxy) Snapshot() error       { return nil }
func (noopProxy) SetProxy(string) error { return nil }
func (noopProxy) Restore() error        { return nil }

// currentRuntimeConfig — конфиг последнего успешного Start/SelectChannel
// (для live-тестов и диагностики). nil = ядро не запущено.
func currentRuntimeConfig() *render.ClientConfig {
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	return runtime_.cc
}

// ListChannels — фактический состав селектора из дескрипторов (FR-002).
func (a *App) ListChannels() ([]ChannelView, error) {
	channels, err := render.LoadDescriptors()
	if err != nil {
		return nil, err
	}
	out := make([]ChannelView, 0, len(channels))
	runtime_.mu.Lock()
	selected := map[string]bool{}
	if runtime_.cc != nil {
		for _, tag := range runtime_.cc.SelectorTags {
			selected[tag] = true
		}
	}
	runtime_.mu.Unlock()
	for _, ch := range channels {
		out = append(out, ChannelView{
			ID:         ch.ID,
			Transport:  ch.Protocol + "+" + ch.Transport,
			Server:     ch.Hostname,
			Port:       int(ch.Port),
			Validation: ch.ValidationStatus,
			Enabled:    selected[ch.ID],
		})
	}
	return out, nil
}

// GetSelectorState — отладочный/трейсинговый снимок для журнала (без секретов).
func selectorSummary(raw []byte) string {
	cfg, err := config.Parse(raw)
	if err != nil {
		return "invalid: " + err.Error()
	}
	b, _ := json.Marshal(map[string]any{
		"selector": cfg.ProtectedSelector(),
		"final":    cfg.Route.Final,
	})
	return strings.TrimSpace(string(b))
}
