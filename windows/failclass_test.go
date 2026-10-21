package main

// failclass_test.go — таблицы классификатора v1 без сети (чистые функции).
// Сценарии — формы отказа из THEORY §2.10 п.16 и живых эпизодов V2-026/039/051:
// RST, таймаут/чёрная дыра, обрыв середины потока, подмена egress, частичная
// деградация, allowlist. Границы честно: timeout-on-443 без улик = unknown.

import (
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/core"
)

func step(name string, passed bool, detail string) core.ProbeResult {
	return core.ProbeResult{Name: name, Passed: passed, Detail: detail}
}

func tunnelStep(passed bool, detail string) core.ProbeResult {
	return step("HTTPS https://api.ipify.org", passed, detail)
}

func directStep(passed bool, detail string) core.ProbeResult {
	return step("Direct leak check", passed, detail)
}

func dnsStep(passed bool, detail string) core.ProbeResult {
	return step("DNS через туннель (DoH)", passed, detail)
}

// mkReport собирает отчёт с одной tunnel-целью; для двух — дублирует шаг.
func mkReport(tunnel, direct, dns core.ProbeResult, extra ...core.ProbeResult) *core.ProbeReport {
	r := &core.ProbeReport{AllPassed: tunnel.Passed && direct.Passed && dns.Passed}
	for _, s := range append([]core.ProbeResult{tunnel, tunnel, direct, dns}, extra...) {
		r.Results = append(r.Results, s)
		if !s.Passed {
			r.AllPassed = false
		}
	}
	return r
}

func TestClassifyResetInjected(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: Get \"https://api.ipify.org\": read tcp: connection reset by peer"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	fc := ClassifyFailure(r, 443, 0)
	if fc.Code != fcResetInjected {
		t.Fatalf("want reset_injected, got %s", fc.Code)
	}
	if fc.Confidence != confResetInjected {
		t.Fatalf("confidence %v", fc.Confidence)
	}
	if !strings.Contains(fc.Hint, "SNI") {
		t.Fatalf("hint: %s", fc.Hint)
	}
}

func TestClassifyServerUnreachableOnRefused(t *testing.T) {
	// Активное отвержение цели (RST с сервера) при живом direct — ServerUnreachable
	// (AGENTS §3.3), не port_blackholed: блэкхол ТСПУ молчит (V2-051).
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: connect: connection refused"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if fc := ClassifyFailure(r, 8445, 0); fc.Code != fcServerUnreach {
		t.Fatalf("want server_unreachable, got %s", fc.Code)
	}
	// Тот же refused, но direct недоступен — атрибутировать отказ некому.
	r2 := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: connect: connection refused"),
		directStep(true, "direct-проверка недоступна из этой сети (i/o timeout); leak-контроль ограничен"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if fc := ClassifyFailure(r2, 8445, 0); fc.Code != fcUnknown {
		t.Fatalf("refused without direct evidence must be unknown, got %s", fc.Code)
	}
}

func TestClassifyPortBlackholedOnOriginPort(t *testing.T) {
	// V2-051: не-443 TCP к VPS — чёрная дыра (SYN уходит, 0 ответов);
	// direct жив, DNS жив. Живой direct = транспортная форма в тексте нет —
	// success-detail всегда «other» (содержит egress-IP).
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: i/o timeout"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if fc := ClassifyFailure(r, 8445, 0); fc.Code != fcPortBlackholed {
		t.Fatalf("want port_blackholed, got %s", fc.Code)
	}
	// Тот же таймаут на не-443, но direct недоступен — улик мало: unknown.
	r2 := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: i/o timeout"),
		directStep(true, "direct-проверка недоступна из этой сети (dial tcp: i/o timeout); leak-контроль ограничен"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if fc := ClassifyFailure(r2, 8445, 0); fc.Code != fcUnknown {
		t.Fatalf("timeout on origin port without direct evidence must be unknown, got %s", fc.Code)
	}
}

func TestClassifyTimeoutOn443IsUnknown(t *testing.T) {
	// Честная граница: таймаут на 443 без других улик не даёт гипотезу DPI.
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp: i/o timeout"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	fc := ClassifyFailure(r, 443, 0)
	if fc.Code != fcUnknown {
		t.Fatalf("want unknown, got %s (%s)", fc.Code, fc.Hint)
	}
	if fc.Confidence != confUnknown {
		t.Fatalf("confidence of unknown must be 0, got %v", fc.Confidence)
	}
}

