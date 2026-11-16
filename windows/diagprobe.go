package main

// Production controlled-volume observations (PLAN PC-03). Each fixed range
// reports received bytes, time and observed error shape. Incomplete bodies or
// timeout do not establish DPI, a volume threshold or the cause of failure.
// The configured mixed listener and OS-route requests can share another VPN;
// physical independence is unknown. Our TUN omits the comparison requests.
// The GUI binds the original subject/context and retains optional admission.

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Fixed application body sizes, not a proven failure threshold or baseline.
// One owned copy is validated and reserved before any request.
var diagSteps = []int{8 << 10, 16 << 10, 32 << 10, 64 << 10}

// diagReadTimeout — таймаут на чтение каждой ступени. Заморозка = тишина
// ПОСЛЕ частичных данных; полный таймаут до данных = отдельная форма
// (port_blackholed-класс, не freeze).
const diagReadTimeout = 8 * time.Second

// diagOutcome — исход одной ступени.
//
// L0-тайминги (V2-079): TTFBus — время до первого байта ответа (заголовки),
// ElapsedUs — полное время ступени. Form различает ФОРМУ заморозки:
//
//	silence — поток не закрывался, клиент ждал до таймаута (тихий drop ТСПУ,
//	          THEORY §2.4a/§2.10 п.16: «не RST — соединение просто перестаёт
//	          получать данные»);
//	abrupt  — EOF/RST до таймаута (активное закрытие: сервер/edge/политика) —
//	          ДРУГАЯ наблюдаемая форма при том же статусе freeze;
//	no_data — данных не было вовсе (класс port_blackholed/неответ, не freeze).
//
// Различение нужно для калибровки: раньше обе формы сливались в один freeze.
type diagOutcome struct {
	Name      string `json:"name"`   // "туннель 16 КБ" / "direct 16 КБ"
	OK        bool   `json:"ok"`     // true = объём получен целиком
	Status    string `json:"status"` // complete|freeze|reset|timeout|error|not_independent
	Form      string `json:"form"`   // silence|abrupt|no_data|"" (V2-079)
	Detail    string `json:"detail"`
	Got       int    `json:"gotBytes"`
	Want      int    `json:"wantBytes"`
	TTFBus    int    `json:"ttfbUs"`    // время до первого байта ответа (мкс)
	ElapsedUs int    `json:"elapsedUs"` // полное время ступени (мкс)
}

// diagReport — итог диагностики.
type diagReport struct {
	OK                  bool          `json:"ok"`            // all planned tunnel bodies completed; no causal diagnosis
	DirectControl       bool          `json:"directControl"` // OS-route requests attempted outside our TUN, independence unknown
	Steps               []diagOutcome `json:"steps"`
	Summary             string        `json:"summary"` // однострочный факт для журнала
	expectedTunnelSteps int           // exact captured workload, private; no empty success
	observedTunnelSteps int
}

// runFreezeDiagnostic — прогон ступеней через туннель и, если это возможно,
// напрямую. В TUN auto_route «direct» HTTP-клиент тоже попадает в туннель;
// второй запрос там не является контрольной группой и намеренно не выполняется
// (V2-071, аудит D4a). DirectControl=false — честное ограничение измерения.
// socksAddr — mixed-инбаунд ядра; diagURL — controlled-volume эндпоинт
// канала (дескриптор diagnostic_url). budget ограничивает общее время.
func runFreezeDiagnostic(ctx context.Context, socksAddr, diagURL string, budget time.Duration, tunMode bool) diagReport {
	return runFreezeDiagnosticOwned(ctx, socksAddr, diagURL, budget, tunMode, nil)
}

