package main

// infobindings.go — биндинги фактов для информационных разделов UI (панель,
// сеть, трафик, профиль доставки, автозащита). Правило фактов (AGENTS.md §3.6):
// нет данных — пустая строка/false/0 + честная причина, никаких выдумок.
// Значения НЕ хардкодятся: сеть — из ОС (PowerShell/netsh, Windows-сборщик),
// egress — из HTTP-наблюдения, профиль доставки — из metadata-стора,
// автозащита — из failoverPolicy.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

// NetworkFacts — факты о текущем сетевом подключении хоста.
// Пустая строка = факт недоступен (UI покажет «нет данных»).
type NetworkFacts struct {
	ConnectionType string `json:"connectionType"` // Wi-Fi | Ethernet | "" (неизвестно)
	SSID           string `json:"ssid"`           // имя Wi-Fi сети (netsh)
	LocalIP        string `json:"localIP"`        // локальный IPv4 интерфейса по умолчанию
	ISP            string `json:"isp"`            // провайдер прямого канала (пусто = недоступен)
	PublicIP       string `json:"publicIP"`       // egress-IP, видимый из этой машины (прямой)
	Country        string `json:"country"`        // страна egress (echo сервиса)
	CountryOrigin  string `json:"countryOrigin"`  // источник факта страны
	DNSViaTunnel   bool   `json:"dnsViaTunnel"`   // DNS/HTTP реально проходит через 127.0.0.1:1080 (socks5h)
	NetClass       string `json:"netClass"`       // wifi | ethernet | mobile | "" (V2-054, MediaType фактом)
	CheckedAt      string `json:"checkedAt"`      // RFC3339
}

// NetStats — счётчики адаптера маршрута по умолчанию (Traffic-карточка).
// Значения — из Get-NetAdapterStatistics (факт ОС), а не выдумка UI.
type NetStats struct {
	InOctets  uint64 `json:"inOctets"`
	OutOctets uint64 `json:"outOctets"`
	SpeedBps  uint64 `json:"speedBps"`
	Alias     string `json:"alias"` // имя интерфейса
}

// DeliveryProfile — факты доставленного подписанного metadata-envelope (FR-008).
type DeliveryProfile struct {
	Present         bool   `json:"present"`         // подписанный envelope применён (false = встроенный набор)
	Version         uint64 `json:"version"`         // metadata_version принятого envelope
	KeyID           string `json:"keyId"`           // key_id подписанта (16 hex)
	ExpiresAt       string `json:"expiresAt"`       // RFC3339 (действует до)
	TrustedKeys     int    `json:"trustedKeys"`     // размер таблицы доверенных ключей
	ChannelsApplied int    `json:"channelsApplied"` // каналов из обновления (после ревокаций)
	Error           string `json:"error"`           // причина, почему envelope не применён (факт, не гадание)
}

// FailoverStatus — снимок сторожа автозащиты (failoverPolicy, A4).
type FailoverStatus struct {
	Enabled     bool   `json:"enabled"`     // сторож реально тикает (watchdog запущен)
	State       string `json:"state"`       // closed | open | half-open
	Switches    int    `json:"switches"`    // авто-переключений за текущий инцидент
	LastError   string `json:"lastError"`   // последняя причина (без секретов)
	NextCheckIn int    `json:"nextCheckIn"` // сек до следующей автопопытки (0 = сейчас/не применимо)
}

// ---------- кеш фактов (PowerShell-опрос дорогой — не ддосим сами себя) ----------

const factsTTL = 5 * time.Second

var factsCache struct {
	mu      sync.Mutex
	at      time.Time
	refresh bool // идёт сбор прямо сейчас
	facts   NetworkFacts
	stats   NetStats
	err     error
}

// snapshotFacts — свежий (≤TTL) снимок фактов; при устаревании пересобирает.
func snapshotFacts() (NetworkFacts, NetStats, error) {
	factsCache.mu.Lock()
	defer factsCache.mu.Unlock()
	now := time.Now()
	if !factsCache.at.IsZero() && !factsCache.refresh && now.Sub(factsCache.at) < factsTTL {
		return factsCache.facts, factsCache.stats, factsCache.err
	}
	if factsCache.refresh {
		// Сбор уже идёт в другой горутине: отдаём прошлое значение (может быть пустым при первом вызове — это честно).
		return factsCache.facts, factsCache.stats, factsCache.err
	}
	factsCache.refresh = true
	// Сбор под кеш-мьютексом: вызов биндинга редкий (UI-поллинг), а конкурентный
	// двойной PowerShell-опрос дороже, чем короткая блокировка второго читателя.
	factsCache.mu.Unlock()
	facts, stats, err := collectFacts()
	factsCache.mu.Lock()
	factsCache.refresh = false
	factsCache.at = now
	factsCache.facts, factsCache.stats, factsCache.err = facts, stats, err
	return facts, stats, err
}

