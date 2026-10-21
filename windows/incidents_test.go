package main

// Тесты D3 (V2-065, PLAN Phase D3; plan_gpt «D3 — Incident clustering:
// A + B + C failure → one incident»). Чистые, без сети: кольцо отказов
// наполняется напрямую через observeSrc/observeBg. Проверяются: двойная
// шкала окна (D3Window между соседями, D3Coalesce до центра), консервативная
// семантика (сомнительные случаи = отдельные инциденты), детерминизм,
// честность (1 событие = 1 инцидент), смена эпохи хоронит кольцо,
// волна-строка журнала.

import (
	"reflect"
	"testing"
	"time"
)

// d3Events — синтетическое кольцо: id + смещения в секундах от базы.
func d3Events(base time.Time, spec map[string][]float64) []failEvent {
	var out []failEvent
	for id, offsets := range spec {
		for _, off := range offsets {
			out = append(out, failEvent{ID: id, At: base.Add(time.Duration(off * float64(time.Second))), Class: fcUnknown, Src: srcProbe})
		}
	}
	return out
}

// TestD3SingleEventHonest — одно событие = один инцидент, IsWave=false
// («волна» не рисуется из одного факта, AGENTS §3.6).
func TestD3SingleEventHonest(t *testing.T) {
	base := time.Now()
	clusters := ClusterIncidents(d3Events(base, map[string][]float64{"a": {0}}))
	if len(clusters) != 1 {
		t.Fatalf("один кластер, получено %d", len(clusters))
	}
	if clusters[0].IsWave || clusters[0].Count != 1 || len(clusters[0].Channels) != 1 {
		t.Fatalf("одиночный факт не волна: %+v", clusters[0])
	}
}

// TestD3ThreeChannelsOneWave — канонический случай plan_gpt: A+B+C упали
// вместе → ОДИН инцидент с тремя каналами.
func TestD3ThreeChannelsOneWave(t *testing.T) {
	base := time.Now()
	events := d3Events(base, map[string][]float64{
		"a": {0}, "b": {8}, "c": {16},
	})
	clusters := ClusterIncidents(events)
	if len(clusters) != 1 {
		t.Fatalf("A+B+C за 16с = одна волна, получено кластеров: %d (%+v)", len(clusters), clusters)
	}
	cl := clusters[0]
	if cl.Count != 3 || len(cl.Channels) != 3 || !cl.IsWave {
		t.Fatalf("волна из трёх каналов: %+v", cl)
	}
	if cl.Channels[0] != "a" || cl.Until != base.Add(16*time.Second).Format(time.RFC3339) {
		t.Fatalf("порядок/границы волны: %+v", cl)
	}
	if cl.ClusterID == "" {
		t.Fatal("детерминированный id волны обязателен")
	}
}

// TestD3DistantEventsSeparate — отказы через большой промежуток = РАЗДЕЛЬНЫЕ
// инциденты (пересклеить независимые отказы хуже, чем показать два).
func TestD3DistantEventsSeparate(t *testing.T) {
	base := time.Now()
	events := d3Events(base, map[string][]float64{
		"a": {0}, "b": {600}, // 10 минут между отказами
	})
	clusters := ClusterIncidents(events)
	if len(clusters) != 2 {
		t.Fatalf("события через 10м = два инцидента, получено %d", len(clusters))
	}
	for _, cl := range clusters {
		if cl.IsWave {
			t.Fatalf("одиночные отказы не должны помечаться волной: %+v", cl)
		}
	}
	// Новейшие первыми: b (600с) сверху.
	if clusters[0].Channels[0] != "b" {
		t.Fatalf("новейший инцидент должен быть первым: %+v", clusters[0])
	}
}

// TestD3WindowChainNotStretched — цепочка отказов с равными паузами:
// окно между соседями (90с) захватывает, но центр волны (среднее) уезжает,
// и далёкое событие откалывается — двойная шкала РАЗРЫВАЕТ растянутую цепь
// (консервативная семантика: волна не растягивается на весь эпизод).
func TestD3WindowChainNotStretched(t *testing.T) {
	base := time.Now()
	// 0с, 40с, 80с: одна волна (центр 20-40с, отклонения ≤60с); 120с отстоит
	// от центра первых трёх (40с) на 80с > D3Coalesce → разрыв.
	events := d3Events(base, map[string][]float64{"a": {0, 40, 80, 120}})
	clusters := ClusterIncidents(events)
	if len(clusters) != 2 {
		t.Fatalf("двойная шкала обязана разорвать цепь: %d кластеров (%+v)", len(clusters), clusters)
	}
	if clusters[0].Count != 1 || clusters[1].Count != 3 {
		t.Fatalf("отколотый хвост отдельно, волна из трёх целая: %+v", clusters)
	}
}

