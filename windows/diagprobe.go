package main

// diagprobe.go — диагностический probe заморозки ТСПУ (PLAN §4 Phase A2;
// plan_gpt §15/§40–42; THEORY §2.10 п.17 — методика Cheburcheck «тест объёма»).
//
// Проблема (честная граница V2-056): protected probe качает ≤256 байт на цель
// и НЕ ВИДИТ 16КБ-заморозку — механику «успех до ~N КБ, затем тишина без
// RST» (§2.9/§2.4a). Диагностика:
//
//  1. скачивает контролируемые диапазоны (8КБ → 64КБ) через ЖИВОЙ туннель
//     (SOCKS 127.0.0.1:1080 — mixed-инбаунд есть и в TUN-режиме, V2-037);
//  2. тот же объём НАПРЯМУЮ (direct) — контрольная группа: direct-обрыв
//     при живом туннеле = механика на пути к origin; оба живы = заморозки
//     нет СЕЙЧАС (это не гарантия на будущее);
//  3. результат — факты по каждому объёму: получено байт / ждалось /
//     форма исхода (complete | freeze | reset | timeout), классификация
//     изометрична failclass.go.
//
// Границы (честно, THEORY §2.10 п.11): серверная разбивка контента —
// гипотеза; здесь измеряется ТОЛЬКО факт прохождения объёма, а не «лечение».
// Заморозка детектируется по таймауту чтения ПОСЛЕ частичных данных
// (заголовки пришли, тело оборвалось) — точную границу N даёт ступенчатый
// прогон 8/16/32/64КБ.
//
// Секретов не несёт; значения не логируются. Все функции чистые, кроме
// самой загрузки (HTTP по переданному клиенту) — тестируются httptest'ом.

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// diagSteps — ступени объёма (байт). 8КБ = база (всегда жив), 16КБ = порог
// заморозки §2.4a, 32/64КБ = надпороговые (confirmation). Суммарно ≤120КБ —
// диагностика не должна сама плодить «объёмный профиль».
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
	OK            bool          `json:"ok"`            // true = заморозка НЕ обнаружена (все tunnel-ступени прошли)
	DirectControl bool          `json:"directControl"` // direct действительно независим (false в TUN)
	Steps         []diagOutcome `json:"steps"`
	Summary       string        `json:"summary"` // однострочный факт для журнала
}

