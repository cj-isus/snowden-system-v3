package main

// Тесты AdaptiveController state machine (V2-060, PLAN C1; plan_gpt §22–26).
// Чистые, без сети и движка: проверяются переходы и guard'ы. Критерий
// «поверх, не вместо»: тесты failover_policy без правок зелёные — машина
// решения не принимает, fail-closed семантика V2-037/V2-053 не изменена.

import (
	"testing"
	"time"
)

func ts(min int) time.Time { return time.Now().Add(time.Duration(min) * time.Minute) }

// TestSMHealthyToSuspectedAndBack — §22/§23: одиночный провал = SUSPECTED,
// успех возвраща�� в HEALTHY (transient-провал ничего не ломает).
func TestSMHealthyToSuspectedAndBack(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	if st := sm.snapshot(); st.State != ctrlHealthy {
		t.Fatalf("старт = %s, want HEALTHY", st.State)
	}
	sm.markTick(false, "подтверждаю серией", ts(-4))
	if st := sm.snapshot(); st.State != ctrlSuspected {
		t.Fatalf("одиночный провал = %s, want SUSPECTED", st.State)
	}
	sm.markTick(true, "", ts(-3))
	if st := sm.snapshot(); st.State != ctrlHealthy || st.Report != "" {
		t.Fatalf("восстановление = %s/%q, want HEALTHY/пусто", st.State, st.Report)
	}
}

// TestSMSuspectedStable — повторный провал ДО подтверждения остаётся
// SUSPECTED (не прыгает через классификацию).
func TestSMSuspectedStable(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	sm.markTick(false, "провал 1", ts(-4))
	sm.markTick(false, "провал 2", ts(-3))
	if st := sm.snapshot(); st.State != ctrlSuspected {
		t.Fatalf("повторный провал без серии = %s, want SUSPECTED", st.State)
	}
}

// TestSMConfirmedSeriesClassifiesThenDegrades — §22: серия FAIL →
// CLASSIFYING (код THEORY, включая честный unknown) → DEGRADED.
func TestSMConfirmedSeriesClassifiesThenDegrades(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	sm.markTick(false, "первый провал", ts(-4))
	sm.markClassified(fcUnknown, "unexpected eof", ts(-3))
	if st := sm.snapshot(); st.State != ctrlClassifying || st.Class != fcUnknown {
		t.Fatalf("классификация = %s/%s, want CLASSIFYING/unknown", st.State, st.Class)
	}
	sm.markDegraded(ts(-2))
	if st := sm.snapshot(); st.State != ctrlDegraded {
		t.Fatalf("после решения = %s, want DEGRADED", st.State)
	}
}

// TestSMClassifiedGuards — классификация не из наблюдения (HEALTHY/пусто)
// отвергается: диагноз без инцидента невозможен.
func TestSMClassifiedGuards(t *testing.T) {
	var sm controllerSM
	sm.markClassified("reset_injected", "s", ts(-1)) // пустое состояние — guard
	if st := sm.snapshot(); st.State != "" {
		t.Fatalf("классификация без инцидента = %q, want пусто", st.State)
	}
	sm.markStarted(ts(-2))
	sm.markClassified("reset_injected", "s", ts(-1)) // HEALTHY — guard
	if st := sm.snapshot(); st.State != ctrlHealthy {
		t.Fatalf("классификация в HEALTHY = %s, want HEALTHY", st.State)
	}
}

// TestSMDegradedIgnoredSuspectedTick — одиночный провал не «разбавляет»
// DEGRADED/CLASSIFYING: там уже подтверждённый факт.
func TestSMDegradedIgnoredSuspectedTick(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	sm.markTick(false, "провал", ts(-4))
	sm.markClassified(fcUnknown, "s", ts(-3))
	sm.markDegraded(ts(-3))
	sm.markTick(false, "ещё провал", ts(-2))
	if st := sm.snapshot(); st.State != ctrlDegraded {
		t.Fatalf("провал в DEGRADED = %s, want DEGRADED", st.State)
	}
}

