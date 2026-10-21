package main

// Тесты C2 candidate scoring (V2-061, PLAN Phase C2; plan_gpt §27–30).
// Чистые, без сети: реестр health подменяется напрямую, дескрипторы —
// синтетические. Проверяются: вес независимости (§28), честные no_data
// дефолты, EvidenceAge (§29), стабильность при пустых данных (порядок
// failoverCandidates не ломается), детерминизм.

import (
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// synthDescs — дескрипторы трёх каналов разных семей (для тестов scoring).
func synthDescs() []render.ChannelDescriptor {
	return []render.ChannelDescriptor{
		{ID: "a-ws", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", Enabled: true},
		{ID: "b-quic", Protocol: "hysteria2", Transport: "quic", OriginServer: "1.2.3.4", Port: 8443, Enabled: true},
		{ID: "c-reality", Protocol: "vless", Transport: "tcp-reality", Hostname: "www.donor.com", OriginServer: "5.6.7.8", Port: 443, Enabled: true},
	}
}

func lookupOf(items map[string]*healthState) func(string) (*healthState, bool) {
	return func(id string) (*healthState, bool) {
		st, ok := items[id]
		return st, ok
	}
}

// TestScoreIndependenceDominates — §28: кандидат, не делящий ни одного FD
// с отказавшим, обходит делящего. Против отказавшего c-reality (tcp-reality,
// origin 5.6.7.8, tcp/443, donor www.donor.com): a-ws делит tcp-слой
// (Indep 0.95), b-quic (udp/8443/origin 1.2.3.4) не делит ничего (Indep 1.0)
// → b-quic впереди при прочих равных (оба no_data).
func TestScoreIndependenceDominates(t *testing.T) {
	descs := synthDescs()
	cands := []string{"a-ws", "b-quic"}
	ranked, scores := rankCandidates(cands, descs, "c-reality", lookupOf(nil), nil, time.Now())
	if len(ranked) != 2 || ranked[0] != "b-quic" || ranked[1] != "a-ws" {
		t.Fatalf("независимый кандидат обязан быть впереди: %v", ranked)
	}
	if scores["b-quic"].Indep <= scores["a-ws"].Indep {
		t.Fatalf("indep b-quic (%.2f) <= a-ws (%.2f)", scores["b-quic"].Indep, scores["a-ws"].Indep)
	}
	// Вторая пара против отказавшего a-ws (ws/cf/tcp-слой): честная НИЧЬЯ —
	// b-quic делит port_family «other» (0.95), c-reality — tcp-слой (0.95).
	// Равный score = прежний порядок (стабильная сортировка).
	ranked2, scores2 := rankCandidates([]string{"b-quic", "c-reality"}, descs, "a-ws", lookupOf(nil), nil, time.Now())
	if scores2["b-quic"].Indep != 0.95 || scores2["c-reality"].Indep != 0.95 {
		t.Fatalf("ожидание ничьей 0.95: b-quic=%.2f c-reality=%.2f", scores2["b-quic"].Indep, scores2["c-reality"].Indep)
	}
	if ranked2[0] != "b-quic" || ranked2[1] != "c-reality" {
		t.Fatalf("равный score обязан сохранить прежний порядок: %v", ranked2)
	}
}

// TestScoreHealthBeatsEqualIndependence — при равной независимости больной
// кандидат опускается.
func TestScoreHealthBeatsEqualIndependence(t *testing.T) {
	descs := []render.ChannelDescriptor{
		{ID: "x", Protocol: "vless", Transport: "ws", Hostname: "cdn1.example", Enabled: true},
		{ID: "y", Protocol: "vless", Transport: "ws", Hostname: "cdn2.example", Enabled: true},
		{ID: "failed", Protocol: "vless", Transport: "tcp-reality", Hostname: "donor.example", OriginServer: "9.9.9.9", Port: 443, Enabled: true},
	}
	// x и y симметричны относительно failed (одинаковое родство) —
	// различаем только health: y failing, x healthy.
	now := time.Now()
	items := map[string]*healthState{
		"x": {OK: 3, LastOKAt: now},
		"y": {FAIL: 2, LastFailAt: now, LastClass: fcUnknown},
	}
	ranked, scores := rankCandidates([]string{"x", "y"}, descs, "failed", lookupOf(items), nil, now)
	if ranked[0] != "x" {
		t.Fatalf("healthy должен быть впереди failing при равном родстве: %v", ranked)
	}
	if scores["x"].Score <= scores["y"].Score {
		t.Fatalf("score healthy (%.2f) <= failing (%.2f)", scores["x"].Score, scores["y"].Score)
	}
}

// TestScoreNoDataNeutral — §30 честность: отсутствие данных = 0.5 (нейтрально),
// не «болен» (0) и не «здоров» (1); evidence-компонент честно 0.
func TestScoreNoDataNeutral(t *testing.T) {
	descs := synthDescs()
	now := time.Now()
	_, scores := rankCandidates([]string{"b-quic"}, descs, "a-ws", lookupOf(nil), nil, now)
	cs := scores["b-quic"]
	if cs.Health != 0.5 {
		t.Fatalf("no_data health = %.2f, want 0.50", cs.Health)
	}
	if cs.Evidence != 0 || cs.Fresh != 0 {
		t.Fatalf("no_data evidence/fresh = %.2f/%.2f, want 0/0", cs.Evidence, cs.Fresh)
	}
	// База против a-ws: b-quic (udp/8443) делит с ним только port_family
	// «other» (8443 и 0 оба не-443) → Indep = 0.95 — честно задокументировано.
	if cs.Indep != 0.95 {
		t.Fatalf("indep no_data-кандидата = %.2f, want 0.95 (общий port_family other)", cs.Indep)
	}
	want := wHealth*0.5 + wIndependence*0.95
	if diff := cs.Score - want; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("score no_data = %.4f, want %.4f", cs.Score, want)
	}
}

// TestScoreFreshnessCounts — §29: свежий успех добавляет freshness,
// протухший — нет (EvidenceAge через healthState.verdict).
func TestScoreFreshnessCounts(t *testing.T) {
	descs := synthDescs()
	now := time.Now()
	stale := now.Add(-2 * evidenceFreshMax)
	fresh := map[string]*healthState{"b-quic": {OK: 1, LastOKAt: now}}
	staled := map[string]*healthState{"b-quic": {OK: 1, LastOKAt: stale}}
	_, sFresh := rankCandidates([]string{"b-quic"}, descs, "a-ws", lookupOf(fresh), nil, now)
	_, sStale := rankCandidates([]string{"b-quic"}, descs, "a-ws", lookupOf(staled), nil, now)
	if sFresh["b-quic"].Fresh != 1.0 {
		t.Fatalf("свежий успех: fresh = %.2f, want 1.0", sFresh["b-quic"].Fresh)
	}
	if sStale["b-quic"].Fresh != 0.0 {
		t.Fatalf("протухший успех: fresh = %.2f, want 0.0", sStale["b-quic"].Fresh)
	}
	if sFresh["b-quic"].Score <= sStale["b-quic"].Score {
		t.Fatal("свежий кандидат обязан скорить выше протухшего")
	}
}

// TestRankStableOnEmptyData — пустая эпоха + равная независимость = прежний
// порядок failoverCandidates (scoring только пересортировывает, не выдумывает).
func TestRankStableOnEmptyData(t *testing.T) {
	descs := synthDescs()
	cands := []string{"a-ws", "b-quic", "c-reality"}
	// a-ws и c-reality оба tcp+443... нет: a-ws — ws. Подбираем пару с
	// равным score: x/y — одинаковые дескрипторы кроме имени.
	descs2 := []render.ChannelDescriptor{
		{ID: "x", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", Enabled: true},
		{ID: "y", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", Enabled: true},
		{ID: "failed", Protocol: "hysteria2", Transport: "quic", OriginServer: "1.1.1.1", Port: 8443, Enabled: true},
	}
	ranked, _ := rankCandidates([]string{"x", "y"}, descs2, "failed", lookupOf(nil), nil, time.Now())
	if ranked[0] != "x" || ranked[1] != "y" {
		t.Fatalf("равные score обязаны сохранить прежний порядок: %v", ranked)
	}
	_ = cands
	_ = descs
}

// TestScoredCandidatesFallsBack — дескриптор отказавшего канала не найден →
// порядок прежний и честная строка журнала (независимость не считается).
func TestScoredCandidatesFallsBack(t *testing.T) {
	a := NewApp()
	out, log := a.scoredCandidates([]string{"x", "y"}, synthDescs(), "no-such")
	if len(out) != 2 || out[0] != "x" || out[1] != "y" {
		t.Fatalf("fallback: порядок обязан остаться прежним: %v", out)
	}
	if log == "" {
		t.Fatal("обязательна честная строка журнала о fallback")
	}
}

// TestScoredCandidatesSingle — один кандидат: scoring не нужен, порядок прежний.
func TestScoredCandidatesSingle(t *testing.T) {
	a := NewApp()
	out, _ := a.scoredCandidates([]string{"only"}, synthDescs(), "a-ws")
	if len(out) != 1 || out[0] != "only" {
		t.Fatalf("один кандидат: %v", out)
	}
}

// TestScoreDeterministic — два вызова на одинаковых фактах дают одинаковый
// порядок (детерминизм policy-модели; plan_gpt §6/§27).
func TestScoreDeterministic(t *testing.T) {
	descs := synthDescs()
	now := time.Now()
	items := map[string]*healthState{"b-quic": {OK: 2, LastOKAt: now}}
	cands := []string{"a-ws", "b-quic", "c-reality"}
	r1, s1 := rankCandidates(cands, descs, "c-reality", lookupOf(items), nil, now)
	r2, s2 := rankCandidates(cands, descs, "c-reality", lookupOf(items), nil, now)
	for i := range r1 {
		if r1[i] != r2[i] || s1[r1[i]] != s2[r2[i]] {
			t.Fatalf("недетерминизм: %v vs %v", r1, r2)
		}
	}
}

// TestScoreSelfIndependenceZero — деградировавший канал не может быть своим
// кандидатом, но guard на всякий случай: Independence(x,x)=0 → последний.
func TestScoreSelfIndependenceZero(t *testing.T) {
	descs := synthDescs()
	_, scores := rankCandidates([]string{"a-ws"}, descs, "a-ws", lookupOf(nil), nil, time.Now())
	if scores["a-ws"].Indep != 0 {
		t.Fatalf("Independence(x,x) = %.2f, want 0", scores["a-ws"].Indep)
	}
}

// TestRankFailingNeverBeatsNoData — регрессия V2-075 (аудит D6): подтверждённо
// отказавший кандидат не может опередить непроверенного даже при большей
// независимости. При прежних весах их score совпадал побитово
// (0.30·0 + 0.15·1 ≡ 0.30·0.5 + 0.15·0 = 0.15 + 0.35·I), поэтому порядок
// решала разница независимости — то есть отказ не понижал кандидата вообще.
func TestRankFailingNeverBeatsNoData(t *testing.T) {
	descs := []render.ChannelDescriptor{
		// bad максимально независим от failed: udp, другой origin, порт 8444.
		{ID: "bad", Protocol: "hysteria2", Transport: "quic", OriginServer: "1.2.3.4", Port: 8444, Enabled: true},
		// unknown делит с failed слой tcp и класс порта 443 → независимость ниже.
		{ID: "unknown", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", OriginServer: "5.6.7.8", Port: 443, Enabled: true},
		{ID: "failed", Protocol: "vless", Transport: "tcp-reality", Hostname: "donor.example", OriginServer: "9.9.9.9", Port: 443, Enabled: true},
	}
	now := time.Now()
	items := map[string]*healthState{"bad": {FAIL: 1, ProbeFails: 1, LastFailAt: now, LastClass: fcChannelUnavail}}
	ranked, scores := rankCandidates([]string{"bad", "unknown"}, descs, "failed", lookupOf(items), nil, now)
	if scores["bad"].Indep <= scores["unknown"].Indep {
		t.Fatalf("предпосылка теста нарушена: failing должен быть независимее (%.2f vs %.2f)",
			scores["bad"].Indep, scores["unknown"].Indep)
	}
	if scores["bad"].Verdict != "failing" || scores["unknown"].Verdict != "no_data" {
		t.Fatalf("вердикты: bad=%q unknown=%q", scores["bad"].Verdict, scores["unknown"].Verdict)
	}
	if ranked[0] != "unknown" {
		t.Fatalf("непроверенный кандидат обязан быть впереди подтверждённо отказавшего: %v", ranked)
	}
}

// TestEvidenceOnlyForLiveFact — evidence-бонус только за ЖИВОЙ факт (V2-075):
// протухший успех даёт verdict no_data и не должен получать +0.15 над каналом,
// который вообще не проверялся (при одинаковом вердикте в UI).
func TestEvidenceOnlyForLiveFact(t *testing.T) {
	descs := synthDescs()
	now := time.Now()
	stale := map[string]*healthState{"b-quic": {OK: 1, LastOKAt: now.Add(-2 * evidenceFreshMax)}}
	_, ss := rankCandidates([]string{"b-quic"}, descs, "a-ws", lookupOf(stale), nil, now)
	if ss["b-quic"].Verdict != "no_data" {
		t.Fatalf("протухший успех обязан давать no_data, получено %q", ss["b-quic"].Verdict)
	}
	if ss["b-quic"].Evidence != 0 {
		t.Fatalf("протухший факт не должен давать evidence-бонус: %.2f", ss["b-quic"].Evidence)
	}
	_, fresh := rankCandidates([]string{"b-quic"}, descs, "a-ws",
		lookupOf(map[string]*healthState{"b-quic": {OK: 1, LastOKAt: now}}), nil, now)
	if fresh["b-quic"].Evidence != 1.0 || fresh["b-quic"].Verdict != "healthy" {
		t.Fatalf("свежий успех обязан давать evidence и healthy: %+v", fresh["b-quic"])
	}
}
