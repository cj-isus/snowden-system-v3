package main

// scoring.go — Candidate scoring для авто-failover (PLAN Phase C2, V2-061;
// plan_gpt §27/§28/§29/§30, «C2 — Candidate selection P0»: health, confidence,
// independence, freshness, failure history).
//
// Принцип §28 «Independence должна иметь большой вес»: при отказе канала
// выживает не «исторически самый стабильный» (он обычно той же семьи
// отказов), а МИНИМАЛЬНО КОРРЕЛИРОВАННЫЙ survivor. Поэтому независимость
// пары (кандидат, деградировавший канал) входит с наибольшим весом.
//
// Score (все компоненты [0..1], веса — инженерные константы до калибровки
// на ground truth Phase E2, как и веса Independence V2-059):
//
//	S(c) = 0.30·H(health) + 0.35·I(independence) + 0.20·F(freshness) + 0.15·E(evidence)
//
//   - H (health, §30): здоровый=1.0, no_data=0.5 (нейтрально: «не проверялся»
//     — не «болен»), failing=0.0;
//   - I (independence, §28): render.Independence(кандидат, деградировавший);
//     это policy-модель V2-059, не физика;
//   - F (freshness, §29): свежий успех в эпохе = 1.0; протухший/нет = 0.0.
//     Интервалы §29 — policy; у нас freshness = evidenceFreshMax (A0);
//   - E (evidence confidence): есть записи в текущей эпохе = 1.0, нет = 0.0.
//
// Честные границы:
//   - пустая эпоха (нет ни одного наблюдения): все no_data → score
//     определяется ТОЛЬКО независимостью (единственный известный факт) —
//     это осознанный дефолт, а не «данные потеряны»;
//   - порядок при равных score = прежний (failoverCandidates) — scoring
//     только ПЕРЕСОРТИРОВЫВАЕТ список, кандидатов не добавляет и не
//     убирает; гейты enabled/live-verified/в-селекторе остаются прежними;
//   - детерминировано: никаких случайностей, только факты реестра и
//     дескрипторов.

import (
	"sort"
	"strconv"
	"time"

	"github.com/snowden-system/windows/backend/render"
)

// scoreWeights — веса слагаемых C2 (Σ=1.0). §28: независимость — наибольший
// вес, «минимально коррелированный survivor» важнее исторической стабильности.
const (
	wHealth       = 0.30
	wIndependence = 0.35
	wFreshness    = 0.20
	wEvidenceConf = 0.15
)

// verdictHealth — H(c) по вердикту реестра (§30, честные нейтральные no_data).
func verdictHealth(v string) float64 {
	switch v {
	case "healthy":
		return 1.0
	case "failing":
		return 0.0
	default: // "no_data" — не проверялся в этой эпохе: нейтрально, не «болен»
		return 0.5
	}
}

// candidateScore — компоненты score одного кандидата против деградировавшего
// канала. current — дескриптор отказавшего канала (не nil: независимость
// считается ОТ НЕГО).
type candidateScore struct {
	Health   float64
	Indep    float64
	Fresh    float64
	Evidence float64
	Score    float64
	Verdict  string // verdict реестра ("" = записей не было; V2-075)
	D2State  string // D2-состояние кандидата (тай-брейк равных score, V2-064)
}

// scoreCandidate — вычислить score кандидата (чистая функция от фактов).
func scoreCandidate(cand, failed render.ChannelDescriptor, hs *healthState, hasData bool, now time.Time) candidateScore {
	cs := candidateScore{}
	verdict := "no_data"
	if hasData && hs != nil {
		verdict = hs.verdict(now)
		// V2-075 (аудит D6): evidence confidence — это наличие ФАКТА о канале,
		// а не наличие записи. Протухший успех даёт verdict=no_data (EvidenceAge),
		// поэтому Evidence=1.0 давала ему +0.15 над каналом, который вообще не
		// проверялся, — при одинаковом вердикте в UI. Пишем evidence только для
		// реально классифицированного состояния.
		if verdict != "no_data" {
			cs.Evidence = 1.0
		}
		if verdict == "healthy" {
			cs.Fresh = 1.0
		}
	}
	cs.Verdict = verdict
	cs.Health = verdictHealth(verdict)
	cs.Indep = render.Independence(cand, failed)
	cs.Score = wHealth*cs.Health + wIndependence*cs.Indep + wFreshness*cs.Fresh + wEvidenceConf*cs.Evidence
	return cs
}

// healthBucket — структурный приоритет состояния кандидата (V2-075, аудит D6).
// Меньше = предпочтительнее. Живое доказательство > нет данных > подтверждённый
// отказ. Причина введения отдельного ключа: при текущих весах C2 слагаемые
// `failing` и `no_data` дают РАВНЫЙ score (0.15 + 0.35·I каждая), поэтому
// подтверждённо упавший канал мог выбираться раньше непроверенного. Первичный
// бакет делает демоцию гарантированной и не зависящей от калибровки весов
// (Phase E2 ещё не проводилась), при этом внутри бакета порядок по-прежнему
// определяет score — включая независимость как главный вес (§28).
func healthBucket(verdict string) int {
	switch verdict {
	case "healthy":
		return 0
	case "no_data", "":
		return 1
	default: // failing и любой будущий подтверждённый отказ
		return 2
	}
}

