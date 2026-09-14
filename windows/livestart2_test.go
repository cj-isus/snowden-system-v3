//go:build live

// livestart2 — массовый live-прогон: поднять реальный туннель (хранилище
// владельца, реальный Manager.Start) и прогнать через него матрицу сайтов
// разных стран и категорий. Русские сайты обязательны: проверяем и «мировые
// через VPN» (egress = NL), и RU-сайты через туннель (RU-исключения
// маршрутизации — отдельный контракт FR-001; в SOCKS-режиме весь трафик
// теста идёт через туннель, RU-сайты должны быть достижимы И ЧЕРЕЗ NL —
// они публичные; это проверка «не сломали ли RU для путешественника»).
//
// Запуск: go test -tags "live,with_utls,with_gvisor,with_quic" -run TestLiveMultiSite -v
// Требования: секреты в DPAPI (AppData), сеть с рабочим return-путём.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/secretvault"
)

// site — одна цель матрицы.
type site struct {
	name     string // человекочитаемое имя
	url      string // что запрашивать
	country  string // страна хостинга/принадлежности (для отчёта)
	category string // поисковик / соцсеть / мессенджер / видео / инфо / RU
	// expectCode — ожидаемый HTTP-код (или префикс «2xx/3xx»); 0 = любой <400
	expectCode int
	// minBytes — минимальный размер ответа (отсекаем пустые заглушки)
	minBytes int
}

// siteMatrix — разноцветная выборка: RU обязательно, дальше — US/EU/Asia,
// разные CDN и категории (поиск, соцсети, мессенджеры, видео, хостинги,
// энциклопедии, стриминг, гос-сайты других стран). Большие таймауты —
// мобильный/домашний интернет.
func siteMatrix() []site {
	return []site{
		// --- RU-блок (обязательный) ---
		{"Яндекс", "https://ya.ru/", "RU", "поисковик", 200, 1000},
		{"Яндекс поиск", "https://yandex.ru/", "RU", "поисковик", 200, 1000},
		{"Mail.ru", "https://mail.ru/", "RU", "портал", 200, 1000},
		{"ВКонтакте", "https://vk.com/", "RU", "соцсеть", 200, 1000},
		{"Одноклассники", "https://ok.ru/", "RU", "соцсеть", 200, 500},
		{"Авито", "https://www.avito.ru/", "RU", "маркетплейс", 200, 1000},
		{"Госуслуги", "https://www.gosuslugi.ru/", "RU", "гос", 200, 500},
		{"Сбербанк", "https://www.sberbank.ru/", "RU", "банк", 200, 500},
		{"Лента.ру", "https://lenta.ru/", "RU", "новости", 200, 1000},
		{"Wiki RU", "https://ru.wikipedia.org/", "RU/US", "энциклопедия", 200, 5000},

		// --- Search / knowledge (US/EU) ---
		{"Google", "https://www.google.com/generate_204", "US", "поисковик", 204, 0},
		{"Bing", "https://www.bing.com/", "US", "поисковик", 200, 1000},
		{"DuckDuckGo", "https://duckduckgo.com/", "US", "поисковик", 200, 500},
		{"Wikipedia EN", "https://en.wikipedia.org/wiki/Main_Page", "US", "энциклопедия", 200, 5000},
		{"GitHub", "https://github.com/", "US", "разработка", 200, 1000},
		{"StackOverflow", "https://stackoverflow.com/", "US", "разработка", 200, 1000},
		{"Habr", "https://habr.com/ru/", "RU/EU", "разработка", 200, 1000},

		// --- Social / messaging (US/EU) ---
		{"YouTube", "https://www.youtube.com/", "US", "видео", 200, 1000},
		{"X/Twitter", "https://x.com/", "US", "соцсеть", 200, 100},
		{"Instagram", "https://www.instagram.com/", "US", "соцсеть", 200, 100},
		{"Facebook", "https://www.facebook.com/", "US", "соцсеть", 200, 100},
		{"Reddit", "https://www.reddit.com/", "US", "соцсеть", 200, 1000},
		{"Telegram web", "https://web.telegram.org/", "UK/NL", "мессенджер", 200, 100},
		{"Discord", "https://discord.com/", "US", "мессенджер", 200, 500},
		{"WhatsApp web", "https://web.whatsapp.com/", "US", "мессенджер", 200, 100},

		// --- AI / big API (US) ---
		{"ChatGPT", "https://chatgpt.com/", "US", "AI", 200, 100},
		{"OpenAI API", "https://api.openai.com/v1/models", "US", "AI", 401, 0}, // 401 = живой, без ключа
		{"Claude", "https://claude.ai/", "US", "AI", 200, 100},
		{"Gemini", "https://gemini.google.com/", "US", "AI", 200, 100},

		// --- Streaming / media (US/EU/Asia) ---
		{"Netflix", "https://www.netflix.com/", "US", "стриминг", 200, 1000},
		{"Spotify", "https://open.spotify.com/", "SE/US", "музыка", 200, 500},
		{"Twitch", "https://www.twitch.tv/", "US", "стриминг", 200, 500},
		{"Deezer", "https://www.deezer.com/", "FR", "музыка", 200, 500},

		// --- Infra / CDN / clouds (разные страны) ---
		{"Cloudflare", "https://www.cloudflare.com/", "US", "CDN", 200, 1000},
		{"Fastly", "https://www.fastly.com/", "US", "CDN", 200, 500},
		{"Hetzner", "https://www.hetzner.com/", "DE", "хостинг", 200, 500},
		{"OVH", "https://www.ovhcloud.com/", "FR", "хостинг", 200, 500},
		{"Selectel", "https://selectel.ru/", "RU", "хостинг", 200, 500},
		{"Alibaba", "https://www.alibaba.com/", "CN", "маркетплейс", 200, 500},
		{"Baidu", "https://www.baidu.com/", "CN", "поисковик", 200, 500},
		{"Naver", "https://www.naver.com/", "KR", "поисковик", 200, 500},
		{"Rakuten", "https://www.rakuten.com/", "JP", "маркетплейс", 200, 100},
		{"Amazon", "https://www.amazon.com/", "US", "маркетплейс", 200, 500},
		{"Ebay", "https://www.ebay.com/", "US/DE", "маркетплейс", 200, 500},
		{"Booking", "https://www.booking.com/", "NL", "сервисы", 200, 500},
		{"Steam", "https://store.steampowered.com/", "US", "игры", 200, 500},
		{"EpicGames", "https://store.epicgames.com/", "US", "игры", 200, 100},
		{"ProtonMail", "https://proton.me/", "CH", "почта", 200, 500},
		{"Tutanota", "https://tuta.com/", "DE", "почта", 200, 100},
		{"Mullvad", "https://mullvad.net/", "SE", "приватность", 200, 500},
		{"OONI Explorer", "https://explorer.ooni.org/", "GLOBAL", "censorship-metrics", 200, 100},
		{"Net4people", "https://ntc.party/", "GLOBAL", "censorship-forum", 200, 500},
	}
}

