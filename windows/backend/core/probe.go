package core

// ProtectedProbe — ограниченный gate HTTPS/egress/OS control/DoH (FR-001/003),
// не доказательство native DNS/UDP/IPv6 или отсутствия любой утечки.
// Отличия от старого ядра (docs/old-core-review.md §3):
//   - expected egress ОБЯЗАТЕЛЕН: egress должен совпасть с ожидаемым IP
//     канала (данные из дескриптора), а не просто быть консистентным;
//   - OS-control сравнивает сообщённые IP; совпадение/расхождение само по себе
//     не доказывает локальный путь, утечку или отсутствие утечек;
//   - DoH-шаг проверяет DNS wire response через HTTPS/SOCKS. Он не
//     доказывает native DNS/UDP/IPv6 клиента или независимость измерителя.
//
// Отчёт структурен и пригоден для UI; секретов в нём нет.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/netprobe"
)

const (
	maxProbeBody           = 4 << 10
	defaultLeakCheckTarget = "https://api.ipify.org"
	dnsCheckHost           = netprobe.DNSProbeHost
	// Legacy source name, shared with the actual compiled resolver defaults.
	dnsCheckEndpoint = metadata.PrimaryDNSProbeTarget
)

type ProbeResult struct {
	Name        string           `json:"name"`
	Passed      bool             `json:"passed"`
	Status      string           `json:"status,omitempty"` // pass | fail | unavailable | leak
	Detail      string           `json:"detail"`
	Observation ProbeObservation `json:"observation,omitzero"`
	Comparison  ProbeComparison  `json:"comparison,omitzero"`
	Requirement string           `json:"requirement,omitempty"` // missing|required are required; finite informational allowlist
	// Fixed value slots keep report copies isolated without another shared slice.
	ResolverAttempts     [metadata.MaxDNSProbeTargets]ProbeObservation `json:"resolver_attempts,omitzero"`
	ResolverAttemptCount int                                           `json:"resolver_attempt_count,omitempty"`
}

type ProbeReport struct {
	AllPassed             bool             `json:"all_passed"`
	LeakConfirmed         bool             `json:"leak_confirmed"` // independent local violation evidence, never an IP relation
	ProtectionUnavailable bool             `json:"protection_unavailable,omitempty"`
	CleanupUnconfirmed    bool             `json:"cleanup_unconfirmed,omitempty"` // finite custody fault, never a network diagnosis
	TrustUnavailable      bool             `json:"trust_unavailable,omitempty"`
	TrustReason           string           `json:"trust_reason,omitempty"`
	Results               []ProbeResult    `json:"results"`
	SessionID             uint64           `json:"session_id,omitempty"`
	OwnerID               uint64           `json:"owner_id,omitempty"`
	OwnerGeneration       uint64           `json:"owner_generation,omitempty"`
	IntentGeneration      uint64           `json:"intent_generation,omitempty"`
	ObservationID         uint64           `json:"observation_id,omitempty"`
	NetworkEpoch          int              `json:"network_epoch,omitempty"`
	StartedAt             time.Time        `json:"started_at,omitzero"`
	CompletedAt           time.Time        `json:"completed_at,omitzero"`
	Stale                 bool             `json:"stale,omitempty"`
	operationSource       *OperationSource // private negative identity, never serialized or an admission grant
	observationPath       *atomic.Bool
	restrictionObserved   bool // original scoped work produced a local/unsafe fact; never a public report assertion
}

func (r *ProbeReport) Clone() *ProbeReport {
	if r == nil {
		return nil
	}
	out := *r
	out.Results = append([]ProbeResult(nil), r.Results...)
	return &out
}

// Summary — однострочный redacted-итог для логов/ошибок.
func (r *ProbeReport) Summary() string {
	if r == nil {
		return "empty report"
	}
	failed, required, information := 0, 0, 0
	first := ""
	for _, res := range r.Results {
		res = res.NormalizeDiagnostic()
		if res.Informational() {
			information++
			continue
		}
		required++
		if !res.Passed || res.Status != "" && res.Status != "pass" || res.Observation.Schema != 0 && (res.Observation.Schema != 1 || !res.Observation.BodyComplete || res.Observation.ErrorKind != "") {
			failed++
			if first == "" {
				first = res.Name + ": " + res.Detail
			}
		}
	}
	if required == 0 {
		return fmt.Sprintf("no required diagnostic steps; %d informational", information)
	}
	if information != 0 {
		if failed == 0 {
			return fmt.Sprintf("%d/%d required passed; %d informational", required, required, information)
		}
		return fmt.Sprintf("%d of %d required steps failed; %d informational; first: %s", failed, required, information, first)
	}
	if failed == 0 {
		return fmt.Sprintf("%d/%d passed", required, required)
	}
	return fmt.Sprintf("%d of %d steps failed; first: %s", failed, required, first)
}

