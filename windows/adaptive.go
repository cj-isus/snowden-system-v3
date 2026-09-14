package main

// adaptive.go — адаптивный слой против IP-блоков датацентра (V2-041).
//
// Контекст: AI-сервисы (chatgpt.com, claude.ai) и подобные им отдают 403
// JS-challenge на egress-IP хостера независимо от канала — чинится только
// сменой egress. Серверная часть (V2-041): sing-box endpoint warp-ep
// (Cloudflare WARP) + route rule domain_suffix → warp-ep. Клиентская часть
// здесь: детектор «сайт отдаёт challenge/блок через туннель», классификация
// ответа, статус для UI и персистентная карта доменов (факт, не гадание).
//
// Правило фактов (AGENTS §3.6): nil/пусто = «не проверялось», никогда ноль
// вместо отсутствия измерения. Значения секретов здесь не участвуют.

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SiteStatus — класс ответа сайта через туннель (факт последней проверки).
type SiteStatus struct {
	Domain    string `json:"domain"`    // проверенный домен
	Code      int    `json:"code"`      // HTTP-код (0 = не дошло)
	OK        bool   `json:"ok"`        // 2xx/3xx = доступен
	Challenge bool   `json:"challenge"` // 403 + маркер челленджа = блок IP датацентра
	Detail    string `json:"detail"`    // человеческое пояснение (рус.)
	CheckedAt string `json:"checkedAt"` // RFC3339
}

// adaptiveState — общий статус адаптивного слоя для UI.
type AdaptiveStatus struct {
	Sites       []SiteStatus `json:"sites"`       // последние проверки
	WarpDomains []string     `json:"warpDomains"` // домены, обслуживаемые через WARP (server rule, V2-041)
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

// warpDomainsServer — зеркало server-route rule (V2-041): что реально уходит
// через WARP egress на сервере. Константа договора с /etc/sing-box/config.json;
// рассинхрон ловит live-тест chatgpt (200 = правило работает).
var warpDomainsServer = []string{
	"chatgpt.com", "openai.com", "oaistatic.com", "oaiusercontent.com",
	"claude.ai", "anthropic.com", "gemini.google.com", "x.ai", "grok.com",
	// V2-044: расширение по результатам live-матриц (сервисы, известные
	// challenge'ами на датацентровых IP).
	"perplexity.ai", "poe.com", "copilot.microsoft.com", "character.ai",
	"chat.deepseek.com",
}

var adaptiveMu sync.Mutex
var adaptiveLast []SiteStatus

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
func classifySite(ctx context.Context, socksAddr, domain string) SiteStatus {
	st := SiteStatus{Domain: domain, CheckedAt: time.Now().Format(time.RFC3339)}
	u := "https://" + domain + "/"
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{NextProtos: []string{"http/1.1"}},
		Proxy:           http.ProxyURL(&url.URL{Scheme: "socks5h", Host: socksAddr}),
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		st.Detail = "запрос не собран: " + err.Error()
		return st
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := client.Do(req)
	if err != nil {
		// Сеть/туннель не живы — это не «сайт блокирует», это «нет данных».
		st.Detail = "нет ответа (туннель/сеть): " + err.Error()
		return st
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	st.Code = resp.StatusCode
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		st.OK = true
		st.Detail = fmt.Sprintf("доступен (HTTP %d)", resp.StatusCode)
	case resp.StatusCode == 403 || resp.StatusCode == 429:
		if isChallengeBody(body) {
			st.Challenge = true
			st.Detail = "блок egress-IP (anti-bot challenge) — обслуживается через WARP-egress (V2-041)"
			return st
		}
		st.Detail = fmt.Sprintf("отказ HTTP %d (без маркера челленджа)", resp.StatusCode)
	default:
		st.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return st
}

// RunAdaptiveCheck — проверка всех целей через живой туннель. Параллельно,
// с общим бюджетом 30с; результат кешируется в памяти и на диск (AppData).
func (a *App) RunAdaptiveCheck() AdaptiveStatus {
	runtime_.mu.Lock()
	running := runtime_.manager != nil && runtime_.cc != nil
	runtime_.mu.Unlock()

	out := AdaptiveStatus{WarpDomains: append([]string(nil), warpDomainsServer...),
		Note: "AI-сервисы блокируют IP датацентра challenge-страницей. Сервер (V2-041) направляет их через Cloudflare WARP — отдельный egress с лучшей репутацией. Остальной трафик идёт напрямую с VPS."}

	if !running {
		out.Note = "VPN не запущен — проверка через туннель невозможна (включите VPN и повторите). " + out.Note
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]SiteStatus, len(adaptiveCheckTargets))
	for i, dom := range adaptiveCheckTargets {
		wg.Add(1)
		go func(slot int, domain string) {
			defer wg.Done()
			results[slot] = classifySite(ctx, "127.0.0.1:1080", domain)
		}(i, dom)
	}
	wg.Wait()

	adaptiveMu.Lock()
	adaptiveLast = results
	adaptiveMu.Unlock()
	persistAdaptive(results)

	out.Sites = results
	return out
}

// GetAdaptiveStatus — последние сохранённые результаты (без новых запросов).
func (a *App) GetAdaptiveStatus() AdaptiveStatus {
	out := AdaptiveStatus{WarpDomains: append([]string(nil), warpDomainsServer...),
		Note: "AI-сервисы блокируют IP датацентра challenge-страницей. Сервер (V2-041) направляет их через Cloudflare WARP — отдельный egress с лучшей репутацией. Остальной трафик идёт напрямую с VPS."}
	if cached := loadAdaptive(); cached != nil {
		out.Sites = cached
	}
	adaptiveMu.Lock()
	if adaptiveLast != nil {
		out.Sites = adaptiveLast
	}
	adaptiveMu.Unlock()
	return out
}

// ---------- персистентность (AppData, атомарная запись) ----------

func adaptivePath() string {
	return filepath.Join(appDataDir(), "adaptive-sites.json")
}

func persistAdaptive(sites []SiteStatus) {
	p := adaptivePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}

func loadAdaptive() []SiteStatus {
	data, err := os.ReadFile(adaptivePath())
	if err != nil {
		return nil
	}
	var sites []SiteStatus
	if json.Unmarshal(data, &sites) != nil {
		return nil
	}
	return sites
}