// GUI ownership retains the same optional reservation until the original call,
// transport defers and cancellation watcher finish. Generic callers keep their
// existing lexical release. Neither is a native/library task retirement proof.
func runFreezeDiagnosticOwned(ctx context.Context, socksAddr, diagURL string, budget time.Duration, tunMode bool, owner *manualDiagnosticTask) diagReport {
	rep := diagReport{OK: true, DirectControl: !tunMode}
	if diagURL == "" {
		rep.OK = false
		rep.Summary = "диагностика недоступна: у канала нет diagnostic_url (дескриптор)"
		return rep
	}
	sizes := append([]int(nil), diagSteps...)
	if len(sizes) == 0 || len(sizes) > 8 {
		rep.OK, rep.Summary = false, "диагностика недоступна: нет допустимого ограниченного плана объёмов"
		return rep
	}
	rep.expectedTunnelSteps = len(sizes)
	requests, bodyBytes := len(sizes), int64(0)
	for _, size := range sizes {
		if size <= 0 || size > 64<<10 {
			rep.OK, rep.Summary = false, "диагностика недоступна: неверный размер ступени"
			return rep
		}
		bodyBytes += int64(size)
	}
	if !tunMode {
		requests, bodyBytes = requests*2, bodyBytes*2
	}
	ctx, release, err := core.AdmitDiagnostic(ctx, core.DiagnosticVolume, requests, bodyBytes, budget)
	if err != nil {
		rep.OK, rep.Summary = false, "диагностика не выполнена: общий бюджет занят/исчерпан; это не факт отказа сети"
		return rep
	}
	if owner == nil {
		defer release()
	} else if owner.release != nil {
		release() // unused second reservation; never replace the original owner
		rep.OK, rep.Summary = false, "диагностика недоступна: исходный бюджет уже занят"
		return rep
	} else {
		owner.release = release // synchronous call/finisher only, not the watcher
	}
	tunnel := &http.Client{
		Timeout: diagReadTimeout,
		Transport: &http.Transport{
			DisableCompression:     true,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 16 << 10,
			Proxy:                  socksProxyURL(socksAddr),
			DialContext:            dialerFor(diagReadTimeout).DialContext,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	direct := &http.Client{Timeout: diagReadTimeout, Transport: &http.Transport{DisableCompression: true, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if owner != nil {
		tunnelTransport := tunnel.Transport.(*http.Transport)
		tunnelTransport.DialContext = owner.ownedDialer(tunnelTransport.DialContext)
		directTransport := direct.Transport.(*http.Transport)
		var directDialer net.Dialer // same zero-dialer behavior as Transport's nil DialContext fallback
		directTransport.DialContext = owner.ownedDialer(directDialer.DialContext)
		directTransport.ForceAttemptHTTP2 = true // preserve default HTTP/2 eligibility after installing a custom dial hook
	}
	defer tunnel.CloseIdleConnections()
	defer direct.CloseIdleConnections()
	for _, want := range sizes {
		if !lifecyclePublicationCurrent(ctx) || owner != nil && owner.currentError() != nil {
			rep.OK = false
			rep.Summary += " — бюджет исчерпан, ступени выше не проверены"
			break
		}
		// First the configured mixed listener; then an OS-route comparison.
		oT := fetchRange(ctx, tunnel, diagURL, want, "туннель")
		rep.observedTunnelSteps++
		rep.Steps = append(rep.Steps, oT)
		if tunMode {
			rep.Steps = append(rep.Steps, diagOutcome{
				Name:   fmt.Sprintf("direct %d КБ", want>>10),
				OK:     false,
				Status: "not_independent",
				Detail: "в TUN-режиме direct-путь автоматически попадает в тот же туннель — независимый контроль не выполнялся",
				Want:   want,
			})
			if !oT.OK {
				rep.OK = false
			}
			continue
		}
		if !lifecyclePublicationCurrent(ctx) || owner != nil && owner.currentError() != nil {
			rep.OK = false
			break
		}
		oD := fetchRange(ctx, direct, diagURL, want, "direct")
		rep.Steps = append(rep.Steps, oD)
		if !oT.OK {
			rep.OK = false
		}
	}
	rep.Summary = summarizeDiag(rep)
	if !lifecyclePublicationCurrent(ctx) || rep.observedTunnelSteps != rep.expectedTunnelSteps || owner != nil && owner.currentError() != nil {
		rep.OK = false
		rep.Summary += "; общий deadline истёк/операция отменена, оставшиеся объёмы не измерены"
	}
	return rep
}

// fetchRange — GET с Range: bytes=0-(want-1). Читает ровно want байт
// (или до обрыва/таймаута). Имя ступени содержит объём для UI.
func fetchRange(ctx context.Context, client *http.Client, url string, want int, via string) diagOutcome {
	name := fmt.Sprintf("%s %d КБ", via, want>>10)
	out := diagOutcome{Name: name, Want: want}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		out.Status = "error"
		out.Detail = "запрос не создан; причина по этому наблюдению не установлена"
		return out
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", want-1))
	resp, err := client.Do(req)
	if err != nil {
		out.ElapsedUs = diagUs(time.Since(start))
		out.Status, out.Detail = classifyDiagErr(err.Error())
		out.Detail = fmt.Sprintf("%s (всего %s)", out.Detail, diagDurText(out.ElapsedUs))
		if out.Status == "timeout" {
			out.Form = "no_data"
		}
		return out
	}
	defer resp.Body.Close()
	// TTFB: заголовки получены — фиксируем L0-задержку до первого байта.
	out.TTFBus = diagUs(time.Since(start))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		out.Status = "error"
		out.Detail = fmt.Sprintf("HTTP %d (ожидался 200/206; TTFB %s)", resp.StatusCode, diagDurText(out.TTFBus))
		return out
	}
	got, rerr := io.CopyN(io.Discard, resp.Body, int64(want))
	out.Got = int(got)
	out.ElapsedUs = diagUs(time.Since(start))
	switch {
	case rerr == nil:
		out.OK, out.Status = true, "complete"
		out.Detail = fmt.Sprintf("получено %d/%d байт — объём прошёл (TTFB %s, всего %s)",
			out.Got, want, diagDurText(out.TTFBus), diagDurText(out.ElapsedUs))
	case got > 0:
		// Любая ошибка чтения ПОСЛЕ частичных данных — наблюдаемая форма
		// «объём начал проходить, затем поток перестал доставлять данные».
		// Форма (silence vs abrupt) и L0-тайминги фиксируются отдельно: это
		// evidence для классификатора и калибровки (V2-079).
		out.Status = "freeze"
		out.Form = freezeForm(rerr, time.Since(start), client.Timeout)
		out.Detail = fmt.Sprintf("поток перестал доставлять данные после %d/%d байт — %s (%s; TTFB %s, всего %s)",
			out.Got, want, shortDiagError(rerr), freezeFormLabel(out.Form), diagDurText(out.TTFBus), diagDurText(out.ElapsedUs))
	default:
		out.Status, out.Detail = classifyDiagErr(rerr.Error())
		out.Form = "no_data"
		out.Detail = fmt.Sprintf("оборван на %d/%d байт: %s (всего %s)", out.Got, want, out.Detail, diagDurText(out.ElapsedUs))
	}
	return out
}

// diagUs — длительность в микросекундах: усечение до миллисекунд теряло
// тайминги быстрых ступеней (<1 мс) — ровно тот evidence, ради которого
// L0-замеры и добавлены (V2-079).
//
// Честная граница платформы: монотонные часы Windows могут иметь крупный тик —
// измерение 200 000 self-замеров time.Since дало min=0 ns, то есть локальная
// ступень короче тика читается как 0 мкс. Это факт часов, не дефект;
// поэтому строгие «>0» ассерты применяются только к многотиковым интервалам
// (таймауты, stall), а для быстрых ступеней проверяется наличие evidence.
func diagUs(d time.Duration) int { return int(d.Microseconds()) }

// diagDurText — человекочитаемая длительность (мс при ≥1 мс, иначе мкс).
func diagDurText(us int) string {
	if us >= 1000 {
		return fmt.Sprintf("%dмс", us/1000)
	}
	return fmt.Sprintf("%dмкс", us)
}

// Observed read-error shape only. Timing proximity cannot turn EOF/reset into
// silence. Keep the existing call signature; no inferred DPI or wire RST proof.
func freezeForm(err error, _, _ time.Duration) string {
	if err == nil {
		return ""
	}
	m := strings.ToLower(err.Error())
	timedOut := strings.Contains(m, "timeout") || strings.Contains(m, "context deadline")
	abrupt := strings.Contains(m, "reset") || strings.Contains(m, "eof")
	switch {
	case timedOut:
		return "silence"
	case abrupt:
		return "abrupt"
	default:
		return "unknown"
	}
}

// freezeFormLabel — человекочитаемая форма для Detail/журнала (без секретов).
func freezeFormLabel(form string) string {
	switch form {
	case "silence":
		return "форма: тишина до таймаута (поток не закрывался)"
	case "abrupt":
		return "форма: резкий обрыв EOF/RST до таймаута"
	case "no_data":
		return "форма: данных не было"
	case "unknown":
		return "форма: неизвестна"
	default:
		return "форма: не зафиксирована"
	}
}

// classifyDiagErr — форма ошибки загрузки (изометрично failclass.go):
// полный таймаут до данных = "timeout" (это НЕ freeze), RST = "reset".
func classifyDiagErr(msg string) (status, detail string) {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "timeout") || strings.Contains(m, "context deadline"):
		return "timeout", "запрос завершился таймаутом до данных; причина не установлена"
	case strings.Contains(m, "connection reset"):
		return "reset", "ошибка запроса содержит connection reset; пакетная причина не установлена"
	default:
		return "error", "запрос завершился ошибкой; причина по этому наблюдению не установлена"
	}
}

func shortDiagError(err error) string {
	if err == nil {
		return "без ошибки"
	}
	m := strings.ToLower(err.Error())
	switch {
	case strings.Contains(m, "timeout") || strings.Contains(m, "context deadline"):
		return "read-timeout после частичных данных"
	case strings.Contains(m, "reset"):
		return "connection reset после частичных данных"
	case strings.Contains(m, "eof"):
		return "EOF после частичных данных"
	default:
		return "ошибка чтения после частичных данных"
	}
}

// summarizeDiag — однострочный факт: что показал прогон.
func summarizeDiag(rep diagReport) string {
	complete := 0
	for _, step := range rep.Steps {
		if strings.HasPrefix(step.Name, "туннель ") && step.OK {
			complete++
		}
	}
	return fmt.Sprintf("Полностью получено tunnel-ступеней: %d из %d. Остальные наблюдения — в шагах; причина сбоя и физическая независимость маршрутов не установлены.", complete, rep.expectedTunnelSteps)
}

// ---------- интеграция со страницей «Проверки» (runTest) ----------

// GUI wrapper: original applied subject before/after, exact captured workload,
// unavailable without that scope/plan; never start an independent Background ctx.
func (a *App) testFreezeCheck(task *manualDiagnosticTask) (*bool, string, []ProbeStepView) {
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	subject := a.appliedProbeSubject(cc)
	if !subject.current(a, task.ctx, cc) {
		return diagnosticUnavailable("Нет текущего применённого scope для диагностики объёма. Запустите VPN и проверьте защищённый путь.")
	}
	if cc.DiagnosticURL == "" {
		return diagnosticUnavailable("У текущего канала нет diagnostic_url; измерение не выполнялось.")
	}
	target, err := url.Parse(cc.DiagnosticURL)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" || target.User != nil || target.Fragment != "" {
		return diagnosticUnavailable("Назначение диагностики должно быть явным HTTPS URL без userinfo/fragment; запрос не отправлен.")
	}
	mixed := net.JoinHostPort(render.MixedListenAddress, strconv.Itoa(render.MixedListenPort))
	rep := runFreezeDiagnosticOwned(task.ctx, mixed, cc.DiagnosticURL, 80*time.Second, tunRequested(), task)
	steps := make([]ProbeStepView, 0, len(rep.Steps))
	for _, s := range rep.Steps {
		status := "fail"
		if s.Status == "not_independent" {
			status = "skipped"
		} else if s.OK {
			status = "pass"
		}
		steps = append(steps, ProbeStepView{Name: s.Name, Status: status, Detail: s.Detail})
	}
	if !subject.current(a, task.ctx, cc) || task.currentError() != nil {
		return diagnosticUnavailable("Исходный runtime/owner/intent/путь изменился; наблюдение не публикуется для преемника.")
	}
	if rep.expectedTunnelSteps == 0 || rep.observedTunnelSteps != rep.expectedTunnelSteps {
		detail := rep.Summary + " Полный план не измерен; результат неизвестен."
		steps = append(steps, ProbeStepView{Name: "полнота измерения", Status: "unavailable", Detail: detail})
		return nil, detail, steps
	}
	ok := rep.OK
	return &ok, rep.Summary, steps
}

// socksProxyURL / dialerFor — локальные хелперы (избегаем зависимости от
// внутренностей probe): URL прокси и диалер с таймаутом.
func socksProxyURL(addr string) func(*http.Request) (*url.URL, error) {
	return http.ProxyURL(&url.URL{Scheme: "socks5h", Host: addr})
}

func dialerFor(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout}
}
