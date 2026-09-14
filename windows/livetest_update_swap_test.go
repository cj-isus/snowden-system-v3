package main

// updateapp_livetest.go — live-проверка атомарной подмены и перезапуска
// (V2-050/F15). Запускается ТОЛЬКО с тегом live_update:
//
//	go test -tags live_update -run TestLiveUpdateSwap -v
//
// Проверяет ApplyUpdate против УСТАНОВЛЕННОГО приложения: манифест 2.0.1 из
// каталога обновления (payload = тот же exe, подписан update-ключом из
// trusted_keys), подмена exe, старт нового процесса, выход старого.
// Подменяется runningVersion через ldflags при сборке тестового бинаря,
// поэтому проверка политики версий здесь — реальная (2.0.0 → 2.0.1).
// Не юнит-тест: трогает реальную установку и перезапускает приложение.

import (
	"os"
	"testing"
	"time"
)

func TestLiveUpdateSwap(t *testing.T) {
	if runningVersion == "dev" {
		t.Skip("dev-сборка: live-swap осмыслен только с прошитой версией")
	}
	updateDir := os.Getenv("SNOWDEN_UPDATE_DIR")
	if updateDir == "" {
		t.Fatal("задайте SNOWDEN_UPDATE_DIR (каталог с update.json + payload)")
	}
	a := &App{}
	prev, err := a.CheckUpdate(updateDir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !prev.OK {
		t.Fatalf("check отказал: %s", prev.Reason)
	}
	t.Logf("превью ok: %s → %s", prev.CurrentVersion, prev.Version)

	msg, err := a.ApplyUpdate(updateDir)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Log(msg)
	// Процесс уйдёт на перезапуск: даём журналу дожить.
	time.Sleep(2 * time.Second)
}
