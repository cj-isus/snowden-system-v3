package main

// controller.go — AdaptiveController state machine (PLAN Phase C1, V2-060;
// plan_gpt §22–26). Формализация того, что сторож и так делает с V2-037:
//
//	HEALTHY ──probe fail──▶ SUSPECTED ──fast reprobe PASS──▶ HEALTHY
//	   ▲                        │
//	   │                        ▼ FAIL (подтверждённая серия)
//	   │                   CLASSIFYING (классификатор THEORY, V2-056)
//	   │                        │
//	   │                        ▼
//	   │                    DEGRADED (решение: канал деградировал)
//	   │                        │
//	   │                        ▼ SWITCHING (tryFailover, бесшовно, rollback)
//	   │                    PASS → HEALTHY ; FAIL → DEGRADED / NEXT CANDIDATE
//	   │                        │ исчерпаны
//	   │                        ▼
//	   └──────────────────  BLOCKED (fail-closed, FR-001)
//
//	RECOVERING — fail-closed с запланированным авто-ретраем (V2-053):
//	BLOCKED/failCore → RECOVERING → (успешный startCore) → HEALTHY.
//
// Правила честности (те же, что у failoverPolicy):
//   - машина СОБИРАЕТ формализованную картину и пишет журнал; все решения
//     (failover, fail-closed, ретраи) остаются в существующих механизмах —
//     fail-closed семантика V2-037/V2-053 не менялась (тесты failover_test.go
//     зеленеют без правок — это и есть критерий «поверх, не вместо»);
//   - «SUSPECTED» = один провал тика до подтверждения серией (§23: transient
//     network failure не должен ничего ломать) — ровно то, что watchCore
//     делает fast-репробами с V2-037, теперь с именем;
//   - CLASSIFYING ≠ «навесили диагноз» бездоказательно: сюда машина попадает
//     только ПОСЛЕ подтверждённой серии, код даёт детерминированный
//     классификатор (failclass.go), unknown — честный unknown;
//   - состояние в памяти процесса (как failoverPolicy), не переживает
//     рестарт — факты канала не переносятся между процессами (AGENTS §3.3);
//   - пустое состояние ("") = цикла нет (нет туннеля / ручной Stop): честно
//     «не наблюдаем», а не «здоров».

import (
	"sync"
	"time"
)

// Состояния машины (plan_gpt §22 + RECOVERING §1416).
const (
	ctrlHealthy     = "HEALTHY"
	ctrlSuspected   = "SUSPECTED"
	ctrlClassifying = "CLASSIFYING"
	ctrlDegraded    = "DEGRADED"
	ctrlSwitching   = "SWITCHING"
	ctrlBlocked     = "BLOCKED"
	ctrlRecovering  = "RECOVERING"
)

// controllerSM — состояние адаптивного цикла. Собственный мьютекс: машина
// обновляется из разных горутин (watchCore, netChangeReprobe, startCore,
// failCore), ни с чем другим не соревнуется.
type controllerSM struct {
	mu     sync.Mutex
	state  string // "" = цикл не активен (нет туннеля)
	since  time.Time
	class  string    // код THEORY последнего классифицированного отказа
	report string    // краткая форма последнего провального Summary()
	repAt  time.Time // когда зафиксирован последний провал
}

// transition — единая точка смены состояния (журналируется вызывающими).
// nil-переход = guard не пустил (состояние осталось прежним).
func (c *controllerSM) transition(to string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = to
	c.since = now
}

// markStarted — туннель поднят (startCore успех, в т.ч. после авто-ретрая):
// RECOVERING → HEALTHY, любой старт → HEALTHY.
func (c *controllerSM) markStarted(now time.Time) {
	c.transition(ctrlHealthy, now)
}

// markStopped — туннель остановлен пользователем/закрытием: цикл погас.
func (c *controllerSM) markStopped(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state, c.since, c.class, c.report, c.repAt = "", time.Time{}, "", "", time.Time{}
}

// markTick — исход одного наблюдения (тик сторожа / netChangeReprobe /
// старт-гейт). ok=true → HEALTHY из любого состояния (восстановление всегда
// выигрывает — согласовано с fpol.hardReset). ok=false → SUSPECTED только из
// HEALTHY/пустого (§23: одиночный провал — подозрение); DEGRADED/CLASSIFYING
// не «разбавляются» одиночным провалом — там уже подтверждённый факт.
func (c *controllerSM) markTick(ok bool, summary string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ok {
		c.state, c.since, c.report, c.repAt = ctrlHealthy, now, "", time.Time{}
		return
	}
	if c.state == "" || c.state == ctrlHealthy || c.state == ctrlSuspected {
		c.state, c.since = ctrlSuspected, now
		c.report, c.repAt = summary, now
	}
}

// markRecovered — серия прервалась успехом (мерцание поглощено): HEALTHY.
func (c *controllerSM) markRecovered(now time.Time) {
	c.transition(ctrlHealthy, now)
}

// markClassified — серия подтверждена, классификатор дал код (V2-056).
// Пускаем из SUSPECTED/CLASSIFYING/DEGRADED: повторная серия после
// неудачной попытки классифицируется заново — это тот же инцидент.
func (c *controllerSM) markClassified(class, summary string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != ctrlSuspected && c.state != ctrlClassifying && c.state != ctrlDegraded {
		return
	}
	c.state, c.since = ctrlClassifying, now
	c.class, c.report, c.repAt = class, summary, now
}

// markDegraded — решение «канал деградировал» принято, попытка failover
// допустима (§24). Только из CLASSIFYING: деградация требует класса.
func (c *controllerSM) markDegraded(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != ctrlClassifying {
		return
	}
	c.state, c.since = ctrlDegraded, now
}

// markSwitching — попытка переключения начата (beginAttemptIfDue прошёл).
func (c *controllerSM) markSwitching(now time.Time) {
	c.transition(ctrlSwitching, now)
}

// markSwitched — переключение удалось (probe нового канала прошёл): HEALTHY.
func (c *controllerSM) markSwitched(now time.Time) {
	c.transition(ctrlHealthy, now)
}

// markSwitchFailed — попытка не удалась: откат на прежний канал (жив) или
// ожидание cooldown → назад в DEGRADED (§22 NEXT CANDIDATE).
func (c *controllerSM) markSwitchFailed(now time.Time) {
	c.transition(ctrlDegraded, now)
}

// markBlocked — кандидатов нет / лимит исчерпан: fail-closed BLOCKED (FR-001).
func (c *controllerSM) markBlocked(now time.Time) {
	c.transition(ctrlBlocked, now)
}

// markRecovering — fail-closed с запланированным авто-ретраем (V2-053):
// из любого состояния — fail-closed всегда выигрывает.
func (c *controllerSM) markRecovering(now time.Time) {
	c.transition(ctrlRecovering, now)
}

// controllerSnapshot — снимок для UI/журнала (без секретов, только факты).
type controllerSnapshot struct {
	State    string    `json:"state"`     // "" = цикл не активен
	Since    time.Time `json:"since"`     // с какого момента в состоянии
	Class    string    `json:"class"`     // код THEORY последнего отказа
	Report   string    `json:"report"`    // краткая форма последнего провала
	ReportAt time.Time `json:"report_at"` // когда зафиксирован провал
}

// snapshot — снимок под мьютексом.
func (c *controllerSM) snapshot() controllerSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return controllerSnapshot{State: c.state, Since: c.since, Class: c.class, Report: c.report, ReportAt: c.repAt}
}
