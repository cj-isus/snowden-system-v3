package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — composition root UI↔backend (PLAN.md §4.1).
//
// Статус Phase A2: секреты-хранилище реализовано (DPAPI), lifecycle VPN —
// ещё нет (Manager/Engine не написаны). Соответственно Start/Stop/RunProbe
// возвращают честную ошибку fail-closed, а не имитируют работу: UI показывает
// ошибку, состояние остаётся stopped.
type App struct {
	ctx     context.Context
	vault   *secretvault.Manager
	mu      sync.Mutex
	state   AppState
	logs    []LogLine
	tests   map[string]*bool  // последний результат по id (nil = не запускался)
	tdetail map[string]string // детализация последнего результата
	tsteps  map[string][]ProbeStepView
	tlast   map[string]time.Time
	closing atomic.Bool     // true = идёт контролируемое завершение (окно закрыто)
	fpol    *failoverPolicy // A4: автоматическое переключение каналов (failover.go)
	// logFileCh — асинхронный durable-писатель (V2-034): appendLog под a.mu
	// больше не делает файловый I/O (MkdirAll+Open+write на КАЖДУЮ строку —
	// на всплеске 224 строк это сотни syscall'ов под мьютексом, тормозящих
	// все UI-биндинги). Строки уходят в канал, один писатель батчит их в диск.
	logFileCh   chan LogLine
	logFileDone chan struct{}
	// engineLogDone — сигнал остановки tailer'у engine-лога sing-box (V2-046).
	engineLogDone chan struct{}
	// retry — планировщик авто-повтора после fail-closed (V2-053): растущий
	// backoff 30с→10м, попытка = полный startCore с probe-гейтом. Все поля
	// под a.mu; см. autoretry.go. retryLaunch — точка запуска попытки
	// (nil = реальный startCore; тесты подменяют, чтобы не ходить в сеть).
	retry       retryTimer
	retryLaunch func()
}

// NewApp — конструктор для main.go.
func NewApp() *App {
	return &App{
		tests:         map[string]*bool{},
		tdetail:       map[string]string{},
		tsteps:        map[string][]ProbeStepView{},
		tlast:         map[string]time.Time{},
		fpol:          &failoverPolicy{},
		engineLogDone: make(chan struct{}),
	}
}

