package main

// Failure classification describes bounded observations, never a censor or
// an inferred network policy. Legacy names/weights remain for old history;
// the live path consumes typed ProbeObservation and abstains without facts.

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/snowden-system/windows/backend/core"
)

// FailureClass — машинный диагноз поверх probe-отчёта.
type FailureClass struct {
	Code       string  // словарь THEORY §2.10 п.16 (failclassCodes)
	Confidence float64 // legacy heuristic weight, not calibrated probability
	Hint       string  // рекомендуемая реакция (наблюдаемая, не гипотеза)
}

// Коды словаря (THEORY §2.10 п.16). Изменение списка = правка THEORY + PLAN §5.
const (
	fcResetInjected          = "reset_injected"
	fcPortBlackholed         = "port_blackholed"
	fcSilentFreeze           = "silent_freeze"
	fcTlsPolicing443         = "tls_policing_443"
	fcWhitelistMode          = "whitelist_mode"
	fcChannelUnavail         = "protected_channel_unavailable"
	fcDNSPathFailure         = "dns_path_failure"
	fcEgressDivergence       = "egress_divergence"
	fcLeakConfirmed          = "direct_leak_confirmed"
	fcServerUnreach          = "server_unreachable" // AGENTS §3.3: активный отказ цели, не сеть
	fcDegraded               = "degraded"
	fcUnknown                = "unknown"
	fcObservationUnavailable = "observation_unavailable"
	fcEndpointFailure        = "measurement_endpoint_failure"
	fcTLSVerification        = "tls_verification_failed"
	fcTLSHandshake           = "tls_handshake_failed"
	fcConnectionFailed       = "connection_failed"
	fcRequestTimeout         = "request_timeout"
	fcResponseIncomplete     = "response_incomplete"
)

var failclassHints = map[string]string{
	fcObservationUnavailable: "Измерение не завершено/устарело/не допущено бюджетом; это не отрицательный факт о канале",
	fcEndpointFailure:        "Измеритель ответил HTTP или некорректным payload; общий отказ транспорта не установлен",
	fcTLSVerification:        "Проверка сертификата HTTPS-цели не прошла; часы, PKI, endpoint и путь требуют разделения, DPI не установлен",
	fcTLSHandshake:           "TLS handshake HTTPS-цели не завершён; номер порта не определяет причину",
	fcConnectionFailed:       "HTTP-клиент наблюдал ошибку соединения; конкретный отказавший hop и причина неизвестны",
	fcRequestTimeout:         "Истёк deadline запроса; блокировка порта/allowlist не установлены",
	fcResponseIncomplete:     "Получена часть ответа, затем чтение прервалось; freeze/источник разрыва не установлены",
	fcResetInjected:          "реакция: другой SNI/транспорт/канал (THEORY §2.10 п.16 ResetInjected)",
	fcPortBlackholed:         "реакция: 443-вариант канала (порт-эвристика THEORY §2.4a)",
	fcSilentFreeze:           "реакция: объёмная freeze-сигнатура — попробовать короткоживущий транспорт/UDP; если EOF на живом TLS/mux, проверить mux-контракт обоих концов (server multiplex+padding и per-channel descriptor), затем diagnostic probe",
	fcTlsPolicing443:         "реакция: mux-профиль/другой порт/канал (THEORY §2.10 п.16 TlsPolicing443)",
	fcWhitelistMode:          "реакция: честный BLOCKED — программного обхода allowlist нет (THEORY §2.10 F)",
	fcChannelUnavail:         "реакция: перебор validated каналов (failover)",
	fcDNSPathFailure:         "DoH-измеритель не подтвердил A-ответ; это не измерение native DNS/UDP и не доказательство отказа всего канала",
	fcEgressDivergence:       "Egress не соответствует контракту; защищённый путь не подтверждён, причина расхождения не установлена",
	fcLeakConfirmed:          "реакция: немедленный fail-closed без debounce; путь доказанно небезопасен",
	fcServerUnreach:          "реакция: цель активно отвергла соединение — сначала проверять сервер/порт, не сеть (AGENTS §3.3 ServerUnreachable)",
	fcDegraded:               "Разные цели дали разные результаты; частичный отказ измерителя не опровергает весь канал",
	fcUnknown:                "причина не определена — честно неизвестна; классификация без evidence = шум (AGENTS §3.3)",
}

