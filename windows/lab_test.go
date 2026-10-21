package main

// lab_test.go — Controlled Lab (PLAN §5 Phase E, V2-069; plan_gpt §61–63/§119).
//
// Три пункта фазы в одном тест-файле, три слоя:
//
//  1. Датасет LAB-сценариев — ground truth по определению plan_gpt §61
//     («lab = ground truth, field = noisy observation»): сценарий задаёт
//     Управляемое Зло (Middlebox Behavior — строка из живого языка ТСПУ),
//     фиксированную сеть и канал, а НЕ готовый probe-отчёт. Отчёт выводится
//     лабораторной моделью из поведения сети; истинная причина (True)
//     указана человеком с обоснованием Why из live-эпизодов журнала
//     (V2-026/039/040/041/044/051).
//
//  2. Эмулятор сценариев — лабораторная модель network path → ProbeReport
//     (labRenderReport). Целостность харнесса проверяется отдельным тестом:
//     отчёт здорового пути обязан классифицироваться как «сбоя нет» — иначе
//     харнесс сам инжектирует сбой и весь benchmark лжёт (урок V2-042).
//
//  3. Benchmark классификатора v1 (живой ClassifyFailure, не копия) на
//     датасете: prediction compared with ground truth (plan_gpt §119) —
//     попадания, промахи, распределение предсказаний, уверенность на класс.
//
// Живые эпизоды (LAB-020…, V2-075) — verbatim-формы из журнала владельца;
// новый эпизод входит в датасет со ссылкой на запись и не меняет baseline-
// проверки (TestLabBenchmark фиксирует текущую точку, промахи видны в сводке).
//
// Честные границы (записаны в PLAN V2-069): это лабораторная модель слоя
// TLS-текста поверх SOCKS, не сетевой стенд; дуплекс/конгест/HY2-профили —
// следующие шаги. LAB-015 — намеренно оставленный промах: тихая маскировка
// на 443 непок existential для слоя evidence классификатора v1 (нужен
// diagnostic probe, Phase A2) — benchmark фиксирует её как промах, а не
// подгоняет разметку под ответ (калибровка — Phase E2, данные собирает
// этот отчёт).

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/core"
)

// ---------- 1. Датасет: LAB-сценарии (ground truth) ----------

// labMbox — управляемое поведение middlebox + фиксированная сеть.
type labMbox struct {
	form     string // что видит tunnel-цель: normal|reset|timeout|eof|notls|tls|refused
	dnsMode  string // "pass" | "fail"
	directOk bool   // direct-путь жив (вне туннеля)
	egressIP string // наблюдаемый egress ("" = не получен — путь мёртв)
	wantIP   string // ожидаемый egress из конфигурации probe
}

// labCase — один ground-truth сценарий (план §62: LAB-001…).
type labCase struct {
	ID         string  // LAB-NNN
	Why        string  // обоснование ground truth из live-эпизода (журнал)
	True       string  // истинная причина — словарь THEORY §2.10 ("" = сбоя нет)
	Healthy    bool    // True=="" (матчить иначе, чем коды)
	ConfigFail bool    // структурная ветка: probe-конфиг невалиден
	Partial    bool    // часть tunnel-целей жива (degraded-сценарии)
	Mbox       labMbox // управляемое зло + сеть
	Port       int     // tunnelPort цели (признак 443-эвристики)
}

