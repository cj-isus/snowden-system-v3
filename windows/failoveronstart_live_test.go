//go:build live

package main

// Live-проверка failover-on-start (V2-040): в сети, где CDN-путь деградировал
// (V2-039b), Start должен перебрать каналы и остаться на живом (HY2/QUIC).

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveFailoverOnStart(t *testing.T) {
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	a := NewApp()
	a.vault = secretvault.NewManager(vaultPath)

	cc, err := a.renderAndPrepare("")
	if err != nil {
		t.Fatalf("renderAndPrepare: %v", err)
	}
	t.Logf("default=%s (ожидаем перебор при провале)", cc.SelectorDefault)

	if !a.startWithProbe(cc) {
		channels, lerr := render.LoadDescriptors()
		if lerr != nil {
			t.Fatalf("descriptors: %v", lerr)
		}
		others := failoverCandidates(channels, map[string]bool{}, cc.SelectorDefault)
		started := false
		for _, cand := range others {
			candCC, ok := a.startChannelWithProbe(cand)
			if !ok {
				continue
			}
			cc = candCC
			started = true
			break
		}
		if !started {
			t.Fatalf("failover-on-start не нашёл живой канал из %v", others)
		}
		t.Logf("переключился на %s (автоперебор)", cc.SelectorDefault)
	}
	defer a.stopCore()

	// Живой путь: 3 запроса через туннель.
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