// Уверенности — калибровка E2 по LAB-датасету (V2-070; до того — экспертные
// константы V2-056, не менявшиеся). Правило движения константы записано в
// журнале; полный разбор каждого значения — запись V2-070 PLAN.md §6.
const (
	// 0.95 — туннель ДОКАЗАН жив (цели ответили), но evidence против географии;
	// ось путаницы — только неверный expected-egress в конфиге (гейтится валидацией FR-005).
	confEgressDivergence = 0.95
	// 0.90→0.85 — СНИЖЕНО: tls-форма на не-443 при живом direct конфлатится по regex'у
	// (tls:|x509|certificate) с серверной TLS-ошибкой (истёкший/самоподписанный сертификат
	// цели), а allowlist — это гипотеза о ПОЛИТИКЕ сети, не наблюдаемый факт (AGENTS §3.3).
	confWhitelistMode = 0.85
	// 0.85 — timeout на не-443 при живом direct; ось путаницы — конгестия туннельного
	// пути в одном срезе (V2-051: tcpdump-доказанная тишина — база сигнатуры).
	confPortBlackholed = 0.85
	// 0.80 — notls/tls на 443: сигнатура чистая, но у класса есть непокрытая форма
	// предъявления (LAB-015: тихая маскировка видна как timeout = unknown для слоя).
	confTlsPolicing443 = 0.80
	// 0.70→0.80 — ПОДНЯТО: refused при живом direct — положительный артефакт провода
	// (RST от цели через выход туннеля), а не вывод из тишины; Windows-форма «actively
	// refused» покрыта (V2-026); воспроизведено live при switch (V2-044).
	confServerUnreach = 0.80
	// 0.75 — reset-форма: активный RST на пути; ось путаницы — RST от перегруженного
	// CDN-edge (форма не различает источник сброса — граница в hint).
	confResetInjected = 0.75
	// 0.75 — только DoH умер при живых HTTPS: ветка уникально ключуется, канал жив;
	// ось путаницы — отказ DoH-эндпоинта (не подмена) — реакция одна и та же.
	confDNSPathFailure = 0.75
	// 0.60 — «всё мертво» либо локальный прокси; подветки разной силы: proxy-форма
	// железная (движок мёртв — факт), «всё мертво» — вывод из отсутствия ответов.
	confChannelUnavail = 0.60
	// 0.50 — один срез частичного здоровья; цели probe — внешние сервисы, их
	// собственная флапи — часть шума среза (тест честно требует ≥0.50).
	confDegraded = 0.50
	// 0.55→0.50 — СНИЖЕНО: eof-regex (eof|closed|aborted) конфлатит штатное закрытие
	// потока (GOAWAY/idle-timeout) с принудительной резкой; маркер, не диагноз —
	// точную границу покажет diagnostic probe (Phase A2), граница объявлена в hint.
	confSilentFreeze = 0.50
	// 0.00 — честное «неизвестно»: классификация без evidence = шум (AGENTS §3.3).
	confUnknown = 0.0
)

// паттерны форм отказа цели (нижний регистр, см. classifyProbeStep).
var (
	reConnReset   = regexp.MustCompile(`connection reset|connection was reset`)
	reConnRefused = regexp.MustCompile(`connection refused`)
	reNoTLS       = regexp.MustCompile(`first record does not look like a tls handshake`) // не TLS-ответ на TLS-порте
	reTLSErr      = regexp.MustCompile(`tls: |x509|certificate`)
	reTimeout     = regexp.MustCompile(`timeout|context deadline`)
	reProxyErr    = regexp.MustCompile(`socks connect|socks5h|proxyconnect|proxy: `)
	reEOF         = regexp.MustCompile(`eof|closed|aborted`)
)

