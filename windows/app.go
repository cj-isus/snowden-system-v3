package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
	"github.com/snowden-system/windows/backend/update"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — composition root UI↔backend (PLAN.md §4.1).
//
// Намерение защиты независимо от состояния engine; сбой не равен Disconnect.
type App struct {
	ctx                        context.Context
	ctxFence                   core.OperationFence // original startup lifetime, published once under mu
	vault                      *secretvault.Manager
	mu                         sync.Mutex
	state                      AppState
	logs                       []LogLine
	tests                      map[string]*bool  // последний результат по id (nil = не запускался)
	tdetail                    map[string]string // детализация последнего результата
	tsteps                     map[string][]ProbeStepView
	tlast                      map[string]time.Time
	tscope                     map[string]diagnosticScope // fixed known IDs; original completion/intent/path, guarded by mu
	diagnosticContextLast      *DiagnosticContextView     // immutable last presentation tuple, guarded by mu
	diagnosticContextRevision  uint64
	diagnosticContextExhausted bool
	closing                    atomic.Bool     // true = идёт контролируемое завершение (окно закрыто)
	watchdogBlocked            atomic.Bool     // one-way negative task fence; never health or retirement proof
	lifetimeWake               chan struct{}   // immutable cap1 wake for the existing watchdog; never closed
	fpol                       *failoverPolicy // A4: автоматическое переключение каналов (failover.go)
	sm                         controllerSM    // C1: машина состояний адаптивного цикла (controller.go, V2-060)
	// One retained disk writer. Send/close use mu; early/late records stay in
	// bounded memory/UI, never synchronous filesystem work under this lock.
	logWriter   *appLogWriter
	logFileCh   chan LogLine
	logFileDone chan struct{}
	// engineLogDone — сигнал остановки tailer'у engine-лога sing-box (V2-046).
	engineLogDone    chan struct{}
	engineLogRetired chan struct{}         // original tailer return, guarded by mu; never replaced
	engineLogState   *tailerState          // original I/O owner; results only after engineLogRetired
	backgroundProbe  *backgroundProbeTask  // original task, guarded by bgReg.mu; never replaced
	metricsSealed    atomic.Bool           // irreversible terminal admission fence; original I/O stays in metricsPolls
	diagnosticTask   *manualDiagnosticTask // shared explicit GUI diagnostic, guarded by mu; retained until actual return
	// retry — планировщик авто-повтора после fail-closed (V2-053): растущий
	// backoff 30с→10м, попытка = полный startCore с probe-гейтом. Все поля
	// под a.mu; см. autoretry.go. retryLaunch — точка запуска попытки
	// (nil = реальный startCore; тесты подменяют, чтобы не ходить в сеть).
	retry       retryTimer
	retryLaunch func()
	pinWake     *pinActivationWake // one authenticated future Next; guarded by mu
	// operationMu serializes prepare/apply/publication. Other operation fields
	// are protected by a.mu. Stop cancels before waiting for operationMu.
	operationMu             sync.Mutex
	lifecycleOutcome        *appLifecycleOutcome        // original Start/Switch frame only; guarded by operationMu
	updateMu                sync.Mutex                  // one installer/staging owner in this App process
	pairImage               *core.InstalledPairLease    // original handles retained on uncertain close; guarded by updateMu
	pairRoot                *core.InstallationRootLease // original directory/lock handles; guarded by updateMu
	pairTransaction         *update.PairTransaction     // partial staging/pending owner; guarded by updateMu
	pairRecovery            *update.PairRecovery        // forward-only repair writer receipts; guarded by updateMu
	pairRecoveryCheck       *core.UpdateCodeAcceptance  // original companion verification Job/image, held through pair placement
	recoveryEntrypointImage *core.RecoveryImageLease    // headless companion's original self image; guarded by updateMu
	pairArchive             string                      // last retained/uncertain finalization receipt; not authority; guarded by updateMu
	pairAcceptance          *core.UpdateCodeAcceptance  // prospective image + original verification process/Job; guarded by updateMu
	restartGUI              *core.RestartGUIOwner       // ordinary restart original process/Job/image; guarded by operationMu
	elevatedGUI             *core.ElevatedGUIRequest    // original UAC process/Job/image/token request, not child ACK; operationMu
	updateGUISession        *core.UpdateGUISession      // bootstrap lifetime Job/image; fixed before Wails
	updateGUIStorageReady   bool                        // startup recovery result; guarded by mu
	updateGUIDOMStarted     bool                        // one DOM-ready activation; guarded by mu
	updateGUIRendererEpoch  uint64                      // navigation invalidates a waiting renderer receipt; guarded by mu
	updateGUIStartupDone    chan struct{}               // Wails startup and navigation callbacks run concurrently
	restartCommit           *restartHandoff             // committed apply/launch; guarded by mu
	releaseCommit           *protectionReleaseCommit    // committed Disconnect native phase; guarded by mu
	quitRequested           bool                        // explicit terminal request, not cleanup success; guarded by mu
	quitTaskStarted         bool                        // original terminal worker/backstop reservation; guarded by mu
	terminalRetirementErr   error                       // diagnostic only, never readiness or Release; guarded by mu
	closeTask               *appCloseTask               // single original pre-Quit lifecycle worker; guarded by mu
	startupAttempted        bool                        // original synchronous preparation only; guarded by operationMu
	startupErr              error                       // sticky abnormal preparation; guarded by mu
	installedImageChanged   bool                        // sticky for this mapped process after an install mutation attempt
	publicProfiles          publicProfileSourceState
	onboardingCommit        importCommitFn
	operationCancel         context.CancelFunc
	operationCancellation   *appOperationCancellation // exact original constructor/cancel owner; guarded by mu
	operationID             uint64
	intentSource            *core.OperationSource   // original App intent, survives normal operation completion; guarded by mu
	startupIntent           intentRegistration      // immutable initial bootstrap identity; never adopts a later user action
	lastObservation         uint64                  // newest accepted App observation, guarded by a.mu
	lastHealthObservation   uint64                  // deduplicate strong health publication
	nativeObservation       *nativeProbeObservation // optional facts; never a mandatory probe/health watermark
	protectionDesired       bool
	profileTransition       bool             // blocks starts/retries until profile commit or explicit Disconnect
	policy                  protectionPolicy // nil: production WFP; tests inject no OS effects
	probeFactory            func(*render.ClientConfig) core.Probe
	tunnelBoundary          tunnelBoundary          // nil: native guard; custom probe requires explicit boundary
	observedClock           *metadata.ObservedClock // process clock continuity, not authenticated UTC
	trustBlocked            bool                    // clock/profile/storage correction required; guarded by mu
	proxyConflict           bool                    // automatic work denied until an accepted explicit Start; guarded by mu
	uiLifecycle             *uiLifecycleState       // original UI ordering/callback owners; guarded by mu
	updatePreviewBusy       bool                    // one original preview callback; guarded by mu
	updateApproval          *updatePreviewApproval  // one-use target selection only; guarded by mu
	proxyRecovery           *appProxyRecovery       // original preview/callback owner; guarded by mu, I/O by operationMu
	proxyRecoveryCanceledID string                  // one early-cancel tombstone; producer retains its actual pending request
}

