package main

// Тесты A0/A2 (V2-057): сетевые эпохи, health-реестр, freeze-диагностика.
// Сеть не используется (кроме httptest-сервера локально).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// ---------- netepoch ----------

func TestNetEpochBumpSequence(t *testing.T) {
	netEpoch.resetForTest()
	t0 := time.Now()
	if n, _, _ := netEpoch.current(); n != 0 {
		t.Fatalf("начальное состояние = E0, получено E%d", n)
	}
	if n := netEpoch.bump("up", "wifi", t0); n != 1 {
		t.Fatalf("первый bump = E1, получено E%d", n)
	}
	if n := netEpoch.bump("change", "mobile", t0); n != 2 {
		t.Fatalf("второй bump = E2, получено E%d", n)
	}
	// down НЕ бампит (сети нет — не смена эпохи).
	if n := netEpoch.bump("down", "wifi", t0); n != 2 {
		t.Fatalf("down не должен бампить: E%d", n)
	}
	_, started, cls := netEpoch.current()
	if cls != "mobile" {
		t.Fatalf("класс эпохи = mobile, получено %q", cls)
	}
	if started.IsZero() {
		t.Fatal("момент начала эпохи не зафиксирован")
	}
	if l := netEpoch.label(); l != "E2" {
		t.Fatalf("label = %q, хочу E2", l)
	}
	netEpoch.resetForTest()
}

// Эпоха бампится и без живого App (fire до startup) — журнал просто молчит.
func TestNetEpochBumpWithoutApp(t *testing.T) {
	netEpoch.resetForTest()
	globalAppForEpoch = nil
	t.Cleanup(func() { netEpoch.resetForTest() })
	// Не должно паниковать.
	onNetEvent("up", "wifi", time.Now())
	if n, _, _ := netEpoch.current(); n != 1 {
		t.Fatalf("onNetEvent(up) без App должен бампить: E%d", n)
	}
}

// ---------- chanhealth ----------

func TestChannelHealthFreshEpoch(t *testing.T) {
	netEpoch.resetForTest()
	t0 := time.Now()
	now := func() time.Time { return t0 }

	// Эпоха не открыта: записи идут в реестр 0 и валидны как «эпоха 0».
	chanHealth.observeResult(0, "channel-a-vless", true, "", now())
	h := chanHealth.snapshot(now())
	if h.Epoch != 0 {
		t.Fatalf("эпоха реестра = 0, получено %d", h.Epoch)
	}
	if v := h.Chans[0].Verdict; v != "healthy" {
		t.Fatalf("свежий успех = healthy, получено %q", v)
	}

	// Новая эпоха — реестр чист (прошлые факты не переносятся).
	netEpoch.bump("change", "mobile", t0)
	chanHealth.observeResult(1, "channel-a-hy2", false, "reset_injected", now())
	h = chanHealth.snapshot(now())
	if h.Epoch != 1 {
		t.Fatalf("эпоха после bump = 1, получено %d", h.Epoch)
	}
	for _, e := range h.Chans {
		if e.ID == "channel-a-vless" {
			t.Fatal("факты прошлой эпохи перенеслись в новую — нарушение fail-closed к no_data")
		}
	}
	for _, e := range h.Chans {
		if e.ID == "channel-a-hy2" {
			if e.Verdict != "failing" || e.LastClass != "reset_injected" {
				t.Fatalf("failing + класс = %q/%q", e.Verdict, e.LastClass)
			}
		}
	}
	netEpoch.resetForTest()
	chanHealth.mu.Lock()
	chanHealth.resetLocked(0)
	chanHealth.mu.Unlock()
}

