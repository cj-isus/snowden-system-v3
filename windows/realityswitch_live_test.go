//go:build live

package main

// Live-проверка переключения на channel-a-reality (V2-044): SelectChannel
// (тот же путь, что кнопка «Переключиться») → probe-гейт → egress.

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveSwitchToReality(t *testing.T) {
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	a := NewApp()
	a.vault = secretvault.NewManager(vaultPath)

	a.Start() // асинхронный lifecycle: ждём running polling'ом
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if a.GetState().State == "running" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if a.GetState().State != "running" {
		t.Skip("среда: Start не вошёл в running за 120с (сеть агента деградировала vless+hy2) — проверка переключения не имеет смысла")
	}
	defer a.Stop()

	if err := a.SelectChannel("channel-a-reality"); err != nil {
		t.Fatalf("switch to reality: %v", err)
	}

	client := &http.Client{Transport: socksTransport("127.0.0.1:1080"), Timeout: 10 * time.Second}
	for i := 0; i < 3; i++ {
		resp, err := client.Get("https://api.ipify.org?format=json")
		if err != nil {
			t.Fatalf("try%d: %v", i, err)
		}
		buf := make([]byte, 100)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()
		t.Logf("try%d -> %d %q", i, resp.StatusCode, string(buf[:n]))
	}
}
