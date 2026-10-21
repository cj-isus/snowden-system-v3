package main

// Тесты D2 (V2-064, PLAN Phase D2; plan_gpt «D2 — Recovery state:
// QUARANTINED → RECOVERING → PROBATION → HEALTHY»). Чистые, без сети:
// слабые фоновые факты подменяются напрямую в bgReg (той же эпохой, что и
// health-реестр), сильные — через healthState. Проверяются: приоритет
// сильное-против > сильное-за > слабое-против > слабое-за; полный цикл
// восстановления (включая честный RECOVERING «чужой успех сбросил серию,
// личного ответа ещё не было»); тай-брейк равных score в C2; инвариант
// «поверх, не вместо» — C2-тесты зелёные без правок, d2Rank(nil-источник) не меняет порядок.

import (
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// bgSeed — подменяемая точка текущей эпохи для тестов (обычно currentChannelEpoch()).
func bgEpochForTest(t *testing.T) int {
	t.Helper()
	return currentChannelEpoch() // 0 = эпоха не открыта — тестам достаточно
}

// TestD2DerivePriority — таблица приоритета фактов (шапка quarantine.go).
func TestD2DerivePriority(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	epoch := bgEpochForTest(t)
	now := time.Now()

	// (1) Сильное ПРОТИВ: probe-подтверждённый отказ не отозван успехом —
	// QUARANTINED даже при открытой фоновой серии (сильное против сильнее
	// слабого, но оба — «против», исход один).
	bgReg.confirmFail(epoch, "ch-a", now)
	bgReg.confirmFail(epoch, "ch-a", now)
	st := &healthState{FAIL: 1, ProbeFails: 1, LastFailAt: now, LastClass: fcResetInjected, LastSrc: srcProbe}
	got := deriveChannelRecoveryState("ch-a", epoch, st, true, now)
	if got.State != qsQuarantined {
		t.Fatalf("probe-отказ без успеха = QUARANTINED, получено %s (%s)", got.State, got.Note)
	}
	if got.QuarantineCount != 1 {
		t.Fatalf("QuarantineCount = %d, want 1 (фоновая серия)", got.QuarantineCount)
	}

	// (2) Тот же probe-отказ, но фон уже ответил (слабое ЗА) — сильное ПРОТИВ
	// всё ещё выигрывает: QUARANTINED.
	bgReg.markOK(epoch, "ch-a", now.Add(time.Second))
	got = deriveChannelRecoveryState("ch-a", epoch, st, true, now)
	if got.State != qsQuarantined {
		t.Fatalf("слабое ЗА не отменяет сильного ПРОТИВ: %s (%s)", got.State, got.Note)
	}

	// (3) Сильное ЗА: успех probe новее отказа. Открытая фоновая серия
	// (слабое ПРОТИВ) не может держать канал в карантине против сильного
	// факта — PROBATION, не QUARANTINED.
	st2 := &healthState{OK: 1, FAIL: 1, ProbeFails: 1, LastOKAt: now.Add(2 * time.Second), LastFailAt: now, LastSrc: srcProbe}
	got = deriveChannelRecoveryState("ch-a", epoch, st2, true, now)
	if got.State != qsProbation {
		t.Fatalf("probe-успех новее отказа при фоновой серии = PROBATION, получено %s (%s)", got.State, got.Note)
	}

	// (4) Сильное ЗА без фоновой истории — чистый HEALTHY.
	bgResetForTest()
	got = deriveChannelRecoveryState("ch-a", epoch, st2, true, now)
	if got.State != qsHealthy {
		t.Fatalf("probe-успех без истории = HEALTHY, получено %s (%s)", got.State, got.Note)
	}
}

// TestD2RecoveryLifecycle — полный цикл D1-канала: серия → карантин →
// чужой успех → RECOVERING → личный ответ → PROBATION → (эпоха кончилась
// бы — факты умерли бы с ней; смену эпохи имитируем чистым реестром).
func TestD2RecoveryLifecycle(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	epoch := bgEpochForTest(t)
	now := time.Now()
	id := "ch-cycle"

	// Нет фактов — HEALTHY.
	if got := deriveChannelRecoveryState(id, epoch, nil, false, now); got.State != qsHealthy {
		t.Fatalf("без фактов = HEALTHY, получено %s", got.State)
	}

	// Серия 1: не подтверждена — состояние ещё не карантин (анти-мерцание
	// V2-037: одиночный отказ не пишет факты... но серия ОТКРЫТА).
	bgReg.confirmFail(epoch, id, now)
	if got := deriveChannelRecoveryState(id, epoch, nil, false, now); got.State != qsRecovering {
		t.Fatalf("одиночный слабый отказ (Consec=1) = RECOVERING, получено %s (%s)", got.State, got.Note)
	}

	// Серия 2: подтверждена → FAIL-факт в health (как делает bgProbeChannel).
	bgReg.confirmFail(epoch, id, now)
	chanHealth.observeBg(epoch, id, fcPortBlackholed, now)

	// Чужой успех (живой listener другого канала) сбрасывает серию —
	// теперь RECOVERING: сеть жива, но ЛИЧНО канал не отвечал.
	bgReg.confirmFail(epoch, "ch-other", now)
	bgReg.confirmFail(epoch, "ch-other", now) // серия другого, для чистоты
	bgReg.resetConsec(epoch)                  // успех любого dial'а гасит все серии (V2-063)
	if got := deriveChannelRecoveryState(id, epoch, nil, false, now); got.State != qsRecovering {
		t.Fatalf("серия сброшена чужим успехом = RECOVERING, получено %s (%s)", got.State, got.Note)
	}

	// Личный фоновый ответ: слабое доказательство → PROBATION.
	bgReg.markOK(epoch, id, now.Add(time.Second))
	if got := deriveChannelRecoveryState(id, epoch, nil, false, now); got.State != qsProbation {
		t.Fatalf("личный ответ после отказов = PROBATION, получено %s (%s)", got.State, got.Note)
	}
	if got := deriveChannelRecoveryState(id, epoch, nil, false, now); got.QuarantineCount != 1 {
		t.Fatalf("история отказов не должна стираться в PROBATION: %d", got.QuarantineCount)
	}
}

// TestD2NewEpochHeals — смена эпохи хоронит все факты: фоновые (bgReg
// скоупится эпохой) и health — канал возвращается к честному HEALTHY
// (не «переносит» карантин через смену return-пути, V2-026).
func TestD2NewEpochHeals(t *testing.T) {
	bgResetForTest()
	netEpoch.resetForTest()
	t.Cleanup(func() { bgResetForTest(); netEpoch.resetForTest() })

	e := netEpoch.bump("up", "wifi", time.Now())
	now := time.Now()
	bgReg.confirmFail(e, "ch-epoch", now)
	bgReg.confirmFail(e, "ch-epoch", now)
	chanHealth.observeBg(e, "ch-epoch", fcServerUnreach, now)
	if got := deriveChannelRecoveryState("ch-epoch", e, nil, false, now); got.State != qsQuarantined {
		t.Fatalf("подтверждённая серия = QUARANTINED, получено %s", got.State)
	}
	// Новая эпоха: все факты против умерли вместе со старой сетью.
	e2 := netEpoch.bump("change", "mobile", now.Add(time.Second))
	if got := deriveChannelRecoveryState("ch-epoch", e2, nil, false, now.Add(time.Second)); got.State != qsHealthy {
		t.Fatalf("новая эпоха = HEALTHY (факты не переносятся), получено %s (%s)", got.State, got.Note)
	}
}

// TestD2TieBreakScoring — тай-брейк равных score: QUARANTINED опускается
// ниже HEALTHY при равном родстве; при равном D2-ранге порядок прежний.
func TestD2TieBreakScoring(t *testing.T) {
	descs := []render.ChannelDescriptor{
		{ID: "x", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", Enabled: true},
		{ID: "y", Protocol: "vless", Transport: "ws", Hostname: "cdn.example", Enabled: true},
		{ID: "failed", Protocol: "hysteria2", Transport: "quic", OriginServer: "1.1.1.1", Port: 8443, Enabled: true},
	}
	now := time.Now()

	// x/y симметричны относительно failed (как в TestRankStableOnEmptyData):
	// x — HEALTHY (пустой D2), y — QUARANTINED (подменённый источник).
	d2 := map[string]string{"y": qsQuarantined}
	ranked, scores := rankCandidates([]string{"x", "y"}, descs, "failed",
		lookupOf(nil), func(id string) string { return d2[id] }, now)
	if ranked[0] != "x" || ranked[1] != "y" {
		t.Fatalf("QUARANTINED обязан опускаться при равном score: %v", ranked)
	}
	if scores["y"].D2State != qsQuarantined {
		t.Fatalf("D2State кандидата не зафиксирован: %+v", scores["y"])
	}

	// Оба QUARANTINED — равный D2-ранг, порядок прежний (x, y).
	d2both := map[string]string{"x": qsQuarantined, "y": qsQuarantined}
	ranked2, _ := rankCandidates([]string{"x", "y"}, descs, "failed",
		lookupOf(nil), func(id string) string { return d2both[id] }, now)
	if ranked2[0] != "x" || ranked2[1] != "y" {
		t.Fatalf("равный D2-ранг обязан сохранять прежний порядок: %v", ranked2)
	}
}

// TestD2NilSourceKeepsC2 — nil-источник D2 (старые вызовы/тесты C2) не
// меняет порядок: инвариант «поверх, не вместо» (V2-060) сохранён.
func TestD2NilSourceKeepsC2(t *testing.T) {
	descs := synthDescs()
	r1, _ := rankCandidates([]string{"a-ws", "b-quic"}, descs, "c-reality", lookupOf(nil), nil, time.Now())
	// Эквивалентность nil и «все состояния пустые» (пустой источник):
	// оба пути обязаны дать одинаковый порядок.
	r3, _ := rankCandidates([]string{"a-ws", "b-quic"}, descs, "c-reality",
		lookupOf(nil), func(string) string { return "" }, time.Now())
	for i := range r1 {
		if r1[i] != r3[i] {
			t.Fatalf("nil-источник обязан вести себя как пустой: %v vs %v", r1, r3)
		}
	}
}

// TestD2FactsOf — слабые факты bgReg: счётчик/время подтверждённых отказов
// и последний ответ; серия не переходит эпоху.
func TestD2FactsOf(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	e := netEpoch.bump("up", "wifi", time.Now())
	now := time.Now()
	if bgReg.confirmFail(e, "ch-f", now) {
		t.Fatal("первый отказ не подтверждается")
	}
	if !bgReg.confirmFail(e, "ch-f", now.Add(time.Second)) {
		t.Fatal("второй подряд отказ обязан подтвердиться")
	}
	bgReg.markOK(e, "ch-f", now.Add(2*time.Second))
	f := bgReg.factsOf(e, "ch-f")
	if f.Fails != 1 || f.Consec != 2 || f.LastAt.IsZero() || f.LastOKAt.IsZero() {
		t.Fatalf("факты: %+v", f)
	}
	// Новая эпоха: серия и факты недоступны (fail-closed).
	e2 := netEpoch.bump("change", "mobile", now.Add(3*time.Second))
	f2 := bgReg.factsOf(e2, "ch-f")
	if f2.Fails != 0 || f2.Consec != 0 || f2.LastAt.IsZero() == false {
		t.Fatalf("факты старой эпохи должны быть недоступны: %+v", f2)
	}
	// LastAt/LastOKAt — нулевые в новой эпохе.
	if !f2.LastAt.IsZero() || !f2.LastOKAt.IsZero() {
		t.Fatalf("в новой эпохе слабые факты должны быть zero: %+v", f2)
	}
}

// TestD2SnapshotBinding — снапшот несёт D2-состояние для каналов с записями
// и для честных no_data-строк (GetChannelHealth); id — реальные дескрипторы
// (биндинг строит строки из данных, не из реестра).
func TestD2SnapshotBinding(t *testing.T) {
	bgResetForTest()
	t.Cleanup(bgResetForTest)

	epoch := bgEpochForTest(t)
	now := time.Now()
	bgReg.confirmFail(epoch, "channel-a-reality", now)
	bgReg.confirmFail(epoch, "channel-a-reality", now)
	chanHealth.observeBg(epoch, "channel-a-reality", fcPortBlackholed, now)

	snap := chanHealth.snapshot(now)
	var found bool
	for _, e := range snap.Chans {
		if e.ID == "channel-a-reality" {
			found = true
			if e.D2State != qsQuarantined {
				t.Fatalf("reality: D2State=%q, want QUARANTINED (%s)", e.D2State, e.D2Note)
			}
		}
	}
	if !found {
		t.Fatal("строка reality отсутствует в снапшоте")
	}
	a := NewApp()
	h := a.GetChannelHealth()
	var bgRow, healthyRow bool
	for _, e := range h.Chans {
		if e.ID == "channel-a-reality" {
			bgRow = true
			if e.D2State != qsQuarantined {
				t.Fatalf("биндинг: D2State=%q (%s)", e.D2State, e.D2Note)
			}
		}
		if e.ID == "channel-a-vless" {
			healthyRow = true
			if e.D2State != qsHealthy {
				t.Fatalf("no_data-строка без фактов против = HEALTHY, получено %q (%s)", e.D2State, e.D2Note)
			}
		}
	}
	if !bgRow {
		t.Fatal("биндинг не показал reality с фоновым фактом")
	}
	if !healthyRow {
		t.Fatal("биндинг не показал no_data-строку vless")
	}
}