// rankCandidates — пересортировка прежнего списка кандидатов по score.
// Порядок при равных score сохраняется (стабильная сортировка), т.е. при
// пустой эпохе и равной независимости порядок = failoverCandidates (V2-027);
// при равных score — тай-брейк по D2-состоянию (d2Rank, V2-064), при равных
// и оно — прежний порядок. d2s — подменяемый источник D2-состояний (может
// быть nil: тогда все состояния пустые = прежнее поведение C2).
// scores — для журнала порядка (почему выбран именно этот кандидат).
func rankCandidates(cands []string, descriptors []render.ChannelDescriptor, failedID string, hs func(id string) (*healthState, bool), d2s func(id string) string, now time.Time) ([]string, map[string]candidateScore) {
	byID := make(map[string]render.ChannelDescriptor, len(descriptors))
	for _, ch := range descriptors {
		byID[ch.ID] = ch
	}
	failed, ok := byID[failedID]
	if !ok {
		// Отказавший канал не найден в дескрипторах (нечитаемы/удалён):
		// независимость не считаем честно — порядок прежний.
		return cands, nil
	}
	scores := make(map[string]candidateScore, len(cands))
	type ranked struct {
		id    string
		order int // прежний ранг (тай-брейк стабильности)
		cs    candidateScore
	}
	rs := make([]ranked, 0, len(cands))
	for i, id := range cands {
		cand, ok := byID[id]
		if !ok {
			continue // не должен случиться (кандидаты берутся из дескрипторов)
		}
		st, hasData := hs(id)
		cs := scoreCandidate(cand, failed, st, hasData, now)
		if d2s != nil {
			cs.D2State = d2s(id) // тай-брейк равных score (V2-064)
		}
		scores[id] = cs
		rs = append(rs, ranked{id: id, order: i, cs: cs})
	}
	sort.SliceStable(rs, func(i, j int) bool {
		// V2-075 (аудит D6): первичный ключ — наблюдаемое состояние, и только
		// внутри бакета — score. Так «подтверждённо отказавший» кандидат не
		// может опередить «не проверявшегося» за счёт разницы независимости,
		// которая по текущим весам даёт всего ≤0.07 — меньше разрыва, который
		// раньше возникал из ничего (см. healthBucket).
		if bi, bj := healthBucket(rs[i].cs.Verdict), healthBucket(rs[j].cs.Verdict); bi != bj {
			return bi < bj
		}
		if rs[i].cs.Score != rs[j].cs.Score {
			return rs[i].cs.Score > rs[j].cs.Score
		}
		// D2 (V2-064): тай-брейк равных score — QUARANTINED опускается ниже
		// HEALTHY/PROBATION/RECOVERING (производное состояние recovery,
		// quarantine.go). Список не расширяется и не сокращается — только
		// порядок; при равных score И равном D2-ранге порядок прежний.
		return d2Rank(rs[i].cs.D2State) < d2Rank(rs[j].cs.D2State)
	})
	// Возврат: сначала все, кого удалось оценить (по score), затем —
	// неоцениваемые (кандидат без дескриптора — не должен случаться, но
	// порядок прежний и для них).
	out := make([]string, 0, len(cands))
	for _, r := range rs {
		out = append(out, r.id)
	}
	if len(out) < len(cands) {
		seen := map[string]bool{}
		for _, id := range out {
			seen[id] = true
		}
		for _, id := range cands {
			if !seen[id] {
				out = append(out, id)
			}
		}
	}
	return out, scores
}

// scoredCandidates — интеграция в tryFailover: пересортировка кандидатов
// по score против деградировавшего канала + строка журнала «почему».
// Пустой/неизвестный реестр = честный дефолт (см. шапку файла).
func (a *App) scoredCandidates(cands []string, descriptors []render.ChannelDescriptor, failedID string) ([]string, string) {
	if len(cands) <= 1 {
		return cands, "score: кандидатов меньше двух — порядок прежний"
	}
	epoch, _, _ := netEpoch.current()
	lookup := func(id string) (*healthState, bool) {
		st, ok := chanHealth.stateOf(epoch, id)
		if !ok {
			return nil, false
		}
		return &st, true
	}
	// D2 (V2-064): источник состояний восстановления — производное из
	// слабых фоновых фактов и health (quarantine.go), чтение потокобезопасно.
	d2src := func(id string) string { return a.quarantineStateOf(id).State }
	ranked, scores := rankCandidates(cands, descriptors, failedID, lookup, d2src, time.Now())
	if scores == nil {
		return cands, "score: дескриптор отказавшего канала не найден — порядок прежний"
	}
	if len(ranked) == 0 || ranked[0] == cands[0] {
		// Порядок не изменился — журнал короткий, честный.
		return ranked, "score: порядок прежний (лучший кандидат уже первый)"
	}
	detail := "score: " + ranked[0] + " предпочтительнее " + cands[0]
	if cs, ok := scores[ranked[0]]; ok {
		detail += " (health=" + f2(cs.Health) + " indep=" + f2(cs.Indep) + " fresh=" + f2(cs.Fresh) + ")"
	}
	return ranked, detail
}

// f2 — строка доли с двумя знаками (для журнала, без секретов).
func f2(v float64) string {
	// fmt доступен; избегаем лишних зависимостей в hot-path — строки короткие.
	return strconv.FormatFloat(v, 'f', 2, 64)
}
