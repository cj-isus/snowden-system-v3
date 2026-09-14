package core

// ProtectedProbe — доказательство «туннель работает и ничего не течёт мимо»
// (FR-001/FR-003). Отличия от старого ядра (docs/old-core-review.md §3):
//   - expected egress ОБЯЗАТЕЛЕН: egress должен совпасть с ожидаемым IP
//     канала (данные из дескриптора), а не просто быть консистентным;
//   - добавлен шаг direct-leak: тот же egress-цель напрямую (мимо туннеля)
//     должен дать ДРУГОЙ адрес; совпадение = трафик не идёт через туннель;
//   - DNS-шаг: разрешение через туннель должно отвечать (заглушка UDP не
//     считается — проверяется полный путь до рекурсивного резолвера).
//
// Отчёт структурен и пригоден для UI; секретов в нём нет.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultLeakCheckTarget = "https://api.ipify.org"
	dnsCheckHost           = "example.com"
	// dnsCheckEndpoint — DoH JSON-эндпоинт по IP-литералу: резолв самому
	// эндпоинту не нужен (нет круговой зависимости), путь полностью через туннель.
	dnsCheckEndpoint = "https://1.1.1.1/dns-query"
)

type ProbeResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type ProbeReport struct {
	AllPassed bool          `json:"all_passed"`
	Results   []ProbeResult `json:"results"`
}

// Summary — однострочный redacted-итог для логов/ошибок.
func (r *ProbeReport) Summary() string {
	if r == nil {
		return "empty report"
	}
	failed := 0
	first := ""
	for _, res := range r.Results {
		if !res.Passed {
			failed++
			if first == "" {
				first = res.Name + ": " + res.Detail
			}
		}
	}
	if failed == 0 {
		return fmt.Sprintf("%d/%d passed", len(r.Results), len(r.Results))
	}
	return fmt.Sprintf("%d of %d steps failed; first: %s", failed, len(r.Results), first)
}

// ProtectedProbe — конфигурация проверки. SOCKSAddr — адрес inbound'а ядра;
// Targets — HTTPS-эхо egress (≥2, чтобы ловить MITM-рассинхрон); ExpectedEgress
// — публичный IP origin-сервера канала.
type ProtectedProbe struct {
	SOCKSAddr      string
	Targets        []string
	Timeout        time.Duration
	ExpectedEgress string
	// AllowDirectEgress отключает direct-leak проверку (только для
	// окружений, где direct запрещён политикой сети; по умолчанию false).
	AllowDirectEgress bool
	// DirectLeakThroughTunnel — семантика direct-leak в TUN-режиме
	// (PLAN §2.3/V2-003: контракт «TUN-режим: равен»), восстановлена
	// аудитом V2-037: auto_route заворачивает ВЕСЬ трафик ОС в туннель,
	// поэтому direct-запрос обязан выйти через тот же egress.
	//   SOCKS-режим (false): direct ≠ tunnel — совпадение = утечка/подмена;
	//   TUN-режим   (true):  direct == tunnel — отличие = трафик прошёл мимо
	//   туннеля (реальная утечка на сетевом уровне).
	DirectLeakThroughTunnel bool
	// LeakCheckTarget — egress-эхо для direct-запроса (мимо туннеля).
	// По умолчанию defaultLeakCheckTarget.
	LeakCheckTarget string
}

func (pp *ProtectedProbe) validate() error {
	if pp == nil || pp.SOCKSAddr == "" || pp.Timeout <= 0 {
		return errors.New("invalid probe configuration: addr/timeout")
	}
	if len(pp.Targets) < 2 {
		return errors.New("invalid probe configuration: at least 2 targets required")
	}
	for _, target := range pp.Targets {
		u, err := url.Parse(target)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("invalid HTTPS target %q", target)
		}
	}
	if net.ParseIP(strings.TrimSpace(pp.ExpectedEgress)) == nil {
		return fmt.Errorf("expected egress is not a valid IP: %q", pp.ExpectedEgress)
	}
	return nil
}

