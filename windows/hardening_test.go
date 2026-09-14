package main

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

// modeSwitchArgs — чистая функция переключения режима. Контракт (V2-019 +
// исправление V2-025): --auto-connect только когда VPN РЕАЛЬНО работал,
// иначе новый процесс молча автоподключался «из ниоткуда».
func TestModeSwitchArgs(t *testing.T) {
	tests := []struct {
		name       string
		enable     bool
		wasRunning bool
		want       []string
	}{
		{"enable TUN while running", true, true, []string{"--tun", "--auto-connect"}},
		{"enable TUN while stopped", true, false, []string{"--tun"}},
		{"back to SOCKS while running", false, true, []string{"--auto-connect"}},
		{"back to SOCKS while stopped", false, false, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modeSwitchArgs(tt.enable, tt.wasRunning)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("modeSwitchArgs(%v,%v) = %v, want %v", tt.enable, tt.wasRunning, got, tt.want)
			}
		})
	}
}

// appendLog/pushState вызываются конкурентно из start/stop/probe/watchdog
// горутин: брутфорс-гонка под -race должна пройти чисто.
func TestAppLogStateConcurrency(t *testing.T) {
	a := NewApp()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(3)
		go func(n int) { defer wg.Done(); a.appendLog("info", "concurrent") }(i)
		go func(n int) { defer wg.Done(); a.pushState() }(i)
		go func(n int) { defer wg.Done(); _ = a.TailLogs() }(i)
	}
	wg.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.logs) == 0 {
		t.Fatal("логи не записаны")
	}
}

// channelViewOf — карточка канала из дескрипторов, а не из хардкода (FR-002).
func TestChannelViewOfFromDescriptors(t *testing.T) {
	cc := &render.ClientConfig{
		ChannelIDs: map[string]string{"channel-a-hy2": "channel-a-hy2"},
		Descriptors: []render.ChannelDescriptor{
			{ID: "channel-a-hy2", Protocol: "hysteria2", Transport: "quic",
				Hostname: "vpn.example.com", Port: 8444, ValidationStatus: "live-verified"},
		},
	}
	v := channelViewOf(cc, "channel-a-hy2")
	if v == nil {
		t.Fatal("known channel must resolve from descriptors")
	}
	if v.Transport != "hysteria2+quic" || v.Port != 8444 || v.Server != "vpn.example.com" {
		t.Fatalf("descriptor fields not propagated: %+v", v)
	}
	if v.Validation != "live-verified" {
		t.Fatalf("validation status must come from descriptor, got %q", v.Validation)
	}
	// Неизвестный канал = честный nil, не выдуманная карточка.
	if got := channelViewOf(cc, "no-such-channel"); got != nil {
		t.Fatalf("unknown channel must give nil view, got %+v", got)
	}
	if got := channelViewOf(nil, "x"); got != nil {
		t.Fatal("nil config must give nil view")
	}
}

// watchdogInterval — разумная постоянная (контракт: не самоддо, но замечаем).
func TestWatchdogIntervalSane(t *testing.T) {
	if watchdogInterval < 10*time.Second || watchdogInterval > 2*time.Minute {
		t.Fatalf("watchdogInterval = %v, вне разумного диапазона 10с..2м", watchdogInterval)
	}
}

// Регрессия аудита 2026-09-11: bindig-методы, которые под a.mu зовут
// appendLog/pushState (внутренне берущие тот же a.mu), само-дедлочились —
// sync.Mutex нерекурсивен. Детектор: метод обязан завершиться за таймаут.
func TestAppBindingMethodsDoNotSelfDeadlock(t *testing.T) {
	a := NewApp()
	a.vault = secretvault.NewManager(filepath.Join(t.TempDir(), "vault.v1.json"))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.ListSecrets()                               // error-путь appendLogLocked
		_, _ = a.SaveSecret("no-such-id", "x")               // err → appendLogLocked
		_, _ = a.AddSecret("custom", "t", "value-1234", "h") // ok/err — обе ветки
		_ = a.DeleteSecret("no-such-id")                     // err
		_, _ = a.VerifySecret("no-such-id")                  // err		_, _, _ = a.TestSecret("no-such-id")    // err
		_, _ = a.RevealSecret("no-such-id")                  // err
		_ = a.runTest("core-wired")                          // appendLogLocked в конце
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("биндинг не завершился за 10с — само-дедлок: appendLog/pushState вызваны под удержанным a.mu")
	}
}

// stopCore при незавершённом Start обязан дождаться горутины, а не объявлять
// stopped под живой старт (регрессия аудита 2026-09-11).
func TestStopCoreWaitsForInFlightStart(t *testing.T) {
	a := NewApp()
	a.vault = secretvault.NewManager(filepath.Join(t.TempDir(), "vault.v1.json"))
	runtime_.mu.Lock()
	runtime_.startBusy = true
	runtime_.mu.Unlock()
	go func() {
		time.Sleep(300 * time.Millisecond)
		runtime_.mu.Lock()
		runtime_.startBusy = false
		runtime_.mu.Unlock()
	}()
	done := make(chan struct{})
	go func() { _ = a.stopCore(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stopCore не дождался startBusy за 5с")
	}
	runtime_.mu.Lock()
	busy := runtime_.startBusy
	runtime_.mu.Unlock()
	if busy {
		t.Fatal("stopCore завершился до снятия startBusy")
	}
}

// ensureWatchdog — ровно один сторож независимо от числа вызовов (раньше
// startup и startCore запускали по тикеру с независимыми guard'ами).
func TestEnsureWatchdogSpawnsOnce(t *testing.T) {
	a := NewApp()
	runtime_.mu.Lock()
	runtime_.watchOn = false
	runtime_.mu.Unlock()
	defer func() {
		runtime_.mu.Lock()
		runtime_.watchOn = false
		runtime_.mu.Unlock()
	}()
	for i := 0; i < 3; i++ {
		a.ensureWatchdog()
	}
	runtime_.mu.Lock()
	watchOn := runtime_.watchOn
	runtime_.mu.Unlock()
	if !watchOn {
		t.Fatal("watchdog должен быть помечен запущенным")
	}
}
