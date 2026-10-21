package main

// incidents.go — D3 Incident clustering (PLAN Phase D3, V2-065; plan_gpt
// «D3 — Incident clustering: A + B + C failure → one incident»).
//
// Вопрос, на который отвечает файл: «каналы падают вместе» — это ОДНА волна
// сети или N независимых инцидентов? Это диагностический факт, не решение:
// волна объясняет, почему failover «вдруг» перебрал всех кандидатов (все
// легли одной волной), и не раздувает картину инцидентов до N строк.
//
// Принципы:
//   - кластеризация — ЧИСТАЯ функция от фактов (кольцо подтверждённых
//     отказов эпохи chanHealth.events — те же события, что кормят FAIL,
//     только с временем каждого); время — единственная связь, «причина»
//     не выдумывается (AGENTS §3.3: корреляция ≠ причинность);
//   - двойная шкала (стандартная практика кластеризации): событие входит
//     в текущую волну, если оно не дальше D3Window от ПОСЛЕДНЕГО события
//     волны И не дальше D3Coalesce от её центра (среднего времени членов);
//     широкое окно захватывает кандидатов, узкий порог склеивает — семантика
//     консервативная: сомнительные случаи остаются РАЗДЕЛЬНЫМИ инцидентами
//     (пересклеить независимые отказы хуже, чем показать два);
//   - детерминизм: вход сортируется по времени, пороги чистые (plan_gpt §6);
//   - честность: 1 событие = 1 инцидент («волна» не рисуется из одного
//     факта — AGENTS §3.6); эпоха 0 (сеть не наблюдалась) — честный отказ
//     биндинга, не пустая картинка.
//
// Потокобезопасность: чтение кольца — под мьютексом реестра; кластеризация
// — чистая функция; результат — значение без разделяемых полей.

import (
	"fmt"
	"sort"
	"time"
)

// Параметры D3 (policy-константы v1, как веса C2/порог B2 — до калибровки
// на ground truth Phase E2). Обоснование шкалы: сторож тикает раз в 30с,
// фоновый раунд раз в минуту — отказы одной волны обычно приходят сериями
// в пределах пары минут; 30с-центр склеивает одновременные подтверждения,
// 90с-окно не даёт волком растянуться на весь эпизод.
const (
	// D3Window — максимальная пауза между соседними событиями волны.
	D3Window = 90 * time.Second
	// D3Coalesce — максимальное отклонение события от центра волны (среднего
	// времени членов). Калибровка v1: волны реальных эпизодов (V2-039/045)
	// разнесены на десятки секунд — CDN-путь умирает раньше QUIC; 60с держит
	// такие волны целыми, но не даёт цепи тянуться бесконечно (средний центр
	// уезжает, и далёкие события от него откалываются).
	D3Coalesce = 60 * time.Second
	// incidentRingCap — ёмкость кольца отказов эпохи (новейшие остаются).
	incidentRingCap = 128
)

// failEvent — одно подтверждённое событие отказа канала в эпохе.
type failEvent struct {
	ID       string
	At       time.Time
	Class    string // класс failclass ("" = не классифицировано)
	Src      string // srcProbe | srcBackground
	NetClass string // snapshot класса сети в момент события
	ISP      string // snapshot наблюдаемого оператора; "" = факта не было
}

// IncidentCluster — сгруппированная волна отказов (диагностический факт).
type IncidentCluster struct {
	Since      string   `json:"since"`      // момент первого отказа волны (RFC3339)
	Until      string   `json:"until"`      // момент последнего отказа волны (RFC3339)
	Channels   []string `json:"channels"`   // уникальные id каналов волны (порядок первого отказа)
	Count      int      `json:"count"`      // событий в волне (может быть > числа каналов)
	Classes    []string `json:"classes"`    // уникальные классы отказов (порядок появления)
	NetClasses []string `json:"netClasses"` // классы сети, зафиксированные в событиях
	ISPs       []string `json:"isps"`       // операторы, зафиксированные в событиях
	IsWave     bool     `json:"isWave"`     // склеено ли в волну (false = одиночный инцидент)
	ClusterID  string   `json:"clusterId"`  // детерминированный id (для ссылок из журнала/UI)
}