// viaTunnelHTTPClient — клиент строго через локальный SOCKS VPN (失败 = fail).
// HTTP/2 по умолчанию; fallback на 1.1 — в тесте каждого сайта.
func viaTunnelHTTPClient() *http.Client {
	return &http.Client{
		Transport: socksTransport("127.0.0.1:1080"),
		Timeout:   25 * time.Second,
	}
}

// checkSite — один запрос через клиент с ретраями (CGNAT мерцание) и
// HTTP/1.1-fallback. Возвращает (ok, detail).
func checkSite(client *http.Client, client11 *http.Client, s site) (bool, string) {
	lastErr := ""
	for attempt := 0; attempt < 2; attempt++ {
		cl := client
		if attempt == 1 {
			cl = client11 // HTTP/2 может не поддерживаться некоторыми хостами
		}
		code, size, err := fetchOnce(cl, s.url)
		if err == nil {
			if s.expectCode != 0 {
				if code == s.expectCode {
					return true, fmt.Sprintf("code=%d bytes=%d (expected %d)", code, size, s.expectCode)
				}
				lastErr = fmt.Sprintf("code=%d, expected %d (bytes=%d)", code, size, s.expectCode)
				continue
			}
			if code >= 200 && code < 400 {
				if s.minBytes > 0 && size < s.minBytes {
					lastErr = fmt.Sprintf("code=%d, bytes=%d < min %d (подозрительно пустой ответ)", code, size, s.minBytes)
					continue
				}
				return true, fmt.Sprintf("code=%d bytes=%d", code, size)
			}
			lastErr = fmt.Sprintf("code=%d bytes=%d", code, size)
			continue
		}
		lastErr = err.Error()
	}
	return false, lastErr
}

func fetchOnce(client *http.Client, url string) (int, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 16*1024)
	for len(buf) < 512*1024 {
		n, rerr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if rerr != nil {
			break
		}
	}
	return resp.StatusCode, len(buf), nil
}

