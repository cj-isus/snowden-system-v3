package main

// Тесты netwatch (V2-054): fingerprint-переходы в run-цикле (тайминги
// сжаты переменными), exactly-once авто-ретрая (begin-гвард), маршрутизация
// реакции onNetPathChanged. Сеть в тестах не используется:
// currentNetFingerprint подменяется, реакции — подменённые точки
// netWatchProbeStart/netWatchWakeRetry.

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// stubFingerprinter — подмена сборщика фактов: последовательность на тик.
func stubFingerprinter(t *testing.T, seq []netFingerprint) {
	t.Helper()
	i := 0
	var mu sync.Mutex
	currentNetFingerprint = func() netFingerprint {
		mu.Lock()
		defer mu.Unlock()
		if i < len(seq) {
			fp := seq[i]
			i++
			return fp
		}
		return seq[len(seq)-1] // плато
	}
	t.Cleanup(func() { currentNetFingerprint = netFingerprintOS })
}

func TestNetFingerprintIdentity(t *testing.T) {
	zero := netFingerprint{}
	if zero.present() {
		t.Fatal("нулевой fingerprint не должен быть present")
	}
	a := netFingerprint{ifaceIndex: 9, ip: "192.168.1.2", alias: "Ethernet"}
	if !a.present() {
		t.Fatal("непустой fingerprint должен быть present")
	}
	if !a.equal(netFingerprint{ifaceIndex: 9, ip: "192.168.1.2", alias: "Ethernet"}) {
		t.Fatal("одинаковые факты должны быть equal")
	}
	if a.equal(netFingerprint{ifaceIndex: 9, ip: "192.168.1.3", alias: "Ethernet"}) {
		t.Fatal("смена IP = другая сеть (change)")
	}
	if a.equal(netFingerprint{ifaceIndex: 12, ip: "192.168.1.2", alias: "Mobile"}) {
		t.Fatal("смена адаптера = другая сеть (change)")
	}
}

// stubRunningRuntime — живой туннель в runtime_: onNetPathChanged уходит в
// probe-реакцию (up/change; down — только журнал, хука не имеет).
func stubRunningRuntime(t *testing.T) {
	t.Helper()
	runtime_.mu.Lock()
	runtime_.cc = &render.ClientConfig{}
	runtime_.manager = core.NewManager()
	runtime_.mu.Unlock()
	t.Cleanup(func() {
		runtime_.mu.Lock()
		runtime_.cc = nil
		runtime_.manager = nil
		runtime_.mu.Unlock()
	})
}

// stubFingerprinterSlow — как stubFingerprinter, но каждый факт держится
// hold тиков (переходы разнесены шире debounce — коалесинг не склеит).
func stubFingerprinterSlow(t *testing.T, seq []netFingerprint, hold int) {
	t.Helper()
	calls := 0
	var mu sync.Mutex
	currentNetFingerprint = func() netFingerprint {
		mu.Lock()
		defer mu.Unlock()
		idx := calls / hold
		calls++
		if idx >= len(seq) {
			return seq[len(seq)-1]
		}
		return seq[idx]
	}
	t.Cleanup(func() { currentNetFingerprint = netFingerprintOS })
}

// Сценарий: нет сети → Wi-Fi → LTE (смена) → нет сети → Wi-Fi. Переходы
// разнесены шире debounce — ожидаются 4 доставки (up, change, down, up).
func TestNetWatcherTransitions(t *testing.T) {
	wifi := netFingerprint{ifaceIndex: 9, ip: "192.168.1.2", alias: "Wi-Fi 1"}
	lte := netFingerprint{ifaceIndex: 21, ip: "10.228.1.5", alias: "Mobile Broadband"}

	oldPoll, oldDebounce, oldCap := netwatchPoll, netwatchDebounce, netwatchDebounceCap
	netwatchPoll = 10 * time.Millisecond
	netwatchDebounce = 30 * time.Millisecond
	netwatchDebounceCap = 200 * time.Millisecond
	t.Cleanup(func() { netwatchPoll, netwatchDebounce, netwatchDebounceCap = oldPoll, oldDebounce, oldCap })

	// 5 тиков на факт: переходы каждые ~50мс > debounce 30мс.
	stubFingerprinterSlow(t, []netFingerprint{{}, wifi, wifi, lte, {}, wifi}, 5)

	w := &netWatcher{}
	stop := make(chan struct{})
	done := make(chan struct{})

	// Реакции подменяются счётчиком. down журнально-молчалив (хука не имеет
	// по дизайну), up/change уходят в probe-реакцию при живом туннеле.
	stubRunningRuntime(t)
	var fired atomic.Int32
	oldProbe := netWatchProbeStart
	netWatchProbeStart = func(app *App) { fired.Add(1) }
	t.Cleanup(func() { netWatchProbeStart = oldProbe })
	w.app = &App{} // fire дойдёт до подменённых реакций (и в честный журнал)

	go func() {
		w.run(stop)
		close(done)
	}()
	// 6 фактов × 5 тиков × 10мс + 3 доставки × 30мс debounce + запас.
	time.Sleep(900 * time.Millisecond)
	close(stop)
	<-done

	// up (Wi-Fi), change (LTE), down (журнал), up (Wi-Fi) → 3 probe-реакции.
	if got := fired.Load(); got != 3 {
		t.Fatalf("probe-реакций = %d, want 3 (up, change, up; down — только журнал)", got)
	}
}

