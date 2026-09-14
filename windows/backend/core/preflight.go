package core

// preflight.go — резолв адресов туннельных серверов ДО старта движка (V2-037).
//
// Проблема (docs/CENSORSHIP-THEORY.md §2.1): единственный bootstrap-DNS в
// конфиге клиента (tls://1.1.1.1, порт 853, direct) — тихая точка отказа в
// сетях с фильтрацией DoT/DoH (типично для РФ): домен туннеля не резолвится
// → канал A (default) не стартует вообще, при полностью рабочем CDN-пути.
//
// Решение: мультистратегийный резолв в НАШЕМ коде (политикой владеет наш
// контроллер, AGENTS §2.2), результат — IP-диал в рендере (server=IP,
// SNI/Host=домен за CDN). Стратегии параллельно, побеждает первая:
//  1. plain UDP DNS к флоту резолверов (независимые друг от друга);
//  2. DoH JSON по IP-литералу (без порта 853 и без резолва эндпоинта);
//  3. кэш последнего хорошего IP (TTL 7 дней) — на случай «всё фильтруют».
//
// Подмена ответа не опасна: атакующий IP всё равно обязан предъявить
// валидный TLS-сертификат домена туннеля (uTLS/стандартная проверка),
// иначе соединение падает до любых данных; итоговый контроль — protected
// probe (expected egress). Это доступность, а не новое доверие.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// preflightUDPResolvers — флот plain-UDP резолверов: независимые провайдеры,
// включая доступные из РФ (Yandex/AdGuard) — резолв НЕ заблокированного
// домена за CDN любой из них выполняет корректно.
var preflightUDPResolvers = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
	"77.88.8.8:53",    // Yandex DNS
	"94.140.14.14:53", // AdGuard
}

// preflightDoHEndpoints — DoH JSON API по IP-литералу (без DNS-зависимости).
var preflightDoHEndpoints = []struct {
	ip   string
	path string
}{
	{"1.1.1.1", "/dns-query"}, // Cloudflare, accept: application/dns-json
	{"8.8.8.8", "/resolve"},   // Google JSON API
}

// PreflightResolver — см. комментарий файла. Потокобезопасен.
type PreflightResolver struct {
	// CachePath — файл кэша {host: {ip, at}}; "" = без кэша.
	CachePath string
	// Timeout — бюджет всей операции (все стратегии параллельно).
	Timeout time.Duration
	// Logf — необязательный журнал фактов (стратегия/источник).
	Logf func(format string, args ...any)
}

const preflightCacheTTL = 7 * 24 * time.Hour

type preflightCacheEntry struct {
	IP string    `json:"ip"`
	At time.Time `json:"at"`
}

func (r *PreflightResolver) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

// Resolve возвращает IPv4 для host. cached=true, если IP взят из кэша
// (сети недоступны для всех стратегий). Ошибка = не решили никак.
func (r *PreflightResolver) Resolve(ctx context.Context, host string) (ip string, cached bool, err error) {
	if net.ParseIP(host) != nil {
		return host, false, nil // уже IP-литерал (origin-серверы дескрипторов)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		ip       string
		strategy string
	}
	done := make(chan result, 1)

	var wg sync.WaitGroup
	launch := func(strategy string, fn func() (string, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip, err := fn()
			if err == nil && ip != "" {
				select {
				case done <- result{ip, strategy}:
				default:
				}
			}
		}()
	}
	for _, server := range preflightUDPResolvers {
		server := server
		launch("udp:"+server, func() (string, error) { return r.udpResolve(ctx, server, host) })
	}
	for _, ep := range preflightDoHEndpoints {
		ep := ep
		launch("doh:"+ep.ip, func() (string, error) { return r.dohResolve(ctx, ep.ip, ep.path, host) })
	}
	go func() {
		wg.Wait()
		select {
		case done <- result{}: // все стратегии промолчали
		default:
		}
	}()

	select {
	case <-ctx.Done():
		// Бюджет исчерпан: кэш как последний рубеж.
		if ip, ok := r.cacheGet(host); ok {
			r.logf("preflight: %s → %s (кэш, стратегии не успели)", host, ip)
			return ip, true, nil
		}
		return "", false, fmt.Errorf("preflight resolve %s: %w", host, ctx.Err())
	case res := <-done:
		if res.ip != "" {
			r.cachePut(host, res.ip)
			r.logf("preflight: %s → %s (%s)", host, res.ip, res.strategy)
			return res.ip, false, nil
		}
		// Все стратегии ответили пусто: кэш.
		if ip, ok := r.cacheGet(host); ok {
			r.logf("preflight: %s → %s (кэш, стратегии молчат)", host, ip)
			return ip, true, nil
		}
		return "", false, fmt.Errorf("preflight resolve %s: все стратегии не ответили", host)
	}
}

// udpResolve — A-запись через конкретный сервер (PreferGo + кастомный Dial:
// резолвер обязан спрашивать ИМЕННО этот сервер, не системный).
func (r *PreflightResolver) udpResolve(ctx context.Context, server, host string) (string, error) {
	res := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "udp", server)
		},
	}
	ips, err := res.LookupHost(ctx, host)
	if err != nil {
		return "", err
	}
	return firstPublicIPv4(ips)
}

// dohResolve — DoH JSON API по IP-литералу.
func (r *PreflightResolver) dohResolve(ctx context.Context, ip, path, host string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("https://%s%s?name=%s&type=A", ip, path, host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("doh %s: HTTP %d", ip, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return "", err
	}
	return parseDoHAnswer(ip, body)
}

// parseDoHAnswer — JSON-форма DoH-ответа (Status + Answer[].type/data).
func parseDoHAnswer(endpoint string, body []byte) (string, error) {
	var payload struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Status != 0 {
		return "", fmt.Errorf("doh %s: DNS status %d", endpoint, payload.Status)
	}
	var ips []string
	for _, ans := range payload.Answer {
		if ans.Type == 1 {
			ips = append(ips, ans.Data)
		}
	}
	return firstPublicIPv4(ips)
}

// ---------- кэш последнего хорошего ----------

func (r *PreflightResolver) loadCache() map[string]preflightCacheEntry {
	out := map[string]preflightCacheEntry{}
	if r.CachePath == "" {
		return out
	}
	data, err := os.ReadFile(r.CachePath)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

func (r *PreflightResolver) cacheGet(host string) (string, bool) {
	e, ok := r.loadCache()[host]
	if !ok || time.Since(e.At) > preflightCacheTTL || net.ParseIP(e.IP) == nil {
		return "", false
	}
	return e.IP, true
}

func (r *PreflightResolver) cachePut(host, ip string) {
	if r.CachePath == "" {
		return
	}
	cache := r.loadCache()
	cache[host] = preflightCacheEntry{IP: ip, At: time.Now()}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(r.CachePath), 0o700)
	tmp := r.CachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, r.CachePath)
}

// firstPublicIPv4 — первый «настоящий» публичный IPv4 из ответа: bogon'ы
// (RFC1918/loopback/link-local) пропускаются — резолвер мог ответить мусором.
func firstPublicIPv4(ips []string) (string, error) {
	for _, s := range ips {
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			continue
		}
		return ip.String(), nil
	}
	return "", fmt.Errorf("нет публичного IPv4 среди %v", ips)
}
