//go:build live

// livetun — TUN-режим end-to-end: рендер с TUN-inbound, полный Start/Stop
// ядра через Manager (реальные secrets из DPAPI-хранилища владельца).
// Требует elevated-запуск (wintun) — из не-elevated среды будет честный fail
// на старте TUN-адаптера.
//
// Запуск: go test -tags "live,with_utls,with_gvisor,with_quic" -run TestLiveTUNStart -v
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveTUNStart(t *testing.T) {
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault not found: %s", vaultPath)
	}
	a := &App{vault: secretvault.NewManager(vaultPath)}

	// 1. Рендер с TUN-inbound (строгий парсер обязан принять). Pin-PEM —
	// тот же поиск, что в приложении (vaultDir() → AppData secrets).
	channels, err := render.LoadDescriptors()
	if err != nil {
		t.Fatalf("descriptors: %v", err)
	}
	src := &vaultSecrets{vault: a.vault}
	if pin := pinForChannel("channel-a-hy2"); pin != "" {
		src.setPin("channel-a-hy2", pin)
	}
	cc, err := render.RenderFrom(channels, src, render.WithTUN())
	if err != nil {
		t.Fatalf("render with TUN: %v", err)
	}

	// 2. Полный запуск ядра (system proxy не трогаем — noop).
	manager := core.NewManager()
	manager.SetSystemProxy(noopProxy{})
	engine := core.NewEngine(cc.Raw)
	manager.SetEngine(engine)
	manager.SetProbe(newAppProbe(cc))

	startCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start (TUN): %v", err)
	}

	// 3. Пост-start probe через живой туннель (контракт отдельного engine-ctx).
	time.Sleep(2 * time.Second)
	report := newAppProbe(cc).Run(context.Background())
	if !report.AllPassed {
		_ = manager.Stop(context.Background())
		t.Fatalf("post-start probe failed: %s", report.Summary())
	}
	t.Logf("TUN start ok: %s", report.Summary())

	// 4. Чистый Stop.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStop()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("manager.Stop: %v", err)
	}
}