// TestD3MixedSourcesSameWave — сильные (probe) и слабые (background) отказы
// одного канала за окном склеиваются в одну волну (источник не разделяет
// волны: событие подтверждено в обоих случаях, различие — в силе факта D2).
func TestD3MixedSourcesSameWave(t *testing.T) {
	base := time.Now()
	events := []failEvent{
		{ID: "ch", At: base, Class: fcPortBlackholed, Src: srcBackground},
		{ID: "ch", At: base.Add(10 * time.Second), Class: fcResetInjected, Src: srcProbe},
	}
	clusters := ClusterIncidents(events)
	if len(clusters) != 1 || !clusters[0].IsWave {
		t.Fatalf("probe+bg отказы одного канала за 10с = одна волна: %+v", clusters)
	}
	if len(clusters[0].Classes) != 2 {
		t.Fatalf("оба класса должны быть в волне: %+v", clusters[0].Classes)
	}
}

// TestD3Deterministic — тот же набор фактов (в другом порядке прихода) даёт
// тот же результат (детерминизм policy-модели, plan_gpt §6).
func TestD3Deterministic(t *testing.T) {
	base := time.Now()
	events := d3Events(base, map[string][]float64{"a": {0}, "b": {10}, "c": {25}, "d": {300}})
	shuffled := []failEvent{events[2], events[0], events[3], events[1]}
	c1 := ClusterIncidents(events)
	c2 := ClusterIncidents(shuffled)
	if !reflect.DeepEqual(c1, c2) {
		t.Fatalf("кластеризация недетерминирована: %+v vs %+v", c1, c2)
	}
}

// TestD3RingAndEpoch — кольцо хоронится сменой эпохи (resetLocked), усекается
// до incidentRingCap (новейшие остаются), failureEvents fail-closed по эпохе.
func TestD3RingAndEpoch(t *testing.T) {
	bgResetForTest()
	netEpoch.resetForTest()
	t.Cleanup(func() { bgResetForTest(); netEpoch.resetForTest() })

	e := netEpoch.bump("up", "wifi", time.Now())
	now := time.Now()
	chanHealth.observeBg(e, "ch1", fcPortBlackholed, now)
	chanHealth.observeBg(e, "ch1", fcPortBlackholed, now.Add(time.Second))
	if got := chanHealth.failureEvents(e); len(got) != 2 {
		t.Fatalf("кольцо: %d событий, want 2", len(got))
	}
	// Новая эпоха: чтение ТЕКУЩЕЙ эпохи честно пусто (реестр ещё на старой).
	e2 := netEpoch.bump("change", "mobile", now.Add(2*time.Second))
	if got := chanHealth.failureEvents(e2); len(got) != 0 {
		t.Fatalf("новая эпоха = чистое кольцо, получено %d", len(got))
	}
	// Первая запись новой эпохи переоткрывает реестр (lazy reset, семантика
	// observeResult V2-057): старое кольцо хоронится, старая эпоха недоступна.
	chanHealth.observeBg(e2, "ch2", fcPortBlackholed, now.Add(2*time.Second))
	if got := chanHealth.failureEvents(e); len(got) != 0 {
		t.Fatal("факты старой эпохи недоступны после переоткрытия (fail-closed)")
	}
}

// TestD3WaveLogLine — строка журнала берёт ПЕРВУЮ актуальную волну и молчит
// без волн (одиночные инциденты волновую строку не дают).
func TestD3WaveLogLine(t *testing.T) {
	base := time.Now()
	if line := waveLogLine(ClusterIncidents(d3Events(base, map[string][]float64{"a": {0}}))); line != "" {
		t.Fatalf("без волны строка пуста: %q", line)
	}
	line := waveLogLine(ClusterIncidents(d3Events(base, map[string][]float64{"a": {0}, "b": {5}, "c": {12}})))
	if line == "" {
		t.Fatal("волна обязана дать строку журнала")
	}
}

// TestD3BindingEpochZero — биндинг при эпохе 0 честно отказывается (не пустая
// картинка — «нет данных»), при живой эпохе отдаёт кластеры.
func TestD3BindingEpochZero(t *testing.T) {
	bgResetForTest()
	netEpoch.resetForTest()
	t.Cleanup(func() { bgResetForTest(); netEpoch.resetForTest() })

	a := NewApp()
	if _, err := a.GetIncidents(); err == nil {
		t.Fatal("эпоха 0 обязана дать честный отказ биндинга")
	}
	e := netEpoch.bump("up", "wifi", time.Now())
	now := time.Now()
	chanHealth.observeBg(e, "ch-x", fcServerUnreach, now)
	chanHealth.observeBg(e, "ch-y", fcServerUnreach, now.Add(5*time.Second))
	clusters, err := a.GetIncidents()
	if err != nil {
		t.Fatalf("живая эпоха: %v", err)
	}
	if len(clusters) != 1 || !clusters[0].IsWave || len(clusters[0].Channels) != 2 {
		t.Fatalf("биндинг: %+v", clusters)
	}
}