// startLogWriter — один фоновый писатель durable-журнала на процесс.
// Буфер 1024 строк: всплеск конкурентного теста (сотни строк за <30мс)
// не блокирует вызывающих; при переполнении строка пропускается честно
// (лог не должен ломать lifecycle — тот же контракт, что у молчаливых
// ошибок записи). Флаш: каждые 250мс или 64 строки, что раньше.
func (a *App) startLogWriter() {
	a.logFileCh = make(chan LogLine, 1024)
	a.logFileDone = make(chan struct{})
	go func() {
		defer close(a.logFileDone)
		const flushEvery = 64
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		pending := make([]LogLine, 0, flushEvery)
		flush := func() {
			if len(pending) == 0 {
				return
			}
			persistLogLines(pending)
			pending = pending[:0]
		}
		for {
			select {
			case line, ok := <-a.logFileCh:
				if !ok {
					flush()
					return
				}
				pending = append(pending, line)
				if len(pending) >= flushEvery {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}

// stopLogWriter — дренировать канал и дождаться финального флаша.
func (a *App) stopLogWriter() {
	if a.logFileCh != nil {
		close(a.logFileCh)
		<-a.logFileDone
		a.logFileCh = nil
	}
}

// maxAppLogSize — предел durable-журнала: при превышении app.log ротируется
// в app.log.1 (хранится одна предыдущая копия). Дефект v2 (V2-052): ротации
// не было, и за несколько дней волн деградации app.log вырос до 15 ГБ —
// риск исчерпания диска на машине агента. Оценка сверху: 2×maxAppLogSize.
var maxAppLogSize int64 = 64 << 20 // 64 MiB

// rotateAppLog — одноразовая ротация при превышении предела. Ошибки молча
// опускаются: лог не должен ломать lifecycle.
func rotateAppLog(dir string) {
	path := filepath.Join(dir, "app.log")
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxAppLogSize {
		return
	}
	_ = os.Rename(path, path+".1") // прошлая .1 перезаписывается Rename'ом
}

// persistLogLines — батч-запись строк durable-журнала (одно открытие файла
// на батч вместо трёх syscall'ов на строку). Ошибки записи молча опускаются:
// лог не должен ломать lifecycle. Секретов в логе нет — appendLog зовётся
// только с человеческими сообщениями (§7).
func persistLogLines(lines []LogLine) {
	if len(lines) == 0 {
		return
	}
	dir := logsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	rotateAppLog(dir)
	path := filepath.Join(dir, "app.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, line := range lines {
		fmt.Fprintf(w, "%s [%s] %s\n", line.T, line.Level, line.Text)
	}
	_ = w.Flush()
}

// startup — сохраняем контекст Wails и готовим хранилище.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.closing.Store(false)

	// Режим dead-man helper kill switch (V2-038): процесс запущен с
	// --killswitch-expire родительским TUN-процессом. Он НЕ GUI: спит expire,
	// снимает блок-политику и выходит — ДО любого другого lifecycle.
	if secs, ok := killSwitchExpireRequested(); ok {
		code := runKillSwitchExpire(time.Duration(secs) * time.Second)
		os.Exit(code)
	}

	// Асинхронный durable-писатель журнала (V2-034) — до любых appendLog.
	a.startLogWriter()

	// Tailer engine-лога sing-box (V2-046) — до первого Start ядра.
	a.startEngineLogTailer()

	// Одноразовый перенос хранилища из build/bin (стирался ребилдом) в AppData.
	migrateLegacyVault()

	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	a.vault = secretvault.NewManager(vaultPath)
	// Предзаполняем типовые слоты (без значений). Ошибка не фатальна:
	// UI покажет список пустым, а конкретная ошибка уйдёт в журнал.
	if err := a.vault.Seed(); err != nil {
		a.appendLog("warn", "seed: "+err.Error())
	}

	// Startup self-heal: если прошлый процесс погиб, не восстановив системный
	// прокси (kill/crash/сон), ProxyEnable=1 указывает на мёртвый loopback —
	// чиним до любых операций с ядром (иначе probe пойдёт в мёртвый прокси).
	if healed, herr := core.HealStaleSystemProxy(); herr != nil {
		a.appendLog("warn", "proxy self-heal: "+herr.Error())
	} else if healed {
		a.appendLog("info", "proxy self-heal: устранён зависший системный прокси после аварийного завершения")
	}

	// Остатки kill switch прошлого сбоя (V2-038): state-файл есть, политика
	// могла остаться (helper умер раньше expire). Снимаем до любого lifecycle;
	// expire-helper прошлого запуска — страховка, если и здесь не выйдет.
	cleanupStaleKillSwitch(a.appendLog)

	a.state = AppState{
		State:        "stopped",
		ProxyMode:    "",
		Active:       nil,
		Probe:        nil,
		Error:        "",
		CoreReady:    false,
		CoreBlockMsg: "Ядро VPN не подключено к Wails. Start/Probe недоступны: это fail-closed, не ошибка UI.",
	}
	a.appendLog("info", "vault: "+vaultPath)
	a.pushState()

	// Системный трей (V2-048/F13): меню = lifecycle + каналы + автозапуск.
	a.startTray()

	// Наблюдатель смены сети (V2-054): up/change при живом туннеле →
	// внеочередной probe; после fail-closed → немедленный авто-ретрай.
	a.startNetWatch()

	// Автоподключение после переключения режима (TUN⇄SOCKS): VPN работал до
	// перезапуска — восстанавливаем состояние без второго нажатия.
	if autoConnectRequested() {
		a.appendLog("info", "автоподключение после переключения режима")
		go a.startCore() // startCore сам запускает сторож (ensureWatchdog)
		return
	}
	// Наблюдение за туннелем: на каждом тике повторный protected probe;
	// деградация защищённого пути → fail-closed Stop (см. corebridge.go).
	a.ensureWatchdog()
}

// ---------- контракт §4.1: состояние и lifecycle ----------

// GetState возвращает текущее фактическое состояние (без выдумок).
// GetSplitDirect — активный список процессов прямого обхода (V2-048/F10,
// read-only: список доставляется подписанным envelope, не редактируется в UI).
func (a *App) GetSplitDirect() []string { return render.SplitDirect() }

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// Start — запуск VPN: полный путь ядра (render → engine → system proxy →
// protected probe). Асинхронно: UI наблюдает переходы через события state.
func (a *App) Start() error {
	a.mu.Lock()
	busy := a.state.State == "starting" || a.state.State == "running" || a.state.State == "stopping"
	a.mu.Unlock()
	if busy {
		return fmt.Errorf("Start: операция уже выполняется (состояние %s)", a.state.State)
	}
	a.mu.Lock()
	a.cancelFailClosedRetryLocked() // ручной запуск важнее автоматики (V2-053)
	a.mu.Unlock()
	go a.startCore()
	return nil
}

// Stop — остановка VPN (синхронно: операция короткая).
func (a *App) Stop() error {
	a.mu.Lock()
	a.cancelFailClosedRetryLocked() // ручная остановка отменяет авто-повтор (V2-053)
	a.mu.Unlock()
	return a.stopCore()
}

// RunProbe — повторный protected probe по работающему туннелю.
func (a *App) RunProbe() error {
	return a.probeCore()
}

// OpenLogsDir открывает папку с журналом приложения.
func (a *App) OpenLogsDir() error {
	dir := logsDir()
	// explorer.exe с папкой; ошибки не роняют приложение.
	return execExplorer(dir)
}

// ---------- TUN-режим: переключение через elevated-перезапуск ----------
//
// Wintun-адаптер требует прав администратора, а принцип минимальных
// привилегий запрещает требовать их на каждый запуск (SOCKS-режим работает
// без elevation). Контракт WIN-TUN-1: TUN доступен только elevated-процессу.
// Поэтому переключение = остановка туннеля → перезапуск exe с --tun (UAC) →
// выход. Новый процесс стартует уже в нужном режиме; владелец нажимает
// «Подключить» (или состояние восстанавливается через автозапуск позже).

// EnableTUN — включить TUN-режим (системный трафик через snowden0).
// Останавливает VPN, перезапускает exe elevated с --tun, завершает процесс.
func (a *App) EnableTUN() error {
	return a.switchMode(true)
}

// DisableTUN — вернуться в SOCKS-режим: перезапуск БЕЗ elevation (--tun
// снимается). VPN останавливается перед перезапуском.
func (a *App) DisableTUN() error {
	return a.switchMode(false)
}

// IsTUNMode — фактический режим этого процесса (для UI-переключателя).
func (a *App) IsTUNMode() bool { return tunRequested() }

func (a *App) switchMode(enable bool) error {
	if enable == tunRequested() {
		return fmt.Errorf("режим уже %s", map[bool]string{true: "TUN", false: "SOCKS"}[enable])
	}
	// 1. Остановить работающий туннель (если был) — два экземпляра на одном
	// SOCKS-порте недопустимы.
	if err := a.stopCore(); err != nil {
		a.appendLog("warn", "переключение режима: остановка: "+err.Error())
	}
	// 2. Перезапуск: TUN требует elevation (UAC), SOCKS — обычный запуск.
	// --auto-connect добавляется только если VPN был запущен до переключения
	// (иначе новый процесс честно остаётся stopped — дефект V2-019:
	// старый код ALWAYS добавлял флаг и молча автоподключался «из ниоткуда»).
	a.mu.Lock()
	wasRunning := a.state.State == "running"
	a.mu.Unlock()
	args := modeSwitchArgs(enable, wasRunning)
	if err := restartWithArgs(enable, args...); err != nil {
		a.appendLog("error", "переключение режима: "+err.Error())
		a.mu.Lock()
		a.state.State = "error"
		a.state.Error = "переключение режима: " + err.Error()
		a.pushStateLocked()
		a.mu.Unlock()
		return err
	}
	a.appendLog("info", "переключение режима: перезапуск "+map[bool]string{true: "в TUN (elevated)", false: "в SOCKS"}[enable])
	a.pushState()
	// Дренировать журнал: перезапуск убьёт процесс, отложенные строки пропали бы.
	a.stopEngineLogTailer()
	a.stopLogWriter()
	// Гарантированный выход (Quit+fallback): проглоченный Quit оставил бы старый
	// процесс живым поверх мьютекса SingleInstance — новый не смог бы стартовать.
	a.requestQuit(300 * time.Millisecond) // окно на доставку событий UI
	return nil
}

// wasRunningFlag — маркер автоподключения для следующего процесса:
// переключение режима не должно требовать второго нажатия «Подключить».
const autoConnectFlag = "--auto-connect"

// autoConnectRequested — флаг из argv (задаётся switchMode, если VPN был
// запущен до переключения режима).
func autoConnectRequested() bool {
	for _, arg := range os.Args[1:] {
		if arg == autoConnectFlag {
			return true
		}
	}
	return false
}

// modeSwitchArgs — аргументы перезапуска при переключении режима (чистая
// функция для юнит-тестов): [--tun, если включаем] + [--auto-connect, если
// VPN был запущен — состояние восстанавливается без второго нажатия].
func modeSwitchArgs(enable, wasRunning bool) []string {
	args := []string{}
	if enable {
		args = append(args, "--tun")
	}
	if wasRunning {
		args = append(args, autoConnectFlag)
	}
	return args
}

// OnBeforeClose — контролируемое завершение при закрытии окна: если туннель
// ещё работает, останавливаем его (restore системного прокси) и только затем
// выходим. Без этого kill-on-close оставлял ProxyEnable=1 на мёртвый порт —
// ровно тот инцидент, от которого защищает HealStaleSystemProxy.
// Выход — через requestQuit, а не голый Quit: live 2026-09-11 Quit от горутины
// был проглочен event-loop'ом и окно пережило закрытие, пока владелец не
// нажал крест второй раз.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.closing.Swap(true) {
		return false // уже завершаемся — не блокируем
	}
	a.mu.Lock()
	a.cancelFailClosedRetryLocked() // закрытие окна — тоже ручное действие (V2-053)
	a.mu.Unlock()
	_ = ctx // выход идёт через a.ctx (см. requestQuit); параметр — контракт Wails
	go func() {
		if err := a.stopCore(); err != nil {
			a.appendLog("warn", "закрытие: остановка туннеля: "+err.Error())
		}
		// Дренировать журнал до выхода: os.Exit в requestQuit пропускает
		// финальный флаш писателя (V2-034).
		a.stopEngineLogTailer()
		a.stopNetWatch()
		a.stopLogWriter()
		a.requestQuit(0)
	}()
	return true // отменить закрытие: приложение закроется из горутины
}

// quitFallbackDelay — сколько ждать после runtime.Quit перед жёстким выходом.
const quitFallbackDelay = 3 * time.Second

// requestQuit — гарантированный выход из GUI-приложения. runtime.Quit, посланный
// из горутины, может быть проглочен event-loop'ом (наблюдаемо live 2026-09-11:
// окно пережило OnBeforeClose → Quit и висло до второго WM_CLOSE; тот же путь
// используется switchMode, где незакрытый процесс держал бы SingleInstance-мьютекс
// и ломал перезапуск в TUN/SOCKS). Поэтому: Quit → короткое окно → os.Exit(0).
// Все критические cleanup-пути (stopCore: движок + restore системного прокси)
// выполняются ДО вызова; os.Exit пропускает только defers, не реестр и не порты.
func (a *App) requestQuit(delay time.Duration) {
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		if a.ctx != nil {
			runtime.Quit(a.ctx)
		}
		// Quit сработал → процесса уже нет и эта строка не выполняется.
		time.Sleep(quitFallbackDelay)
		os.Exit(0)
	}()
}

