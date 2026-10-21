package main

// failclass.go — детерминированный классификатор причин срыва защищённого
// пути v1 (THEORY §2.10 п.16; PLAN §5; plan_gpt §17/§19/§63).
//
// Правило (AGENTS §3.3): категория описывает НАБЛЮДАЕМЫЙ факт, а не гипотезу
// «это точно DPI». Вход — структурный *core.ProbeReport (секретов не несёт),
// выход — FailureClass {Code, Confidence, Hint}: код из словаря THEORY,
// confidence — калиброванная константа (E2, V2-070): поднята там, где
// сигнатура — положительный артефакт провода (активный RST/refused от
// конкретного источника), снижена там, где класс выводится из отсутствия
// улик или широкого паттерна; база — LAB-датасет (windows/lab_test.go,
// V2-069: 18 сценариев, 0 внутри-словарных путаниц).
//
// Честные границы v1 (не «серебряная пуля», план §8):
//   - probe качает ≤256 байт на цель: 16КБ-заморозка ТСПУ и volume-policing
//     невидимы для этого слоя (нужен diagnostic probe — PLAN Phase A2);
//   - base-классификация различает только форму отказа цели (reset/timeout/
//     обрыв середины/HTTP-статус), не фазу TLS — RST-vs-TLS-полиция
//     разделяется портом (443-эвристика THEORY §2.4a), а не разбором ClientHello;
//   - classifier влияет только на безопасную policy: leak/allowlist закрывают
//     путь, transport/port-классы меняют порядок validated кандидатов;
//     применение всё равно идёт через failoverPolicy и Manager-only путь.
//
// Все функции чистые — тестируются таблицами без сети (failclass_test.go).

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
	Confidence float64 // 0..1, экспертная константа v1
	Hint       string  // рекомендуемая реакция (наблюдаемая, не гипотеза)
}

// Коды словаря (THEORY §2.10 п.16). Изменение списка = правка THEORY + PLAN §5.
const (
	fcResetInjected    = "reset_injected"
	fcPortBlackholed   = "port_blackholed"
	fcSilentFreeze     = "silent_freeze"
	fcTlsPolicing443   = "tls_policing_443"
	fcWhitelistMode    = "whitelist_mode"
	fcChannelUnavail   = "protected_channel_unavailable"
	fcDNSPathFailure   = "dns_path_failure"
	fcEgressDivergence = "egress_divergence"
	fcLeakConfirmed    = "direct_leak_confirmed"
	fcServerUnreach    = "server_unreachable" // AGENTS §3.3: активный отказ цели, не сеть
	fcDegraded         = "degraded"
	fcUnknown          = "unknown"
)