func TestChannelHealthEvidenceAge(t *testing.T) {
	netEpoch.resetForTest()
	base := time.Now().Add(-time.Hour)
	chanHealth.observeResult(0, "channel-a-vless", true, "", base)
	h := chanHealth.snapshot(time.Now())
	for _, e := range h.Chans {
		if e.ID == "channel-a-vless" && e.Verdict != "no_data" {
			t.Fatalf("протухший факт (>evidenceFreshMax) = no_data, получено %q", e.Verdict)
		}
	}
	// FAIL новее OK = failing даже при наличии успехов.
	chanHealth.observeResult(0, "channel-a-hy2", true, "", time.Now().Add(-2*time.Minute))
	chanHealth.observeResult(0, "channel-a-hy2", false, "port_blackholed", time.Now())
	h = chanHealth.snapshot(time.Now())
	for _, e := range h.Chans {
		if e.ID == "channel-a-hy2" && e.Verdict != "failing" {
			t.Fatalf("последний факт fail = failing, получено %q", e.Verdict)
		}
	}
	netEpoch.resetForTest()
	chanHealth.mu.Lock()
	chanHealth.resetLocked(0)
	chanHealth.mu.Unlock()
}

// Concurrency: observe из сторожа/старта/биндингов — без гонок на мьютексе
// (структурно; -race в среде недоступен — как в V2-025/028, честно фиксируем).
func TestChannelHealthConcurrent(t *testing.T) {
	netEpoch.resetForTest()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			chanHealth.observeResult(0, fmt.Sprintf("ch-%d", n%4), n%2 == 0, "", time.Now())
			_ = chanHealth.snapshot(time.Now())
		}(i)
	}
	wg.Wait()
	netEpoch.resetForTest()
	chanHealth.mu.Lock()
	chanHealth.resetLocked(0)
	chanHealth.mu.Unlock()
}

// ---------- freeze diagnostic (httptest) ----------

// controlledServer — контролируемый объём по Range (имитация /range на VPS):
// при freezeAt>0 обрывает поток после freezeAt байт БЕЗ ошибки (имитация
// заморозки: клиент видит EOF до запрошенного объёма).
func controlledServer(t *testing.T, total int, freezeAt int) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := total
		if rng := r.Header.Get("Range"); rng != "" {
			var lo, hi int
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &lo, &hi); err == nil {
				want = hi - lo + 1
			}
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusPartialContent)
		buf := make([]byte, 1024)
		sent := 0
		for sent < want {
			n := want - sent
			if n > len(buf) {
				n = len(buf)
			}
			for i := range buf[:n] {
				buf[i] = byte(sent + i)
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			sent += n
			if freezeAt > 0 && sent >= freezeAt {
				// Обрыв без ошибки: закрыть соединение без финальных данных.
				panic(http.ErrAbortHandler) // это и есть «тишина» середины потока
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFreezeDiagnosticAllClear(t *testing.T) {
	srv := controlledServer(t, 64<<10, 0) // весь объём отдаётся
	// Тест вызывает через прямой клиент к серверу (без SOCKS): прокси-URL
	// тут не участвует — runFreezeDiagnostic строит клиенты с прокси, поэтому
	// прогоняем fetchRange напрямую для обеих групп.
	ctx := context.Background()
	client := srv.Client()
	client.Timeout = 5 * time.Second
	for _, want := range diagSteps {
		o := fetchRange(ctx, client, srv.URL, want, "туннель")
		if !o.OK || o.Status != "complete" {
			t.Fatalf("ступень %d: %s (%s)", want, o.Status, o.Detail)
		}
		if o.Got != want {
			t.Fatalf("ступень %d: got %d", want, o.Got)
		}
	}
}

func TestFreezeDiagnosticDetectsMidStreamCut(t *testing.T) {
	// Обрыв после 9КБ: ступени 8КБ проходят, 16/32/64КБ ловят «тишину».
	srv := controlledServer(t, 64<<10, 9<<10)
	ctx := context.Background()
	client := srv.Client()
	client.Timeout = 5 * time.Second
	o8 := fetchRange(ctx, client, srv.URL, 8<<10, "туннель")
	if !o8.OK {
		t.Fatalf("8КБ должен пройти при обрыве на 9КБ: %s", o8.Detail)
	}
	o16 := fetchRange(ctx, client, srv.URL, 16<<10, "туннель")
	if o16.OK || o16.Status != "freeze" {
		t.Fatalf("16КБ при обрыве на 9КБ = freeze, получено %q: %s", o16.Status, o16.Detail)
	}
	if o16.Got == 0 || o16.Got >= 16<<10 {
		t.Fatalf("частичные данные обязаны быть: got=%d", o16.Got)
	}
	// V2-079: форма обрыва — panic(ErrAbortHandler) закрывает соединение
	// раньше таймаута, поэтому это резкий обрыв (abrupt), а не тишина.
	if o16.Form != "abrupt" {
		t.Fatalf("abort до таймаута = форма abrupt, получено %q", o16.Form)
	}
	// Граница честности: на Windows монотонные часы могут иметь крупный тик
	// (измерено: time.Since self-measure дал min=0ns), поэтому локальная
	// sub-tick ступень честно читается как 0 мкс — строгий >0 неверен.
	// Проверяем, что L0-evidence присутствует в Detail и значения неотрицательны.
	if o16.TTFBus < 0 || o16.ElapsedUs < 0 {
		t.Fatalf("L0-тайминги не могут быть отрицательными: ttfb=%d elapsed=%d", o16.TTFBus, o16.ElapsedUs)
	}
	if !strings.Contains(o16.Detail, "TTFB") {
		t.Fatalf("Detail обязан нести L0-тайминг: %q", o16.Detail)
	}
}

func TestFreezeDiagnosticClassifiesTimeout(t *testing.T) {
	// Полный таймаут БЕЗ данных — не freeze, а timeout (разные классы реакции).
	hanging := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // дольше клиентского таймаута
	}))
	t.Cleanup(hanging.Close)
	client := hanging.Client()
	client.Timeout = 200 * time.Millisecond
	out := fetchRange(context.Background(), client, hanging.URL, 8<<10, "туннель")
	if out.OK || out.Status != "timeout" {
		t.Fatalf("тишина без данных = timeout, получено %q (%s)", out.Status, out.Detail)
	}
	// Классификация честная: timeout ≠ freeze (разные реакции по THEORY).
	if out.Status == "freeze" {
		t.Fatal("тишина без данных классифицирована как freeze")
	}
	if out.Form != "no_data" {
		t.Fatalf("полный таймаут без данных = форма no_data, получено %q", out.Form)
	}
	if out.ElapsedUs <= 0 {
		t.Fatalf("L0-тайминг полного таймаута обязан быть зафиксирован: %dмкс", out.ElapsedUs)
	}
}