// ---------- контракт §4.1: тесты ----------

// testIDs — реестр тестов Phase A. Каждый элемент — проверяемый факт.
func testIDs() []struct {
	id   string
	desc string
} {
	return []struct {
		id   string
		desc string
	}{
		{"udp-availability", "UDP доступен в текущей сети (QUIC к 8.8.8.8:443). Определяет, можно ли проверять HY2 здесь."},
		{"hy2-reachability", "UDP-пакеты до канала HY2 (8444/udp) не глохнут на пути. Требует заданный пароль HY2."},
		{"vless-config-present", "Слоты секретов VLESS/HY2 заполнены (значения заданы и проверены локально)."},
		{"elevated-process", "Процесс запущен с правами администратора (нужно для TUN-режима)."},
		{"core-wired", "Ядро VPN (backend/core) подключено к Wails и готово к запуску."},
	}
}

// ListTests — карточки тестов с последними результатами (nil = не запускался).
func (a *App) ListTests() []TestResultView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]TestResultView, 0)
	for _, t := range testIDs() {
		tv := TestResultView{ID: t.id, Detail: t.desc, CheckedAt: ""}
		if ok, exists := a.tests[t.id]; exists {
			tv.OK = ok
			tv.Detail = a.tdetail[t.id]
			if last, l := a.tlast[t.id]; l {
				tv.CheckedAt = last.Format(time.RFC3339)
			}
			tv.Steps = a.tsteps[t.id]
		} else {
			tv.Detail = t.desc
		}
		out = append(out, tv)
	}
	return out
}

