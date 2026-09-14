//go:build live

// Живой тест TUN через ФЛАГ процесса (--tun), как из UI EnableTUN.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveTUNViaFlag(t *testing.T) {
	if !isAdmin() {
		t.Skip("нужны права администратора")
	}
	os.Args = []string{"app.exe", "--tun"} // эмулируем запуск с флагом
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	a := &App{vault: secretvault.NewManager(vaultPath)}

	cc, err := a.renderAndPrepare("")
	if err != nil {
		t.Fatalf("renderAndPrepare(--tun): %v", err)
	}
	// json.Marshal пишет без пробелов после двоеточий.
	if !strings.Contains(string(cc.Raw), `"interface_name":"snowden0"`) {
		t.Fatalf("флаг --tun должен добавить snowden0 в конфиг; raw фрагмент: %.120s", rawSnippet(cc.Raw))
	}
	manager := core.NewManager()
	manager.SetSystemProxy(noopProxy{}) // как в startCore для TUN
	manager.SetEngine(core.NewEngine(cc.Raw))
	manager.SetProbe(newAppProbe(cc))
	startCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start(TUN via flag): %v", err)
	}
	defer func() {
		stopCtx, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		_ = manager.Stop(stopCtx)
	}()
	out, _ := exec.Command("cmd", "/c", "netsh", "interface", "show", "interface").CombinedOutput()
	if !strings.Contains(strings.ToLower(string(out)), "snowden0") {
		t.Fatalf("snowden0 не найден:\n%s", out)
	}
	t.Logf("TUN VIA FLAG OK: adapter snowden0 active")
	report := newAppProbe(cc).Run(context.Background())
	if !report.AllPassed {
		t.Fatalf("probe FAILED: %s", report.Summary())
	}
	t.Logf("PROBE OK: %s", report.Summary())
}

// rawSnippet — первые 200 байт inbounds-секции для диагностики.
func rawSnippet(raw []byte) string {
	s := string(raw)
	idx := indexOf(s, `"inbounds"`)
	if idx < 0 {
		return s[:min(200, len(s))]
	}
	end := min(idx+200, len(s))
	return s[idx:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