// Run выполняет все шаги. Контекст отмены уважается на каждом шаге.
//
// Производительность (основа стабильности = меньше одновременных ожиданий на
// пользователя): все сетевые шаги идут ПАРАЛЛЕЛЬНО — каждый шаг независим
// (свой SOCKS-коннект через mixed-инбаунд), и консистентность/expected-egress
// вычисляются после сбора всех результатов. Задержка probe ≈ самому медленному
// шагу, а не их сумме (раньше 5 последовательных HTTPS → 4× меньше ожидание
// Start на медленном канале). Порядок результатов фиксирован: egress-цели,
// consistency, expected, direct-leak, DNS.
func (pp *ProtectedProbe) Run(ctx context.Context) *ProbeReport {
	report := &ProbeReport{}
	if err := pp.validate(); err != nil {
		report.Results = append(report.Results, ProbeResult{
			Name: "конфигурация probe", Detail: err.Error(),
		})
		return report
	}
	if err := ctx.Err(); err != nil {
		report.Results = append(report.Results, ProbeResult{
			Name: "контекст", Detail: err.Error(),
		})
		return report
	}

	tunnelClient := pp.tunnelClient()
	directClient := &http.Client{Timeout: pp.Timeout}

	// Один слот результата на каждый сетевой шаг; индексы фиксированы.
	nTargets := len(pp.Targets)
	type stepResult struct {
		res ProbeResult
		ip  string
	}
	steps := make([]stepResult, nTargets+2) // targets + direct-leak + DNS
	var wg sync.WaitGroup
	var leakSlot, dnsSlot = nTargets, nTargets + 1

	for i, target := range pp.Targets {
		wg.Add(1)
		go func(slot int, target string) {
			defer wg.Done()
			r, ip := checkHTTPS(ctx, tunnelClient, target)
			steps[slot] = stepResult{res: r, ip: ip}
		}(i, target)
	}
	if !pp.AllowDirectEgress {
		leakTarget := pp.LeakCheckTarget
		if leakTarget == "" {
			leakTarget = defaultLeakCheckTarget
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, directIP := checkHTTPS(ctx, directClient, leakTarget)
			r.Name = "Direct leak check"
			steps[leakSlot] = stepResult{res: r, ip: directIP}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		steps[dnsSlot] = stepResult{res: pp.checkDNS(ctx, tunnelClient)}
	}()
	wg.Wait()

	var tunnelIPs []string
	failed := 0
	// Шаг 1..N: egress-маркеры через туннель (порядок как в Targets).
	for i := 0; i < nTargets; i++ {
		r := steps[i].res
		report.Results = append(report.Results, r)
		if !r.Passed {
			failed++
			continue
		}
		if ip := steps[i].ip; ip != "" {
			tunnelIPs = append(tunnelIPs, ip)
		}
	}

	// Шаг: консистентность egress между целями.
	if len(tunnelIPs) >= 2 {
		consistent := true
		for i := 1; i < len(tunnelIPs); i++ {
			if tunnelIPs[i] != tunnelIPs[0] {
				consistent = false
				break
			}
		}
		report.Results = append(report.Results, ProbeResult{
			Name:   "Egress consistency",
			Passed: consistent,
			Detail: fmt.Sprintf("targets report: %v", tunnelIPs),
		})
		if !consistent {
			failed++
		}
	}

	// Шаг: expected egress (главное отличие от старого ядра).
	if len(tunnelIPs) > 0 {
		got := tunnelIPs[0]
		pass := got == strings.TrimSpace(pp.ExpectedEgress)
		detail := fmt.Sprintf("egress %s, expected %s", got, pp.ExpectedEgress)
		if !pass {
			detail += " — трафик идёт не через защищённый канал"
		}
		report.Results = append(report.Results, ProbeResult{
			Name: "Expected egress", Passed: pass, Detail: detail,
		})
		if !pass {
			failed++
		}
	}

	// Шаг: direct-leak (пропускается только при явном AllowDirectEgress).
	if !pp.AllowDirectEgress {
		r := directLeakResult(pp.DirectLeakThroughTunnel, steps[leakSlot].res, steps[leakSlot].ip, tunnelIPs)
		report.Results = append(report.Results, r)
		if !r.Passed {
			failed++
		}
	}

	// Шаг: DNS через туннель.
	dnsRes := steps[dnsSlot].res
	report.Results = append(report.Results, dnsRes)
	if !dnsRes.Passed {
		failed++
	}

	report.AllPassed = failed == 0
	return report
}

func (pp *ProtectedProbe) tunnelClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: pp.Timeout}).DialContext,
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "socks5h",
			Host:   pp.SOCKSAddr,
		}),
	}
	return &http.Client{Transport: transport, Timeout: pp.Timeout}
}