// runFreezeDiagnostic — прогон ступеней через туннель и, если это возможно,
// напрямую. В TUN auto_route «direct» HTTP-клиент тоже попадает в туннель;
// второй запрос там не является контрольной группой и намеренно не выполняется
// (V2-071, аудит D4a). DirectControl=false — честное ограничение измерения.
// socksAddr — mixed-инбаунд ядра; diagURL — controlled-volume эндпоинт
// канала (дескриптор diagnostic_url). budget ограничивает общее время.
func runFreezeDiagnostic(ctx context.Context, socksAddr, diagURL string, budget time.Duration, tunMode bool) diagReport {
	rep := diagReport{OK: true, DirectControl: !tunMode}
	if diagURL == "" {
		rep.OK = false
		rep.Summary = "диагностика недоступна: у канала нет diagnostic_url (дескриптор)"
		return rep
	}
	tunnel := &http.Client{
		Timeout: diagReadTimeout,
		Transport: &http.Transport{
			Proxy:       socksProxyURL(socksAddr),
			DialContext: dialerFor(diagReadTimeout).DialContext,
		},
	}
	direct := &http.Client{Timeout: diagReadTimeout, Transport: &http.Transport{}}
	deadline := time.Now().Add(budget)
	for _, want := range diagSteps {
		if time.Now().After(deadline) {
			rep.OK = false
			rep.Summary += " — бюджет исчерпан, ступени выше не проверены"
			break
		}
		// Сначала туннель (это и есть защищённый путь); direct — контроль.
		oT := fetchRange(ctx, tunnel, diagURL, want, "туннель")
		rep.Steps = append(rep.Steps, oT)
		if tunMode {
			rep.Steps = append(rep.Steps, diagOutcome{
				Name:   fmt.Sprintf("direct %d КБ", want>>10),
				OK:     true,
				Status: "not_independent",
				Detail: "в TUN-режиме direct-путь автоматически попадает в тот же туннель — независимый контроль не выполнялся",
				Want:   want,
			})
			if !oT.OK {
				rep.OK = false
			}
			continue
		}
		oD := fetchRange(ctx, direct, diagURL, want, "direct")
		rep.Steps = append(rep.Steps, oD)
		if !oT.OK {
			rep.OK = false
		}
	}
	rep.Summary = summarizeDiag(rep)
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
		out.Detail = "запрос не собран: " + err.Error()
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

// freezeForm — ФОРМА заморозки по ошибке чтения и длительности (V2-079).
// Чистая функция: таймаут = тишина (клиент выждал до своего лимита);
// EOF/RST до границы таймаута = резкий обрыв; обрыв практически на границе
// таймаута тоже считается тишиной (поток молчал, закрытие пришло в конце).
func freezeForm(err error, elapsed, timeout time.Duration) string {
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
		if timeout > 0 && elapsed >= timeout*8/10 {
			return "silence"
		}
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
		return "timeout", "таймаут без данных — путь не отвечает (не заморозка: данных не было)"
	case strings.Contains(m, "connection reset"):
		return "reset", "соединение сброшено (RST)"
	default:
		return "error", msg
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
	if rep.OK {
		if !rep.DirectControl {
			return "заморозка не обнаружена на tunnel-ступенях; direct-контроль недоступен в TUN-режиме"
		}
		for _, step := range rep.Steps {
			if strings.HasPrefix(step.Name, "direct ") && !step.OK {
				return "заморозка не обнаружена на tunnel-ступенях; direct-контроль не прошёл: " + step.Name + ": " + step.Status
			}
		}
		return "заморозка не обнаружена: все ступени прошли и через туннель, и напрямую"
	}
	var worst string
	for _, s := range rep.Steps {
		if !s.OK && (worst == "" || s.Name != "") {
			worst = s.Name + ": " + s.Status
			break
		}
	}
	return "обнаружен отказ объёма (" + worst + ") — см. ступени"
}

// ---------- интеграция со страницей «Проверки» (runTest) ----------

// testFreezeCheck — карточка «freeze-check»: требует живого VPN (SOCKS 1080)
// и diagnostic_url активного канала. Не проходит = честный fail с формой.
func (a *App) testFreezeCheck() (bool, string, []ProbeStepView) {
	runtime_.mu.Lock()
	cc := runtime_.cc
	running := runtime_.manager != nil
	runtime_.mu.Unlock()
	if !running || cc == nil {
		return false, "VPN не запущен — диагностика заморозки требует живой защищённый путь", nil
	}
	if cc.DiagnosticURL == "" {
		return false, "у активного канала нет diagnostic_url (дескриптор) — диагностика недоступна, это не отказ", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rep := runFreezeDiagnostic(ctx, "127.0.0.1:1080", cc.DiagnosticURL, 80*time.Second, tunRequested())
	steps := make([]ProbeStepView, 0, len(rep.Steps))
	for _, s := range rep.Steps {
		status := "fail"
		if s.OK {
			status = "pass"
		}
		steps = append(steps, ProbeStepView{Name: s.Name, Status: status, Detail: s.Detail})
	}
	// direct-ступени не валят общий вердикт: контрольная группа — информация.
	allTunnelOK := true
	for _, s := range rep.Steps {
		if strings.HasPrefix(s.Name, "туннель") && !s.OK {
			allTunnelOK = false
		}
	}
	return allTunnelOK, rep.Summary, steps
}

// socksProxyURL / dialerFor — локальные хелперы (избегаем зависимости от
// внутренностей probe): URL прокси и диалер с таймаутом.
func socksProxyURL(addr string) func(*http.Request) (*url.URL, error) {
	return http.ProxyURL(&url.URL{Scheme: "socks5h", Host: addr})
}

func dialerFor(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout}
}
