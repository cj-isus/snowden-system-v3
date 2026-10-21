package main

// Тесты failover-политики A4/V2-037 (failover.go) — чистые, без сети и
// движка: проверяется логика circuit breaker (AGENTS §3.3) в новой семантике
// «подтверждённая серия → попытка» (мерцания отфильтровывает fastReprobe в
// стороже, не policy), порядок кандидатов (FR-002/FR-007) и fail-closed
// исход «кандидатов нет» (FR-001). Живой e2e — отдельная live-проверка.

import (
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// TestFailoverConfirmedSeriesOpensAttempt — подтверждённая серия (решение
// принимает watchCore после fast-репроб) сразу открывает попытку: раньше
// требовались два 30с-тика, среднее время до переключения ~60-90с.
func TestFailoverConfirmedSeriesOpensAttempt(t *testing.T) {
	p := &failoverPolicy{}
	now := time.Now()
	if !p.beginAttemptIfDue(now) {
		t.Fatal("первая подтверждённая серия должна открывать попытку")
	}
	if p.beginAttemptIfDue(now.Add(time.Second)) {
		t.Fatal("параллельная вторая попытка должна быть отклонена")
	}
	state, _, _ := p.snapshot()
	if state != "half-open" {
		t.Fatalf("state = %q, want half-open", state)
	}
}

// TestFailoverCooldownGatesAttempts — после неудачной попытки cooldown
// обратный экспоненциальный (30с·2^switches: первая неудача → 60с).
func TestFailoverCooldownGatesAttempts(t *testing.T) {
	p := &failoverPolicy{}
	now := time.Now()
	if !p.beginAttemptIfDue(now) {
		t.Fatal("первая попытка должна пройти")
	}
	if p.endAttempt(false, "channel-b") {
		t.Fatal("первая неудача не исчерпывает лимит")
	}
	if p.beginAttemptIfDue(now.Add(59 * time.Second)) {
		t.Fatal("попытка раньше 60с cooldown не допускается")
	}
	if !p.beginAttemptIfDue(now.Add(61 * time.Second)) {
		t.Fatal("после 60с cooldown попытка должна быть разрешена")
	}
}

// TestFailoverSuccessResetsIncident — успешная попытка закрывает инцидент.
func TestFailoverSuccessResetsIncident(t *testing.T) {
	p := &failoverPolicy{}
	now := time.Now()
	p.beginAttemptIfDue(now)
	p.endAttempt(false, "channel-b")
	p.beginAttemptIfDue(now.Add(61 * time.Second))
	if p.endAttempt(true, "channel-b") {
		t.Fatal("успех не может означать BLOCKED")
	}
	state, switches, lastErr := p.snapshot()
	if state != "closed" || switches != 0 || lastErr != "" {
		t.Fatalf("после успеха state=%q switches=%d err=%q, want closed/0/empty", state, switches, lastErr)
	}
	if !p.beginAttemptIfDue(now.Add(time.Minute)) {
		t.Fatal("после успеха новая серия должна сразу открывать попытку")
	}
}

// TestFailoverMaxSwitchesBlocked — исчерпание лимита = BLOCKED, а не
// бесконечные попытки.
func TestFailoverMaxSwitchesBlocked(t *testing.T) {
	p := &failoverPolicy{}
	now := time.Now()
	p.beginAttemptIfDue(now)
	if p.endAttempt(false, "channel-a-hy2") {
		t.Fatal("после первой неудачной попытки BLOCKED рано: осталась вторая")
	}
	if !p.beginAttemptIfDue(now.Add(61 * time.Second)) {
		t.Fatal("beginAttemptIfDue после cooldown должен пройти")
	}
	if !p.endAttempt(false, "channel-a-hy2") {
		t.Fatal("лимит переключений исчерпан — должен быть BLOCKED")
	}
}

// TestFailoverHardReset — успешный тик сторожа / явный Start сбрасывают
// открытый инцидент.
func TestFailoverHardReset(t *testing.T) {
	p := &failoverPolicy{}
	now := time.Now()
	p.beginAttemptIfDue(now)
	p.endAttempt(false, "channel-b")
	p.hardReset()
	state, switches, _ := p.snapshot()
	if state != "closed" || switches != 0 {
		t.Fatalf("после hardReset state=%q switches=%d, want closed/0", state, switches)
	}
	if !p.beginAttemptIfDue(now.Add(time.Hour)) {
		t.Fatal("после hardReset новая попытка должна быть разрешена сразу")
	}
}

// TestFailoverCandidatesOrder — порядок попыток: сначала прочие каналы живого
// селектора, затем остальные enabled live-verified; HY2-configured и выключенные
// не участвуют (FR-007, FR-002); текущий канал исключён.
func TestFailoverCandidatesOrder(t *testing.T) {
	descriptors := []render.ChannelDescriptor{
		{ID: "channel-a-vless", ValidationStatus: "live-verified", Enabled: true},
		{ID: "channel-a-hy2", ValidationStatus: "live-verified", Enabled: true},
		{ID: "channel-b-vless", ValidationStatus: "live-verified", Enabled: true},
		{ID: "channel-c-xhttp", ValidationStatus: "configured", Enabled: true}, // не verified
		{ID: "channel-d", ValidationStatus: "live-verified", Enabled: false},   // выключен
	}
	got := failoverCandidates(descriptors, map[string]bool{"channel-a-vless": true, "channel-a-hy2": true}, "channel-a-vless")
	want := []string{"channel-a-hy2", "channel-b-vless"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidates = %v, want %v (порядок важен)", got, want)
		}
	}
}

