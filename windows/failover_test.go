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