// Шторм переходов внутри одного debounce-окна коалесцирует в ОДНО событие
// (последний переход) — flapping Wi-Fi⇄LTE не превращается в самоддо.
func TestNetWatcherCoalescesBurst(t *testing.T) {
	wifi := netFingerprint{ifaceIndex: 9, ip: "192.168.1.2", alias: "Wi-Fi 1"}
	lte := netFingerprint{ifaceIndex: 21, ip: "10.228.1.5", alias: "Mobile Broadband"}

	oldPoll, oldDebounce := netwatchPoll, netwatchDebounce
	netwatchPoll = 10 * time.Millisecond
	netwatchDebounce = 500 * time.Millisecond // окно шире всего сценария
	t.Cleanup(func() { netwatchPoll, netwatchDebounce = oldPoll, oldDebounce })

	// Все 5 фактов за первые 5 тиков (~50мс) — все переходы в одном окне.
	stubFingerprinter(t, []netFingerprint{{}, wifi, lte, {}, wifi, lte})

	stubRunningRuntime(t)
	w := &netWatcher{}
	var fired atomic.Int32
	oldProbe := netWatchProbeStart
	netWatchProbeStart = func(app *App) { fired.Add(1) }
	t.Cleanup(func() { netWatchProbeStart = oldProbe })
	w.app = &App{}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		w.run(stop)
		close(done)
	}()
	// Сценарий 50мс + debounce 500мс + запас.
	time.Sleep(900 * time.Millisecond)
	close(stop)
	<-done

	if got := fired.Load(); got != 1 {
		t.Fatalf("реакций = %d, want 1 (весь шторм склеился в последний переход)", got)
	}
}

// Exactly-once: два подряд wake дают ОДНУ попытку (begin-гвард).
func TestWakeExactlyOnce(t *testing.T) {
	a := &App{}
	a.state.State = "stopped"
	a.state.BlockedReason = "all_channels_failed"
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()

	var launches atomic.Int32
	a.retryLaunch = func() { launches.Add(1) }

	a.wakeFailClosedRetry()
	a.wakeFailClosedRetry() // второе — уже done, no-op
	if got := launches.Load(); got != 1 {
		t.Fatalf("launches = %d, want 1 (exactly-once)", got)
	}
	a.mu.Lock()
	state := a.state.State
	a.mu.Unlock()
	if state != "starting" {
		t.Fatalf("state = %q, want starting", state)
	}
}

// Wake без fail-closed и без активного расписания — no-op.
func TestWakeWithoutScheduleIsNoop(t *testing.T) {
	a := &App{}
	a.state.State = "stopped"
	var launches atomic.Int32
	a.retryLaunch = func() { launches.Add(1) }
	a.wakeFailClosedRetry()
	if launches.Load() != 0 {
		t.Fatal("wake без fail-closed не должен запускать попытку")
	}
	a.state.State = "error" // fail-closed, но расписания нет (cancel был)
	a.wakeFailClosedRetry()
	if launches.Load() != 0 {
		t.Fatal("wake без активного таймера не должен запускать попытку")
	}
}

// Таймер и wake «одновременно»: одна попытка.
func TestTimerAndWakeRaceExactlyOnce(t *testing.T) {
	a := &App{}
	a.state.State = "error"
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()

	var launches atomic.Int32
	a.retryLaunch = func() { launches.Add(1) }
	a.wakeFailClosedRetry()  // begin забрал право
	a.fireFailClosedRetry(1) // таймер: токен совпадает, но done=true
	if got := launches.Load(); got != 1 {
		t.Fatalf("launches = %d, want 1", got)
	}
}

