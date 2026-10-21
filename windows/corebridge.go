package main

// corebridge.go — подключение нового ядра (backend/core + render) к UI.
// Правила: только фактические состояния; секреты в UI/логи не попадают;
// probe-отчёт структурен; операции асинхронны и не блокируют UI.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	return v.vault.ValueByKind(ref)
}

func (v *vaultSecrets) GetByID(id, kind string) (string, error) {
	return v.vault.ValueFor(id, kind)
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

	metrics *metricsState // read-only метрики clash_api (V2-048), nil = выключены
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
	return a.renderAndPrepareContext(context.Background(), channelID, nil)
}

func (a *App) renderAndPrepareContext(ctx context.Context, channelID string, metrics *metricsState) (*render.ClientConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	resolveCtx, resolveCancel := context.WithTimeout(ctx, 6*time.Second)
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
	// Preparing a candidate must not truncate the running engine's log.
	enginePath := engineFileLogPath()
	_ = os.MkdirAll(logsDir(), 0o700)
	opts = append(opts, render.WithEngineLogPath(enginePath))
	// One controller endpoint/secret per engine. A selector-only switch keeps
	// that session; a new engine publishes its metrics only after probe pass.
	if metrics == nil {
		metrics = newMetricsSession()
	}
	opts = append(opts, metrics.renderOption()...)
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
	a.mu.Lock()
	id := a.operationID
	a.mu.Unlock()
	a.startCoreGeneration(id)
}

func (a *App) startCoreGeneration(id uint64) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	ctx, cancel, ok := a.beginOperation(id)
	if !ok {
		return
	}
	defer a.endOperation(cancel)
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
	if a.operationID != id || !a.protectionDesired || ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.state.State = "starting"
	a.state.Error = ""
	a.state.BlockedReason = "" // прошлый BLOCKED (например, исчерпан failover) не висит над новым запуском
	a.pushStateLocked()
	a.mu.Unlock()
	// Protect even render/preflight failures on the first Connect, before
	// there is a Manager to tear down. No automatic error releases this policy.
	if tunRequested() {
		if err := a.protectionPolicy().Arm(); err != nil {
			a.failCore("WFP: барьер не подтверждён до подготовки конфига", err)
			return
		}
	}
	// Авто-выбор первого канала по классу сети (V2-054): mobile → HY2/QUIC
	// первым (CGNAT-валоны бьют по CDN-пути, V2-026), wifi/ethernet → vless+ws
	// первым. Probe-безопасно: выбор влияет только на ПОРЯДОК попыток —
	// failover-on-start всё равно переберёт validated каналы при провале;
	// нет класса/нет совпадения/дескрипторы нечитаемы — прежний pickDefault
	// (рендер с "" = первый live-verified из дескрипторов).
	defaultCh := ""
	if netClass := currentNetClass(); netClass != "" {
		if descs, derr := render.LoadDescriptors(); derr == nil {
			if want := pickDefaultChannel(descs, netClass); want != "" {
				defaultCh = want
				a.appendLog("info", "запуск: класс сети "+classLabel(netClass)+" — первым пробую канал "+want)
			}
		}
	}
	cc, err := a.renderAndPrepareContext(ctx, defaultCh, nil)
	if err != nil {
		if ctx.Err() == nil {
			a.failCore("рендер конфига", err)
		}
		return
	}

	if !a.startWithProbeContext(ctx, cc) {
		if ctx.Err() != nil {
			return
		}
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
			if ctx.Err() != nil {
				return
			}
			candCC, ok := a.startChannelWithProbeContext(ctx, cand)
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
			select {
			case <-ctx.Done():
				return
			case <-time.After(20 * time.Second):
			}
			for _, cand := range others {
				if ctx.Err() != nil {
					return
				}
				candCC, ok := a.startChannelWithProbeContext(ctx, cand)
				if !ok {
					continue
				}
				cc = candCC
				started = true
				break
			}
		}
		if ctx.Err() != nil {
			return
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
	if ctx.Err() != nil || !a.operationCurrent(id) {
		return // Stop owns cleanup; an obsolete generation cannot publish running.
	}
	a.ensureWatchdog()
	a.fpol.hardReset()
	a.sm.markStarted(time.Now()) // C1 (V2-060): туннель поднят (в т.ч. после авто-ретрая) → HEALTHY

	a.mu.Lock()
	if a.operationID != id || !a.protectionDesired {
		a.mu.Unlock()
		return
	}
	a.cancelFailClosedRetryLocked() // successful gate ends the recovery episode
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

	a.appendLog("info", "запуск: protected probe пройден — канал "+cc.SelectorDefault)
	// V2-057 (A0): успешный старт-гейт = первый health-факт канала в эпохе.
	// channelID — фактический активный (default мог быть скорректирован перебором).
	chanHealth.observeResult(currentChannelEpoch(), cc.SelectorDefault, true, "", time.Now())
	a.appendLog("info", "VPN запущен: селектор "+fmt.Sprint(cc.SelectorTags)+", ожидаемый egress "+cc.ExpectedEgress)
	a.pushState()
}