func TestClassifySilentFreezeFromEOF(t *testing.T) {
	// V2-026: ответные байты до клиента не доходят — обрыв без RST.
	r := mkReport(
		tunnelStep(false, "request failed: read: connection reset by peer: EOF"),
		directStep(true, "direct egress 192.0.2.55"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	// reset-паттерн сильнее eof в текущем порядке; это зафиксировано:
	// «reset by peer: EOF» = reset. Чистый EOF (без reset) даёт freeze.
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcResetInjected {
		t.Fatalf("want reset_injected (reset wins over eof), got %s", fc.Code)
	}
	r2 := mkReport(
		tunnelStep(false, "request failed: Get \"https://api.ipify.org\": unexpected EOF"),
		directStep(true, "direct egress ok"),
		dnsStep(true, "ok"),
	)
	if fc := ClassifyFailure(r2, 443, 0); fc.Code != fcSilentFreeze {
		t.Fatalf("want silent_freeze, got %s", fc.Code)
	}
	if fc := ClassifyFailure(r2, 443, 0); fc.Confidence != confSilentFreeze {
		t.Fatalf("freeze confidence must stay low: %v", fc.Confidence)
	}
	if hint := fcSilentFreeze; !strings.Contains(failclassHints[hint], "mux") || !strings.Contains(failclassHints[hint], "объём") {
		t.Fatalf("silent_freeze hint must mention both volume freeze and mux-contract branches: %q", failclassHints[hint])
	}
}

func TestClassifyEgressDivergence(t *testing.T) {
	// Цели отвечают, но чужим IP — путь подменён, канал не «падал».
	r := &core.ProbeReport{AllPassed: false}
	r.Results = append(r.Results,
		tunnelStep(true, "egress IP: 104.28.219.140"),
		tunnelStep(true, "egress IP: 104.28.219.140"),
		directStep(true, "direct egress 192.0.2.55"),
		dnsStep(true, "ok"),
		step("Expected egress", false, "egress 104.28.219.140, expected 203.0.113.10 — трафик идёт не через защищённый канал"),
	)
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcEgressDivergence {
		t.Fatalf("want egress_divergence, got %s", fc.Code)
	}
}

func TestClassifyDNSOnlyFailure(t *testing.T) {
	r := mkReport(
		tunnelStep(true, "egress IP: 203.0.113.10"),
		directStep(true, "ok"),
		dnsStep(false, "DoH через туннель недоступен: context deadline exceeded"),
	)
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcDNSPathFailure {
		t.Fatalf("want dns_path_failure, got %s", fc.Code)
	}
}

func TestClassifyDegradedPartial(t *testing.T) {
	r := &core.ProbeReport{AllPassed: false}
	r.Results = append(r.Results,
		tunnelStep(true, "egress IP: 203.0.113.10"),
		tunnelStep(false, "request failed: i/o timeout"),
		directStep(true, "ok"),
		dnsStep(true, "ok"),
	)
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcDegraded {
		t.Fatalf("want degraded, got %s", fc.Code)
	}
}

func TestClassifyChannelUnavailableWhenAllDead(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "socks connect tcp 127.0.0.1:1080: connection refused"),
		directStep(true, "ok"),
		dnsStep(false, "DoH через туннель недоступен: connection refused"),
	)
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcChannelUnavail {
		t.Fatalf("want protected_channel_unavailable, got %s", fc.Code)
	}
}

func TestClassifyTLSNotLSOnTLSPort(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: tls: first record does not look like a TLS handshake"),
		directStep(true, "ok"),
		dnsStep(true, "ok"),
	)
	if fc := ClassifyFailure(r, 443, 0); fc.Code != fcTlsPolicing443 {
		t.Fatalf("want tls_policing_443, got %s", fc.Code)
	}
}

func TestClassifyWhitelistModeOnOriginPort(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: tls: handshake failure"),
		directStep(true, "ok"),
		dnsStep(true, "ok"),
	)
	if fc := ClassifyFailure(r, 8445, 0); fc.Code != fcWhitelistMode {
		t.Fatalf("want whitelist_mode, got %s", fc.Code)
	}
}

func TestClassifyNilAndSuccessfulReport(t *testing.T) {
	if fc := ClassifyFailure(nil, 443, 0); fc.Code != fcUnknown {
		t.Fatalf("nil report must be unknown, got %s", fc.Code)
	}
	ok := mkReport(
		tunnelStep(true, "egress IP: 203.0.113.10"),
		directStep(true, "ok"),
		dnsStep(true, "ok"),
	)
	if fc := ClassifyFailure(ok, 443, 0); fc.Code != fcUnknown {
		t.Fatalf("successful report is nothing to classify, got %s", fc.Code)
	}
}