// Провал авто-попытки → failCore → schedule: счётчик растёт (регресс
// V2-053 не должен вернуться после рефакторинга begin).
func TestBackoffCounterStillGrows(t *testing.T) {
	a := &App{}
	a.state.State = "stopped"
	a.state.BlockedReason = "all_channels_failed"
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now()) // attempt 1
	a.cancelFailClosedRetryLocked()             // сброс эпизода
	a.scheduleFailClosedRetryLocked(time.Now()) // новый эпизод → attempt 1
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()
	a.mu.Lock()
	if a.retry.attempt != 1 {
		t.Fatalf("новый эпизод: attempt = %d, want 1", a.retry.attempt)
	}
	a.mu.Unlock()
}

// Живой туннель → probe-реакция, не wake.
func TestOnNetPathChangedLiveTunnel(t *testing.T) {
	a := &App{}
	runtime_.mu.Lock()
	runtime_.cc = &render.ClientConfig{}
	runtime_.manager = core.NewManager()
	runtime_.mu.Unlock()
	t.Cleanup(func() {
		runtime_.mu.Lock()
		runtime_.cc = nil
		runtime_.manager = nil
		runtime_.mu.Unlock()
	})
	var probed atomic.Int32
	oldProbe := netWatchProbeStart
	netWatchProbeStart = func(app *App) { probed.Add(1) }
	t.Cleanup(func() { netWatchProbeStart = oldProbe })

	a.onNetPathChanged()
	if probed.Load() != 1 {
		t.Fatalf("probe-реакция = %d, want 1", probed.Load())
	}
}

// Fail-closed с активным расписанием → wake-реакция.
func TestOnNetPathChangedFailClosed(t *testing.T) {
	a := &App{}
	a.state.State = "stopped"
	a.state.BlockedReason = "all_channels_failed"
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()

	var woken atomic.Int32
	oldWake := netWatchWakeRetry
	netWatchWakeRetry = func(app *App) { woken.Add(1) }
	t.Cleanup(func() { netWatchWakeRetry = oldWake })

	a.onNetPathChanged()
	if woken.Load() != 1 {
		t.Fatalf("wake-реакция = %d, want 1", woken.Load())
	}
}

// Fail-closed БЕЗ активного расписания: полный реальный путь (wake не
// подменён) — begin-гвард требует активный таймер, запуска не должно быть.
func TestOnNetPathChangedFailClosedNoSchedule(t *testing.T) {
	a := &App{}
	a.state.State = "error" // fail-closed, но cancel был — расписания нет
	var launches atomic.Int32
	a.retryLaunch = func() { launches.Add(1) }

	a.onNetPathChanged() // реальный netWatchWakeRetry → wakeFailClosedRetry
	if launches.Load() != 0 {
		t.Fatal("fail-closed без расписания не должен запускать попытку")
	}
}

// Классификация адаптера по ifType/MediaType — чистая функция (факты ОС
// подставляются числами, сети и syscall нет).
func TestClassifyNetClass(t *testing.T) {
	cases := []struct {
		name              string
		ifType, mediaType uint32
		want              string
	}{
		{"wi-fi по ifType", 71, 0, "wifi"},
		{"ethernet по ifType", 6, 0, "ethernet"},
		{"wwan 243", 243, 0, "mobile"},
		{"wwan 244", 244, 0, "mobile"},
		{"wifi по MediaType (ifType=other)", 1, 16, "wifi"},
		{"wwan по MediaType (ifType=other)", 1, 9, "mobile"},
		{"802.3 media (ifType=other)", 1, 0, "ethernet"},
		{"loopback не классифицируется", 24, 0, ""},
		{"tunnel не классифицируется", 131, 0, ""},
		{"неизвестные значения", 999, 999, ""},
	}
	for _, c := range cases {
		if got := classifyNetClass(c.ifType, c.mediaType); got != c.want {
			t.Fatalf("%s: classifyNetClass(%d,%d) = %q, want %q", c.name, c.ifType, c.mediaType, got, c.want)
		}
	}
}

// Классификация переходов — чистая функция.
func TestClassifyTransition(t *testing.T) {
	wifi := netFingerprint{ifaceIndex: 9, ip: "192.168.1.2", alias: "Wi-Fi 1"}
	lte := netFingerprint{ifaceIndex: 21, ip: "10.228.1.5", alias: "Mobile"}
	cases := []struct {
		name     string
		prev, fp netFingerprint
		want     string
	}{
		{"no-net → wifi", netFingerprint{}, wifi, "up"},
		{"wifi → no-net", wifi, netFingerprint{}, "down"},
		{"wifi → lte", wifi, lte, "change"},
		{"wifi → wifi", wifi, wifi, ""},
		{"no-net → no-net", netFingerprint{}, netFingerprint{}, ""},
	}
	for _, c := range cases {
		if got := classifyTransition(c.prev, c.fp); got != c.want {
			t.Fatalf("%s: classifyTransition = %q, want %q", c.name, got, c.want)
		}
	}
}