// NewApp — конструктор для main.go.
func NewApp() *App {
	initial := core.NewOperationSource()
	return &App{
		lifetimeWake:  make(chan struct{}, 1),
		intentSource:  initial, // initial maintenance identity only; zero cannot start forwarding
		startupIntent: intentRegistration{source: initial},
		tests:         map[string]*bool{},
		tdetail:       map[string]string{},
		tsteps:        map[string][]ProbeStepView{},
		tlast:         map[string]time.Time{},
		fpol:          &failoverPolicy{},
		engineLogDone: make(chan struct{}),
	}
}

// startup — сохраняем контекст Wails и готовим хранилище.
func (a *App) startup(ctx context.Context) {
	profileErr, startServices := a.prepareStartup(ctx)
	if startServices {
		a.startNetworkServices(profileErr, a.startupIntent)
	}
}

// Reuse the lifecycle serializer for synchronous resource acquisition. Close
// either wins before this scope (no startup effects) or cleans up after it.
// Release this lock before network setup/automatic Start can acquire it again.
func (a *App) prepareStartup(ctx context.Context) (profileErr error, startServices bool) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	outcome := &startupOutcome{app: a}
	defer outcome.finish(&profileErr, &startServices)
	profileErr, startServices = a.prepareStartupBody(ctx, outcome)
	outcome.returned = true
	return
}