// labDataset — 12 канонических профилей (plan_gpt §62) + 6 структурных веток
// ClassifyFailure (каждая ветка решающего дерева покрыта сценарием) + живые
// эпизоды владельца (LAB-020…, verbatim-формы из журнала; V2-075). Каждый
// Why — ссылка на live-эпизод журнала: датасет остаётся воспроизводимым
// артефактом, а не выдумкой теста.
func labDataset() []labCase {
	return []labCase{
		{ID: "LAB-001 normal", Why: "канал жив — сбоя нет (V2-044: reality probe pass, egress ×3)",
			True: "", Healthy: true,
			Mbox: labMbox{form: "normal", dnsMode: "pass", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 8443},
		{ID: "LAB-002 tcp-rst", Why: "RST на пути после CDN (V2-026: класс CDN-ветки)",
			True: "reset_injected",
			Mbox: labMbox{form: "reset", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-003 tls-rst", Why: "RST на 443 — TLS-полиция не отличима от обычного RST без улик (THEORY §2.4a; V2-026)",
			True: "reset_injected",
			Mbox: labMbox{form: "reset", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-004 blackhole", Why: "тишина на не-443 origin-порту — доказано live tcpdump'ом (V2-051: 8443/8445 из сети владельца = 0 пакетов)",
			True: "port_blackholed",
			Mbox: labMbox{form: "timeout", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 8445},
		{ID: "LAB-005 freeze-8k", Why: "обрыв середины потока без RST — ближайший v1-маркер объёмной механики (V2-039: mux-стримы рвались EOF)",
			True: "silent_freeze",
			Mbox: labMbox{form: "eof", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-006 freeze-16k", Why: "та же форма на большем объёме — v1 не видит объём, класс тот же (граница слоя задокументирована в failclass.go)",
			True: "silent_freeze",
			Mbox: labMbox{form: "eof", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-007 freeze-25pkt", Why: "пакетная механика невидима для TLS-текста probe — та же форма",
			True: "silent_freeze",
			Mbox: labMbox{form: "eof", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-008 dns-subversion", Why: "DNS умер при живых HTTPS — путь жив, отказа канала нет (V2-041: egress-измерение независимо от транспорта)",
			True: "dns_path_failure",
			Mbox: labMbox{form: "normal", dnsMode: "fail", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-009 return-path-loss", Why: "путь умер целиком — DNS и HTTPS вместе (V2-026: полная деградация CDN-ветки)",
			True: "protected_channel_unavailable",
			Mbox: labMbox{form: "timeout", dnsMode: "fail", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-010 loss+latency", Why: "шумовая деградация — часть целей жива (V2-040: мерцание, fastReprobe без убийства туннеля)",
			True: "degraded", Partial: true,
			Mbox: labMbox{form: "reset", dnsMode: "pass", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-011 high-load", Why: "частичная деградация под нагрузкой — та же алгебра одного живого одного мёртвого (V2-040)",
			True: "degraded", Partial: true,
			Mbox: labMbox{form: "timeout", dnsMode: "pass", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-012 intermittent", Why: "чередование жив/мёртв — v1 честно видит срез момента: один живой target = degraded",
			True: "degraded", Partial: true,
			Mbox: labMbox{form: "eof", dnsMode: "pass", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-014 allowed-x509", Why: "allowlist-режим: TLS-ошибка сертификата на не-443 при живом direct — программного обхода нет (THEORY §2.10 F; V2-051)",
			True: "whitelist_mode",
			Mbox: labMbox{form: "tls", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 8445},
		{ID: "LAB-015 passive-443", Why: "НАМЕРЕННЫЙ ПРОМАХ: тихая маскировка на 443 без улик непокрыема для слоя evidence v1 (443-эвристика THEORY §2.4a; граница задокументирована в failclass.go) — истинная причина tls_policing_443, честный ответ v1 = unknown",
			True: "tls_policing_443",
			Mbox: labMbox{form: "timeout", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-016 config-invalid", Why: "невалидный probe-конфиг — классификация без evidence = шум, честный unknown (AGENTS §3.3; FR-003)",
			True: "unknown", ConfigFail: true,
			Mbox: labMbox{form: "normal", dnsMode: "pass", directOk: true, egressIP: "203.0.113.10", wantIP: "203.0.113.10"}, Port: 0},
		{ID: "LAB-017 egress-divergence", Why: "egress отвечает чужим IP — подмена пути (V2-041: live-проверка, класс с максимальной уверенностью)",
			True: "egress_divergence",
			Mbox: labMbox{form: "normal", dnsMode: "pass", directOk: true, egressIP: "203.0.113.77", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-018 server-unreach", Why: "цель активно отвергла соединение при живом direct — сервер/порт, не сеть (AGENTS §3.3; классификация Windows-формы refused)",
			True: "server_unreachable",
			Mbox: labMbox{form: "refused", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 8443},
		{ID: "LAB-019 notls-8445", Why: "не-TLS-ответ на TLS-порте — активная подмена контента (THEORY §2.10: TlsPolicing)",
			True: "tls_policing_443",
			Mbox: labMbox{form: "notls", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 8445},
		// --- Живые эпизоды владельца (verbatim-формы из журнала; V2-075) ---
		{ID: "LAB-020 live-mux-eof", Why: "LIVE V2-037 п.8: mux-инцидент — клиентский mux + серверный инбаунд без multiplex: сервер рвал mux-стримы при живом CDN-пути, probe «3 of 4 failed: EOF»; обе tunnel-цели одной eof-формой (один механизм, один срез)",
			True: "silent_freeze",
			Mbox: labMbox{form: "eof", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 443},
		{ID: "LAB-021 live-dead-inbound", Why: "LIVE V2-028/V2-044 (форма из failclass.go): SOCKS-инбаунд ядра недоступен — канал не срывался, движок мёртв; proxy-форма железная (движок мёртв — факт, константа channel_unavail 0.60 подветка «proxy»)",
			True: "protected_channel_unavailable",
			Mbox: labMbox{form: "proxy", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 0},
		{ID: "LAB-022 live-actively-refused", Why: "LIVE V2-026 (verbatim-захват в failclass_test.go): Windows-форма «request failed: dial tcp 203.0.113.10:8445: connect: connection refused» — активное отвержение цели через выход туннеля при живом direct; серверная подветка server_unreachable (V2-070: 0.80 — положительный артефакт провода)",
			True: "server_unreachable",
			Mbox: labMbox{form: "refused", dnsMode: "pass", directOk: true, egressIP: "", wantIP: "203.0.113.10"}, Port: 8445},
	}
}

// ---------- 2. Эмулятор: поведение сети → ProbeReport ----------

// labFailDetail — формы отказа tunnel-цели (язык живых логов, V2-026/051;
// «proxy» — мёртвый локальный SOCKS-инбаунд, форма из failclass.go/V2-028).
var labFailDetail = map[string]string{
	"reset":   `request failed: Get "https://api.ipify.org": read tcp 203.0.113.7:4431->93.184.216.34:443: connection reset by peer`,
	"timeout": `request failed: Get "https://api.ipify.org": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
	"eof":     `request failed: Get "https://api.ipify.org": unexpected EOF`,
	"notls":   `request failed: Get "https://api.ipify.org": tls: first record does not look like a TLS handshake`,
	"tls":     `request failed: Get "https://api.ipify.org": tls: failed to verify certificate: x509: certificate signed by unknown authority`,
	"refused": `request failed: Get "https://api.ipify.org": dial tcp 93.184.216.34:443: connect: connection refused`,
	"proxy":   `request failed: Get "https://api.ipify.org": proxyconnect tcp: dial tcp 127.0.0.1:1080: connect: connection refused`,
}

// labRenderReport — лабораторная модель пути: выставляет шаги ProbeReport
// по поведению middlebox и структурным флагам сценария. Две tunnel-цели —
// как в живом probe (2..3 цели, V2-037): форма Зла одна на обе (формы
// разводятся в одном отчёте только в failclass_test), Partial оставляет
// первую живой — срез момента для degraded-сценариев.
func labRenderReport(c labCase) *core.ProbeReport {
	rep := &core.ProbeReport{AllPassed: true}

	if c.ConfigFail {
		rep.Results = append(rep.Results, core.ProbeResult{Name: "конфигурация probe", Passed: false, Detail: "lab: невалидная конфигурация"})
		rep.AllPassed = false
		return rep
	}

	tunnel := func(name, passed bool) { /* placeholder removed */ }
	_ = tunnel

	t1Fail := c.Mbox.form != "normal"
	t2Fail := t1Fail && !c.Partial
	for _, st := range []struct {
		name string
		fail bool
	}{
		{"HTTPS https://api.ipify.org", t1Fail},
		{"HTTPS https://vpn.example.com/ip", t2Fail},
	} {
		if st.fail {
			rep.Results = append(rep.Results, core.ProbeResult{Name: st.name, Passed: false, Detail: labFailDetail[c.Mbox.form]})
			rep.AllPassed = false
		} else {
			rep.Results = append(rep.Results, core.ProbeResult{Name: st.name, Passed: true, Detail: "lab: egress 203.0.113.10 (лабораторная модель, не сеть)"})
		}
	}

	if c.Mbox.dnsMode == "fail" {
		rep.Results = append(rep.Results, core.ProbeResult{Name: "DNS через туннель (DoH)", Passed: false, Detail: "lab: dns lookup timed out"})
		rep.AllPassed = false
	} else {
		rep.Results = append(rep.Results, core.ProbeResult{Name: "DNS через туннель (DoH)", Passed: true, Detail: "lab: resolved via tunnel"})
	}

	// Direct-путь (TUN-aware семантика V2-037: живой direct в SOCKS-режиме
	// обязан показывать ЧУЖОЙ egress; detail без маркеров форм отказа —
	// alive-форма «other» для классификатора).
	if c.Mbox.directOk {
		rep.Results = append(rep.Results, core.ProbeResult{Name: "Direct leak check", Passed: true,
			Detail: "lab: direct egress 203.0.113.9 отличается от туннельного — утечки нет"})
	} else {
		rep.Results = append(rep.Results, core.ProbeResult{Name: "Direct leak check", Passed: true,
			Detail: "lab: direct недоступен — не доказательство (pass с оговоркой)"})
	}

	switch {
	case c.Mbox.egressIP == "":
		rep.Results = append(rep.Results, core.ProbeResult{Name: "Expected egress", Passed: false, Detail: "lab: egress не получен"})
		rep.AllPassed = false
	case c.Mbox.egressIP != c.Mbox.wantIP:
		rep.Results = append(rep.Results, core.ProbeResult{Name: "Expected egress", Passed: false,
			Detail: "lab: egress " + c.Mbox.egressIP + " != expected " + c.Mbox.wantIP})
		rep.AllPassed = false
	default:
		rep.Results = append(rep.Results, core.ProbeResult{Name: "Expected egress", Passed: true, Detail: "lab: egress ok"})
	}

	rep.Results = append(rep.Results, core.ProbeResult{Name: "Egress consistency", Passed: true, Detail: "lab: consistent"})
	return rep
}

// ---------- 3. Benchmark: prediction compared with ground truth ----------

// labMiss — расхождение предсказания с ground truth.
type labMiss struct {
	ID, True, Got string
}

// labBench — результат одного прогона benchmark.
type labBench struct {
	Total  int
	Hit    int
	Miss   []labMiss          // отсортировано по ID
	ByCode map[string]int     // распределение предсказанных кодов
	Conf   map[string]float64 // максимальная уверенность классификатора на класс ground truth
}

// labRunBenchmark — прогон живого ClassifyFailure по всему датасету.
// Никакой сети, никакой мутации прод-кода — чистое сравнение с разметкой.
func labRunBenchmark() labBench {
	b := labBench{ByCode: map[string]int{}, Conf: map[string]float64{}}
	for _, c := range labDataset() {
		fc := ClassifyFailure(labRenderReport(c), c.Port, 0)
		b.Total++
		b.ByCode[fc.Code]++
		if conf, ok := b.Conf[c.True]; !ok || fc.Confidence > conf {
			b.Conf[c.True] = fc.Confidence
		}
		hit := fc.Code == c.True
		if c.Healthy {
			// «Сбоя нет» в терминах классификатора: успешная ветка
			// (нулевая уверенность + честный hint «отчёт успешен»).
			hit = fc.Confidence == 0 && strings.Contains(fc.Hint, "успешен")
		}
		if hit {
			b.Hit++
		} else {
			b.Miss = append(b.Miss, labMiss{ID: c.ID, True: c.True, Got: fc.Code})
		}
	}
	sort.Slice(b.Miss, func(i, j int) bool { return b.Miss[i].ID < b.Miss[j].ID })
	return b
}

// labSummary — стабильная однострочная сводка (отчёт E3 для калибровки).
func labSummary(b labBench) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "total=%d hit=%d accuracy=%.2f", b.Total, b.Hit, float64(b.Hit)/float64(b.Total))
	if len(b.Miss) > 0 {
		sb.WriteString("; misses:")
		for _, m := range b.Miss {
			fmt.Fprintf(&sb, " %s[true=%s got=%s]", m.ID, m.True, m.Got)
		}
	}
	codes := make([]string, 0, len(b.ByCode))
	for k := range b.ByCode {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	sb.WriteString("; predicted:")
	for _, k := range codes {
		fmt.Fprintf(&sb, " %s=%d", k, b.ByCode[k])
	}
	return sb.String()
}

// TestLabDatasetIntegrity — датасет самосогласован до прогона классификатора:
// разметка из словаря THEORY, Зло не противоречит физике пути.
func TestLabDatasetIntegrity(t *testing.T) {
	known := map[string]bool{"": true, fcResetInjected: true, fcPortBlackholed: true,
		fcSilentFreeze: true, fcTlsPolicing443: true, fcWhitelistMode: true,
		fcChannelUnavail: true, fcDNSPathFailure: true, fcEgressDivergence: true,
		fcLeakConfirmed: true, fcServerUnreach: true, fcDegraded: true, fcUnknown: true}
	seen := map[string]bool{}
	for _, c := range labDataset() {
		if !known[c.True] {
			t.Errorf("%s: True=%q не из словаря THEORY §2.10", c.ID, c.True)
		}
		if seen[c.ID] {
			t.Errorf("%s: дубликат ID", c.ID)
		}
		seen[c.ID] = true
		if (c.True == "") != c.Healthy {
			t.Errorf("%s: Healthy и True=\"\" рассинхронизированы", c.ID)
		}
		if c.Mbox.form != "normal" && !c.Partial && c.Mbox.egressIP != "" {
			t.Errorf("%s: путь мёртв, но egress-эхо получено — зло противоречит физике", c.ID)
		}
		if c.Mbox.form == "normal" && c.Mbox.dnsMode == "pass" && c.Mbox.egressIP == c.Mbox.wantIP && !c.Healthy && !c.ConfigFail {
			t.Errorf("%s: здоровый путь размечен как сбой %q", c.ID, c.True)
		}
	}
}

// TestLabHarnessHealthyPasses — целостность харнесса: отчёт здорового пути
// ОБЯЗАН классифицироваться как «сбоя нет». Иначе харнесс сам инжектирует
// сбой и benchmark лжёт (урок V2-042: тест, врущий при живой системе,
// хуже отсутствия теста).
func TestLabHarnessHealthyPasses(t *testing.T) {
	healthy := labDataset()[0] // LAB-001
	if !healthy.Healthy {
		t.Fatal("LAB-001 обязан быть здоровым сценарием")
	}
	fc := ClassifyFailure(labRenderReport(healthy), healthy.Port, 0)
	if fc.Confidence != 0 || !strings.Contains(fc.Hint, "успешен") {
		t.Fatalf("здоровый отчёт классифицирован как сбой: %+v", fc)
	}
}

// TestLabBenchmark — E3: prediction compared with ground truth (plan_gpt §119).
// Значения зафиксированы на момент v1: деградация классификатора ломает тест
// (сигнал), а рост числа промахов виден в сводке для калибровки Phase E2.
// LAB-015 — единственный записанный промах: 443-маскировка непокрыема на
// этом слое evidence (нужен diagnostic probe), подгонять разметку запрещено.
func TestLabBenchmark(t *testing.T) {
	b := labRunBenchmark()
	t.Logf("LAB benchmark: %s", labSummary(b))
	if b.Total != 21 || b.Hit != 20 || len(b.Miss) != 1 ||
		!strings.HasPrefix(b.Miss[0].ID, "LAB-015") || b.Miss[0].True != fcTlsPolicing443 || b.Miss[0].Got != fcUnknown {
		t.Fatalf("baseline v1 изменился — пересмотреть датасет/код: %s", labSummary(b))
	}
	// Согласованность v1 с самим собой: каждый класс датасета обязан
	// достигать уверенности, которую v1 за него заявляет (экспертная
	// константа без подтверждения на датасете — шум для калибровки E2).
	declared := map[string]float64{
		fcResetInjected: confResetInjected, fcPortBlackholed: confPortBlackholed,
		fcSilentFreeze: confSilentFreeze, fcTlsPolicing443: confTlsPolicing443,
		fcWhitelistMode: confWhitelistMode, fcChannelUnavail: confChannelUnavail,
		fcDNSPathFailure: confDNSPathFailure, fcEgressDivergence: confEgressDivergence,
		fcServerUnreach: confServerUnreach, fcDegraded: confDegraded, fcUnknown: confUnknown,
	}
	for code, want := range declared {
		if b.Conf[code] < want {
			t.Errorf("класс %s: достигнутая уверенность %v < заявленной v1 %v", code, b.Conf[code], want)
		}
	}
}

// TestLabBenchmarkStable — детерминизм (план §6): два прогона дают один отчёт.
func TestLabBenchmarkStable(t *testing.T) {
	if s1, s2 := labSummary(labRunBenchmark()), labSummary(labRunBenchmark()); s1 != s2 {
		t.Fatalf("нестабильный benchmark:\n%s\n%s", s1, s2)
	}
}