// TestFreezeDiagnosticClassifiesPartialReadTimeoutAsFreeze — основная форма
// TSPU-freeze: сервер успел отдать часть тела, затем перестал отвечать, и
// клиент получил read-timeout. Это НЕ полный timeout без данных.
func TestFreezeDiagnosticClassifiesPartialReadTimeoutAsFreeze(t *testing.T) {
	stall := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := 16 << 10
		if rng := r.Header.Get("Range"); rng != "" {
			var lo, hi int
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &lo, &hi); err == nil {
				want = hi - lo + 1
			}
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusPartialContent)
		partial := 9 << 10
		if partial > want {
			partial = want
		}
		buf := make([]byte, partial)
		if _, err := w.Write(buf); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Держим соединение открытым дольше client.Timeout: имитация тихого
		// drop после частичной полезной нагрузки, без FIN/RST.
		time.Sleep(2 * time.Second)
	}))
	t.Cleanup(stall.Close)
	client := stall.Client()
	client.Timeout = 150 * time.Millisecond
	out := fetchRange(context.Background(), client, stall.URL, 16<<10, "туннель")
	if out.OK || out.Status != "freeze" {
		t.Fatalf("частичные данные + read-timeout = freeze, получено %q (%s)", out.Status, out.Detail)
	}
	if out.Got == 0 || out.Got >= out.Want {
		t.Fatalf("ожидались частичные данные: got=%d want=%d", out.Got, out.Want)
	}
	if strings.Contains(strings.ToLower(out.Detail), "данных не было") {
		t.Fatalf("detail ложно утверждает отсутствие данных: %q", out.Detail)
	}
	// V2-079: форма и L0-тайминги — stall упёрся в таймаут клиента (тишина).
	if out.Form != "silence" {
		t.Fatalf("stall до таймаута = форма silence, получено %q", out.Form)
	}
	if out.ElapsedUs < 100000 || out.TTFBus <= 0 {
		t.Fatalf("L0-тайминги silence неверны: ttfb=%dмкс elapsed=%dмкс", out.TTFBus, out.ElapsedUs)
	}
}