func (a *App) prepareStartupBody(ctx context.Context, outcome *startupOutcome) (error, bool) {
	a.mu.Lock()
	if a.startupAttempted || a.ctx != nil {
		a.mu.Unlock()
		return nil, false
	}
	a.startupAttempted = true
	outcome.owned = true
	outcome.done = a.updateGUIStartupDone // only this original preparation may close it
	outcome.registration = a.intentRegistrationLocked()
	a.ctx = ctx
	a.ctxFence = core.OperationFence{} // no positive lifetime before original capture returns
	skip := ctx == nil || a.closing.Load() || a.quitRequested
	a.mu.Unlock()
	if skip {
		return nil, false
	}
	fence, err := core.CaptureOperationFence(ctx) // original callbacks outside App.mu, before resources
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false
		}
		a.markStartupFailure(errAppLifetimeUnavailable)
		return errAppLifetimeUnavailable, false
	}
	a.mu.Lock()
	if a.closing.Load() || a.quitRequested || !fence.Current() {
		a.mu.Unlock()
		return nil, false
	}
	a.ctxFence = fence // startupAttempted/operationMu retain this exact original ctx
	a.mu.Unlock()

	// Асинхронный durable-писатель журнала (V2-034) — до любых appendLog.
	a.startLogWriter()

	// The SDK raw logger is disabled at its private configuration boundary.
	// Do not tail an old engine.log into a new App session. Existing files stay
	// untouched; lifecycle/probe producers emit the operational App journal.

	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	vault := secretvault.NewManager(vaultPath)
	a.mu.Lock()
	a.vault = vault
	a.mu.Unlock()
	profileErr := a.recoverOnboardingImport()
	// Migration/seed use the same bound vault writer as UI/import. A failed
	// storage operation must not authorize startup readiness or auto-connect.
	if profileErr != nil {
		a.appendLog("warn", "Восстановление импорта профиля не подтверждено; подключение заблокировано до восстановления профиля.")
	} else {
		profileErr = migrateLegacyVault(a.vault)
		if profileErr != nil {
			a.appendLog("warn", "Перенос локального хранилища не подтверждён; подключение заблокировано до восстановления профиля.")
		} else if profileErr = a.vault.Seed(); profileErr != nil {
			a.appendLog("warn", "Подготовка локального хранилища не подтверждена; подключение заблокировано до восстановления профиля.")
		}
	}

	// A loopback port/dead listener is not an original write receipt. Startup
	// never resets foreign HKCU settings. Cross-process crash recovery remains
	// unavailable until the persistent proxy owner/receipt protocol is accepted.

	// Startup never releases a retained policy, including legacy/unknown state.
	reportRetainedKillSwitch(a.appendLog)

	retained := strictProtectionRetained() // filesystem observation outside value lock
	ready := func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closing.Load() || a.quitRequested || !a.appLifetimeCurrentLocked() {
			return false
		}
		if a.startupIntent.currentLocked(a) {
			// Preserve a Start/Stop that registered while waiting for operationMu.
			a.state = AppState{
				State:        "stopped",
				ProxyMode:    "",
				Active:       nil,
				Probe:        nil,
				Error:        "",
				CoreReady:    false,
				CoreBlockMsg: "Ядро не запущено; подключение проходит обязательную проверку защищённого пути.",
			}
			if retained {
				a.protectionDesired = true
				a.state.State = "error"
				a.state.BlockedReason = "protection_retained"
				a.state.Error = "Сохранено состояние системной защиты. Strict WFP допускает повторное подключение; устаревшее или повреждённое состояние сначала требует явного отключения с правами администратора."
			}
			if profileErr != nil {
				a.state.State = "error"
				a.state.BlockedReason = "profile_recovery_required"
				a.state.Error = "Импорт профиля не завершён; восстановление данных не удалось. Подключение заблокировано до восстановления профиля."
			}
			if a.installedImageChanged {
				a.blockChangedImageLocked()
				a.state.BlockedReason = "update_recovery_required"
				a.state.Error = "Целостность установки не подтверждена. Восстановите полный комплект приложения перед подключением."
			}
		}
		return true
	}()
	if !ready {
		return profileErr, false
	}
	if profileErr == nil {
		a.appendLog("info", "Локальное хранилище подготовлено.")
	}
	a.pushState()

	// Системный трей (V2-048/F13): меню = lifecycle + каналы + автозапуск.
	a.startTray()
	a.mu.Lock()
	defer a.mu.Unlock()
	return profileErr, !a.installedImageChanged && !a.closing.Load() && !a.quitRequested && a.appLifetimeCurrentLocked()
}

