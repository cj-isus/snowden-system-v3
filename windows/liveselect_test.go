//go:build live

// Живой тест A1.4: SelectChannel — переключение канала через Reload с probe.
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

func TestLiveSelectChannelReload(t *testing.T) {
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault not found: %s", vaultPath)
	}
	a := &App{vault: secretvault.NewManager(vaultPath)}

	cc, err := a.renderAndPrepare("") // default = vless
	if err != nil {
		t.Fatalf("renderAndPrepare: %v", err)
	}
	manager := core.NewManager()
	manager.SetSystemProxy(noopProxy{})
	manager.SetEngine(core.NewEngine(cc.Raw))
	manager.SetProbe(newAppProbe(cc))

	startCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start: %v", err)
	}
	defer func() {
		stopCtx, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		_ = manager.Stop(stopCtx)
	}()
	t.Logf("START OK (default=%s), переключаюсь на hy2...", cc.SelectorDefault)

	// SelectChannel работает через runtime_ приложения — регистрируем
	// менеджер так же, как это делает startCore.
	runtime_.mu.Lock()
	runtime_.manager = manager
	runtime_.cc = cc
	runtime_.mu.Unlock()
	defer func() {
		runtime_.mu.Lock()
		runtime_.manager, runtime_.cc = nil, nil
		runtime_.mu.Unlock()
	}()

	// HY2 в configured — рендер допустим только с pin-PEM; он установлен в
	// AppData/secrets. Reload: новый конфиг + обязательный probe.
	if err := a.SelectChannel("channel-a-hy2"); err != nil {
		t.Fatalf("SelectChannel(channel-a-hy2): %v", err)
	}
	t.Logf("SELECT OK: active=%s", a.state.ActiveID)

	// Соединение живо после переключения: повторный probe (runtime_.cc
	// обновлён в SelectChannel после успешного Reload).
	time.Sleep(2 * time.Second)
	report := newAppProbe(currentRuntimeConfig()).Run(context.Background())
	if !report.AllPassed {
		t.Fatalf("post-select probe FAILED: %s", report.Summary())
	}
	t.Logf("POST-SELECT PROBE OK: %s", report.Summary())
}
