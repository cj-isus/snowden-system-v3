package main

// netepoch.go — NetworkEpoch (PLAN §6; plan_gpt §8): период между сменами
// NetworkFacts. Любое health/history-суждение о канале имеет смысл только
// ВНУТРИ эпохи: «канал A жив» без указания эпохи — не факт (одна и та же
// сеть в 10:00 и 14:00 может фильтровать по-разному; доказано V2-026:
// вчера pass — сегодня fail на том же Wi-Fi).
//
// Источник переходов — netwatch (V2-054): события up/change = смена
// return-пути = новая эпоха. down — НЕ смена эпохи (сеть отсутствует:
// состояние наблюдателя «вне сети», при возврате up начнётся новая).
//
// Требование AGENTS §3.4 (метрики не выдумывать): эпоха — счётчик событий
// наблюдателя, а не выдуманный идентификатор. Пока физический интерфейс
// не виден (TUN-режим, класс «неизвестен»), эпоха НЕ инкрементируется —
// честная граница V2-054a (happ-tun в дефолте = сигнала нет).
//
// Все функции чистые/потокобезопасные; тестируются таблицами (netepoch_test.go).

import (
	"fmt"
	"sync"
	"time"
)

// networkEpoch — монотонная эпоха сети процесса.
type networkEpoch struct {
	mu      sync.Mutex
	num     int       // 1.. — растёт на каждом up/change; 0 = ещё не было успешного факта
	started time.Time // момент начала текущей эпохи
	netCls  string    // класс сети на момент начала (mobile/wifi/ethernet/«»)
}

var netEpoch = &networkEpoch{}

// bump — зафиксировать смену сети. Только up/change открывают эпоху:
// down = «сети нет» (состояние наблюдателя «вне сети»), при возврате up
// начнётся новая эпоха. cls — класс сети на момент перехода (может быть
// «» = неизвестен, это честный факт). Возвращает номер новой эпохи или
// текущий, если событие эпоху не меняет (down).
func (e *networkEpoch) bump(kind, cls string, now time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if kind != "up" && kind != "change" {
		return e.num
	}
	e.num++
	e.started = now
	e.netCls = cls
	return e.num
}

// current — текущая эпоха: номер, момент начала, класс. Номер 0 = ни одного
// успешного факта сети за жизнь процесса (наблюдатель ещё не видел сеть).
func (e *networkEpoch) current() (int, time.Time, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.num, e.started, e.netCls
}

// label — человекочитаемый идентификатор для журнала/health: "E42" либо
// "E0" (факта сети ещё не было). Класс добавляется вызывающим при нужде.
func (e *networkEpoch) label() string {
	n, _, _ := e.current()
	return fmt.Sprintf("E%d", n)
}

// resetForTest — только тесты.
func (e *networkEpoch) resetForTest() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.num, e.started, e.netCls = 0, time.Time{}, ""
}

// onNetEvent — точка интеграции: зовётся из netwatch.fire для up/change.
// down не бампит (см. шапку). Заглушка в тестах не нужна: bump идемпотентен
// по числу вызовов и не ходит в сеть/ОС.
func onNetEvent(kind string, cls string, now time.Time) {
	switch kind {
	case "up", "change":
		n := netEpoch.bump(kind, cls, now)
		if globalAppForEpoch != nil {
			globalAppForEpoch.appendLog("info", "сеть: новая эпоха "+fmt.Sprintf("E%d", n)+" ("+classLabel(cls)+") — health-счётчики каналов переведены в свежую эпоху")
		}
	}
}

// globalAppForEpoch — ссылка на App для журнала эпох (избегаем цикла
// импорта netwatch→app через подменяемую точку; ставится в startup).
var globalAppForEpoch *App