// startWithProbe — Start текущего cc через Manager; провал = остановленный
// менеджер и false. Единая точка для startCore и перебора каналов.
func (a *App) startWithProbe(cc *render.ClientConfig) bool {
	return a.startWithProbeContext(context.Background(), cc)
}

func (a *App) startWithProbeContext(ctx context.Context, cc *render.ClientConfig) bool {
	if ctx.Err() != nil {
		return false
	}
	runtime_.mu.Lock()
	previous := runtime_.manager
	runtime_.mu.Unlock()
	if previous != nil {
		if err := previous.Stop(context.Background()); err != nil {
			a.appendLog("error", "cleanup предыдущего manager не завершён; новый engine не запускается: "+err.Error())
			return false
		}
	}
	tunMode := tunRequested()
	if tunMode {
		if err := a.protectionPolicy().Arm(); err != nil {
			a.appendLog("error", "WFP: TUN заблокирован до безопасного применения policy: "+err.Error())
			return false
		}
	}
	engine := core.NewEngine(cc.Raw)
	manager := core.NewManager()
	manager.SetEngine(engine)
	manager.SetProbe(a.protectedProbe(cc))
	// TUN-режим: маршрутизацию делает адаптер snowden0 на IP-уровне, системный
	// прокси-реестр не трогаем (он же не действует на весь трафик — зачем).
	if tunMode {
		manager.SetSystemProxy(noopProxy{})
	}

	runtime_.mu.Lock()
	runtime_.manager = manager
	runtime_.engine = engine
	runtime_.mu.Unlock()

	if err := manager.Start(ctx); err != nil {
		_ = manager.Stop(context.Background())
		return false
	}
	if tunMode {
		if err := permitWFPTUN(wfpTunName, 8*time.Second); err != nil {
			_ = manager.Stop(context.Background()) // policy remains active while routes disappear
			a.appendLog("error", "WFP: интерфейс TUN не разрешён/не проверен: "+err.Error())
			return false
		}
		if err := externalTUNProbe(cc.ExpectedEgress, 15*time.Second); err != nil {
			_ = manager.Stop(context.Background())
			a.appendLog("error", "WFP: внешний процесс не подтвердил protected TUN: "+err.Error())
			return false
		}
		a.appendLog("info", "WFP: policy и внешний protected TUN probe подтверждены")
	}
	if ctx.Err() != nil {
		_ = manager.Stop(context.Background())
		return false
	}
	runtime_.mu.Lock()
	runtime_.cc = cc
	runtime_.metrics = metricsForConfig(cc.Raw)
	runtime_.mu.Unlock()
	return true
}

// startChannelWithProbe — полный путь «рендер под канал → Start с probe».
// Используется failover-on-start: каждый кандидат получает свежий рендер
// (expected egress/probe-цели следуют за каналом, V2-032).
func (a *App) startChannelWithProbe(channelID string) (*render.ClientConfig, bool) {
	return a.startChannelWithProbeContext(context.Background(), channelID)
}