// ClusterIncidents — ЧИСТАЯ кластеризация: сортировка по времени → жадное
// группирование по двойной шкале. Центр волны — среднее арифметическое
// времени её членов (пересчитывается при каждом добавлении; группы
// короткие, стоимость мала).
func ClusterIncidents(events []failEvent) []IncidentCluster {
	if len(events) == 0 {
		return nil
	}
	sorted := make([]failEvent, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })

	type group struct {
		events []failEvent
		center time.Time
	}
	groups := make([]group, 0, 4)
	for _, ev := range sorted {
		if len(groups) > 0 {
			g := &groups[len(groups)-1]
			if ev.At.Sub(g.events[len(g.events)-1].At) <= D3Window && absDur(ev.At.Sub(g.center)) <= D3Coalesce {
				g.events = append(g.events, ev)
				var sum time.Duration
				for _, e := range g.events {
					sum += e.At.Sub(g.events[0].At)
				}
				g.center = g.events[0].At.Add(sum / time.Duration(len(g.events)))
				continue
			}
		}
		groups = append(groups, group{events: []failEvent{ev}, center: ev.At})
	}

	out := make([]IncidentCluster, 0, len(groups))
	for _, g := range groups {
		cl := IncidentCluster{
			Since:  g.events[0].At.Format(time.RFC3339),
			Until:  g.events[len(g.events)-1].At.Format(time.RFC3339),
			Count:  len(g.events),
			IsWave: len(g.events) > 1,
		}
		seenCh := map[string]bool{}
		seenCls := map[string]bool{}
		seenNet := map[string]bool{}
		seenISP := map[string]bool{}
		for _, ev := range g.events {
			if !seenCh[ev.ID] {
				seenCh[ev.ID] = true
				cl.Channels = append(cl.Channels, ev.ID)
			}
			if ev.Class != "" && !seenCls[ev.Class] {
				seenCls[ev.Class] = true
				cl.Classes = append(cl.Classes, ev.Class)
			}
			if ev.NetClass != "" && !seenNet[ev.NetClass] {
				seenNet[ev.NetClass] = true
				cl.NetClasses = append(cl.NetClasses, ev.NetClass)
			}
			if ev.ISP != "" && !seenISP[ev.ISP] {
				seenISP[ev.ISP] = true
				cl.ISPs = append(cl.ISPs, ev.ISP)
			}
		}
		cl.ClusterID = clusterIDOf(cl)
		out = append(out, cl)
	}
	// Новейшие волны первыми (журнал/UI читаются сверху).
	sort.SliceStable(out, func(i, j int) bool { return out[i].Since > out[j].Since })
	return out
}

// absDur — модуль длительности (событие может быть раньше центра волны
// при неравномерных сериях — центровое отклонение берётся по модулю).
func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// clusterIDOf — детерминированный короткий id волны: время первого события
// + число каналов (без секретов; достаточно для ссылок в журнале/UI).
func clusterIDOf(cl IncidentCluster) string {
	return fmt.Sprintf("wave-%s-%dch", cl.Since, len(cl.Channels))
}

// GetIncidents — биндинг UI: волны подтверждённых отказов ТЕКУЩЕЙ эпохи.
// Эпоха 0 = сеть ещё не наблюдалась: честный отказ (инцидентов вне эпохи
// не бывает — факты старой эпохи недоступны, fail-closed).
func (a *App) GetIncidents() ([]IncidentCluster, error) {
	epoch, _, _ := netEpoch.current()
	if epoch == 0 {
		return nil, fmt.Errorf("эпоха сети не открыта — наблюдений отказов ещё не было (это честное «нет данных», не ошибка)")
	}
	return ClusterIncidents(chanHealth.failureEvents(epoch)), nil
}

// waveLogLine — строка журнала об актуальной волне (или "" — волн нет).
// Чистая функция: тестируется без App/журнала.
func waveLogLine(clusters []IncidentCluster) string {
	for _, cl := range clusters {
		if !cl.IsWave {
			continue
		}
		return fmt.Sprintf("инциденты: %d отказов каналов %v за %s—%s — похоже на одну волну сети, а не на %d независимых отказов (классы: %v)",
			cl.Count, cl.Channels, cl.Since, cl.Until, len(cl.Channels), cl.Classes)
	}
	return ""
}

// logWaveIfAny — одна строка журнала об актуальной волне (D3). Вызывается
// при неудавшемся переключении: владелец в журнале видит «все легли одной
// волной», а не серию отдельных отказов. Молчит, если волн нет.
func (a *App) logWaveIfAny() {
	if line := waveLogLine(ClusterIncidents(chanHealth.failureEvents(currentChannelEpoch()))); line != "" {
		a.appendLog("warn", line)
	}
}