func TestClassifyConfirmedDirectLeak(t *testing.T) {
	r := &core.ProbeReport{LeakConfirmed: true, Results: []core.ProbeResult{{
		Name: "Direct leak check", Status: "leak", Detail: "direct egress совпадает с туннельным",
	}}}
	fc := ClassifyFailure(r, 443, 443)
	if fc.Code != fcLeakConfirmed || fc.Confidence != 1.0 {
		t.Fatalf("confirmed leak = %+v, want %s/1.0", fc, fcLeakConfirmed)
	}
}

func TestClassifyLogLineFormat(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: i/o timeout"),
		directStep(true, "ok"),
		dnsStep(true, "ok"),
	)
	line := classifyFailureLogLine(r, 8445, 0)
	if !strings.HasPrefix(line, "класс отказа: port_blackholed") {
		t.Fatalf("log line: %q", line)
	}
	if !strings.Contains(line, "уверенность v1") {
		t.Fatalf("log line misses confidence: %q", line)
	}
}

func TestProbeTunnelPort(t *testing.T) {
	cases := []struct {
		in   []string
		want int
	}{
		{[]string{"https://vpn.example.com/ip", "https://api.ipify.org"}, 443},
		{[]string{"https://api.ipify.org"}, 443},
		{[]string{"http://x.example:8080/ip"}, 8080},
		{nil, 0},
	}
	for _, c := range cases {
		if got := probeTunnelPort(c.in); got != c.want {
			t.Fatalf("probeTunnelPort(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// ---------- V2-068 (аудит D1): порт КАНАЛА приоритетнее порта probe-цели ----------

// Регрессия: у ВСЕХ встроенных дескрипторов probe-цели — https без явного порта
// (=> 443), поэтому до V2-068 классификатор всегда получал tunnelPort=443 и
// ветка port_blackholed (нужен != 443) была недостижима в проде: живой класс
// V2-051 (SYN-блэкхол 8443/8445) диагностировался как unknown.
func TestClassifyChannelPortWinsOverProbePort(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: i/o timeout"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	// Реальный прод-кейс: порт канала 8445, probe-цель https/443.
	if fc := ClassifyFailure(r, 8445, 443); fc.Code != fcPortBlackholed {
		t.Fatalf("want port_blackholed (порт канала 8445 при probe-цели 443), got %s", fc.Code)
	}
	// Документированная регрессия «как было»: порт канала неизвестен, а порт
	// probe-цели 443 → ветка недостижима → честный unknown (не выдумка).
	if fc := ClassifyFailure(r, 0, 443); fc.Code != fcUnknown {
		t.Fatalf("без порта канала диагноз обязан быть честным unknown, got %s", fc.Code)
	}
	// Обратная сторона: порт канала 443 (CDN-ветка) — таймаут остаётся unknown
	// даже если probe-цель стоит на нестандартном порту (порт канала главнее).
	if fc := ClassifyFailure(r, 443, 8445); fc.Code != fcUnknown {
		t.Fatalf("want unknown (порт канала 443), got %s", fc.Code)
	}
}

// Фолбэк: канал без дескриптора (envelope-доставленный, порт не задан) —
// используется порт probe-цели; честная деградация вместо выдумки порта.
func TestClassifyProbePortFallbackWhenChannelUnknown(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: i/o timeout"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if fc := ClassifyFailure(r, 0, 8445); fc.Code != fcPortBlackholed {
		t.Fatalf("want port_blackholed по фолбэк-порту probe-цели, got %s", fc.Code)
	}
}

// Журнальная строка классифицируется тем же портом, что и решение (V2-068):
// иначе журнал и verdict расходятся.
func TestClassifyLogLineUsesChannelPort(t *testing.T) {
	r := mkReport(
		tunnelStep(false, "request failed: dial tcp 203.0.113.10:8445: i/o timeout"),
		directStep(true, "direct egress 192.0.2.55 отличается от туннельного 203.0.113.10 — утечки нет"),
		dnsStep(true, "example.com → [93.184.216.34]"),
	)
	if line := classifyFailureLogLine(r, 8445, 443); !strings.HasPrefix(line, "класс отказа: port_blackholed") {
		t.Fatalf("log line: %q", line)
	}
}