func (a *App) startChannelWithProbeContext(ctx context.Context, channelID string) (*render.ClientConfig, bool) {
	cc, err := a.renderAndPrepareContext(ctx, channelID, nil)
	if err != nil {
		a.appendLog("warn", "старт канала "+channelID+": рендер не собран: "+err.Error())
		return nil, false
	}
	return cc, a.startWithProbeContext(ctx, cc)
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
		if serr := a.stopCoreLocked(stopFailure); serr != nil {
			a.appendLog("warn", "cleanup после ошибки: "+serr.Error())
			err = errors.Join(err, serr)
		}
	}
	// V2-056: если в момент failCore есть свежий probe-отчёт — классифицируем
	// его причину (журнал; поведение не меняется). runtime_.cc читаем под
	// runtime_.mu (правило конкурентного слоя V2-028).
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	a.mu.Lock()
	if last := a.state.Probe; last != nil && !last.OK {
		lreport := &core.ProbeReport{AllPassed: false}
		for _, st := range last.Steps {
			lreport.Results = append(lreport.Results, core.ProbeResult{
				Name: st.Name, Passed: st.Status == "pass", Detail: st.Detail,
			})
		}
		channelPort, probePort := 0, 0
		if cc != nil {
			channelPort, probePort = cc.TunnelPort, probeTunnelPort(cc.ProbeTargets)
		}
		a.appendLogLocked("error", stage+": "+classifyFailureLogLine(lreport, channelPort, probePort))
	}
	a.mu.Unlock()
	a.mu.Lock()
	if !a.protectionDesired {
		a.mu.Unlock()
		return
	}
	a.state.State = "error"
	a.state.Error = stage + ": " + err.Error()
	a.state.ProxyMode = ""
	a.state.Active = nil
	a.state.ActiveID = ""
	// V2-053: авто-ретрай с растущим backoff — волна деградации может
	// кончиться, когда никто не смотрит в окно. Планируем только здесь:
	// живой туннель после отката переключения сюда не попадает.
	a.scheduleFailClosedRetryLocked(time.Now())
	a.sm.markRecovering(time.Now()) // C1 (V2-060): fail-closed с авто-ретраем → RECOVERING
	a.appendLogLocked("error", a.state.Error)
	a.mu.Unlock()
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
	return a.fastReprobeAfter(ctx, cc, fastReprobeGap)
}

func (a *App) fastReprobeAfter(ctx context.Context, cc *render.ClientConfig, gap time.Duration) (recovered bool, report *core.ProbeReport) {
	for i := 0; i < fastReprobeCount; i++ {
		select {
		case <-ctx.Done():
			return false, nil
		case <-time.After(gap):
		}
		if a.closing.Load() {
			return false, nil
		}
		if !a.sessionCurrent(cc) {
			return false, nil // туннель переключили/остановили, пока мы ждали
		}
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		r := a.protectedProbe(cc).Run(probeCtx)
		cancel()
		if ctx.Err() != nil || !a.sessionCurrent(cc) {
			return false, nil
		}
		if r.LeakConfirmed {
			return false, r // confirmed leak must never wait for another probe
		}
		if r.AllPassed {
			return true, r
		}
		report = r
	}
	return false, report
}

// blockUnsafePath is reserved for positive evidence of an unsafe route.
// No debounce or live selector switch is allowed once such evidence exists.
func (a *App) blockUnsafePath(cc *render.ClientConfig, reason string, report *core.ProbeReport, failureClass string) {
	if !a.failSession(cc, report, reason, false) {
		return
	}
	now := time.Now()
	a.sm.markTick(false, report.Summary(), now)
	a.sm.markClassified(failureClass, report.Summary(), now)
	chanHealth.observeResult(currentChannelEpoch(), a.activeChannelIDForHealth(), false, failureClass, now)
	a.appendLog("error", reason+" — немедленная остановка")
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
	a.watchCoreTicks(ctx, ticker.C)
}

