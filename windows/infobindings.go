package main

// infobindings.go — биндинги фактов для информационных разделов UI (панель,
// сеть, трафик, профиль доставки, автозащита). Правило фактов (AGENTS.md §3.6):
// нет данных — пустая строка/false/0 + честная причина, никаких выдумок.
// Сеть — из native route/MIB; SSID — bounded system netsh observation,
// egress — из HTTP-наблюдения, профиль доставки — из metadata-стора,
// автозащита — из failoverPolicy.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// NetworkFacts — факты о текущем сетевом подключении хоста.
// Пустая строка = факт недоступен (UI покажет «нет данных»).
type NetworkFacts struct {
	ConnectionType   string              `json:"connectionType"`   // Wi-Fi | Ethernet | "" (неизвестно)
	SSID             string              `json:"ssid"`             // имя Wi-Fi сети (netsh)
	LocalIP          string              `json:"localIP"`          // source address of the locally selected route
	ISP              string              `json:"isp"`              // organization reported by OS-routed HTTPS, not physical ISP identity
	PublicIP         string              `json:"publicIP"`         // OS-routed response; another VPN is not excluded
	TunnelIP         string              `json:"tunnelIP"`         // egress-IP через локальный SOCKS; пусто = путь не доказан
	Country          string              `json:"country"`          // страна egress (echo сервиса)
	CountryOrigin    string              `json:"countryOrigin"`    // источник факта страны
	DNSViaTunnel     bool                `json:"dnsViaTunnel"`     // legacy SOCKS HTTPS outcome, not DNS/TUN path proof
	NetClass         string              `json:"netClass"`         // wifi | ethernet | mobile | "" (V2-054, MediaType фактом)
	CheckedAt        string              `json:"checkedAt"`        // RFC3339
	LocalRouteTarget string              `json:"localRouteTarget"` // local route lookup, not a sent packet
	Egress           *NetworkEgressFacts `json:"egress,omitempty"` // per-route optional observations, not admission
}

// RuntimeInfo — факты именно о запущенном бинарнике. Версия приходит из
// ldflags production-сборки, поэтому UI не дублирует wails.json вручную.
type RuntimeInfo struct {
	Version      string                `json:"version"`
	OS           string                `json:"os"`
	Arch         string                `json:"arch"`
	PID          int                   `json:"pid"`
	CoreVersion  string                `json:"coreVersion"`
	ShellVersion string                `json:"shellVersion"`
	ProcessMode  *ProcessModeFactsView `json:"processMode,omitempty"`
}

// NetStats — route-selected whole-adapter counters, including virtual adapters.
// Route lookup is local; these are not VPN-only counters or throughput.
type NetStats struct {
	InOctets     uint64 `json:"inOctets"`
	OutOctets    uint64 `json:"outOctets"`
	SpeedBps     uint64 `json:"speedBps"`
	Alias        string `json:"alias"` // имя интерфейса
	SourceSchema int    `json:"sourceSchema"`
	Source       string `json:"source"`
	AdapterID    string `json:"adapterId"` // native LUID:index, not protection authority
	RouteTarget  string `json:"routeTarget"`
}

// FailoverStatus — снимок сторожа автозащиты (failoverPolicy, A4).
type FailoverStatus struct {
	Enabled     bool   `json:"enabled"`     // исходная задача вошла в цикл и ещё не остановлена; не freshness/health proof
	State       string `json:"state"`       // closed | open | half-open
	Switches    int    `json:"switches"`    // авто-переключений за текущий инцидент
	LastError   string `json:"lastError"`   // последняя причина (без секретов)
	NextCheckIn int    `json:"nextCheckIn"` // сек до следующей автопопытки (0 = сейчас/не применимо)
	SmState     string `json:"smState"`     // C1 (V2-060): состояние машины адаптивного цикла ("" = нет живого туннеля)
	SmSince     string `json:"smSince"`     // RFC3339: с какого момента в этом состоянии
	SmClass     string `json:"smClass"`     // код THEORY последнего классифицированного отказа ("" = нет)
}

// ---------- single-entry facts cache; local counters bypass it ----------

