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
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// metricsHTTPTimeout is the request/context budget. It does not prove actual
// native dial/socket/body retirement or a hard deadline for the Wails frame.
const metricsHTTPTimeout = 2 * time.Second

// metricsState — состояние сборщика на текущую сессию ядра.
type metricsState struct {
	port   string // "127.0.0.1:9097"
	secret string
	id     string // process-local collector identity; no endpoint/secret or health authority

	mu                   sync.Mutex
	baseUp               int64 // uploadTotal на первом замере сессии
	baseDown             int64 // downloadTotal на первом замере сессии
	lastUp               int64 // прошлый сырой счётчик (для скорости)
	lastDown             int64
	seenUp               int64 // most recent accepted totals, including short rate intervals
	seenDown             int64
	lastAt               time.Time // момент прошлого сырого замера (0 = ещё не было)
	rateUp               float64
	rateDown             float64
	started              bool // база «за сессию» зафиксирована
	counterDiscontinuity bool // no counter epoch protocol: totals stay unknown until a new session
}

// metricsView / connView — контракты UI (зеркало в contract.ts).
type MetricsView struct {
	Collector   string                 `json:"collector,omitempty"`
	Context     *DiagnosticContextView `json:"context,omitempty"` // same value commit, negative display fence only
	Available   bool                   `json:"available"`
	Note        string                 `json:"note,omitempty"`
	RateUp      float64                `json:"rateUp"` // байт/сек
	RateDown    float64                `json:"rateDown"`
	SessionUp   int64                  `json:"sessionUp"` // от первого замера сессии
	SessionDown int64                  `json:"sessionDown"`
	At          time.Time              `json:"at"`
	Connections []ConnView             `json:"connections"`
}

type ConnView struct {
	Host    string `json:"host"`    // домен (или IP:порт, если имени нет)
	Network string `json:"network"` // tcp | udp
	Chain   string `json:"chain"`   // цепочка outbound'ов ядра
	Process string `json:"process"` // процесс-инициатор (TUN-режим)
	Up      int64  `json:"up"`
	Down    int64  `json:"down"`
	Since   string `json:"since"`
}