func TestLiveMultiSite(t *testing.T) {
	// Хранилище владельца (AppData).
	vaultPath := filepath.Join(vaultDir(), "vault.v1.json")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skipf("vault not found: %s", vaultPath)
	}
	a := &App{vault: secretvault.NewManager(vaultPath)}

	cc, err := a.renderAndPrepare("")
	if err != nil {
		t.Fatalf("renderAndPrepare: %v", err)
	}
	t.Logf("render: selector=%v default=%s expected_egress=%s", cc.SelectorTags, cc.SelectorDefault, cc.ExpectedEgress)

	// Поднять реальный туннель (системный прокси — no-op).
	manager := core.NewManager()
	manager.SetSystemProxy(noopProxy{})
	engine := core.NewEngine(cc.Raw)
	manager.SetEngine(engine)
	manager.SetProbe(newAppProbe(cc))

	startCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start: %v", err)
	}
	defer func() {
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelStop()
		_ = manager.Stop(stopCtx)
	}()

	// Убедиться, что egress = ожидаемый (иначе смысл матрицы теряется).
	time.Sleep(1 * time.Second)
	report := newAppProbe(cc).Run(context.Background())
	if !report.AllPassed {
		t.Fatalf("probe failed BEFORE matrix (сеть деградировала?): %s", report.Summary())
	}
	t.Logf("probe OK: %s", report.Summary())

	client := viaTunnelHTTPClient()
	client11 := socksHTTP11Client()

	type result struct {
		site site
		ok   bool
		det  string
		dur  time.Duration
	}
	results := make([]result, 0, len(siteMatrix()))
	var passCnt, failCnt int
	start := time.Now()
	for _, s := range siteMatrix() {
		st := time.Now()
		ok, det := checkSite(client, client11, s)
		dur := time.Since(st)
		results = append(results, result{s, ok, det, dur})
		if ok {
			passCnt++
		} else {
			failCnt++
		}
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		t.Logf("%-4s %-18s %-9s %-8s %6.2fs  %s", status, s.name, s.country, s.category, dur.Seconds(), det)
	}

	// Сводка.
	total := time.Since(start)
	t.Logf("=== ИТОГО: %d/%d прошли за %.1fs (avg %.2fs/сайт)",
		passCnt, len(results), total.Seconds(), total.Seconds()/float64(len(results)))

	// Разбивка по странам/категориям — чтобы видеть классы.
	byCountry := map[string]struct{ pass, total int }{}
	byCategory := map[string]struct{ pass, total int }{}
	for _, r := range results {
		c := byCountry[r.site.country]
		c.total++
		if r.ok {
			c.pass++
		}
		byCountry[r.site.country] = c
		k := byCategory[r.site.category]
		k.total++
		if r.ok {
			k.pass++
		}
		byCategory[r.site.category] = k
	}
	for c, v := range byCountry {
		t.Logf("по стране %-8s: %d/%d", c, v.pass, v.total)
	}
	for c, v := range byCategory {
		t.Logf("по категории %-20s: %d/%d", c, v.pass, v.total)
	}

	// Русские сайты обязательны — явный гейт.
	var ruFailed []string
	for _, r := range results {
		if strings.Contains(r.site.country, "RU") && !r.ok {
			ruFailed = append(ruFailed, r.site.name+" ("+r.det+")")
		}
	}
	if len(ruFailed) > 0 {
		t.Errorf("RU-сайты через VPN не открылись: %v", ruFailed)
	}
	if failCnt > len(results)/3 {
		t.Errorf("слишком много отказов (%d из %d) — туннель деградирует, а не выборочно сайты", failCnt, len(results))
	}
}

// socksTransport — HTTP-клиент через локальный SOCKS (golang.org/x/net уже
// indirect-зависимость sing-box; используем proxy.SOCKS5). TLS — дефолтный
// (верификация включена; SNP сайты с плохими сертификатами — это FAIL).
func socksTransport(socksAddr string) http.RoundTripper {
	tlsCfg := &tls.Config{}
	return &http.Transport{
		DialContext:           socksContextDialer(socksAddr),
		TLSClientConfig:       tlsCfg,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func socksHTTP11Client() *http.Client {
	// V2-035: ForceAttemptHTTP2=false НЕ отключает h2 — TLS ALPN всё равно
	// предлагает h2 (поймано live: «malformed HTTP response \x00\x00\x12\x04»
	// — это HTTP/2 SETTINGS-фрейм, который h1-транспорт не понимает).
	// Отключение ALPN h2 — только через явный NextProtos ["http/1.1"].
	tr := socksTransport("127.0.0.1:1080").(*http.Transport)
	tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
	tr.ForceAttemptHTTP2 = false
	return &http.Client{Transport: tr, Timeout: 25 * time.Second}
}