// RunTest выполняет один тест и возвращает фактический результат.
func (a *App) RunTest(id string) (TestResultView, error) {
	tv := a.runTest(id)
	return tv, nil
}

func (a *App) runTest(id string) TestResultView {
	var ok bool
	var detail string
	var steps []ProbeStepView
	switch id {
	case "udp-availability":
		ok, detail, steps = testUDPAvailability()
	case "hy2-reachability":
		ok, detail, steps = a.testHY2Reachability()
	case "vless-config-present":
		ok, detail = a.testSecretsPresent()
	case "elevated-process":
		ok, detail = testElevated()
	case "core-wired":
		a.mu.Lock()
		ok = a.state.CoreReady
		detail = a.state.CoreBlockMsg
		a.mu.Unlock()
	default:
		ok = false
		detail = "неизвестный тест: " + id
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	BoolPtr := &ok
	a.tests[id] = BoolPtr
	a.tdetail[id] = detail
	a.tsteps[id] = steps
	a.tlast[id] = now
	level := "info"
	if !ok {
		level = "warn"
	}
	a.appendLogLocked(level, "тест «"+id+"»: "+resultWord(ok)+" — "+detail)
	return TestResultView{ID: id, OK: BoolPtr, Detail: detail, CheckedAt: now.Format(time.RFC3339), Steps: steps}
}

func resultWord(ok bool) string {
	if ok {
		return "пройдено"
	}
	return "не пройдено"
}

// testUDPAvailability — реальная проверка: ждём ответ (любой) на QUIC Initial
// к 8.8.8.8:443. Таймаут 3с. В сети агента это закономерно молчит.
func testUDPAvailability() (bool, string, []ProbeStepView) {
	steps := []ProbeStepView{
		{Name: "QUIC Initial → 8.8.8.8:443", Status: "running", Detail: "ожидание любого ответа, 3 с"},
	}
	success := false
	for attempt := 0; attempt < 2 && !success; attempt++ {
		conn, err := net.DialTimeout("udp", "8.8.8.8:443", 3*time.Second)
		if err != nil {
			continue
		}
		// QUIC v1 Initial: случайные 1200 байт достаточно, чтобы Router/NAT
		// пропустил, а ответ (или его отсутствие) показал фильтрацию UDP.
		payload := make([]byte, 1200)
		if _, err := rand.Read(payload); err == nil {
			_, _ = conn.Write(payload)
			buf := make([]byte, 1500)
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			if n, rerr := conn.Read(buf); rerr == nil && n > 0 {
				success = true
			}
		}
		_ = conn.Close()
	}
	steps[0].Status = map[bool]string{true: "pass", false: "fail"}[success]
	if success {
		return true, "UDP работает: получен ответ на QUIC-пробу (HY2 можно проверять из этой сети)", steps
	}
	return false, "UDP отфильтрован: ответа нет (типично для сети агента). HY2 проверять из сети без UDP-фильтра", steps
}

// testHY2Reachability — UDP-пакет на сервер канала HY2. Секрет не читается:
// проверяется только достижимость порта (ответ/тишина). Адрес и порт — из
// дескрипторов (FR-002), не из хардкода.
//
// V2-042, два урока из live: (1) проба обязана идти на АКТИВНЫЙ канал
// (a.state.ActiveID), а не на «первый hysteria2 по списку»: failover-on-start
// переключает активный канал, и у активного vless цель hy2 ничего не говорит
// о текущем пути (живой туннель через HY2 при «fail» теста — противоречие,
// пойманное в журнале владельца). (2) Hysteria2 сознательно молчит на мусорные
// пакеты (anti-probe по дизайну) — тишина ≠ «порт фильтруется». Доказательство
// достижимости — только транспортный probe самого HY2-канала; UDP-проба —
// утилитарная подсказка, не вердикт.
func (a *App) testHY2Reachability() (bool, string, []ProbeStepView) {
	a.mu.Lock()
	activeID := a.state.ActiveID
	running := a.state.State == "running"
	a.mu.Unlock()

	_, host, port := testEndpoints()
	if activeID != "" {
		if ch := descriptorByID(activeID); ch != nil && ch.Protocol == "hysteria2" {
			host = ch.OriginServer
			port = strconv.Itoa(int(ch.Port))
		}
	}
	steps := []ProbeStepView{
		{Name: "UDP → канал HY2", Status: "running", Detail: fmt.Sprintf("порт %s/udp хоста %s", port, host)},
	}

	if running && activeID != "" && descriptorByID(activeID) != nil && descriptorByID(activeID).Protocol != "hysteria2" {
		// Активен другой канал (например, vless): проба HY2 не описывает
		// текущий путь — честная пометка вместо ложного fail.
		steps[0].Status = "skipped"
		steps[0].Detail = "активен канал " + activeID + " (не HY2) — достижимость HY2 не про текущий путь"
		return true, "пропущено как нерелевантное: сейчас активен " + activeID + ", туннель идёт не через HY2. Достижимость HY2 проверит его собственный probe при переключении", steps
	}

	conn, err := net.DialTimeout("udp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		steps[0].Status = "fail"
		steps[0].Detail = "dial: " + err.Error()
		return false, "UDP-сокет до сервера не создан: " + err.Error(), steps
	}
	defer conn.Close()
	payload := make([]byte, 64)
	_, _ = rand.Read(payload)
	_, _ = conn.Write(payload)
	buf := make([]byte, 1500)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, rerr := conn.Read(buf); rerr == nil && n > 0 {
		steps[0].Status = "pass"
		return true, "сервер ответил на UDP-пробу (путь до HY2 жив; аутентификация проверяется probe) ", steps
	}
	steps[0].Status = "fail"
	steps[0].Detail = "ответа нет за 3 с"
	return false, "тишина на " + port + "/udp: hysteria2 по дизайну молчит на мусорные пакеты (anti-probe), поэтому это НЕ доказательство фильтрации. Доказательство достижимости — транспортный probe канала HY2 (probe pass при активации = путь жив)", steps
}

// descriptorByID — дескриптор канала по ID (nil = нет такого).
func descriptorByID(id string) *render.ChannelDescriptor {
	channels, err := render.LoadDescriptors()
	if err != nil {
		return nil
	}
	for i := range channels {
		if channels[i].ID == id {
			return &channels[i]
		}
	}
	return nil
}

// testSecretsPresent — все ли ключевые слоты заполнены и проверены локально.
func (a *App) testSecretsPresent() (bool, string) {
	metas, err := a.vault.List()
	if err != nil {
		return false, "хранилище недоступно: " + err.Error()
	}
	need := map[secretvault.Kind]bool{
		secretvault.KindVlessUUID:   false,
		secretvault.KindHy2Password: false,
	}
	missing, unverified := 0, 0
	for _, mt := range metas {
		k := secretvault.Kind(mt.Kind)
		if _, req := need[k]; req {
			if mt.StoredValue == "" {
				missing++
			} else if mt.VerifyStatus != "ok" {
				unverified++
			}
		}
	}
	switch {
	case missing > 0:
		return false, fmt.Sprintf("не задано значений: %d (заполните слоты в разделе «Секреты»)", missing)
	case unverified > 0:
		return false, fmt.Sprintf("значений без успешной проверки: %d (нажмите «Проверить» у карточки)", unverified)
	default:
		return true, "ключевые слоты заполнены и проверены локально (актуальность подтвердит probe)"
	}
}

// testElevated — проверка прав администратора (нужно для TUN).
func testElevated() (bool, string) {
	if isAdmin() {
		return true, "процесс Elevated: TUN-режим доступен"
	}
	// V2-042: не-elevated в SOCKS-режиме — ШТАТНОЕ состояние приложения
	// (принцип минимальных привилегий, контракт WIN-TUN-1), а не ошибка.
	// Fail честен только как «TUN сейчас недоступен».
	return false, "процесс без elevation: TUN-режим недоступен (контракт WIN-TUN-1), SOCKS-режим — штатный и работает без прав администратора"
}

// ---------- контракт §4.1: секреты ----------

// ListSecrets — карточки секретов (без значений).
func (a *App) ListSecrets() ([]secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	metas, err := a.vault.List()
	if err != nil {
		a.appendLogLocked("error", "list secrets: "+err.Error())
		return nil, err
	}
	return metas, nil
}

// SaveSecret — сохранить значение в существующий слот.
func (a *App) SaveSecret(id, value string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.vault.Save(id, value); err != nil {
		a.appendLogLocked("warn", "save secret: "+err.Error())
		return secretvault.Meta{}, err
	}
	a.appendLogLocked("info", "secret: значение обновлено (fingerprint изменён, статус проверки сброшен)")
	return a.refetchMeta(id)
}

// AddSecret — создать/обновить секрет по типу (для «своих» значений).
func (a *App) AddSecret(kind, title, value, hint string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	meta, err := a.vault.AddSave(kind, title, value, hint)
	if err != nil {
		a.appendLogLocked("warn", "add secret: "+err.Error())
		return secretvault.Meta{}, err
	}
	a.appendLogLocked("info", "secret: «"+title+"» сохранён (DPAPI)")
	return meta, nil
}

// DeleteSecret — удалить слот.
func (a *App) DeleteSecret(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.vault.Remove(id); err != nil {
		a.appendLogLocked("warn", "delete secret: "+err.Error())
		return err
	}
	a.appendLogLocked("info", "secret: слот удалён")
	return nil
}

// VerifySecret — локальная проверка формата (без сети).
func (a *App) VerifySecret(id string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	meta, err := a.vault.Verify(id)
	if err != nil {
		// Ошибка валидации — нормальный результат проверки, не паника.
		a.appendLogLocked("warn", "verify secret: "+err.Error())
		if meta.ID != "" {
			return meta, err
		}
		return secretvault.Meta{}, err
	}
	a.appendLogLocked("info", "secret: формат значения корректен")
	return meta, nil
}

// ---------- контракт §4.1: живые тесты секретов ----------

// TestSecret — живой тест значения секрета (кнопка «Тест» в карточке):
//   - канальные секреты: сверка SHA256-хешей с deployed-конфигом сервера по SSH;
//   - vps-ssh-key: настоящий вход на сервер ключом из хранилища;
//   - cf-api-token: живой вызов /user/tokens/verify;
//   - custom: честный skip.
//
// Значение не покидает процесс (кроме как по прямому назначению — SSH-подпись
// или заголовок Authorization) и никогда не попадает в логи/UI.
func (a *App) TestSecret(id string) (SecretTestReport, secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rep, err := a.runSecretTest(id)
	if err != nil {
		a.appendLogLocked("warn", "тест секрета: "+err.Error())
		return SecretTestReport{}, secretvault.Meta{}, err
	}
	level := "info"
	word := "пройден"
	if !rep.OK {
		level, word = "warn", "не пройден"
	}
	meta, merr := a.refetchMeta(id)
	if merr == nil {
		a.appendLogLocked(level, fmt.Sprintf("тест секрета «%s» %s: шагов %d", meta.Title, word, len(rep.Steps)))
		return rep, meta, nil
	}
	a.appendLogLocked(level, fmt.Sprintf("тест секрета %s: %s (шагов %d)", id, word, len(rep.Steps)))
	return rep, secretvault.Meta{}, nil
}

// RevealSecret — значение по явному запросу (UI копирует в буфер).
func (a *App) RevealSecret(id string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, err := a.vault.Reveal(id)
	if err != nil {
		a.appendLogLocked("warn", "reveal secret: "+err.Error())
		return "", err
	}
	return v, nil
}

func (a *App) refetchMeta(id string) (secretvault.Meta, error) {
	metas, err := a.vault.List()
	if err != nil {
		return secretvault.Meta{}, err
	}
	for _, m := range metas {
		if m.ID == id {
			return m, nil
		}
	}
	return secretvault.Meta{}, secretvault.ErrNotFound
}

// ---------- журнал ----------

// LogLine — запись журнала для UI.
type LogLine struct {
	T     string `json:"t"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// TailLogs отдаёт последние записи журнала backend.
func (a *App) TailLogs() []LogLine {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]LogLine, len(a.logs))
	copy(out, a.logs)
	return out
}

func (a *App) appendLog(level, text string) {
	a.mu.Lock()
	a.appendLogLocked(level, text)
	a.mu.Unlock()
}

// appendLogLocked — то же под уже удержанным a.mu. sync.Mutex нерекурсивен:
// вызов appendLog из-под Lock() = само-дедлок на той же горутине (ловушка
// найдена аудитом 2026-09-11 в secret-методах, runTest и switchMode). Для
// таких мест вызывающий сам держит a.mu и зовёт Locked-вариант.
func (a *App) appendLogLocked(level, text string) {
	// Ловушка гонки: без мьютекса события уходят в UI до записи в a.logs и
	// TailLogs после «всплеска» событий отдаёт хвост без свежих строк.
	line := LogLine{T: time.Now().Format(time.RFC3339Nano), Level: level, Text: text}
	a.logs = append(a.logs, line)
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	ctx := a.ctx
	if a.logFileCh != nil {
		select {
		case a.logFileCh <- line:
		default:
			// Канал полон (штатно невозможно): строка теряется в памяти,
			// но не в UI и не в lifecycle — лог не критичный путь.
		}
	} else {
		// Писатель ещё не поднят (ранние строки до startup): синхронный фолбэк.
		persistLogLines([]LogLine{line})
	}
	if ctx != nil {
		runtime.EventsEmit(ctx, "log", line)
	}
}

// pushState публикует состояние в UI (снимок под мьютексом: событие не должно
// нести полуобновлённый стейт при конкурентных Start/probe/heal).
func (a *App) pushState() {
	a.mu.Lock()
	a.pushStateLocked()
	a.mu.Unlock()
}

// pushStateLocked — публикация стейта под уже удержанным a.mu (см.
// appendLogLocked: нерекурсивный мьютекс запрещает вложенный Lock).
func (a *App) pushStateLocked() {
	// V2-053: поля авто-ретрая проецируются здесь — state не может разойтись
	// с планировщиком ни на одном push. Пока attempt>0, NextRetryAt берётся
	// из планировщика на КАЖДЫЙ push (отсчёт в UI честный даже при
	// просроченном due — попытка вот-вот). attempt==0 → поля пустые.
	if a.retry.attempt > 0 {
		a.state.RetryAttempt = a.retry.attempt
		if a.retry.due.IsZero() {
			a.state.NextRetryAt = "" // попытка уже стреляет — расписание кончилось
		} else {
			a.state.NextRetryAt = a.retry.due.UTC().Format(time.RFC3339)
		}
	} else {
		a.state.RetryAttempt = 0
		a.state.NextRetryAt = ""
	}
	snapshot := a.state
	ctx := a.ctx
	if ctx != nil {
		runtime.EventsEmit(ctx, "state", snapshot)
	}
}