const factsTTL = 5 * time.Second

var errNetworkFactsUnavailable = errors.New("network facts observation unavailable")

// A cache/publication frame only. Optional HTTP owns its separate original
// resource domain; this token is not a native network-resource receipt.
type factsCollectSource struct {
	app     *App
	parent  context.Context
	path    *networkPathSource
	epoch   int
	tun     bool
	started time.Time
	egress  factsEgressSource
}

func (source *factsCollectSource) current() bool {
	return source != nil && source.app.observationParentCurrent(source.parent) &&
		source.path != nil && source.path.current.Load() && source.tun == tunRequested() && source.egressCurrent()
}

func (source *factsCollectSource) same(other *factsCollectSource) bool {
	return source != nil && other != nil && source.app == other.app && source.parent == other.parent && source.path == other.path && source.epoch == other.epoch && source.tun == other.tun && source.egress == other.egress
}

var factsCache struct {
	mu             sync.Mutex
	at             time.Time
	active, source *factsCollectSource
	facts          NetworkFacts
	err            error
}

// snapshotFacts — свежий (≤TTL) снимок фактов; при устаревании пересобирает.
func (a *App) snapshotFacts(parent context.Context) (facts NetworkFacts, result error) {
	if !a.observationParentCurrent(parent) {
		return facts, errNetworkFactsUnavailable
	}
	netEpoch.mu.Lock()
	path, epoch := netEpoch.pathSourceLocked(), netEpoch.num
	netEpoch.mu.Unlock()
	source := &factsCollectSource{app: a, parent: parent, path: path, epoch: epoch, tun: tunRequested(), started: time.Now()}
	source.captureEgressSource()
	if !source.current() {
		return facts, errNetworkFactsUnavailable
	}
	cached, reserved := false, false
	func() {
		factsCache.mu.Lock()
		defer factsCache.mu.Unlock() // only this short metadata frame owns the lock
		if factsCache.active != nil {
			result = errNetworkFactsUnavailable
			return
		}
		if source.same(factsCache.source) && !factsCache.at.IsZero() && time.Since(factsCache.at) < factsTTL {
			facts, result, cached = factsCache.facts, factsCache.err, true
			return
		}
		factsCache.active, reserved = source, true
	}()
	if cached {
		if !source.current() {
			return NetworkFacts{}, errNetworkFactsUnavailable
		}
		return facts, result
	}
	if !reserved {
		return NetworkFacts{}, errNetworkFactsUnavailable
	}
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			facts, result = NetworkFacts{}, errNetworkFactsUnavailable
		}
		current := returned && source.current()
		factsCache.mu.Lock()
		defer factsCache.mu.Unlock()
		if factsCache.active != source {
			facts, result = NetworkFacts{}, errNetworkFactsUnavailable
			return
		}
		if !returned {
			factsCache.err = errNetworkFactsUnavailable
			return
		} // original interrupted frame retained
		factsCache.active = nil
		if !current {
			factsCache.source, factsCache.at, factsCache.facts, factsCache.err = nil, time.Time{}, NetworkFacts{}, errNetworkFactsUnavailable
			facts, result = NetworkFacts{}, errNetworkFactsUnavailable
			return
		}
		if result != nil {
			facts = NetworkFacts{}
		}
		factsCache.source, factsCache.at, factsCache.facts, factsCache.err = source, source.started, facts, result
	}()
	facts, result = collectFacts(source)
	returned = true
	return facts, result
}

// Value-only historical organization label, never a physical ISP identity.
// No App lock under health/epoch/cache locks; match the requested observed epoch.
func cachedFactsISP(epoch int) string {
	factsCache.mu.Lock()
	source, at, isp, err, active := factsCache.source, factsCache.at, factsCache.facts.ISP, factsCache.err, factsCache.active
	factsCache.mu.Unlock()
	if source == nil || active != nil || err != nil || at.IsZero() || time.Since(at) >= factsTTL ||
		source.epoch != epoch || source.parent.Err() != nil || source.app.closing.Load() || source.path == nil || !source.path.current.Load() || !source.egress.registration.source.Current() || source.egress.control != nil && !source.egress.control.Current() {
		return ""
	}
	return isp
}

