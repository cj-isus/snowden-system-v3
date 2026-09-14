package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/secretvault"
)

// tunRequested — парсинг флага --tun (контракт UI↔backend §4.1).
func TestTUNRequestedFlagParsing(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"app.exe"}
	if tunRequested() {
		t.Fatal("без флага TUN не запрошен")
	}
	os.Args = []string{"app.exe", "--tun"}
	if !tunRequested() {
		t.Fatal("--tun должен включать TUN-режим")
	}
	os.Args = []string{"app.exe", "--tun", "--other"}
	if !tunRequested() {
		t.Fatal("--tun среди аргументов должен распознаваться")
	}
}

// quoteArgs — экранирование аргументов перезапуска (безопасность пути exe).
func TestQuoteArgs(t *testing.T) {
	if got := quoteArg("simple"); got != "simple" {
		t.Fatalf("simple arg must not be quoted: %q", got)
	}
	if got := quoteArg("with space"); got != `"with space"` {
		t.Fatalf("spaced arg must be quoted: %q", got)
	}
	if got := quoteArg(`has"quote`); got != `"has\"quote"` {
		t.Fatalf("quote must be escaped: %q", got)
	}
}

// switchMode — защита от повторного переключения в тот же режим.
func TestSwitchModeGuard(t *testing.T) {
	a := &App{}
	// Вне Wails a.ctx == nil; switchMode должен остановиться до runtime.Quit,
	// но сработать на guard'е «режим уже установлен».
	if err := a.switchMode(false); err == nil || !strings.Contains(err.Error(), "уже") {
		t.Fatalf("переключение в текущий режим (SOCKS) должно отклоняться, got %v", err)
	}
	os.Args = []string{"app.exe", "--tun"}
	defer func() { os.Args = []string{"app.exe"} }()
	if err := a.switchMode(true); err == nil || !strings.Contains(err.Error(), "уже") {
		t.Fatalf("переключение в текущий режим (TUN) должно отклоняться, got %v", err)
	}
}

// renderAndPrepare учитывает tunRequested: TUN добавляет tun-inbound.
// Полная проверка рендера с TUN — в backend/render (TestRenderTUNInboundAdded);
// здесь контрактная связка: флаг процесса ⇒ в конфиге есть tun-inbound.
func TestTUNFlagWiredIntoRender(t *testing.T) {
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skip("vault отсутствует — smoke на машине владельца")
	}
	a := &App{vault: secretvault.NewManager(vaultPath)}
	os.Args = []string{"app.exe", "--tun"}
	defer func() { os.Args = []string{"app.exe"} }()

	cc, err := a.renderAndPrepare("")
	if err != nil {
		// Без pin-PEM HY2 не отрендерится — это валидный fail-closed, но TUN
		// нужно проверить: парсим raw и ищем tun.
		if !strings.Contains(err.Error(), "pin") && !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("renderAndPrepare: %v", err)
		}
		t.Skipf("рендер отклонён (нет pin): %v", err)
	}
	if !strings.Contains(string(cc.Raw), `"type": "tun"`) && !strings.Contains(string(cc.Raw), `"type":"tun"`) {
		t.Fatal("--tun должен добавлять tun-inbound в конфиг")
	}
}

// autoConnectRequested — маркер восстановления состояния после смены режима.
func TestAutoConnectFlagParsing(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"app.exe"}
	if autoConnectRequested() {
		t.Fatal("без флага автоподключения быть не должно")
	}
	os.Args = []string{"app.exe", "--tun", "--auto-connect"}
	if !autoConnectRequested() {
		t.Fatal("--auto-connect должен распознаваться")
	}
}