// ProtectedProbe — конфигурация проверки. SOCKSAddr — адрес inbound'а ядра;
// Targets — HTTPS-эхо egress (≥2 для сравнения сообщений); ExpectedEgress —
// ожидаемый адрес выхода из принятого профиля, не обязательно адрес origin.
type ProtectedProbe struct {
	// Only the original engine worker supplies this private protected-outbound
	// dialer. It never authenticates to a public/local proxy listener.
	tunnelDialContext func(context.Context, string, string) (net.Conn, error)
	tunBinding        *workerTUNProbeBinding // actual worker phase only; no public plan/report can supply it
	SOCKSAddr         string
	Targets           []string
	DNSProbeTargets   []string
	Timeout           time.Duration
	ExpectedEgress    string
	// AllowDirectEgress отключает OS-control проверку (только для
	// окружений, где direct запрещён политикой сети; по умолчанию false).
	AllowDirectEgress bool
	// DirectLeakThroughTunnel retains the existing caller's mode declaration.
	// It is not evidence of the actual OS route. A same-image control is
	// insufficient for TUN admission regardless of the two reported addresses.
	DirectLeakThroughTunnel bool
	// LeakCheckTarget — legacy field name for the OS-control egress echo.
	// Proxy=nil declares intent; it does not attest the actual OS path.
	// По умолчанию defaultLeakCheckTarget.
	LeakCheckTarget string
}

func (pp *ProtectedProbe) validate() error {
	if pp == nil || pp.SOCKSAddr == "" && pp.tunnelDialContext == nil {
		return errors.New("invalid probe configuration: addr/timeout")
	}
	return pp.validatePlan()
}

// A bounded diagnostic plan is not a ready dialer or a forwarding permit.
func (pp *ProtectedProbe) validatePlan() error {
	if pp == nil || pp.Timeout <= 0 || pp.Timeout > 30*time.Second {
		return errors.New("invalid probe timeout")
	}
	if len(pp.Targets) < 2 || len(pp.Targets) > 8 {
		return errors.New("invalid probe configuration: 2..8 targets required")
	}
	for _, target := range pp.Targets {
		u, err := url.Parse(target)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return errors.New("invalid HTTPS observation endpoint")
		}
	}
	if net.ParseIP(strings.TrimSpace(pp.ExpectedEgress)) == nil {
		return errors.New("expected egress is not a valid IP")
	}
	if err := metadata.ValidateDNSProbeTargets(pp.resolverTargets()); err != nil {
		return err
	}
	return nil
}