// The ordinary startup and a DOM-ready accepted update share these producers.
// A restricted bootstrap must not start them before installation acceptance.
func (a *App) startNetworkServices(profileErr error, registration intentRegistration) {
	if !a.prepareNetworkServices() {
		return
	}

	// Automatic Start acquires the lifecycle serializer itself. Its original
	// registration/closing checks run after producer publication released it.
	if profileErr == nil && autoConnectRequested() {
		if err := a.startForStartupIntent(registration); err == nil {
			a.appendLog("info", "автоподключение после переключения режима")
		}
	}
}

func (a *App) prepareNetworkServices() (ready bool) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	outcome := &startupOutcome{app: a, owned: true}
	a.mu.Lock()
	outcome.registration = a.intentRegistrationLocked()
	a.mu.Unlock()
	var preparationErr error
	defer outcome.finish(&preparationErr, &ready)
	ready = a.prepareNetworkServicesBody()
	outcome.returned = true
	return ready
}

func (a *App) prepareNetworkServicesBody() bool {
	restricted := func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return !a.appLifetimeCurrentLocked() || a.closing.Load() || a.quitRequested || a.installedImageChanged || a.startupErr != nil
	}()
	if restricted {
		return false
	}

	// Наблюдатель смены сети (V2-054): up/change при живом туннеле →
	// внеочередной probe; после fail-closed → немедленный авто-ретрай.
	if err := a.startNetWatch(); err != nil {
		a.appendLog("warn", "наблюдатели сети не подготовлены; исходные задачи сохранены")
		return false
	}
	// Фоновые probe неактивных каналов (D1, V2-063): бюджетная картина
	// «скорее жив / скорее мёртв» для порядка кандидатов failover; решения
	// по-прежнему принимает только probe на переключении (инвариант V2-037).
	if err := a.startBgProbes(); err != nil {
		a.appendLog("warn", "фоновая диагностика не подготовлена; исходная задача сохранена")
	}
	// Сетевые эпохи (V2-057, PLAN A0): журнал эпох живёт на App.
	globalAppForEpoch = a
	// One existing App-lifetime watchdog also restores disconnected observers.
	// Start it before the auto-connect branch can return after a failed Start.
	if err := a.ensureWatchdog(); err != nil {
		a.appendLog("warn", "контроль туннеля пока не подтверждён; исходная задача сохранена")
		return false
	}

	return true // producer publication only, never VPN readiness or Release
}

// ---------- контракт §4.1: состояние и lifecycle ----------

// GetState возвращает текущее фактическое состояние (без выдумок).
// GetSplitDirect returns applied split names while running, otherwise the
// currently loadable configured policy. It performs no network operation.
func (a *App) GetSplitDirect() []string {
	runtime_.mu.Lock()
	cc, manager := runtime_.cc, runtime_.manager
	runtime_.mu.Unlock()
	if cc != nil && manager != nil && manager.State() == core.StateRunning {
		return append([]string{}, cc.SplitDirect...)
	}
	snapshot, err := render.LoadDescriptorSnapshot()
	if err != nil {
		return []string{}
	}
	return append([]string{}, snapshot.SplitDirect...)
}

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stateSnapshotLocked()
}