func (a *App) watchCoreTicks(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-ticks:
			if !open {
				return
			}
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
		if !running || !a.sessionCurrent(cc) {
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		report := a.protectedProbe(cc).Run(probeCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		// Состояние могло измениться, пока probe шёл (Stop/Reload): выходим,
		// если туннель уже не наш.
		stillOurs := a.sessionCurrent(cc)
		if !stillOurs {
			continue
		}
		if report.LeakConfirmed {
			a.blockUnsafePath(cc, "сторож: обнаружена утечка защищённого пути", report, fcLeakConfirmed)
			continue
		}
		if !report.AllPassed {
			// V2-056: детерминированный классификатор (THEORY §2.10 п.16) —
			// диагноз в журнал, решения не меняет (PLAN §5, честная граница v1).
			a.appendLog("warn", "сторож: шаг probe не прошёл — подтверждаю серией быстрых репроб: "+report.Summary())
			a.appendLog("warn", "сторож: "+classifyFailureLogLine(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets)))
			a.sm.markTick(false, report.Summary(), time.Now()) // C1 (V2-060): одиночный провал = SUSPECTED (§23)
			recovered, lastReport := a.fastReprobe(ctx, cc)
			if ctx.Err() != nil {
				return
			}
			if !a.sessionCurrent(cc) {
				continue
			}
			if lastReport != nil {
				report = lastReport
			}
			if report.LeakConfirmed {
				a.blockUnsafePath(cc, "сторож: обнаружена утечка защищённого пути", report, fcLeakConfirmed)
				continue
			}
			if recovered {
				a.appendLog("info", "сторож: путь восстановился (краткое мерцание) — туннель не тронут")
				a.fpol.hardReset()
				a.sm.markRecovered(time.Now()) // C1: серия прервалась успехом → HEALTHY
				if err := a.renewKillSwitchIfTUN(); err != nil {
					a.blockUnsafePath(cc, "WFP: policy не подтверждена", report, fcUnknown)
					continue
				}
				// V2-057 (A0): мерцание = успех пути (последний исход серии),
				// но FAIL не пишем — серия поглощена (семантика V2-037).
				a.recordProbeOutcome(true, report)
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
			fc := ClassifyFailure(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets))
			// C1 (V2-060): подтверждённая серия → классификация (§22 CLASSIFYING;
			// unknown-класс честен — выдумывать диагноз не будем).
			a.sm.markClassified(fc.Code, report.Summary(), time.Now())
			// V2-057 (A0): подтверждённая серия = FAIL записи канала (мерцания — нет).
			a.appendLog("error", "сторож: "+classifyFailureLogLine(report, cc.TunnelPort, probeTunnelPort(cc.ProbeTargets))+" — "+fc.Hint)
			inSelector := map[string]bool{}
			for _, tag := range cc.SelectorTags {
				inSelector[tag] = true
			}
			channels, cerr := render.LoadDescriptors()
			if cerr != nil {
				channels = nil
			}
			a.sm.markDegraded(time.Now()) // C1: класс есть → решение «деградировал» (§24)
			if fc.Code == fcEgressDivergence {
				a.blockUnsafePath(cc, "сторож: egress защищённого пути не совпадает с ожидаемым", report, fc.Code)
				continue
			}
			if fc.Code == fcWhitelistMode {
				a.blockUnsafePath(cc, "сторож: признаки allowlist-ограничения; диагноз требует дополнительной проверки", report, fc.Code)
				continue
			}
			chanHealth.observeResult(currentChannelEpoch(), a.activeChannelIDForHealth(), false, fc.Code, time.Now())
			switched, blocked := a.tryFailoverForClass(channels, inSelector, cc.SelectorDefault, fc.Code, cc)
			if switched {
				continue // попытка удалась: следующий тик проверит новый канал
			}
			if blocked {
				a.appendLog("error", "failover: все каналы исчерпаны — остановка (fail-closed, BLOCKED)")
				a.failSession(cc, report, "failover: защищённый путь недоступен на всех validated каналах (BLOCKED)", true)
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
			a.failSession(cc, report, "защищённый путь недоступен (сторож): "+report.Summary(), false)
			continue
		}
		// Успешный тик: обновляем evidence и сбрасываем риск-серию.
		a.recordProbeOutcome(true, report) // V2-057 (A0): health текущей эпохи
		a.mu.Lock()
		a.state.Probe = toView(report)
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
		a.pushStateLocked()
		a.mu.Unlock()
		a.fpol.hardReset()
		a.sm.markTick(true, "", time.Now()) // C1: успешный тик → HEALTHY (восстановление всегда выигрывает)
		if err := a.renewKillSwitchIfTUN(); err != nil {
			a.blockUnsafePath(cc, "WFP: policy не подтверждена", report, fcUnknown)
			continue
		}
		a.pushState()
	}
}

// renewKillSwitchIfTUN verifies persistent WFP policy; strict mode has no lease.
func (a *App) renewKillSwitchIfTUN() error {
	if !tunRequested() {
		return nil
	}
	return verifyWFPPolicy(true) // strict persistent protection has no expiry
}

// stopCore — фактическая остановка.
func (a *App) stopCore() error {
	return a.stopCoreFor(stopDisconnect)
}

// Caller holds operationMu. Automatic teardown retains strict WFP protection.
func (a *App) stopCoreLocked(reason StopReason) error {
	runtime_.mu.Lock()
	m := runtime_.manager
	runtime_.mu.Unlock()
	a.mu.Lock()
	a.state.State = "stopping"
	a.pushStateLocked()
	a.mu.Unlock()

	var err error
	policyRequired := tunRequested() || strictProtectionRetained()
	if policyRequired && !reason.releasesProtection() {
		err = a.protectionPolicy().BlockDirect()
	}
	if m != nil {
		err = errors.Join(err, m.Stop(context.Background()))
	}
	// Keep WFP active until TUN routes are gone. Only then remove the provider;
	// a process-stop window can therefore be offline, never direct.
	if policyRequired && reason.releasesProtection() && err == nil {
		if kerr := a.protectionPolicy().Release(); kerr != nil {
			err = fmt.Errorf("WFP: снятие защиты не завершено: %w", kerr)
		}
	}
	if err != nil {
		// Keep the ownership handle for a later cleanup attempt.
		a.mu.Lock()
		a.state.State = "error"
		a.state.Error = "остановка: " + err.Error()
		a.state.CoreReady = false
		a.state.ProbeRunning = false
		a.pushStateLocked()
		a.mu.Unlock()
		return err
	}

	runtime_.mu.Lock()
	runtime_.manager = nil
	runtime_.engine = nil
	runtime_.cc = nil
	runtime_.metrics = nil // сессия метрик умирает вместе с ядром (новый порт+секрет на старте)
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
		a.state.ProbeRunning = false
		a.state.BlockedReason = ""
		if policyRequired && !reason.releasesProtection() {
			a.state.BlockedReason = "protection_retained"
		}
	}
	a.mu.Unlock()
	a.sm.markStopped(time.Now()) // C1 (V2-060): туннель остановлен вручную/при закрытии — цикл погас ("" = не наблюдаем)
	if policyRequired && !reason.releasesProtection() {
		a.appendLog("info", "Ядро остановлено; автоматическое снятие WFP запрещено, состояние защиты требует проверки")
	} else {
		a.appendLog("info", "VPN остановлен")
	}
	a.pushState()
	return err
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

// engineLogPathAt — путь engine-лога для tailer'а. Пакетная переменная:
// тесты подменяют на временную директорию (герметичность от AppData).
var engineLogPathAt = engineFileLogPath

// tailerState — межоткрытное состояние tailer'а: на какую позицию файла
// дочитали и какую строку последней доставили (гварды повторов).
type tailerState struct {
	offset int64
	last   string
}

// readNewEngineLines — прочитать строки с позиции st.offset, продвинуть
// оффсет. Частичная строка (ядро ещё пишет) НЕ доставляется и оффсет НЕ
// двигает — дописанный хвост дочитается целиком следующим циклом.
// Файл стал КОРОЧЕ прочитанного (ротация/усечение на рестарте движка,
// V2-046 per-session) = НОВЫЙ файл: оффсет и last-гвард сбрасываются —
// строки новой сессии обязаны дойти, даже если текст совпадает со старой
// (усечение «спрятало бы» перезапуск движка — недопустимо). Честное
// ограничение: перезапись файла той же или большей длины без усечения
// детектом не покрывается (не реальный сценарий записи лога).
// Подряд идущие дубликаты уже доставленного гасятся last-гвардом (штатный
// повтор строки самим ядром при деградации — одна строка, а не шторм).
func readNewEngineLines(path string, st *tailerState) []engineLogMsg {
	f, err := os.Open(path)
	if err != nil {
		return nil // лог ещё не создан/недоступен — честное «новых нет»
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() < st.offset {
		st.offset = 0
		st.last = ""
	}
	if _, err := f.Seek(st.offset, io.SeekStart); err != nil {
		return nil
	}
	var out []engineLogMsg
	deliver := func(line string) {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" || trimmed == st.last {
			return
		}
		if lv := classifyEngineLine(trimmed); lv != "" {
			out = append(out, engineLogMsg{level: lv, text: "движок " + trimmed})
		}
		st.last = trimmed
	}
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, rerr := r.ReadString('\n')
		if rerr != nil {
			// ReadString: err != nil ⟺ строка не кончается '\n' — это
			// частичная запись ядра; держим оффсет на её начале.
			break
		}
		st.offset += int64(len(line))
		deliver(line)
	}
	return out
}

// startEngineLogTailer — хвостовой читатель engine.log (V2-046): sing-box
// пишет в файл (log.output), tailer доставляет новые строки в appendLog.
// Файл переживает рестарты движка — на старте доезжаем до конца (только
// новые строки), при ротации (усечение) начинаем с начала.
//
// V2-054b (P0, найден на живой машине 2026-09-15): прежний tailer ОТКРЫВАЛ
// файл ЗАНОВО каждые 500мс и читал его с нуля — каждое открытие повторно
// клало в журнал ВСЮ историю (за час на busy-машине 57МБ app.log + 67МБ
// app.log.1; одна ошибка — 240+ дублей в UI, владелец видел стену
// «http2: client connection force closed»). Исправление: оффсет между
// открытиями (readNewEngineLines), newReader-семантика для усечения/ротации,
// last-гвард поглощает подряд идущие дубликаты.
func (a *App) startEngineLogTailer() {
	go func() {
		st := &tailerState{}
		for {
			select {
			case <-a.engineLogDone:
				return
			default:
			}
			for _, msg := range readNewEngineLines(engineLogPathAt(), st) {
				a.appendLog(msg.level, msg.text)
			}
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