// classifyProbeStep — форма отказа одного HTTPS-шага probe по его Detail.
// Возвращает "", если шаг не провален или форма не распознана.
//
// Порядок важен: прокси-граница (SOCKS-диалер) ПЕРЕНОСИТ ошибку ОС, поэтому
// её маркер проверяется первым. SOCKS-диалер пишет удалённый адрес после
// "->": форма отказа защищённого пути видна в ХВОСТЕ (reset/timeout/…);
// локальная смерть инбаунда идёт без "->" (dial tcp 127.0.0.1…). Паттерны
// в нижнем регистре — вход уже ToLower (урок теста: `EOF` в регистре
// никогда не совпадал с "unexpected eof" и класс молча уезжал в unknown).
func classifyProbeStep(detail string) string {
	d := strings.ToLower(detail)
	switch {
	case d == "":
		return ""
	case reProxyErr.MatchString(d):
		if i := strings.LastIndex(d, "->"); i >= 0 {
			if f := classifyProbeStep(d[i+2:]); f != "" && f != "other" {
				return f // удалённая форма сильнее маркера границы
			}
		}
		return "proxy"
	case reConnReset.MatchString(d):
		return "reset"
	case reConnRefused.MatchString(d):
		return "refused"
	case reTimeout.MatchString(d):
		return "timeout"
	case reNoTLS.MatchString(d):
		return "notls"
	case reTLSErr.MatchString(d):
		return "tls"
	case reEOF.MatchString(d):
		return "eof"
	default:
		return "other"
	}
}

// splitSteps — разделяет шаги отчёта на tunnel/direct/dns/egress/consistency.
type probeStepsSplit struct {
	tunnelFail  []string // формы отказа tunnel-целей (только проваленных)
	tunnelPass  int
	directForm  string
	directPass  bool
	directLeak  bool
	dnsFail     bool
	dnsDetail   string
	egressFail  bool
	consistFail bool
	configFail  bool
}

func splitSteps(r *core.ProbeReport) probeStepsSplit {
	s := probeStepsSplit{}
	if r == nil {
		return s
	}
	for _, res := range r.Results {
		res = res.NormalizeDiagnostic()
		if res.Informational() {
			continue
		}
		switch {
		case strings.HasPrefix(res.Name, "HTTPS "):
			if res.Passed {
				s.tunnelPass++
			} else {
				s.tunnelFail = append(s.tunnelFail, classifyProbeStep(res.Detail))
			}
		case res.Name == "Direct leak check":
			s.directPass = res.Passed
			s.directLeak = res.Status == "leak"
			s.directForm = classifyProbeStep(res.Detail)
		case strings.HasPrefix(res.Name, "DNS"):
			s.dnsFail = !res.Passed
			s.dnsDetail = strings.ToLower(res.Detail)
		case res.Name == "Expected egress":
			s.egressFail = !res.Passed
		case res.Name == "Egress consistency":
			s.consistFail = !res.Passed
		case res.Name == "конфигурация probe" || res.Name == "контекст":
			s.configFail = true
		}
	}
	return s
}

// allTunnelForms — единая форма отказа всех tunnel-целей (или "").
func allTunnelForms(s probeStepsSplit) string {
	if len(s.tunnelFail) == 0 {
		return ""
	}
	for _, f := range s.tunnelFail {
		if f != s.tunnelFail[0] {
			return ""
		}
	}
	return s.tunnelFail[0]
}

// ClassifyFailure keeps the caller API while ignoring causal port heuristics.
func ClassifyFailure(r *core.ProbeReport, channelPort, probePort int) FailureClass {
	// Ports remain in the signature for existing callers. They do not identify
	// TLS policing, an allowlist, endpoint independence or a censorship cause.
	return classifyObservedFailure(r)
}

// classifyFailureLogLine — однострочный redacted-текст для журнала сторожа.
// Значений секретов нет по построению ProbeReport (FR-003).
func classifyFailureLogLine(r *core.ProbeReport, channelPort, probePort int) string {
	fc := ClassifyFailure(r, channelPort, probePort)
	return "класс отказа: " + fc.Code +
		" (эвристический вес, не вероятность: " + strconv.FormatFloat(fc.Confidence, 'f', 2, 64) + ")"
}

// probeTunnelPort is descriptive endpoint metadata, never a diagnosis.
func probeTunnelPort(targets []string) int {
	if len(targets) == 0 {
		return 0
	}
	u, err := url.Parse(targets[0])
	if err != nil {
		return 0
	}
	if p := u.Port(); p != "" {
		n, _ := strconv.Atoi(p)
		return n
	}
	switch u.Scheme {
	case "https":
		return 443
	case "http":
		return 80
	default:
		return 0
	}
}