// TestFreezeFormClassification — чистая классификация формы заморозки (V2-079):
// таймаут = тишина, EOF/RST до границы таймаута = резкий обрыв.
func TestFreezeFormClassification(t *testing.T) {
	const to = 3 * time.Second
	cases := []struct {
		name    string
		err     error
		elapsed time.Duration
		want    string
	}{
		{"client timeout", errors.New("context deadline exceeded (Client.Timeout or context cancellation while reading body)"), to, "silence"},
		{"eof до таймаута", io.ErrUnexpectedEOF, 50 * time.Millisecond, "abrupt"},
		{"eof на границе таймаута", io.ErrUnexpectedEOF, to, "silence"},
		{"reset до таймаута", errors.New("read tcp: connection reset by peer"), 20 * time.Millisecond, "abrupt"},
		{"nil", nil, to, ""},
		{"неизвестная", errors.New("weird failure"), to, "unknown"},
	}
	for _, c := range cases {
		if got := freezeForm(c.err, c.elapsed, to); got != c.want {
			t.Fatalf("%s: freezeForm = %q, want %q", c.name, got, c.want)
		}
	}
	for _, form := range []string{"silence", "abrupt", "no_data", "unknown", ""} {
		if freezeFormLabel(form) == "" {
			t.Fatalf("подпись формы %q не должна быть пустой", form)
		}
	}
}

// TestFreezeDiagnosticTUNDirectControlHonest — в TUN direct-клиент не может
// быть независимой контрольной группой: auto_route заворачивает оба запроса.
func TestFreezeDiagnosticTUNDirectControlHonest(t *testing.T) {
	rep := diagReport{
		OK:            true,
		DirectControl: false,
		Steps:         []diagOutcome{{Name: "туннель 8 КБ", OK: true, Status: "complete"}, {Name: "direct 8 КБ", OK: true, Status: "not_independent"}},
	}
	summary := summarizeDiag(rep)
	if !strings.Contains(summary, "direct-контроль недоступен") {
		t.Fatalf("TUN summary должен явно сообщать ограничение контроля: %q", summary)
	}
	if rep.Steps[1].Status != "not_independent" {
		t.Fatalf("ожидался not_independent, got %q", rep.Steps[1].Status)
	}
}

func TestFreezeDiagnosticDirectFailureIsReported(t *testing.T) {
	rep := diagReport{
		OK:            true,
		DirectControl: true,
		Steps: []diagOutcome{
			{Name: "туннель 8 КБ", OK: true, Status: "complete"},
			{Name: "direct 8 КБ", OK: false, Status: "timeout"},
		},
	}
	if got := summarizeDiag(rep); !strings.Contains(got, "direct-контроль не прошёл") {
		t.Fatalf("summary завысил direct evidence: %q", got)
	}
}

func TestChannelHealthEqualTimestampUsesEventOrder(t *testing.T) {
	now := time.Now()
	st := &healthState{OK: 1, FAIL: 1, LastOKAt: now, LastFailAt: now, LastOutcome: "fail"}
	if got := st.verdict(now); got != "failing" {
		t.Fatalf("последний fail при равных timestamps = %q, want failing", got)
	}
	st.LastOutcome = "ok"
	if got := st.verdict(now); got != "healthy" {
		t.Fatalf("последний ok при равных timestamps = %q, want healthy", got)
	}
}

// ---------- B2: биндинг независимости (V2-059) ----------