// collectFacts — сбор всех сетевых фактов за один проход.
func collectFacts(source *factsCollectSource) (NetworkFacts, error) {
	out := NetworkFacts{CheckedAt: source.started.UTC().Format(time.RFC3339Nano)}
	if runtime.GOOS != "windows" {
		return out, fmt.Errorf("сетевые факты: сбор поддерживается только на Windows")
	}
	parent := context.WithValue(source.parent, networkPathContextKey{}, source.path) // original captured path, never a replacement
	adapter, err := observeAdapterFacts(parent)
	if err != nil {
		return out, fmt.Errorf("сетевые факты: %w", err)
	}
	out.LocalIP, out.NetClass, out.LocalRouteTarget = adapter.address, adapter.netClass, adapter.stats.RouteTarget
	if source.tun && adapter.stats.Alias == tunAdapterName {
		// A request with Proxy:nil is still routed by the TUN adapter. Do not
		// label that route as an independent physical/direct observation.
		out.LocalIP = ""
		out.NetClass = ""
	}
	switch {
	case out.NetClass == "wifi":
		out.ConnectionType = "Wi-Fi"
		if ssid, err := netshShowInterfaces(parent, adapter.stats.Alias); err == nil {
			out.SSID = ssid
		}
	case out.NetClass == "ethernet":
		out.ConnectionType = "Ethernet"
	}
	if !source.current() {
		return NetworkFacts{}, errNetworkFactsUnavailable
	}
	source.collectEgress(&out)
	return out, nil
}

// ---------- биндинги ----------

// GetNetworkFacts — факты о сети хоста для панелей «Активная сеть» / «Сеть».
func (a *App) GetNetworkFacts() (facts NetworkFacts, err error) {
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			facts, err = NetworkFacts{}, errNetworkFactsUnavailable
		}
	}()
	facts, err = a.snapshotFacts(a.observationParent())
	if err == nil {
		facts = facts.withEgressAge(time.Now())
	}
	returned = true
	return facts, err
}

// GetNetStats — local route-selected counters; no egress HTTP/cache wait.
func (a *App) GetNetStats() (NetStats, error) {
	parent := a.observationParent()
	adapter, err := observeAdapterFacts(parent)
	if err != nil || !a.observationParentCurrent(parent) {
		return NetStats{}, errAdapterObservation
	}
	return adapter.stats, nil
}

// GetRuntimeInfo — версия/платформа текущего процесса, а не frontend-константа.
func (a *App) GetRuntimeInfo() RuntimeInfo {
	pid := os.Getpid()
	return RuntimeInfo{
		Version: runningVersion, OS: runtime.GOOS, Arch: runtime.GOARCH, PID: pid, ProcessMode: processModeFacts(pid),
		CoreVersion:  dependencyVersion("github.com/sagernet/sing-box"),
		ShellVersion: dependencyVersion("github.com/wailsapp/wails/v2"),
	}
}

func dependencyVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			if dep.Replace != nil && dep.Replace.Version != "" {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return ""
}

// GetFailoverStatus — снимок сторожа автозащиты + машины состояний C1.
func (a *App) GetFailoverStatus() (FailoverStatus, error) {
	state, switches, lastErr := a.fpol.snapshot()
	sm := a.sm.snapshot()
	watchOn, watchdogErr := a.watchdogStatus()
	nextCheck := a.fpol.nextAttemptIn(time.Now())
	if !watchOn {
		nextCheck = 0
	}
	if watchdogErr != nil {
		lastErr = errWatchdogUnavailable.Error() // local task failure, never remote channel evidence
	}
	smSince := ""
	if !sm.Since.IsZero() {
		smSince = sm.Since.Format(time.RFC3339)
	}
	return FailoverStatus{
		Enabled:     watchOn,
		State:       state,
		Switches:    switches,
		LastError:   lastErr,
		NextCheckIn: nextCheck,
		SmState:     sm.State,
		SmSince:     smSince,
		SmClass:     sm.Class,
	}, nil
}
