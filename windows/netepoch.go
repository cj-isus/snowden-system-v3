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
// Epoch publication is synchronized; lifecycle context cancellation is done
// after releasing mu. Existing historical tests do not prove current changes.

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// networkEpoch — монотонная эпоха сети процесса.
type networkEpoch struct {
	mu              sync.Mutex
	num             int       // 1.. — растёт на каждом up/change; 0 = ещё не было успешного факта
	started         time.Time // момент начала текущей эпохи
	netCls          string    // класс сети на момент начала (mobile/wifi/ethernet/«»)
	operationSerial uint64
	operationCancel context.CancelFunc    // one serialized App lifecycle operation
	path            *networkPathSource    // original process-local continuity source, never a physical-path attestation
	pathIdentity    uint64                // presentation identity of constructed sources only; guarded by mu, saturates
	controlSource   *core.OperationSource // latest actual event, including down; DNS-control denial only
	knownDown       bool
	// Native power callback uses only these atomics, never this mutex.
	powerBlocked atomic.Bool
	pathValue    atomic.Pointer[networkPathSource]
}

// Revoked before cancellation callbacks run. A source is never made current
// again: a later observed up/change gets a different original pointer. Initial
// unknown continuity allows preparation, without manufacturing a network epoch.
type networkPathSource struct {
	current  atomic.Bool
	identity uint64 // immutable, private UI correlation only; never replaces pointer checks
}

// Epoch.mu held. This is not the observed network epoch counter. Zero is
// unknown/exhausted and cannot revive an older identifier on wrap.
func (e *networkEpoch) newPathSourceLocked(available bool) *networkPathSource {
	source := &networkPathSource{}
	if e.pathIdentity != ^uint64(0) {
		e.pathIdentity++
		source.identity = e.pathIdentity
	}
	source.current.Store(available && source.identity != 0)
	return source
}

type networkPathContextKey struct{}

// Returned directly from the actual event commit, never reconstructed from
// whatever happens to be current after log/event callbacks have returned.
type networkControlEvent struct {
	kind   string
	path   *networkPathSource
	source *core.OperationSource
}

func (e *networkEpoch) pathSourceLocked() *networkPathSource {
	if e.path == nil {
		e.path = e.newPathSourceLocked(!e.knownDown && !e.powerBlocked.Load())
		e.pathValue.Store(e.path)
		if e.powerBlocked.Load() {
			e.path.current.Store(false)
		}
	}
	return e.path
}

func (e *networkEpoch) pathSource() *networkPathSource {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pathSourceLocked()
}

func (e *networkEpoch) scopeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	if source, ok := ctx.Value(networkPathContextKey{}).(*networkPathSource); ok && source != nil {
		return ctx // a stale operation cannot silently adopt the replacement path
	}
	return context.WithValue(ctx, networkPathContextKey{}, e.pathSource())
}

var netEpoch = &networkEpoch{}

// Called under the original poller's short lease, not epoch/App/lifecycle
// locks. This is a process-local negative fence, not synchronous WFP revocation.
func (e *networkEpoch) revokePowerContinuityValue() {
	e.powerBlocked.Store(true)
	if path := e.pathValue.Load(); path != nil {
		path.current.Store(false)
	}
}

func (e *networkEpoch) cancelPowerOperation() {
	e.mu.Lock()
	e.controlSource.Revoke() // existing poller stack, never the native callback
	cancel := e.operationCancel
	e.operationCancel = nil
	e.operationSerial++
	e.mu.Unlock()
	if cancel != nil {
		cancel() // existing poller stack, never the native callback
	}
}

// Registration has succeeded on this original new producer. Caller has already
// confirmed actual engine/helper/native predecessor retirement on replacement,
// or an isolated observer-only scope with no engine and actual helper/native
// retirement. Neither path grants engine admission from an empty runtime.
// Initial unknown continuity permits preparation, never physical-path proof.
func (e *networkEpoch) enterPowerObservedPoller(owner *networkPollerOwner) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.stopping || owner.ctx.Err() != nil || owner.fault != nil || owner.powerRetirement {
		return false
	}
	if e.powerBlocked.Load() {
		e.path = e.newPathSourceLocked(!e.knownDown) // never reactivate an original revoked source
		e.pathValue.Store(e.path)
		e.powerBlocked.Store(false)
	}
	owner.running = true
	return true
}

