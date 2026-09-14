//go:build live

// Живой e2e-тест реального пути приложения (не упрощённого): хранилище из
// AppData → renderAndPrepare → Manager.Start → protected probe через туннель.
// Значения секретов читаются из DPAPI-хранилища и не печатаются.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveAppStartFlow(t *testing.T) {
	// Хранилище владельца (AppData), как в startup().
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault not found: %s", vaultPath)
	}
	a := &App{vault: secretvault.NewManager(vaultPath)}

	// 1. renderAndPrepare: дескрипторы + DPAPI-секреты → конфиг.
	cc, err := a.renderAndPrepare("")
	if err != nil {
		t.Fatalf("renderAndPrepare: %v", err)
	}
	t.Logf("render ok: selector=%v default=%s expected_egress=%s",
		cc.SelectorTags, cc.SelectorDefault, cc.ExpectedEgress)

	// 2. Реальный Manager.Start (engine → system proxy → probe).
	// Системный прокси подменён на no-op: тест не должен трогать реестр.
	manager := core.NewManager()
	manager.SetSystemProxy(noopProxy{})
	engine := core.NewEngine(cc.Raw)
	manager.SetEngine(engine)
	manager.SetProbe(newAppProbe(cc))

	startCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start: %v", err)
	}
	t.Logf("START OK: state=%s probe=ALL PASSED (egress должен быть %s)", manager.State(), cc.ExpectedEgress)

	// 3. Соединение переживает закрытие operation-контекста (урок P0-2).
	time.Sleep(2 * time.Second)
	report := newAppProbe(cc).Run(context.Background())
	if !report.AllPassed {
		t.Fatalf("post-start probe FAILED (engine ctx bug?): %s", report.Summary())
	}
	t.Logf("POST-START PROBE OK: %s", report.Summary())

	// 4. Чистая остановка.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStop()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("manager.Stop: %v", err)
	}
	t.Logf("STOP OK: state=%s", manager.State())
}