var failclassHints = map[string]string{
	fcResetInjected:    "реакция: другой SNI/транспорт/канал (THEORY §2.10 п.16 ResetInjected)",
	fcPortBlackholed:   "реакция: 443-вариант канала (порт-эвристика THEORY §2.4a)",
	fcSilentFreeze:     "реакция: объёмная freeze-сигнатура — попробовать короткоживущий транспорт/UDP; если EOF на живом TLS/mux, проверить mux-контракт обоих концов (server multiplex+padding и per-channel descriptor), затем diagnostic probe",
	fcTlsPolicing443:   "реакция: mux-профиль/другой порт/канал (THEORY §2.10 п.16 TlsPolicing443)",
	fcWhitelistMode:    "реакция: честный BLOCKED — программного обхода allowlist нет (THEORY §2.10 F)",
	fcChannelUnavail:   "реакция: перебор validated каналов (failover)",
	fcDNSPathFailure:   "реакция: bootstrap-стратегии уже применяются рендером (V2-037); канал обычно жив",
	fcEgressDivergence: "реакция: egress-измерение независимо от транспорта (V2-041) — канал жив, путь подменён",
	fcLeakConfirmed:    "реакция: немедленный fail-closed без debounce; путь доказанно небезопасен",
	fcServerUnreach:    "реакция: цель активно отвергла соединение — сначала проверять сервер/порт, не сеть (AGENTS §3.3 ServerUnreachable)",
	fcDegraded:         "реакция: серия быстрых репроб (fastReprobe)",
	fcUnknown:          "причина не определена — честно неизвестна; классификация без evidence = шум (AGENTS §3.3)",
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

// ClassifyFailure — детерминированный v1-классификатор поверх отчёта probe.
// ВХОД: только структурный отчёт (секретов нет). ВЫХОД: класс с честным
// unknown, если паттерн не согласован. Никогда не паникует на nil.
//
// Порты (V2-068, аудит D1): channelPort — порт, который клиент РЕАЛЬНО
// набирает у канала (render.ClientConfig.TunnelPort: 8443 reality, 8444 hy2,
// 8445 shadowtls, 443 CDN-ветка); probePort — порт первой probe-цели.
// Приоритет у channelPort: порт-эвристика THEORY §2.4a описывает блокировку
// ORIGIN-порта, а не порта стороннего эхо-сервиса (у всех встроенных
// дескрипторов это https/443 — из-за чего диагноз port_blackholed был
// недостижим в проде до этой правки). probePort участвует только как фолбэк
// (канал без дескриптора / порт не задан) — честная деградация вместо выдумки.
func ClassifyFailure(r *core.ProbeReport, channelPort, probePort int) FailureClass {
	tunnelPort := channelPort
	if tunnelPort == 0 {
		tunnelPort = probePort
	}
	s := splitSteps(r)
	hintOf := func(code string) FailureClass {
		return FailureClass{Code: code, Hint: failclassHints[code]}
	}

	if r == nil || s.configFail {
		c := hintOf(fcUnknown)
		c.Confidence = confUnknown
		return c
	}
	if s.directLeak || r.LeakConfirmed {
		return FailureClass{Code: fcLeakConfirmed, Confidence: 1.0, Hint: failclassHints[fcLeakConfirmed]}
	}
	// Успешный отчёт — классифицировать нечего (вызов без сбоя — ошибка вызова).
	if r.AllPassed && !s.egressFail && !s.consistFail {
		return FailureClass{Code: fcUnknown, Confidence: confUnknown, Hint: "отчёт успешен — срыва нет"}
	}

	// 1) Подмена пути: egress отвечает, но чужим IP (V2-041: измерение egress
	//    независимо от транспорта).
	if s.egressFail && len(s.tunnelFail) == 0 {
		c := hintOf(fcEgressDivergence)
		c.Confidence = confEgressDivergence
		return c
	}

	// 2) Только DNS при живых HTTPS — путь жив, отказа канала нет.
	if s.dnsFail && len(s.tunnelFail) == 0 && !s.egressFail {
		c := hintOf(fcDNSPathFailure)
		c.Confidence = confDNSPathFailure
		return c
	}

	// 3) Часть целей жива — частичная деградация (в т.ч. consistency).
	if s.tunnelPass > 0 && len(s.tunnelFail) > 0 || s.consistFail && s.tunnelPass > 0 {
		c := hintOf(fcDegraded)
		c.Confidence = confDegraded
		return c
	}

	// 4) Все tunnel-цели мертвы — форма отказа разделяет классы.
	form := allTunnelForms(s)
	// directAlive — по ФОРМЕ detail, не по флагу Passed: directLeakResult
	// помечает недоступный direct как Passed=true с оговоркой (V2-037),
	// и живой direct всегда даёт «other» (egress-IP в тексте), а недоступный —
	// транспортную форму (timeout/refused/tls/…).
	directAlive := s.directPass && s.directForm == "other"
	if len(s.tunnelFail) > 0 {
		switch form {
		case "reset":
			c := hintOf(fcResetInjected)
			c.Confidence = confResetInjected
			return c
		case "refused":
			// Активное отвержение цели. Через туннель при живом direct это
			// сервер/порт (sing-box не слушает, порт закрыт), не middlebox:
			// порт-блэкхол ТСПУ молчит (timeout), а не отвечает RST (V2-051).
			if directAlive {
				c := hintOf(fcServerUnreach)
				c.Confidence = confServerUnreach
				return c
			}
			// direct тоже мёртв — атрибутировать отказ некому: честный unknown.
		case "timeout":
			// На 443 таймаут без других улик = unknown (честно; RST-эвристика
			// ТСПУ маскируется). На не-443 origin-порту с живым direct —
			// чёрная дыра порта (V2-051 — доказано live).
			if tunnelPort != 0 && tunnelPort != 443 && directAlive {
				c := hintOf(fcPortBlackholed)
				c.Confidence = confPortBlackholed
				return c
			}
			if s.dnsFail {
				c := hintOf(fcChannelUnavail)
				c.Confidence = confChannelUnavail
				return c
			}
		case "notls":
			// На TLS-порте пришло не-TLS — активная подмена контента.
			c := hintOf(fcTlsPolicing443)
			c.Confidence = confTlsPolicing443
			return c
		case "tls":
			if tunnelPort == 443 || tunnelPort == 0 {
				c := hintOf(fcTlsPolicing443)
				c.Confidence = confTlsPolicing443
				return c
			}
			// TLS-ошибка на не-443 origin-порту при живом direct —
			// allowlist-режим (L6, THEORY §2.10 F): обхода программой нет.
			if directAlive {
				c := hintOf(fcWhitelistMode)
				c.Confidence = confWhitelistMode
				return c
			}
		case "eof":
			// Обрыв без RST — молчаливое отсечение потока (ближайший v1-маркер
			// 16КБ-механики; точную границу даст diagnostic probe, PLAN A2).
			c := hintOf(fcSilentFreeze)
			c.Confidence = confSilentFreeze
			return c
		case "proxy":
			// SOCKS-инбаунд ядра недоступен — канал не срывался, движок мёртв.
			c := hintOf(fcChannelUnavail)
			c.Confidence = confChannelUnavail
			return c
		}
	}

	// 5) DNS тоже мёртв целиком — защищённый путь недоступен целиком.
	if s.dnsFail && len(s.tunnelFail) > 0 {
		c := hintOf(fcChannelUnavail)
		c.Confidence = confChannelUnavail
		return c
	}

	c := hintOf(fcUnknown)
	c.Confidence = confUnknown
	return c
}

// classifyFailureLogLine — однострочный redacted-текст для журнала сторожа.
// Значений секретов нет по построению ProbeReport (FR-003).
func classifyFailureLogLine(r *core.ProbeReport, channelPort, probePort int) string {
	fc := ClassifyFailure(r, channelPort, probePort)
	return "класс отказа: " + fc.Code +
		" (уверенность v1: " + strconv.FormatFloat(fc.Confidence, 'f', 2, 64) + ")"
}

// probeTunnelPort — ФОЛБЭК-порт из первой probe-цели (V2-037). Основной
// источник порта для классификатора — render.ClientConfig.TunnelPort (реальный
// порт канала, V2-068); эта функция применяется только когда порт канала
// неизвестен. Ошибка разбора или пустой список = 0 (тогда 443-эвристика не
// применяется — честный unknown).
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