// Projection only: native callback never calls App/UI code. A later snapshot
// cannot reuse cached Ready while the original power continuity is revoked.
func (a *App) stateSnapshotLocked() AppState {
	snapshot := a.state
	snapshot.DiagnosticContext = a.diagnosticContextLocked()
	if a.watchdogBlocked.Load() {
		snapshot.CoreReady, snapshot.ProbeRunning = false, false
		snapshot.CoreBlockMsg = "Исходная задача контроля туннеля не подтверждена. Нужны очистка и перезапуск приложения."
		if a.protectionDesired && (snapshot.State == "running" || snapshot.State == "starting" || snapshot.State == "reloading") {
			snapshot.State, snapshot.BlockedReason = "error", "cleanup_unconfirmed"
			if snapshot.Error == "" {
				snapshot.Error = snapshot.CoreBlockMsg
			}
		}
	}
	if netEpoch.powerBlocked.Load() {
		snapshot.CoreReady, snapshot.ProbeRunning = false, false
		snapshot.CoreBlockMsg = "Непрерывность системной сессии не подтверждена. Нужны очистка и новый запуск."
		if a.protectionDesired && (snapshot.State == "running" || snapshot.State == "starting" || snapshot.State == "reloading") {
			snapshot.State = "error"
			if snapshot.BlockedReason == "" || snapshot.BlockedReason == "protection_retained" {
				snapshot.BlockedReason = "cleanup_unconfirmed"
			}
			if snapshot.Error == "" {
				snapshot.Error = snapshot.CoreBlockMsg
			}
		}
	}
	if a.proxyConflict && (snapshot.BlockedReason == "" || snapshot.BlockedReason == "protection_retained") {
		snapshot.BlockedReason = "proxy_conflict" // keep the explicit-start requirement visible after manual Stop
	}
	snapshot.LifecycleActions = a.lifecycleActionsLocked(snapshot)
	return snapshot
}

// Start — запуск VPN: полный путь ядра (render → engine → system proxy →
// protected probe). Асинхронно: UI наблюдает переходы через события state.
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.uiLifecycle != nil {
		return errUILifecycleRequest // modern UI cannot bypass ordered request registration
	}
	return a.startLockedForUser(true)
}

// Automatic successor resume must not override a Disconnect/Close/other intent
// that registered while startup/activation was waiting. Same atomic Start path.
func (a *App) startForStartupIntent(expected intentRegistration) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !expected.currentLocked(a) {
		return context.Canceled
	}
	return a.startLocked()
}

// Caller owns mu. Automatic resume cannot acknowledge a foreign proxy change.
func (a *App) startLocked() error {
	return a.startLockedForUser(false)
}

func (a *App) startLockedForUser(explicit bool) error {
	if _, err := a.startRejectionLocked(explicit); err != nil {
		return err
	}
	if err := a.advanceOperationLocked(); err != nil {
		return err
	}
	if explicit {
		a.proxyConflict = false // accepted new intent only; all cleanup/trust/native gates still apply
	}
	a.protectionDesired = true
	a.trustBlocked = false // explicit retry still passes all retained clock/trust gates
	id := a.operationID
	a.state.State = "starting"
	a.cancelFailClosedRetryLocked() // ручной запуск важнее автоматики (V2-053)
	go a.startCoreGeneration(id)
	return nil
}