// collectFacts — сбор всех сетевых фактов за один проход.
func collectFacts() (NetworkFacts, NetStats, error) {
	out := NetworkFacts{CheckedAt: time.Now().Format(time.RFC3339)}
	stats := NetStats{}
	if runtime.GOOS != "windows" {
		return out, stats, fmt.Errorf("сетевые факты: сбор поддерживается только на Windows")
	}
	ip, alias, media, speed, inB, outB, err := defaultAdapterFacts()
	if err != nil {
		return out, stats, fmt.Errorf("сетевые факты: %w", err)
	}
	out.LocalIP = ip
	stats = NetStats{Alias: alias, SpeedBps: speed, InOctets: inB, OutOctets: outB}
	switch {
	case media == "Native802.11":
		out.ConnectionType = "Wi-Fi"
		out.NetClass = "wifi"
		if ssid, err := netshShowInterfaces(); err == nil {
			out.SSID = ssid
		}
	case media == "802.3":
		out.ConnectionType = "Ethernet"
		out.NetClass = "ethernet"
	}
	if out.NetClass == "" {
		// MediaType не распознан (WWAN и прочее) — класс по общей
		// классификации адаптера (V2-054: mobile и т.п.).
		out.NetClass = adapterNetClass(alias)
	}
	out.PublicIP, out.Country, out.CountryOrigin = egressFacts()
	out.DNSViaTunnel = dnsThroughTunnel()
	return out, stats, nil
}

// ---------- биндинги ----------

// GetNetworkFacts — факты о сети хоста для панелей «Активная сеть» / «Сеть».
func (a *App) GetNetworkFacts() (NetworkFacts, error) {
	facts, _, err := snapshotFacts()
	return facts, err
}

// GetNetStats — счётчики трафика адаптера по умолчанию.
func (a *App) GetNetStats() (NetStats, error) {
	_, stats, err := snapshotFacts()
	return stats, err
}

// GetDeliveryProfile — факты доставленного metadata-envelope (FR-008).
func (a *App) GetDeliveryProfile() (DeliveryProfile, error) {
	out := DeliveryProfile{}
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		out.Error = "хранилище метаданных недоступно: " + err.Error()
		return out, err
	}
	store := &metadata.Store{Dir: dir}
	env, present, err := store.LoadEnvelope()
	if err != nil {
		// Повреждённый/неподписанный envelope — факт отказа, UI показывает его.
		out.Error = err.Error()
		return out, nil
	}
	keys, keysPresent, err := store.LoadKeys()
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	out.TrustedKeys = len(keys)
	if !present {
		// Нет envelope — действует встроенный набор: это факт, не ошибка.
		return out, nil
	}
	out.Present = true
	out.Version = env.MetadataVersion
	out.KeyID = env.KeyID
	out.ExpiresAt = env.ExpiresAt.Format(time.RFC3339)
	revoked := map[string]bool{}
	for _, id := range env.Revocations {
		revoked[id] = true
	}
	applied := 0
	for _, c := range env.Channels {
		if !revoked[c.ID] {
			applied++
		}
	}
	out.ChannelsApplied = applied
	if !keysPresent {
		out.Error = "envelope есть, доверенных ключей нет (fail-closed)"
	}
	return out, nil
}

// GetFailoverStatus — снимок сторожа автозащиты.
func (a *App) GetFailoverStatus() (FailoverStatus, error) {
	state, switches, lastErr := a.fpol.snapshot()
	runtime_.mu.Lock()
	watchOn := runtime_.watchOn
	runtime_.mu.Unlock()
	return FailoverStatus{
		Enabled:     watchOn,
		State:       state,
		Switches:    switches,
		LastError:   lastErr,
		NextCheckIn: a.fpol.nextAttemptIn(time.Now()),
	}, nil
}

// ---------- сетевые наблюдения (кроссплатформенные) ----------

// egressFacts — публичный egress-IP и страна: то, что сеть реально видит.
// Источник фиксируется в ответе (честная ссылка на происхождение факта).
func egressFacts() (ip, country, origin string) {
	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://ipapi.co/json/", nil)
	if err != nil {
		return "", "", ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", "", ""
	}
	var payload struct {
		IP          string `json:"ip"`
		CountryName string `json:"country_name"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.IP != "" {
		return payload.IP, payload.CountryName, "https://ipapi.co/json/"
	}
	// Фолбэк: текстовый ответ вида "1.2.3.4".
	if v := string(body); net.ParseIP(trimSpace(v)) != nil {
		return trimSpace(v), "", "https://ipapi.co/ip/"
	}
	return "", "", ""
}

// dnsThroughTunnel — факт работающего пути через туннель: HTTPS через
// socks5h://127.0.0.1:1080 (тот же путь, что у protected-probe: если это
// работает, DNS и egress идут через туннель; если VPN выключен — false).
func dnsThroughTunnel() bool {
	transport := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{Scheme: "socks5h", Host: "127.0.0.1:1080"}),
	}
	client := &http.Client{Transport: transport, Timeout: 6 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64))
	return resp.StatusCode == http.StatusOK
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
