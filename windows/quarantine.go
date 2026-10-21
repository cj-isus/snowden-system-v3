package main

// quarantine.go — D2 Recovery state (PLAN Phase D2, V2-064; plan_gpt
// «D2 — Recovery state: QUARANTINED → RECOVERING → PROBATION → HEALTHY»).
//
// Состояние канала ВЫВОДИТСЯ из уже существующих фактов, ничего не хранит
// и ничего не решает (критерий «поверх, не вместо», V2-060):
//
//   - сильные факты — защищённый probe (healthState): успех/отказ
//     ПОДТВЕРЖДАЮТ защищённый путь;
//   - слабые факты — фон D1 (bgReg.factsOf): отказ = «цель не отвечает на
//     TCP-хендшейк» (вероятен мёртвый origin, но не доказано), ответ =
//     «порт открыт» (≠ живой защищённый путь, AGENTS §3.3).
//
// Цепочка восстановления (все переходы — наблюдаемые события, не таймеры):
//
//	QUARANTINED  — есть факты ПРОТИВ: probe-подтверждённый отказ, либо
//	               открытая фоновая серия;
//	RECOVERING   — серия сброшена (фон увидел живую сеть — чужой успех),
//	               но ЛИЧНОГО ответа канала ещё не было: ждём фонового
//	               раунда;
//	PROBATION    — слабое доказательство получено (фон снова отвечает)
//	               и/или защищённый probe подтвердил живость после фоновых
//	               отказов: канал под наблюдением до конца эпохи;
//	HEALTHY      — нет фактов против.
//
// Приоритет: сильное против > сильное за > слабое против > слабое за
// (probe-успех новее и сильнее фоновой серии — она не может держать канал
// в карантине против сильного факта; probe-отказ, не отозванный успехом,
// сильнее любого фонового «за»).
//
// Честные границы (AGENTS §3.3/§3.4):
//   - состояние производное, не хранится, решения не принимает:
//     failoverPolicy/Manager (V2-037) и controllerSM (V2-060) не тронуты;
//     единственное влияние — тай-брейк РАВНЫХ score в C2 (scoring.go) и
//     экспозиция в UI: список кандидатов не расширяется и не сокращается;
//   - PROBATION/RECOVERING не «прощают» фоновые отказы для C2: FAIL-факт
//     в реестре остаётся, health-компонента score не меняется;
//   - всё скоупится сетевой эпохой (bgReg и healthState ключуются ею
//     одинаково — bgprobe.go пишет оба в одну эпоху);
//   - состояние — мгновенный снимок фактов, без памяти между вызовами
//     (история эпохи живёт в самих фактах).
//
// Потокобезопасность: читает копию healthState (stateOf) и bgReg под его
// мьютексом; результат — значение без разделяемых полей.

import "time"

// Состояния D2 (словарь plan_gpt; строки наружу — UI/журнал).
const (
	qsHealthy     = "HEALTHY"
	qsProbation   = "PROBATION"
	qsRecovering  = "RECOVERING"
	qsQuarantined = "QUARANTINED"
)

// d2Rank — порядок «доверия» для тай-брейка равных score в C2 (scoring.go):
// меньше = предпочтительнее. QUARANTINED не исключается из перебора —
// инвариант V2-037 (список кандидатов не сокращается) не тронут.
func d2Rank(state string) int {
	switch state {
	case qsHealthy:
		return 0
	case qsProbation:
		return 1
	case qsRecovering:
		return 2
	case qsQuarantined:
		return 3
	default:
		return 0
	}
}

// channelRecoveryState — производное D2-состояние одного канала.
type channelRecoveryState struct {
	State string // qsHealthy | qsProbation | qsRecovering | qsQuarantined
	Note  string // человекочитаемое основание (UI/журнал, без секретов)
	// QuarantineCount — подтверждённых фоновых отказов в эпохе (история;
	// 0 = фоновых отказов не было).
	QuarantineCount int
}

// deriveChannelRecoveryState — чистый вывод состояния из фактов.
// epoch — эпоха, в которой живут факты; st/hasData — копия записи health
// (chanHealth.stateOf); сильные и слабые факты ключуются одной эпохой.
func deriveChannelRecoveryState(id string, epoch int, st *healthState, hasData bool, now time.Time) channelRecoveryState {
	bg := bgReg.factsOf(epoch, id)
	out := channelRecoveryState{State: qsHealthy, Note: "оснований для карантина нет", QuarantineCount: bg.Fails}

	strongFor := hasData && st != nil && st.OK > 0 && (st.LastOutcome == "ok" || (st.LastOutcome == "" && st.LastOKAt.After(st.LastFailAt)))
	// 1. Сильное ПРОТИВ: probe-подтверждённый отказ в эпохе, не отозванный
	// успехом (ProbeFails — только защищённый probe; фон не увеличивает).
	if hasData && st != nil && st.ProbeFails > 0 && !strongFor {
		out.State = qsQuarantined
		out.Note = "защищённый probe подтвердил отказ — до успешного защищённого probe канал не считается живым"
		return out
	}
	// 2. Сильное ЗА: последний факт — свежий защищённый успех. Фоновая
	// история эпохи понижает до PROBATION, но не карантинит.
	if strongFor {
		if bg.Fails > 0 {
			out.State = qsProbation
			out.Note = "защищённый probe подтвердил живость после фоновых отказов (фон ошибался) — канал под наблюдением до конца эпохи"
		} else {
			out.Note = "живой probe подтверждает канал"
		}
		return out
	}
	// 3. Слабое ПРОТИВ прямо сейчас: открытая фоновая серия канала.
	if bg.Consec >= 2 {
		out.State = qsQuarantined
		out.Note = "фон: серия TCP-хендшейков подтвердила недоступность origin; окончательное слово за защищённым probe"
		return out
	}
	if bg.Consec == 1 {
		out.State = qsRecovering
		out.Note = "фон: один TCP-хендшейк не прошёл — ждём подтверждения серии"
		return out
	}
	// 4. Слабые против в истории, серия закрыта.
	if bg.Fails > 0 {
		if bg.LastOKAt.After(bg.LastAt) {
			out.State = qsProbation
			out.Note = "фон снова отвечает после отказов — слабое доказательство (открытый порт ≠ защищённый путь); окончательное слово за защищённым probe"
			return out
		}
		// Серия сброшена чужим успехом (сеть жива), но ЛИЧНО канал ещё
		// не отвечал: ждать его фонового раунда.
		out.State = qsRecovering
		out.Note = "серия фоновых отказов сброшена (фон увидел живую сеть), личного ответа канала ещё не было — ждём фонового раунда"
		return out
	}
	// 5. Фактов против нет — HEALTHY.
	if hasData && st != nil && st.verdict(now) == "failing" {
		// Вердикт failing без ProbeFails и без фоновых фактов не бывает
		// (оба источника учтены выше) — защитная ветка, честная заметка.
		out.Note = "вердикт failing без сильных/слабых фактов — противоречие данных, состояние не выдумывается"
	}
	return out
}

// quarantineStateOf — потокобезопасное D2-состояние канала в текущей эпохе
// (для тай-брейка scoring и журнала переключений). Копия healthState берётся
// под мьютексом реестра, вывод — чистая функция.
func (a *App) quarantineStateOf(id string) channelRecoveryState {
	epoch := currentChannelEpoch()
	st, hasData := chanHealth.stateOf(epoch, id)
	return deriveChannelRecoveryState(id, epoch, &st, hasData, time.Now())
}