// newMetricsSession — порт+секрет для новой сессии метрик. Порт: SNOWDEN_CLASH_PORT
// (тесты/отладка) или свободный ephemeral на 127.0.0.1. Не удалось получить порт —
// nil (метрики выключены, это честный деград, не ошибка старта).
func newMetricsSession() *metricsState {
	port := ""
	if p := os.Getenv("SNOWDEN_CLASH_PORT"); p != "" {
		l, err := net.Listen("tcp", "127.0.0.1:"+p)
		if err != nil {
			// занят: не рендерим метрики (ядро упало бы на bind)
			return nil
		}
		_ = l.Close()
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
	return newMetricsState("127.0.0.1:"+port, hex.EncodeToString(buf))
}

// Reconstruct exactly the endpoint/secret applied to the engine, never a
// second random session. The raw bytes remain internal and are not logged.
func metricsForConfig(raw []byte) *metricsState {
	if !clashAPIBuilt {
		return nil
	}
	var cfg struct {
		Experimental struct {
			ClashAPI struct {
				Address string `json:"external_controller"`
				Secret  string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return nil
	}
	c := cfg.Experimental.ClashAPI
	if c.Address == "" || c.Secret == "" {
		return nil
	}
	return newMetricsState(c.Address, c.Secret)
}

var metricsIdentities struct {
	sync.Mutex
	last uint64
}

// A recreated collector gets a new ID even when the same raw config/channel
// is restored. Exhaustion disables metrics instead of wrapping to an old ID.
func newMetricsState(port, secret string) *metricsState {
	metricsIdentities.Lock()
	defer metricsIdentities.Unlock()
	if metricsIdentities.last == ^uint64(0) {
		return nil
	}
	metricsIdentities.last++
	return &metricsState{port: port, secret: secret, id: strconv.FormatUint(metricsIdentities.last, 10)}
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
	UploadTotal   int64       `json:"uploadTotal"`
	DownloadTotal int64       `json:"downloadTotal"`
	Connections   []clashConn `json:"connections"`
}

type clashConn struct {
	ID       string    `json:"id"`
	Upload   int64     `json:"upload"`
	Download int64     `json:"download"`
	Start    time.Time `json:"start"`
	Chains   []string  `json:"chains"`
	Metadata struct {
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
	scope := a.captureMetricsScope()
	if scope == nil {
		return MetricsView{Available: false, Note: "Метрики недоступны: нет текущей сессии или приложение меняет состояние"}
	}
	view, err := scope.metrics.pollOwned(scope.parent, scope)
	if err != nil {
		return MetricsView{Available: false, Note: "Текущий снимок метрик недоступен или не соответствует поддерживаемому формату"}
	}
	// Baseline and view were committed under the same original identity lease.
	// This is an as-of telemetry snapshot, not a transferable health permit.
	return view
}

// poll — один HTTP-опрос + пересчёт скоростей/сессийных сумм.
func (m *metricsState) poll() (MetricsView, error) {
	return m.pollContext(context.Background()) // retained internal caller signature
}

func (m *metricsState) pollContext(parent context.Context) (MetricsView, error) {
	return m.pollOwned(parent, nil) // internal callers still share the process-wide I/O slot
}

// apply — атомарный пересчёт: скорость (дельта/время), сессийные суммы
// Уменьшение счётчика отзывает доступность итогов до новой metrics session.
func (m *metricsState) apply(snap clashSnapshot) MetricsView {
	if !validMetricsSnapshot(snap) {
		return MetricsView{Available: false, Note: "Снимок метрик содержит неподдерживаемые значения"}
	}
	return m.applyPrepared(snap, metricsConnections(snap), time.Now())
}

// Validated snapshot and prepared top20 only. Caller can hold a value-only
// lifecycle lease: no sorting, connection cloning or I/O occurs inside it.
func (m *metricsState) applyPrepared(snap clashSnapshot, conns []ConnView, now time.Time) MetricsView {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counterDiscontinuity {
		return MetricsView{Available: false, Note: "Непрерывность счётчиков этой сессии не подтверждена"}
	}

	if !m.started {
		m.started = true
		m.baseUp, m.baseDown = snap.UploadTotal, snap.DownloadTotal
		m.lastUp, m.lastDown, m.lastAt = snap.UploadTotal, snap.DownloadTotal, now
		m.seenUp, m.seenDown = snap.UploadTotal, snap.DownloadTotal
	} else {
		if snap.UploadTotal < m.seenUp || snap.DownloadTotal < m.seenDown {
			m.counterDiscontinuity = true
			return MetricsView{Available: false, Note: "Счётчики уменьшились; итоги этой сессии недоступны без нового источника"}
		}
		m.seenUp, m.seenDown = snap.UploadTotal, snap.DownloadTotal
		dt := now.Sub(m.lastAt).Seconds()
		if dt > 0.2 { // ниже — отсечка дребезга скорости
			dU, dD := snap.UploadTotal-m.lastUp, snap.DownloadTotal-m.lastDown
			m.rateUp = float64(max64(dU, 0)) / dt
			m.rateDown = float64(max64(dD, 0)) / dt
			m.lastUp, m.lastDown, m.lastAt = snap.UploadTotal, snap.DownloadTotal, now
		}
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

func metricsConnections(snap clashSnapshot) []ConnView {
	top := snap.Connections
	sort.SliceStable(top, func(i, j int) bool {
		return uint64(top[i].Upload)+uint64(top[i].Download) > uint64(top[j].Upload)+uint64(top[j].Download)
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
			Process: c.Metadata.ProcessPath,
		}
		if !c.Start.IsZero() {
			cv.Since = c.Start.Format("15:04:05")
		}
		if h := c.Metadata.Host; h != "" {
			cv.Host = h
		} else if c.Metadata.DestinationIP != "" {
			cv.Host = c.Metadata.DestinationIP
			if c.Metadata.DestinationPort != "" {
				cv.Host = net.JoinHostPort(c.Metadata.DestinationIP, c.Metadata.DestinationPort)
			}
		}
		if n := len(c.Chains); n > 0 {
			cv.Chain = c.Chains[0] // ближний к цели outbound — имя канала
		}
		conns = append(conns, cv)
	}
	return conns
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