// TestSMSwitchCycle — §25/§26: SWITCHING → PASS → HEALTHY; FAIL → DEGRADED.
func TestSMSwitchCycle(t *testing.T) {
	var smPass controllerSM
	smPass.markStarted(ts(-5))
	smPass.markTick(false, "p", ts(-4))
	smPass.markClassified(fcUnknown, "s", ts(-3))
	smPass.markDegraded(ts(-3))
	smPass.markSwitching(ts(-2))
	if st := smPass.snapshot(); st.State != ctrlSwitching {
		t.Fatalf("попытка = %s, want SWITCHING", st.State)
	}
	smPass.markSwitched(ts(-2))
	if st := smPass.snapshot(); st.State != ctrlHealthy {
		t.Fatalf("успешный свитч = %s, want HEALTHY", st.State)
	}

	var smFail controllerSM
	smFail.markStarted(ts(-5))
	smFail.markClassified(fcUnknown, "s", ts(-4))
	smFail.markDegraded(ts(-4))
	smFail.markSwitching(ts(-3))
	smFail.markSwitchFailed(ts(-3))
	if st := smFail.snapshot(); st.State != ctrlDegraded {
		t.Fatalf("неудачный свитч = %s, want DEGRADED (NEXT CANDIDATE)", st.State)
	}
}

// TestSMBlockedRecoveringCycle — исчерпание → BLOCKED → RECOVERING →
// успешный start → HEALTHY (авто-ретрай V2-053 формализован).
func TestSMBlockedRecoveringCycle(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	sm.markTick(false, "p", ts(-4))
	sm.markClassified(fcUnknown, "s", ts(-3))
	sm.markDegraded(ts(-3))
	sm.markSwitching(ts(-2))
	sm.markSwitchFailed(ts(-2))
	sm.markBlocked(ts(-2))
	if st := sm.snapshot(); st.State != ctrlBlocked {
		t.Fatalf("исчерпание = %s, want BLOCKED", st.State)
	}
	sm.markRecovering(ts(-1))
	if st := sm.snapshot(); st.State != ctrlRecovering {
		t.Fatalf("после планирования ретрая = %s, want RECOVERING", st.State)
	}
	sm.markStarted(ts(0))
	if st := sm.snapshot(); st.State != ctrlHealthy {
		t.Fatalf("успешный ретрай = %s, want HEALTHY", st.State)
	}
}

// TestSMRecoveringWinsAlways — fail-closed выигрывает из любого состояния
// (fail-closed семантика не ослаблена).
func TestSMRecoveringWinsAlways(t *testing.T) {
	for _, from := range []string{ctrlSuspected, ctrlClassifying, ctrlDegraded, ctrlSwitching} {
		var sm controllerSM
		sm.markStarted(ts(-5))
		sm.transition(from, ts(-4))
		sm.markRecovering(ts(-3))
		if st := sm.snapshot(); st.State != ctrlRecovering {
			t.Fatalf("из %s: recovering = %s, want RECOVERING", from, st.State)
		}
	}
}

// TestSMRecoveryWinsAlways — успех пути выигрывает из любого состояния
// (восстановление всегда лучше любого другого исхода; согласовано с
// fpol.hardReset на живом туннеле).
func TestSMRecoveryWinsAlways(t *testing.T) {
	for _, from := range []string{ctrlSuspected, ctrlClassifying, ctrlDegraded, ctrlSwitching, ctrlRecovering} {
		var sm controllerSM
		sm.markStarted(ts(-5))
		sm.transition(from, ts(-4))
		sm.markTick(true, "", ts(-3))
		if st := sm.snapshot(); st.State != ctrlHealthy {
			t.Fatalf("из %s: успех = %s, want HEALTHY", from, st.State)
		}
	}
}

// TestSMStoppedClears — ручной Stop гасит цикл: "" = не наблюдаем (честно
// «нет данных», не «здоров»).
func TestSMStoppedClears(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-5))
	sm.markClassified(fcUnknown, "s", ts(-4))
	sm.markStopped(ts(-3))
	st := sm.snapshot()
	if st.State != "" || st.Class != "" || st.Report != "" || !st.Since.IsZero() {
		t.Fatalf("после Stop = %+v, want пустое состояние", st)
	}
}

// TestSMEmptyMeansNotObserved — пустая машина не должна читаться как
// HEALTHY: отсутствие наблюдения ≠ здоровье (AGENTS §3.6).
func TestSMEmptyMeansNotObserved(t *testing.T) {
	var sm controllerSM
	st := sm.snapshot()
	if st.State == ctrlHealthy || st.State == "" && false {
		t.Fatal("недостижимо")
	}
	if st.State != "" {
		t.Fatalf("новая машина = %q, want пусто", st.State)
	}
}

// TestSMSnapshotDeterministic — два снимка подряд идентичны без событий.
func TestSMSnapshotDeterministic(t *testing.T) {
	var sm controllerSM
	sm.markStarted(ts(-1))
	sm.markTick(false, "p", ts(0))
	a1, a2 := sm.snapshot(), sm.snapshot()
	if a1 != a2 {
		t.Fatalf("снимки разошлись: %+v vs %+v", a1, a2)
	}
}
