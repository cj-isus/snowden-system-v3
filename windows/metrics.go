package main

// metrics.go — живые метрики трафика через clash_api ядра (V2-048, F12).
//
// Контракт честного источника (закрывает старый запрет F12 из AGENTS §3.4):
//   - источник данных — сам движок (embedded sing-box), а не выдумка UI:
//     experimental.clash_api на строгом loopback с per-session секретом;
//   - только read-only: GET /connections (снимок счётчиков + соединений).
//     Никаких управляющих вызовов из UI;
//   - «за сессию» = от первого успешного замера текущей сессии ядра, это
//     подписано в UI. Скорость — дельта между опросами.
//
// Сборка без with_clash_api (dev-сборки): clashAPIBuilt=false → блок
// experimental.clash_api не рендерится и метрики честно недоступны
// (Available=false с пояснением), ядро стартует как раньше.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// metricsHTTPTimeout — бюджет одного опроса контроллера. UI поллит каждые 2с;
// зависший контроллер не должен держать биндинг дольше пары секунд.
const metricsHTTPTimeout = 2 * time.Second

// metricsState — состояние сборщика на текущую сессию ядра.
type metricsState struct {
	port   string // "127.0.0.1:9097"
	secret string

	mu        sync.Mutex
	baseUp    int64  // uploadTotal на первом замере сессии
	baseDown  int64  // downloadTotal на первом замере сессии
	lastUp    int64  // прошлый сырой счётчик (для скорости)
	lastDown  int64
	lastAt    time.Time // момент прошлого сырого замера (0 = ещё не было)
	rateUp    float64
	rateDown  float64
	started   bool      // база «за сессию» зафиксирована
}

// metricsView / connView — контракты UI (зеркало в contract.ts).
type MetricsView struct {
	Available   bool      `json:"available"`
	Note        string    `json:"note,omitempty"`
	RateUp      float64   `json:"rateUp"`  // байт/сек
	RateDown    float64   `json:"rateDown"`
	SessionUp   int64     `json:"sessionUp"` // от первого замера сессии
	SessionDown int64     `json:"sessionDown"`
	At          time.Time `json:"at"`
	Connections []ConnView `json:"connections"`
}

type ConnView struct {
	Host      string   `json:"host"`      // домен (или IP:порт, если имени нет)
	Network   string   `json:"network"`   // tcp | udp
	Chain     string   `json:"chain"`     // цепочка outbound'ов ядра
	Process   string   `json:"process"`   // процесс-инициатор (TUN-режим)
	Up        int64    `json:"up"`
	Down      int64    `json:"down"`
	Since     string   `json:"since"`
}

// newMetricsSession — порт+секрет для новой сессии метрик. Порт: SNOWDEN_CLASH_PORT
// (тесты/отладка) или свободный ephemeral на 127.0.0.1. Не удалось получить порт —
// nil (метрики выключены, это честный деград, не ошибка старта).
func newMetricsSession() *metricsState {
	port := ""
	if p := os.Getenv("SNOWDEN_CLASH_PORT"); p != "" {
		if _, err := net.Listen("tcp", "127.0.0.1:"+p); err != nil {
			// занят: не рендерим метрики (ядро упало бы на bind)
			return nil
		}
		port = p
	} else {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil
		}
		port = fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
		_ = l.Close()
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return nil
	}
	return &metricsState{port: "127.0.0.1:" + port, secret: hex.EncodeToString(buf)}
}

// renderOption — WithClashAPI, если сборка с with_clash_api и порт получен.
// Решение о включении — здесь, не в рендере: render-опция остаётся чистой.
func (m *metricsState) renderOption() []render.RenderOption {
	if m == nil || !clashAPIBuilt {
		return nil
	}
	return []render.RenderOption{render.WithClashAPI(portOf(m.port), m.secret)}
}

func portOf(loopbackAddr string) uint16 {
	var p int
	fmt.Sscanf(loopbackAddr, "127.0.0.1:%d", &p)
	return uint16(p)
}

// clashSnapshot — ответ GET /connections (см. trafficontrol Snapshot).
type clashSnapshot struct {
	UploadTotal   int64      `json:"uploadTotal"`
	DownloadTotal int64      `json:"downloadTotal"`
	Connections   []clashConn `json:"connections"`
}