// Stop — остановка VPN (синхронно: операция короткая).
func (a *App) Stop() error {
	return a.stopCoreFor(stopDisconnect)
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

func (a *App) EnableTUN() error {
	return a.switchMode(true)
}

// DisableTUN — вернуться в SOCKS-режим без нового запроса UAC (--tun снимается).
// Обычный spawn наследует token: снижение привилегий — отдельный открытый gate.
// VPN останавливается перед перезапуском.
func (a *App) DisableTUN() error {
	return a.switchMode(false)
}

// IsTUNMode — фактический режим этого процесса (для UI-переключателя).
func (a *App) IsTUNMode() bool { return tunRequested() }

func (a *App) switchMode(enable bool) error {
	// This is a compiled admission gate, not a caller setting or a probe report.
	// Refuse before Stop/UAC: an unavailable target must not disrupt a usable VPN.
	if err := modeRequestRejection(enable); err != nil {
		return err
	}
	intent, err := func() (intentRegistration, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.uiLifecycle != nil {
			return intentRegistration{}, errUILifecycleRequest
		}
		return a.restartIntentLocked()
	}()
	if err != nil {
		return err
	}
	return a.switchModeRegistered(enable, intent)
}

func (a *App) switchModeRegistered(enable bool, intent intentRegistration) error {
	if err := modeRequestRejection(enable); err != nil {
		return err
	}
	if !a.restartIntentCurrent(intent) {
		return context.Canceled
	}
	if enable == tunRequested() {
		return fmt.Errorf("режим уже %s", map[bool]string{true: "TUN", false: "SOCKS"}[enable])
	}
	// 1. Остановить работающий туннель (если был) — два экземпляра на одном
	// SOCKS-порте недопустимы.
	handoff, err := a.beginRestartHandoff(intent)
	if err != nil {
		a.appendLog("warn", "переключение режима: остановка: "+err.Error())
		return err
	}
	defer handoff.release()
	// 2. Перезапуск: TUN требует elevation (UAC), SOCKS — обычный запуск.
	// --auto-connect добавляется только если VPN был запущен до переключения
	// (иначе новый процесс честно остаётся stopped — дефект V2-019:
	// старый код ALWAYS добавлял флаг и молча автоподключался «из ниоткуда»).
	if err := handoff.commit(); err != nil {
		return err
	}
	// Reuse the existing maintenance operation slot with this exact committed
	// registration. A network-path/operation/lifetime loss is a negative fence,
	// never permission to bind the OS request to a newer global operation.
	ctx, cancel, err := a.beginCommittedUpdate(handoff)
	if err != nil {
		handoff.failed(err)
		return err
	}
	defer a.endOperation(cancel)
	ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
	defer timeout()
	guard := func(ctx context.Context) error {
		if !lifecyclePublicationCurrent(ctx) || !handoff.codeAcceptanceCurrent() {
			return context.Canceled
		}
		if enable && !core.NativeTUNForwardingAvailable() {
			return fmt.Errorf("TUN пока недоступен: системный допуск трафика не принят")
		}
		if enable && !core.ElevatedGUIAvailable() {
			return fmt.Errorf("передача новому процессу с правами администратора недоступна")
		}
		return nil
	}
	launched := false
	if enable {
		// Native request ownership is distinct from an R1 child ACK. TUN
		// remains unavailable until its independent compiled admission gate.
		launched, err = a.launchElevatedModeGUI(ctx, handoff, handoff.request.wasRunning, guard)
	} else {
		launched, err = a.launchOwnedModeGUI(ctx, handoff, false, handoff.request.wasRunning, guard)
	}
	if !launched {
		if err == nil {
			err = core.ErrUpdateGUIBootstrap
		}
		a.appendLog("error", "переключение режима: "+err.Error())
		handoff.failed(err)
		return err
	}
	if err != nil {
		a.appendLog("warn", "переключение режима: передача завершена с поздней ошибкой: "+err.Error())
	}
	a.appendLog("info", "переключение режима: перезапуск "+map[bool]string{true: "в TUN (elevated)", false: "в SOCKS"}[enable])
	a.pushState()
	// Гарантированный выход (Quit+fallback): проглоченный Quit оставил бы старый
	// процесс живым поверх мьютекса SingleInstance — новый не смог бы стартовать.
	a.requestQuit(300 * time.Millisecond) // окно на доставку событий UI
	return err
}

// wasRunningFlag — маркер автоподключения для следующего процесса:
// переключение режима не должно требовать второго нажатия «Подключить».
const autoConnectFlag = "--auto-connect"

// autoConnectRequested — флаг из argv (задаётся switchMode, если VPN был
// запущен до переключения режима).
func autoConnectRequested() bool {
	if activeElevatedGUISession != nil {
		_, resume := activeElevatedGUISession.Mode()
		return resume
	}
	if activeRestartGUISession != nil {
		_, resume := activeRestartGUISession.Mode()
		return resume
	}
	if activeUpdateGUISession != nil {
		_, resume := activeUpdateGUISession.Mode()
		return resume
	}
	for _, arg := range os.Args[1:] {
		if arg == autoConnectFlag {
			return true
		}
	}
	return false
}

// modeSwitchArgs — аргументы перезапуска при переключении режима (чистая
// функция для юнит-тестов): [--tun, если включаем] + [--auto-connect, если
// VPN был запущен — состояние восстанавливается без второго нажатия] +
// [--await-previous — новый процесс обязан дождаться выхода старого:
// иначе Wails SingleInstance молча убивает его (V2-080)].
func modeSwitchArgs(enable, wasRunning bool) []string {
	args := []string{}
	if enable {
		args = append(args, "--tun")
	}
	if wasRunning {
		args = append(args, autoConnectFlag)
	}
	args = append(args, modeSwitchAwaitFlag)
	return args
}

// OnBeforeClose — контролируемое завершение при закрытии окна: если туннель
// ещё работает, останавливаем его (restore системного прокси) и только затем
// выходим. Без этого kill-on-close оставлял ProxyEnable=1 на мёртвый порт —
// без persistent original receipt startup не имеет права автоматически сбросить
// такое значение; crash recovery остаётся отдельной задачей W3b.
// Выход — через requestQuit, а не голый Quit: live 2026-09-11 Quit от горутины
// был проглочен event-loop'ом и окно пережило закрытие, пока владелец не
// нажал крест второй раз.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	a.mu.Lock()
	launch := false
	defer func() {
		a.mu.Unlock()
		if launch {
			a.closeAfterLifecycle()
		}
	}()
	if handoff := a.restartCommit; handoff != nil {
		if handoff.launched {
			return false
		}
		handoff.closeRequested = true
		return true // wait for pending apply/UAC result before ordinary terminal cleanup
	}
	if commit := a.releaseCommit; commit != nil {
		commit.closeRequested = true
		return true // the original native Disconnect phase transfers Close after its terminal result
	}
	if a.quitRequested {
		return false // caller has explicitly entered the terminal Quit phase
	}
	if a.closing.Swap(true) {
		return true // repeated window Close cannot bypass the original cleanup task
	}
	launch = true
	a.cancelFailClosedRetryLocked() // закрытие окна — тоже ручное действие (V2-053)
	// Выход идёт через a.ctx; параметр остаётся частью контракта Wails.
	_ = ctx
	return true // отменить закрытие: приложение закроется из горутины
}