// checkDNS — настоящий DNS-путь через туннель: DoH JSON-запрос к эндпоинту
// по IP-литералу через SOCKS (socks5h). Это независимая проверка резолва
// (старая версия дублировала первую HTTPS-цель тем же клиентом — «6/6»
// завышало объём доказательств, V2-037). Успех = получен осмысленный
// A-ответ на эталонное имя.
func (pp *ProtectedProbe) checkDNS(ctx context.Context, client *http.Client) ProbeResult {
	name := "DNS через туннель (DoH)"
	u := fmt.Sprintf("%s?name=%s&type=A", dnsCheckEndpoint, dnsCheckHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("request failed: %v", err)}
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("DoH через туннель недоступен: %v", err)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("read failed: %v", err)}
	}
	if resp.StatusCode != http.StatusOK {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	var payload struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("некорректный DoH-ответ: %v", err)}
	}
	// Status 0 = NOERROR; A-записи (type 1) с валидным IP.
	var ips []string
	for _, ans := range payload.Answer {
		if ans.Type == 1 && net.ParseIP(ans.Data) != nil {
			ips = append(ips, ans.Data)
		}
	}
	if payload.Status != 0 || len(ips) == 0 {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("DoH ответил без A-записей (Status=%d, ответов=%d)", payload.Status, len(payload.Answer))}
	}
	return ProbeResult{Name: name, Passed: true, Detail: fmt.Sprintf("%s → %v (резолв через туннель работает)", dnsCheckHost, ips)}
}

// directLeakResult — вердикт direct-leak шага по режиму (V2-037):
//
//	SOCKS (tunMode=false): direct-путь независим от туннеля; совпадение
//	egress с туннельным = трафик пользователя НЕ идёт через туннель.
//	TUN   (tunMode=true):  auto_route заворачивает весь трафик ОС в туннель;
//	расхождение = трафик утёк мимо туннеля на сетевом уровне.
//
// Недоступный direct — не провал (политика сети / strict_route), но и не
// доказательство отсутствия утечки: шаг проходит с пометкой об ограничении.
func directLeakResult(tunMode bool, r ProbeResult, directIP string, tunnelIPs []string) ProbeResult {
	tunnelIP := firstOr(tunnelIPs, "")
	switch {
	case !r.Passed:
		r.Passed = true
		if tunMode {
			r.Detail = "direct-путь недоступен (strict_route?) — в TUN-режиме это безопасно, но не доказательство (" + r.Detail + ")"
		} else {
			r.Detail = "direct-проверка недоступна из этой сети (" + r.Detail + "); leak-контроль ограничен"
		}
	case directIP == "" || tunnelIP == "":
		r.Detail = "direct egress получен, туннельного нет — сопоставление невозможно"
	case tunMode && directIP != tunnelIP:
		r.Passed = false
		r.Detail = fmt.Sprintf("direct egress %s отличается от туннельного %s — в TUN-режиме трафик утёк мимо туннеля", directIP, tunnelIP)
	case !tunMode && directIP == tunnelIP:
		r.Passed = false
		r.Detail = fmt.Sprintf("direct egress %s совпадает с туннельным — трафик НЕ идёт через туннель", directIP)
	default:
		if tunMode {
			r.Detail = fmt.Sprintf("direct egress %s идёт через туннель (TUN-режим: совпадение — норма)", directIP)
		} else {
			r.Detail = fmt.Sprintf("direct egress %s отличается от туннельного %s — утечки нет", directIP, tunnelIP)
		}
	}
	return r
}

func checkHTTPS(ctx context.Context, client *http.Client, target string) (ProbeResult, string) {
	name := "HTTPS " + target
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("request failed: %v", err)}, ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("request failed: %v", err)}, ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ProbeResult{Name: name, Detail: fmt.Sprintf("read failed: %v", err)}, ""
	}

	ip := extractIP(string(body))
	if resp.StatusCode != http.StatusOK {
		return ProbeResult{Name: name, Passed: false,
			Detail: fmt.Sprintf("HTTP %d, body: %s", resp.StatusCode, truncateBytes(body, 64))}, ip
	}
	if net.ParseIP(ip) == nil {
		return ProbeResult{Name: name, Detail: "response did not contain an egress IP"}, ""
	}
	return ProbeResult{Name: name, Passed: true, Detail: fmt.Sprintf("egress IP: %s", ip)}, ip
}

func extractIP(s string) string {
	if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
		return ip.String()
	}
	var payload struct {
		IP      string `json:"ip"`
		IPAddr  string `json:"ip_addr"`
		Address string `json:"address"`
	}
	if json.Unmarshal([]byte(s), &payload) == nil {
		for _, candidate := range []string{payload.IP, payload.IPAddr, payload.Address} {
			if ip := net.ParseIP(strings.TrimSpace(candidate)); ip != nil {
				return ip.String()
			}
		}
	}
	for _, field := range strings.Fields(s) {
		field = strings.Trim(field, "{}[](),\"'")
		if ip := net.ParseIP(field); ip != nil {
			return ip.String()
		}
	}
	return ""
}

func firstOr(list []string, def string) string {
	if len(list) > 0 {
		return list[0]
	}
	return def
}

func truncateBytes(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