func TestClassAwareCandidateOrder(t *testing.T) {
	descs := []render.ChannelDescriptor{
		{ID: "failed", Protocol: "vless", Transport: "ws", Port: 443},
		{ID: "reality", Protocol: "vless", Transport: "tcp-reality", Port: 8443},
		{ID: "hy2", Protocol: "hysteria2", Transport: "quic", Port: 8444},
	}
	base := []string{"reality", "hy2"}
	if got := orderCandidatesForFailure(base, descs, "failed", fcSilentFreeze); got[0] != "hy2" {
		t.Fatalf("silent freeze должен предпочесть UDP: %v", got)
	}
	if got := orderCandidatesForFailure([]string{"hy2", "reality"}, descs, "failed", fcPortBlackholed); got[0] != "hy2" {
		t.Fatalf("оба кандидата меняют port family; generic order должен сохраниться: %v", got)
	}
	if got := orderCandidatesForFailure(base, descs, "failed", fcUnknown); got[0] != "reality" {
		t.Fatalf("unknown не должен выдумывать контрмеру: %v", got)
	}
}

// TestFailoverNoCandidatesBlocked — validated кандидатов нет → fail-closed
// BLOCKED без попыток и без прямого fallback (FR-001).
func TestFailoverNoCandidatesBlocked(t *testing.T) {
	a := NewApp()
	got := failoverCandidates([]render.ChannelDescriptor{
		{ID: "channel-a-vless", ValidationStatus: "live-verified", Enabled: true},
	}, map[string]bool{"channel-a-vless": true}, "channel-a-vless")
	if len(got) != 0 {
		t.Fatalf("единственный канал не должен давать кандидатов: %v", got)
	}
	switched, blocked := a.tryFailover(nil, map[string]bool{}, "channel-a-vless")
	if switched || !blocked {
		t.Fatalf("tryFailover без кандидатов = switched=%v blocked=%v, want false/true", switched, blocked)
	}
}

// TestRecordCandidateFailureMarksChannel — V2-075 (аудит D2): провалившийся
// кандидат получает ФАКТ отказа в реестре эпохи (сильный probe-счётчик →
// D2 QUARANTINED), иначе следующий инцидент снова выбирает его первым и вторая
// попытка сгорает на нём же — BLOCKED при живом соседнем канале.
func TestRecordCandidateFailureMarksChannel(t *testing.T) {
	a := NewApp()
	now := time.Now()
	netEpoch.bump("up", "wifi", now)
	epoch, _, _ := netEpoch.current()
	chanHealth.mu.Lock()
	chanHealth.resetLocked(epoch)
	chanHealth.mu.Unlock()
	t.Cleanup(func() {
		netEpoch.resetForTest()
		chanHealth.mu.Lock()
		chanHealth.resetLocked(0)
		chanHealth.mu.Unlock()
	})

	a.recordCandidateFailure("chan-x", now)
	st, ok := chanHealth.stateOf(epoch, "chan-x")
	if !ok {
		t.Fatal("факт отказа кандидата не записан в реестр эпохи")
	}
	if st.FAIL != 1 || st.ProbeFails != 1 || st.LastClass != fcChannelUnavail {
		t.Fatalf("запись кандидата неверна: %+v", st)
	}
	if d2 := a.quarantineStateOf("chan-x"); d2.State != qsQuarantined {
		t.Fatalf("сильный отказ обязан карантинить кандидата: %+v", d2)
	}

	// Пустой id — no-op: реестр не выдумывает записи (AGENTS §3.6).
	a.recordCandidateFailure("", now)
	if _, ok := chanHealth.stateOf(epoch, ""); ok {
		t.Fatal("пустой channelID не должен создавать запись")
	}
}