// Closing was registered under App.mu. Shared by the window callback and a
// pending close transferred from an unsuccessful committed restart.
func (a *App) closeAfterLifecycle() {
	a.mu.Lock()
	if a.closeTask != nil || a.quitTaskStarted || a.quitRequested {
		a.mu.Unlock()
		return
	}
	task := &appCloseTask{app: a, done: make(chan struct{})}
	a.closeTask = task // publish the original owner before its goroutine
	a.closing.Store(true)
	a.mu.Unlock()
	go task.run()
}

// Reuse the existing original-task join, with the observer cleanup budget.
// A deadline leaves the same owner pending; only its actual done fence counts.
func joinAppProducer(done <-chan struct{}, unavailable error) error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	return joinAppProducerContext(ctx, done, unavailable)
}

const terminalProducerTimeout = 2 * time.Second

func joinAppProducerContext(ctx context.Context, done <-chan struct{}, unavailable error) error {
	if done == nil {
		return nil // no original task was published
	}
	if ctx == nil {
		return unavailable
	}
	err := joinTerminalNetworkTask(ctx, true, done, unavailable)
	if err != nil {
		select {
		case <-done:
			return nil // final re-read of this original fence after deadline
		default:
		}
	}
	return err
}

// Grace after the producer budget, independent of whether Wails Quit returns.
const quitFallbackDelay = 3 * time.Second