// bump — зафиксировать смену сети. Только up/change открывают эпоху:
// down = «сети нет» (состояние наблюдателя «вне сети»), при возврате up
// начнётся новая эпоха. cls — класс сети на момент перехода (может быть
// «» = неизвестен, это честный факт). Возвращает номер новой эпохи или
// текущий, если событие эпоху не меняет (down).
func (e *networkEpoch) bump(kind, cls string, now time.Time) int {
	number, _ := e.bumpEvent(kind, cls, now)
	return number
}

func (e *networkEpoch) bumpEvent(kind, cls string, now time.Time) (int, networkControlEvent) {
	e.mu.Lock()
	if kind != "up" && kind != "change" && kind != "down" {
		number := e.num
		e.mu.Unlock()
		return number, networkControlEvent{}
	}
	e.controlSource.Revoke()
	e.controlSource = core.NewOperationSource()
	e.pathSourceLocked().current.Store(false) // negative fence before cancel can be delivered
	e.knownDown = kind == "down"
	if !e.knownDown {
		e.num++
		e.started = now
		e.netCls = cls
		e.path = e.newPathSourceLocked(!e.powerBlocked.Load())
		e.pathValue.Store(e.path)
		if e.powerBlocked.Load() {
			e.path.current.Store(false) // close a callback racing the mirror publication
		}
	}
	number := e.num
	event := networkControlEvent{kind: kind, path: e.path, source: e.controlSource}
	cancel := e.operationCancel
	e.operationCancel = nil
	e.operationSerial++ // an old release cannot unregister a newer binding
	e.mu.Unlock()
	if cancel != nil {
		cancel() // cancellation work never runs under the epoch lock
	}
	return number, event
}

// App.operationMu serializes callers. Network change cancels the work that
// prepared an action on the old route, including queued switch/probe gates.
// A successful engine keeps its own lifetime; ending this context does not
// retire a healthy committed worker. No goroutine or subscription map is needed.
func (e *networkEpoch) bindOperation(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	e.mu.Lock()
	source := e.pathSourceLocked()
	ctx = context.WithValue(ctx, networkPathContextKey{}, source)
	previous := e.operationCancel
	e.operationSerial++
	serial := e.operationSerial
	e.operationCancel = cancel
	e.mu.Unlock()
	if previous != nil {
		previous()
	}
	if !source.current.Load() {
		cancel()
	}
	return ctx, func() {
		cancel()
		e.mu.Lock()
		if e.operationSerial == serial {
			e.operationCancel = nil
		}
		e.mu.Unlock()
	}
}

// current — текущая эпоха: номер, момент начала, класс. Номер 0 = ни одного
// успешного факта сети за жизнь процесса (наблюдатель ещё не видел сеть).
func (e *networkEpoch) current() (int, time.Time, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.num, e.started, e.netCls
}

// Publish only to the epoch that actually produced the observation. The
// callback is short in-memory work; never call current/bump or perform I/O in it.
// Lock order for health publication: networkEpoch.mu -> channelHealth.mu.
func (e *networkEpoch) whileCurrent(number int, publish func(string)) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.powerBlocked.Load() || e.knownDown || number <= 0 || number != e.num {
		return false
	}
	publish(e.netCls)
	return true
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

// onNetEvent — точка интеграции: зовётся из netwatch.fire.
// down отзывает path/operation, сохраняя health epoch. Только up/change
// увеличивают номер; обработка не выполняет сетевой или системный I/O.
func onNetEvent(kind string, cls string, now time.Time) networkControlEvent {
	var event networkControlEvent
	switch kind {
	case "down":
		_, event = netEpoch.bumpEvent(kind, cls, now)
	case "up", "change":
		var n int
		n, event = netEpoch.bumpEvent(kind, cls, now)
		if globalAppForEpoch != nil {
			globalAppForEpoch.appendLog("info", "сеть: новая эпоха "+fmt.Sprintf("E%d", n)+" ("+classLabel(cls)+") — health-счётчики каналов переведены в свежую эпоху")
		}
	}
	return event
}

// globalAppForEpoch — ссылка на App для журнала эпох (избегаем цикла
// импорта netwatch→app через подменяемую точку; ставится в startup).
var globalAppForEpoch *App