// Run выполняет все шаги. Контекст отмены уважается на каждом шаге.
//
// Egress, OS control и resolver step идут параллельно в общем bounded context.
// Источники внутри resolver step пробуются последовательно, деля оставшийся
// срок. Это не доказательство независимости путей или измеренного ускорения.
// Порядок результатов фиксирован: egress-цели,
// consistency, expected, OS control, DNS, required TUN boundary when requested.
func (pp *ProtectedProbe) Run(ctx context.Context) (report *ProbeReport) {
	if pp != nil {
		owned := *pp
		owned.Targets, owned.DNSProbeTargets = slices.Clone(pp.Targets), slices.Clone(pp.DNSProbeTargets)
		pp = &owned
	}
	started := time.Now()
	report = &ProbeReport{}
	defer func() { report.StartedAt, report.CompletedAt = started.UTC(), time.Now().UTC() }()
	if ctx == nil {
		return &ProbeReport{Results: []ProbeResult{{Name: "контекст", Status: "unavailable", Detail: "probe context is nil"}}}
	}
	if err := pp.validate(); err != nil {
		report.Results = append(report.Results, ProbeResult{
			Name: "конфигурация probe", Detail: err.Error(),
		})
		return report
	}
	dnsRequests := len(pp.resolverTargets())
	requests := len(pp.Targets) + dnsRequests // reserve every possible resolver attempt
	if !pp.AllowDirectEgress {
		requests++
	}
	ctx, release, err := AdmitDiagnostic(ctx, DiagnosticProtected, requests, int64(requests*(maxProbeBody+1)+dnsRequests*maxDNSProbeQuery), pp.Timeout)
	if err != nil {
		return &ProbeReport{Results: []ProbeResult{{Name: "бюджет probe", Status: "unavailable", Detail: "protected diagnostic budget unavailable; path not measured"}}}
	}
	defer release()
	if err := ctx.Err(); err != nil {
		report.Results = append(report.Results, ProbeResult{
			Name: "контекст", Detail: err.Error(),
		})
		return report
	}

	tunnelClient := pp.tunnelClient()
	defer tunnelClient.CloseIdleConnections()
	// The control must ignore HTTP_PROXY/HTTPS_PROXY; routing in TUN is still
	// governed by the OS, which is why TUN evidence is evaluated separately.
	directClient := &http.Client{Transport: &http.Transport{Proxy: nil, DisableCompression: true, MaxResponseHeaderBytes: 16 << 10}, Timeout: pp.Timeout, CheckRedirect: noProbeRedirect}
	defer directClient.CloseIdleConnections()

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
			r, ip := checkHTTPSVia(ctx, tunnelClient, target, pp.tunnelRoute())
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
			controlContext := ctx
			if pp.DirectLeakThroughTunnel {
				// Informational work gets a shorter child deadline, leaving
				// time in the unchanged operation budget for required checks.
				// Parent cancellation still invalidates the entire observation.
				var cancel context.CancelFunc
				controlContext, cancel = context.WithTimeout(ctx, pp.Timeout/2)
				defer cancel()
			}
			r, directIP := checkHTTPSVia(controlContext, directClient, leakTarget, ProbeRouteOS)
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

	// Both comparisons remain explicit even when no usable addresses arrived.
	for _, result := range []ProbeResult{consistentEgressResult(tunnelIPs), expectedEgressResult(tunnelIPs, pp.ExpectedEgress)} {
		report.Results = append(report.Results, result)
		if !result.Passed {
			failed++
		}
	}

	// Шаг: direct-leak (пропускается только при явном AllowDirectEgress).
	if !pp.AllowDirectEgress {
		r := directLeakResult(pp.DirectLeakThroughTunnel, steps[leakSlot].res, steps[leakSlot].ip, tunnelIPs)
		report.Results = append(report.Results, r)
		if !r.Passed && !r.Informational() {
			failed++
		}
	}

	// Шаг: DNS через туннель.
	dnsRes := steps[dnsSlot].res
	report.Results = append(report.Results, dnsRes)
	if !dnsRes.Passed {
		failed++
	}
	if pp.DirectLeakThroughTunnel {
		// The same-image control is informational. It never replaces this
		// required original native fact or actual existing worker admission.
		step := pp.tunBinding.result()
		report.Results = append(report.Results, step)
		if !step.Passed {
			report.ProtectionUnavailable = true
			failed++
		}
	}

	if ctx.Err() != nil {
		failed++
		report.Results = append(report.Results, ProbeResult{Name: "контекст", Status: "unavailable", Detail: "protected observation deadline/cancellation; path not confirmed"})
	}
	report.AllPassed = failed == 0
	return report
}

func (pp *ProtectedProbe) tunnelClient() *http.Client {
	transport := &http.Transport{
		DisableCompression:     true,
		MaxResponseHeaderBytes: 16 << 10,
		DialContext:            (&net.Dialer{Timeout: pp.Timeout}).DialContext,
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "socks5h",
			Host:   pp.SOCKSAddr,
		}),
	}
	if pp.tunnelDialContext != nil {
		transport.Proxy = nil
		transport.DialContext = pp.tunnelDialContext
	}
	return &http.Client{Transport: transport, Timeout: pp.Timeout, CheckRedirect: noProbeRedirect}
}

func (pp *ProtectedProbe) tunnelRoute() string {
	if pp.tunnelDialContext != nil {
		return ProbeRouteEngineOutbound
	}
	return ProbeRouteSOCKS
}

func noProbeRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func checkHTTPS(ctx context.Context, client *http.Client, target string) (ProbeResult, string) {
	return checkHTTPSVia(ctx, client, target, "unspecified")
}

func checkHTTPSVia(ctx context.Context, client *http.Client, target, route string) (ProbeResult, string) {
	fact, body, err := fetchProbeHTTP(ctx, client, target, route, "egress_http", "", maxProbeBody)
	result := ProbeResult{Name: "HTTPS " + fact.Endpoint, Observation: fact}
	if err != nil {
		result.Status, result.Detail = "unavailable", fact.Summary()
		return result, ""
	}
	ip := extractIP(string(body))
	if fact.HTTPStatus != http.StatusOK {
		result.Observation.ErrorKind = "http_status"
		result.Detail = result.Observation.Summary()
		return result, "" // an error page is not an authenticated egress echo
	}
	if net.ParseIP(ip) == nil {
		result.Observation.ErrorKind = "invalid_response"
		result.Detail = "response did not contain an egress IP; " + result.Observation.Summary()
		return result, ""
	}
	result.Passed, result.Status = true, "pass"
	result.Detail = fmt.Sprintf("egress IP: %s; %s", ip, fact.Summary())
	return result, ip
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
