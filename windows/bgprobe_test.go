package main

// Тесты D1 (V2-063): фоновые дешёвые probe неактивных каналов. Сеть — только
// локальный listener; внешних обращений нет.

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// osTimeoutError — ошибка с настоящим net.Error.Timeout()=true (dial ОС
// возвращает именно такие; строковая форма покрыта отдельным случаем).
type osTimeoutError struct{}

func (osTimeoutError) Error() string   { return "dial: i/o timeout" }
func (osTimeoutError) Timeout() bool   { return true }
func (osTimeoutError) Temporary() bool { return false }

// bgTestDescriptor — дескриптор для таблиц bgTargets (минимальный набор полей).
func bgTestDescriptor(id, protocol, transport, origin string, port uint16, status string) render.ChannelDescriptor {
	return render.ChannelDescriptor{
		ID:               id,
		Protocol:         protocol,
		Transport:        transport,
		OriginServer:     origin,
		Port:             port,
		Enabled:          true,
		ValidationStatus: status,
	}
}

// Таргеты: только enabled live-verified НЕактивные; активный и configured
// исключаются; QUIC помечен как неподдерживаемый для фоновой пробы.
func TestBgTargetsSelection(t *testing.T) {
	descs := []render.ChannelDescriptor{
		bgTestDescriptor("a-vless", "vless", "ws", "192.0.2.10", 443, "live-verified"),
		bgTestDescriptor("b-hy2", "hysteria2", "quic", "192.0.2.10", 8444, "live-verified"),
		bgTestDescriptor("c-reality", "vless", "tcp-reality", "192.0.2.20", 8443, "live-verified"),
		bgTestDescriptor("d-configured", "vless", "shadowtls", "192.0.2.20", 8445, "configured"),
		bgTestDescriptor("e-disabled", "vless", "ws", "192.0.2.30", 443, "live-verified"),
	}
	descs[4].Enabled = false

	got := bgTargets(descs, "a-vless")
	if len(got) != 2 {
		t.Fatalf("таргетов 2 (b-hy2, c-reality), получено %d: %+v", len(got), got)
	}
	byID := map[string]bgTarget{}
	for _, tg := range got {
		byID[tg.ChannelID] = tg
	}
	hy2, ok := byID["b-hy2"]
	if !ok || !hy2.QUIC || hy2.Port != "8444" {
		t.Fatalf("b-hy2: QUIC-таргет с портом 8444, получено %+v", hy2)
	}
	rel, ok := byID["c-reality"]
	if !ok || rel.QUIC || rel.Host != "192.0.2.20" {
		t.Fatalf("c-reality: TCP-таргет, получено %+v", rel)
	}
	for _, id := range []string{"a-vless", "d-configured", "e-disabled"} {
		if _, bad := byID[id]; bad {
			t.Fatalf("%q не должен попадать в фоновые таргеты", id)
		}
	}
}

// Классификация ошибок dial — формы словаря failclass (refused/timeout/прочее).
func TestClassifyBgError(t *testing.T) {
	if got := classifyBgError(nil); got != "" {
		t.Fatalf("nil = \"\", получено %q", got)
	}
	refused := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	if got := classifyBgError(refused); got != fcServerUnreach {
		t.Fatalf("refused → %s, получено %s", fcServerUnreach, got)
	}
	// Настоящий таймаут ОС: тип-тест ошибки, реализующей net.Error с Timeout=true.
	if got := classifyBgError(errors.New("dial tcp: i/o timeout")); got != fcPortBlackholed {
		t.Fatalf("строчный timeout → %s, получено %s", fcPortBlackholed, got)
	}
	if got := classifyBgError(osTimeoutError{}); got != fcPortBlackholed {
		t.Fatalf("net.Error.Timeout → %s, получено %s", fcPortBlackholed, got)
	}
	if got := classifyBgError(errors.New("dial tcp: network is unreachable")); got != fcUnknown {
		t.Fatalf("прочее → unknown, получено %s", got)
	}
}

