//go:build live

// Живой тест матрицы сайтов ПОВЕРХ УЖЕ РАБОТАЮЩЕГО туннеля (владелец нажал
// «Подключить», порт 1080 слушает). Без Start/Stop — только проверка сайтов.
// Так тест не конкурирует с UI-кнопкой и не поднимает второй движок.
//
// Запуск: go test -tags "live,with_utls,with_gvisor,with_quic" -run TestLiveMultiSiteAttach -v
package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestLiveMultiSiteAttach(t *testing.T) {
	// 1. Туннель жив? (probe по уже отрендеренному конфигу из дескрипторов).
	channels, err := render.LoadDescriptors()
	if err != nil {
		t.Fatalf("descriptors: %v", err)
	}
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault not found: %s", vaultPath)
	}
	src := &vaultSecrets{vault: secretvault.NewManager(vaultPath)}
	if pin := pinForChannel("channel-a-hy2"); pin != "" {
		src.setPin("channel-a-hy2", pin)
	}
	cc, err := render.RenderFrom(channels, src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	probe := newAppProbe(cc)
	rep := probe.Run(context.Background())
	if !rep.AllPassed {
		t.Skipf("туннель не жив (владельцу: нажать «Подключить»): %s", rep.Summary())
	}
	t.Logf("туннель жив: %s", rep.Summary())

	// 2. Матрица — тот же список сайтов, что в livestart2 (siteMatrix/checkSite).
	client := &http.Client{Transport: socksTransport("127.0.0.1:1080"), Timeout: 25 * time.Second}
	client11 := socksHTTP11Client()

	type result struct {
		site site
		ok   bool
		det  string
		dur  time.Duration
	}
	var results []result
	passCnt, failCnt := 0, 0
	start := time.Now()

	for _, s := range siteMatrix() {
		st := time.Now()
		ok, det := checkSite(client, client11, s)
		dur := time.Since(st)
		results = append(results, result{s, ok, det, dur})
		status := "PASS"
		if !ok {
			status = "FAIL"
			failCnt++
		} else {
			passCnt++
		}
		t.Logf("%-4s %-22s %-12s %-8s %5.1fs  %s", status, s.name, s.category, s.country, dur.Seconds(), det)
	}

	t.Logf("итог: %d pass / %d fail за %s", passCnt, failCnt, time.Since(start).Round(time.Second))
	if failCnt > 0 {
		t.Logf("честная граница: часть целей может быть недоступна из этой сети (класс V2-026: return-путь)")
	}
}