// Callers attempt lifecycle cleanup or settle accepted successor custody first.
// Reserve once before scheduling, then arm an independent backstop before any
// producer/file/native/UI work. No hard scheduler or cleanup bound is inferred;
// the caller's pre-request lifecycle/mutex wait is outside this backstop.
// Terminal permission is not a successful cleanup or browser-tree receipt.
func (a *App) requestQuit(delay time.Duration) {
	a.mu.Lock()
	if a.quitTaskStarted || a.quitRequested {
		a.mu.Unlock()
		return
	}
	a.quitTaskStarted = true
	a.closing.Store(true) // reject any new producer before taking its original owner
	a.mu.Unlock()
	if delay < 0 {
		delay = 0
	}
	fallback := time.NewTimer(delay + terminalProducerTimeout + quitFallbackDelay)
	go func() {
		<-fallback.C
		os.Exit(0) // not conditional on Flush/Close, callback, or Quit returning
	}()
	go a.runQuitTask(delay)
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

// ---------- контракт §4.1: секреты ----------

// ListSecrets — карточки секретов (без значений).
func (a *App) ListSecrets() ([]secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	metas, err := a.vault.List()
	if err != nil {
		a.appendLogLocked("error", "list secrets: список не получен")
		return nil, err
	}
	return metas, nil
}

// SaveSecret — сохранить значение в существующий слот.
func (a *App) SaveSecret(id, value string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invalidateCredentialDiagnosticLocked()
	if err := a.vault.Save(id, value); err != nil {
		a.appendLogLocked("warn", "save secret: результат сохранения не подтверждён")
		return secretvault.Meta{}, err
	}
	a.appendLogLocked("info", "secret: значение обновлено (fingerprint изменён, статус проверки сброшен)")
	return a.refetchMeta(id)
}

// AddSecret — создать/обновить секрет по типу (для «своих» значений).
func (a *App) AddSecret(kind, title, value, hint string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invalidateCredentialDiagnosticLocked()
	meta, err := a.vault.AddSave(kind, title, value, hint)
	if err != nil {
		a.appendLogLocked("warn", "add secret: результат сохранения не подтверждён")
		return secretvault.Meta{}, err
	}
	a.appendLogLocked("info", "secret: слот сохранён в локальном хранилище")
	return meta, nil
}

// DeleteSecret — удалить слот.
func (a *App) DeleteSecret(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invalidateCredentialDiagnosticLocked()
	if err := a.vault.Remove(id); err != nil {
		a.appendLogLocked("warn", "delete secret: удаление не подтверждено")
		return err
	}
	a.appendLogLocked("info", "secret: слот удалён")
	return nil
}

// VerifySecret — локальная проверка формата (без сети).
func (a *App) VerifySecret(id string) (secretvault.Meta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invalidateCredentialDiagnosticLocked()
	meta, err := a.vault.Verify(id)
	if err != nil {
		// Ошибка валидации — нормальный результат проверки, не паника.
		a.appendLogLocked("warn", "verify secret: локальная проверка не подтверждена")
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
// Wails v2 dispatch expects data,error. Metadata is refreshed separately by the
// existing UI store; a third Go result would drop both report and error there.
func (a *App) TestSecret(id string) (report SecretTestReport, resultErr error) {
	task, vault, err := a.beginSecretTest()
	if err != nil {
		return SecretTestReport{}, errSecretTestUnavailable
	}
	outcome := &manualDiagnosticOutcome{app: a, task: task, credentials: true}
	defer outcome.finishSecret(&report, &resultErr)
	report, resultErr = a.testSecretBody(task, vault, id)
	outcome.returned = true
	return
}

func (a *App) testSecretBody(task *manualDiagnosticTask, vault *secretvault.Manager, id string) (SecretTestReport, error) {
	rep, _, err := runSecretTest(task, vault, id)
	if err != nil {
		return SecretTestReport{}, errSecretTestUnavailable
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.diagnosticTask != task || a.vault != vault || a.closing.Load() || task.credentialsChanged.Load() ||
		!task.registration.currentLocked(a) || task.currentError() != nil {
		return SecretTestReport{}, errSecretTestUnavailable
	}
	return rep, nil
}

// RevealSecret — значение по явному запросу (UI копирует в буфер).
func (a *App) RevealSecret(id string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, err := a.vault.Reveal(id)
	if err != nil {
		a.appendLogLocked("warn", "reveal secret: значение не получено")
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
	defer a.mu.Unlock()
	a.appendLogLocked(level, text)
}

// appendLogLocked — то же под уже удержанным a.mu. sync.Mutex нерекурсивен:
// вызов appendLog из-под Lock() = само-дедлок на той же горутине (ловушка
// найдена аудитом 2026-09-11 в secret-методах, runTest и switchMode). Для
// таких мест вызывающий сам держит a.mu и зовёт Locked-вариант.
func (a *App) appendLogLocked(level, text string) {
	level, text = projectAppLogRecord(level, text)
	// Ловушка гонки: без мьютекса события уходят в UI до записи в a.logs и
	// TailLogs после «всплеска» событий отдаёт хвост без свежих строк.
	line := LogLine{T: time.Now().Format(time.RFC3339Nano), Level: level, Text: text}
	a.logs = append(a.logs, line)
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	ctx := a.ctx
	if owner := a.logWriter; owner != nil && a.logFileCh == owner.input {
		select {
		case <-owner.done:
			owner.dropped.Add(1) // failed original writer is retained; no replacement
		default:
			select {
			case owner.input <- line:
			default:
				owner.dropped.Add(1) // memory/UI remain available under disk backpressure
			}
		}
	}
	if ctx != nil {
		runtime.EventsEmit(ctx, "log", line)
	}
}

// pushState публикует состояние в UI (снимок под мьютексом: событие не должно
// нести полуобновлённый стейт при конкурентных Start/probe/heal).
func (a *App) pushState() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pushStateLocked()
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
	snapshot := a.stateSnapshotLocked()
	ctx := a.ctx
	if ctx != nil {
		runtime.EventsEmit(ctx, "state", snapshot)
	}
}
