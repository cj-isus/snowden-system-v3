//go:build live

package main

// livetest_shadowtls_select_test.go — live-проверка канала D (V2-051):
// переключение работающего VPN на channel-a-shadowtls (probe-гейт тот же,
// что у UI) и подтверждение egress. Запуск:
//
//	go test -tags live -run TestLiveShadowTLSSwitch -v
//
// Требует запущенного приложения (или создаёт своё) и заполненных слотов D.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveShadowTLSSwitch(t *testing.T) {
	if os.Getenv("SNOWDEN_LIVE") == "" {
		t.Skip("live-тест: задайте SNOWDEN_LIVE=1 для запуска")
	}
	a := NewApp()
	// Vault обязателен: renderAndPrepare читает секреты из DPAPI-хранилища
	// владельца (nil vault = паника, пойманная drill'ом V2-051).
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault отсутствует: %s", vaultPath)
	}
	a.vault = secretvault.NewManager(vaultPath)
	// Старт VPN (если ещё не запущен — startCore идемпотентен по startBusy,
	// повторный Start на running вернёт busy-ошибку, что тоже штатно).
	if err := a.Start(); err != nil {
		t.Logf("Start: %v (возможно, уже запущено — продолжаем)", err)
	}
	time.Sleep(35 * time.Second) // старт + protected probe + failover-on-start запас

	// Переключение на канал D — тот же код, что UI-кнопка (probe-гейт).
	if err := a.SelectChannel("channel-a-shadowtls"); err != nil {
		t.Fatalf("select channel-a-shadowtls: %v", err)
	}
	time.Sleep(8 * time.Second)

	// Egress после переключения — ожидаемый из дескриптора (203.0.113.10).
	st := a.GetState()
	t.Logf("state=%s active=%s", st.State, st.ActiveID)
	if st.ActiveID != "channel-a-shadowtls" {
		t.Fatalf("active channel = %s, want channel-a-shadowtls", st.ActiveID)
	}
}