type clashConn struct {
	ID        string `json:"id"`
	Upload    int64  `json:"upload"`
	Download  int64  `json:"download"`
	Start     time.Time `json:"start"`
	Chains    []string  `json:"chains"`
	Metadata  struct {
		Network         string `json:"network"`
		Host            string `json:"host"`
		DestinationIP   string `json:"destinationIP"`
		DestinationPort string `json:"destinationPort"`
		ProcessPath     string `json:"processPath"`
	} `json:"metadata"`
}

// GetMetrics — Wails-биндинг: один опрос контроллера ядра. Доступен всегда,
// но при выключенных метриках честно сообщает об этом (Available=false).
func (a *App) GetMetrics() MetricsView {
	runtime_.mu.Lock()
	m := runtime_.metrics
	runtime_.mu.Unlock()
	if m == nil || !clashAPIBuilt {
		return MetricsView{Available: false, Note: "Метрики выключены: сборка ядра без with_clash_api"}
	}
	view, err := m.poll()
	if err != nil {
		return MetricsView{Available: false, Note: "Ядро не отвечает на метрики: " + err.Error()}
	}
	return view
}

// poll — один HTTP-опрос + пересчёт скоростей/сессийных сумм.
func (m *metricsState) poll() (MetricsView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+m.port+"/connections", nil)
	if err != nil {
		return MetricsView{}, err
	}
	req.Header.Set("Authorization", "Bearer "+m.secret)
	resp, err := metricsHTTPClient.Do(req)
	if err != nil {
		return MetricsView{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MetricsView{}, fmt.Errorf("контроллер вернул %d", resp.StatusCode)
	}
	var snap clashSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return MetricsView{}, err
	}
	return m.apply(snap), nil
}

var metricsHTTPClient = &http.Client{Timeout: metricsHTTPTimeout}

// apply — атомарный пересчёт: скорость (дельта/время), сессийные суммы
// (только положительные дельты — рестарт счётчиков ядра не уводит в минус).
func (m *metricsState) apply(snap clashSnapshot) MetricsView {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		m.started = true
		m.baseUp, m.baseDown = snap.UploadTotal, snap.DownloadTotal
		m.lastUp, m.lastDown, m.lastAt = snap.UploadTotal, snap.DownloadTotal, now
	} else {
		dt := now.Sub(m.lastAt).Seconds()
		if dt > 0.2 { // ниже — отсечка дребезга скорости
			dU, dD := snap.UploadTotal-m.lastUp, snap.DownloadTotal-m.lastDown
			if dU < 0 || dD < 0 {
				// счётчики ядра сбросились (рестарт) — переустановка базы,
				// сессийные суммы сохраняем (факт уже накопленный)
				m.baseUp, m.baseDown = snap.UploadTotal, snap.DownloadTotal
			}
			m.rateUp = float64(max64(dU, 0)) / dt
			m.rateDown = float64(max64(dD, 0)) / dt
		}
		m.lastUp, m.lastDown, m.lastAt = snap.UploadTotal, snap.DownloadTotal, now
	}

	top := snap.Connections
	sort.Slice(top, func(i, j int) bool {
		return top[i].Upload+top[i].Download > top[j].Upload+top[j].Download
	})
	if len(top) > 20 {
		top = top[:20]
	}
	conns := make([]ConnView, 0, len(top))
	for _, c := range top {
		cv := ConnView{
			Network: c.Metadata.Network,
			Up:      c.Upload,
			Down:    c.Download,
			Since:   c.Start.Format("15:04:05"),
			Process: c.Metadata.ProcessPath,
		}
		if h := c.Metadata.Host; h != "" {
			cv.Host = h
		} else {
			cv.Host = net.JoinHostPort(c.Metadata.DestinationIP, c.Metadata.DestinationPort)
		}
		if n := len(c.Chains); n > 0 {
			cv.Chain = c.Chains[0] // ближний к цели outbound — имя канала
		}
		conns = append(conns, cv)
	}
	return MetricsView{
		Available:   true,
		RateUp:      m.rateUp,
		RateDown:    m.rateDown,
		SessionUp:   snap.UploadTotal - m.baseUp,
		SessionDown: snap.DownloadTotal - m.baseDown,
		At:          now,
		Connections: conns,
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
