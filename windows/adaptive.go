package main

// Explicit optional HTTP observations, not an adaptive policy or egress proof.
// Status/challenge markers do not establish DPI, a datacenter-IP ban, server
// routing or the remedy. Task retirement and historical storage are separate
// components; neither can change mandatory health or protection intent.

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SiteStatus — класс ответа сайта через туннель (факт последней проверки).
type SiteStatus struct {
	Domain    string `json:"domain"`    // проверенный домен
	Code      int    `json:"code"`      // HTTP-код; 0 = ответ HTTP не получен
	OK        bool   `json:"ok"`        // 2xx/3xx и прочитанный prefix; не полная страница
	Challenge bool   `json:"challenge"` // 403/429 + маркер; причина не установлена
	Detail    string `json:"detail"`    // человеческое пояснение (рус.)
	CheckedAt string `json:"checkedAt"` // RFC3339
}

// adaptiveState — общий статус адаптивного слоя для UI.
type AdaptiveStatus struct {
	Sites       []SiteStatus `json:"sites"`       // последние проверки
	WarpDomains []string     `json:"warpDomains"` // пусто без подтверждённого источника маршрутизации
	Note        string       `json:"note"`        // пояснение границы (рус.)
}

// challengeMarkers — признаки anti-bot челленджа в теле ответа.
var challengeMarkers = []string{
	"enable javascript and cookies",
	"just a moment",
	"attention required",
	"cf-challenge",
	"cf_chl_opt",
	"unusual activity",
}

// adaptiveCheckTargets — домены, которые владельцу важно видеть в UI.
// Расширяемо; проверка идёт через живой SOCKS туннель.
var adaptiveCheckTargets = []string{
	"chatgpt.com",
	"claude.ai",
	"gemini.google.com",
	"github.com",
	"youtube.com",
	"reddit.com",
	"x.com",
	"vk.com",
}

// isChallengeBody — в теле ответа есть признак anti-bot челленджа.
func isChallengeBody(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, m := range challengeMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// classifySite — один запрос через туннель → классификация.
// h1-only: детектору не нужен h2 (меньше поверхности, тот же факт).
func classifySite(task *manualDiagnosticTask, socksAddr, domain string) SiteStatus {
	st := SiteStatus{Domain: domain, CheckedAt: time.Now().Format(time.RFC3339)}
	u := "https://" + domain + "/"
	var dialer net.Dialer // preserve Transport's previous zero-dialer behavior
	transport := &http.Transport{
		DisableCompression:     true,
		MaxResponseHeaderBytes: 16 << 10,
		MaxConnsPerHost:        1,
		TLSClientConfig:        &tls.Config{NextProtos: []string{"http/1.1"}},
		Proxy:                  http.ProxyURL(&url.URL{Scheme: "socks5h", Host: socksAddr}),
		DialContext:            task.ownedDialer(dialer.DialContext),
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(task.ctx, http.MethodGet, u, nil)
	if err != nil {
		st.Detail = "запрос не создан; HTTP-наблюдение недоступно"
		return st
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := client.Do(req)
	if err != nil {
		st.Detail = "HTTP-ответ не получен; причина по этому наблюдению не установлена"
		return st
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	st.Code = resp.StatusCode
	if readErr != nil {
		st.Detail = fmt.Sprintf("получен HTTP %d, чтение body prefix прервано; полная загрузка не подтверждена", resp.StatusCode)
		return st
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		st.OK = true
		st.Detail = fmt.Sprintf("получен HTTP %d и prefix ответа ≤16 KiB; полная загрузка страницы не проверялась", resp.StatusCode)
	case resp.StatusCode == 403 || resp.StatusCode == 429:
		if isChallengeBody(body) {
			st.Challenge = true
			st.Detail = "получена challenge-страница; причина и серверный egress-маршрут не подтверждены"
			return st
		}
		st.Detail = fmt.Sprintf("отказ HTTP %d (без маркера челленджа)", resp.StatusCode)
	default:
		st.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return st
}

// RunAdaptiveCheck preserves the Wails result shape; all optional work belongs
// to one original manual task. No Background context or independent retry.
func (a *App) RunAdaptiveCheck() (out AdaptiveStatus) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			out = adaptiveCheckUnavailable()
		}
	}()
	out = a.runAdaptiveCheck()
	returned = true
	return
}

// GetAdaptiveStatus — последние сохранённые результаты (без новых запросов).
func (a *App) GetAdaptiveStatus() (out AdaptiveStatus) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			out = adaptiveStatusBase()
			out.Note = "История ответов недоступна; новый запрос к сайтам не выполнялся. " + out.Note
		}
	}()
	out = readAdaptiveHistory()
	returned = true
	return
}

func adaptiveStatusBase() AdaptiveStatus {
	return AdaptiveStatus{Sites: []SiteStatus{}, WarpDomains: []string{}, Note: "HTTP-наблюдения через туннель относятся к отдельным целям. Challenge не определяет причину отказа; текущая серверная маршрутизация и использование WARP не подтверждены."}
}