// Один дешёвый probe: живой listener — успех без фактов; мёртвый порт —
// refusal-формы; consec-подтверждение пишет FAIL только со второго раза.
func TestBgProbeChannelLifecycle(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("локальный listener: %v", err)
	}
	t.Cleanup(func() { lst.Close() })
	host, portStr, _ := net.SplitHostPort(lst.Addr().String())

	alive := &bgTarget{ChannelID: "bg-alive", Host: host, Port: portStr}
	a := NewApp()
	a.bgProbeChannel(alive)
	if st, has := chanHealth.stateOf(0, "bg-alive"); has {
		t.Fatalf("успех фона не должен писать факты, запись есть: %+v", st)
	}
	if bgLastFor(0, "bg-alive").IsZero() {
		t.Fatal("бюджет-отметка проверки не поставлена")
	}

	dead := &bgTarget{ChannelID: "bg-dead", Host: "127.0.0.1", Port: "1"}
	a.bgProbeChannel(dead)
	if _, has := chanHealth.stateOf(0, "bg-dead"); has {
		t.Fatal("первый отказ не должен писать факт (анти-мерцание bgConsecFails)")
	}
	a.bgProbeChannel(dead)
	st, has := chanHealth.stateOf(0, "bg-dead")
	if !has {
		t.Fatal("второй подряд отказ обязан дать FAIL-факт в реестре")
	}
	if st.FAIL != 1 || st.OK != 0 || st.LastSrc != srcBackground {
		t.Fatalf("FAIL=1/src=background, получено %+v", st)
	}
	if st.LastClass != fcServerUnreach && st.LastClass != fcUnknown {
		t.Fatalf("класс отказа из словаря, получено %q", st.LastClass)
	}
	if v := st.verdict(time.Now()); v != "failing" {
		t.Fatalf("вердикт после фонового отказа = failing, получено %q", v)
	}

	// Успех гасит счётчик: следующий одиночный отказ снова не подтверждён
	// (запись от ПРЕЖНЕГО подтверждённого факта в реестре остаётся — FAIL не растёт).
	a.bgProbeChannel(alive)
	a.bgProbeChannel(dead)
	if st, has := chanHealth.stateOf(0, "bg-dead"); !has || st.FAIL != 1 {
		t.Fatalf("после успеха одиночный отказ не подтверждается (FAIL=1), получено %+v has=%v", st, has)
	}
}

// QUIC-канал: probe невозможен (анти-probe HY2) — ни фактов, ни исключения
// из бюджета (отметка проверки ставится).
func TestBgProbeChannelQuicSkip(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	a := NewApp()
	q := &bgTarget{ChannelID: "bg-quic", Host: "127.0.0.1", Port: "1", QUIC: true}
	a.bgProbeChannel(q)
	if _, has := chanHealth.stateOf(0, "bg-quic"); has {
		t.Fatal("QUIC-skip не должен писать факты")
	}
	if bgLastFor(0, "bg-quic").IsZero() {
		t.Fatal("QUIC-skip должен отмечаться в бюджете (не крутить попытки чаще допустимого)")
	}
}

// Планировщик бюджета: непроверенный канал созревает сразу, проверенный —
// через bgChanMinInterval.
func TestNextBgAt(t *testing.T) {
	if !nextBgAt(time.Time{}).IsZero() {
		t.Fatal("непроверенный канал = можно сейчас")
	}
	last := time.Now()
	due := nextBgAt(last)
	if due.Sub(last) != bgChanMinInterval {
		t.Fatalf("созревание = last+interval, получено %v", due.Sub(last))
	}
}

// Эпоха-гонка: отметка и факт, сделанные в старую эпоху, не попадают в новую.
func TestBgEpochRace(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	bgMarkDone(0, "bg-race", time.Now())
	if bgLastFor(0, "bg-race").IsZero() {
		t.Fatal("отметка в свою эпоху обязана сохраниться")
	}
	// Сеть сменилась, пока шёл dial: факт в НОВУЮ эпоху не попадает.
	netEpoch.bump("up", "wifi", time.Now())
	bgMarkDone(0, "bg-race", time.Now())
	if !bgLastFor(1, "bg-race").IsZero() {
		t.Fatal("факт старой эпохи попал в новую (нарушение fail-closed)")
	}
	if bgLastFor(0, "bg-race").IsZero() {
		t.Fatal("отметка своей эпохи пропала")
	}
}

// observeBg: только отказы, источник background, общий вердикт с UI.
func TestObserveBgVerdict(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	chanHealth.observeBg(0, "bg-obs", fcPortBlackholed, time.Now())
	st, has := chanHealth.stateOf(0, "bg-obs")
	if !has || st.LastSrc != srcBackground || st.LastClass != fcPortBlackholed {
		t.Fatalf("FAIL-факт фона: %+v has=%v", st, has)
	}
	snap := chanHealth.snapshot(time.Now())
	for _, e := range snap.Chans {
		if e.ID == "bg-obs" && (e.Verdict != "failing" || e.LastSrc != "background") {
			t.Fatalf("снимок: verdict=%q src=%q", e.Verdict, e.LastSrc)
		}
	}
}

// Биндинг GetChannelHealth показывает строку для неактивного канала с
// фоновым фактом (UI-семантика не менялась: вердикт общий).
func TestGetChannelHealthShowsBgFact(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	chanHealth.observeBg(0, "channel-a-reality", fcPortBlackholed, time.Now())
	a := NewApp()
	h := a.GetChannelHealth()
	found := false
	for _, e := range h.Chans {
		if e.ID == "channel-a-reality" {
			found = true
			if e.Verdict != "failing" || e.LastSrc != "background" {
				t.Fatalf("биндинг: verdict=%q src=%q", e.Verdict, e.LastSrc)
			}
		}
	}
	if !found {
		t.Fatal("строка неактивного канала с фоновым фактом отсутствует в биндинге")
	}
}

// Цикл: стоп-канал закрывается мгновенно — горутина не переживает стоп
// даже до истечения bgStartDelay.
func TestBgLoopStopsEarly(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	a := NewApp()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		a.bgProbeLoop(stop)
		close(done)
	}()
	close(stop)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bgProbeLoop не остановился по стоп-каналу")
	}
}