// GetChannelIndependence по встроенным дескрипторам: 4 enabled-канала = 6 пар,
// детерминированно, без обращений к сети.
func TestGetChannelIndependenceMatrix(t *testing.T) {
	a := NewApp()
	m := a.GetChannelIndependence()
	if len(m) != 6 {
		t.Fatalf("6 пар из 4 каналов, получено %d", len(m))
	}
	seen := map[string]bool{}
	for _, row := range m {
		if row.A == row.B {
			t.Fatalf("самопара %q в матрице", row.A)
		}
		key := row.A + "|" + row.B
		if seen[key] {
			t.Fatalf("дубликат пары %q", key)
		}
		seen[key] = true
		if row.Score < 0 || row.Score > 1 {
			t.Fatalf("score %q/%q вне [0,1]: %.2f", row.A, row.B, row.Score)
		}
		// Consistency: shared_fdoms пусто ⟺ полная независимость (1.0).
		if (row.SharedFDs == "") != (row.Score == 1.0) {
			t.Fatalf("рассинхрон score/shared_fdoms: %q↔%q score=%.2f shared=%q", row.A, row.B, row.Score, row.SharedFDs)
		}
	}
	// Реальные факты из дескрипторов: ws-канал за CDN и reality+shadowtls на одном
	// origin не могут быть полностью независимы (plan_gpt §6).
	for _, row := range m {
		cdnPair := (row.A == "a-vless" && row.B == "b-hy2") || (row.A == "a-vless" && row.B == "c-reality") || (row.A == "a-vless" && row.B == "d-shadowtls")
		realityShadow := (row.A == "c-reality" && row.B == "d-shadowtls")
		if cdnPair && row.Score == 1.0 {
			t.Fatalf("%q↔%q за общим CDN — score обязан быть < 1", row.A, row.B)
		}
		if realityShadow && !row.BelowLimit {
			t.Fatalf("reality↔shadowtls (общий origin+донор) обязан быть ниже порога, score=%.2f", row.Score)
		}
	}
}

// Disable not-found: биндинг не выдумывает строки для отсутствующих каналов.
func TestGetChannelIndependenceUnknownChannel(t *testing.T) {
	a := NewApp()
	m := a.GetChannelIndependence()
	for _, row := range m {
		if row.A == "no-such-channel" || row.B == "no-such-channel" {
			t.Fatalf("выдуманная строка для несуществующего канала: %+v", row)
		}
	}
}

// Детерминированность: два вызова подряд дают идентичную матрицу (policy,
// не физика; plan_gpt §6 «deterministic»).
func TestGetChannelIndependenceDeterministic(t *testing.T) {
	a := NewApp()
	m1 := a.GetChannelIndependence()
	m2 := a.GetChannelIndependence()
	if len(m1) == 0 {
		t.Fatal("матрица пуста — дескрипторы нечитаемы в тестовой среде")
	}
	if len(m1) != len(m2) {
		t.Fatalf("число пар изменилось между вызовами: %d → %d", len(m1), len(m2))
	}
	for i := range m1 {
		if m1[i] != m2[i] {
			t.Fatalf("матрица недетерминирована: %+v vs %+v", m1[i], m2[i])
		}
	}
}

// runTest «freeze-check» — честные отказы без VPN и без diagnostic_url.
func TestFreezeCheckHonestRefusals(t *testing.T) {
	a := NewApp()

	// (1) VPN не запущен: честный fail, не «блок» сети.
	runtime_.mu.Lock()
	runtime_.manager = nil
	runtime_.cc = nil
	runtime_.mu.Unlock()
	ok, detail, steps := a.testFreezeCheck()
	if ok || steps != nil || !strings.Contains(detail, "не запущен") {
		t.Fatalf("без VPN: ok=%v detail=%q", ok, detail)
	}

	// (2) VPN «запущен», но у канала нет diagnostic_url — честный skip.
	// manager подменяем заглушкой через живой тип: нужен *core.Manager.
	runtime_.mu.Lock()
	runtime_.manager = core.NewManager() // не nil, но не запущен — этого достаточно для гейта
	runtime_.cc = &render.ClientConfig{} // DiagnosticURL пуст
	runtime_.mu.Unlock()
	defer func() {
		runtime_.mu.Lock()
		runtime_.manager = nil
		runtime_.cc = nil
		runtime_.mu.Unlock()
	}()
	ok2, detail2, _ := a.testFreezeCheck()
	if ok2 || !strings.Contains(detail2, "diagnostic_url") {
		t.Fatalf("без diagnostic_url: ok=%v detail=%q", ok2, detail2)
	}
}
