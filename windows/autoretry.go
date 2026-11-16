package main

// Авто-ретрай после fail-closed (V2-053).
//
// Проблема: после исчерпания failover-on-start или сторожа приложение остаётся
// в error/BLOCKED, пока владелец не нажмёт «Подключить». Волны деградации
// (V2-026/039, §2.4a) длятся минуты-часы: сеть может восстановиться, когда
// никто не смотрит в окно — VPN будет стоять до ручного клика.
//
// Решение: при входе в fail-closed планируется АВТОМАТИЧЕСКАЯ попытка запуска
// с растущей паузой (backoff): 30с → 1м → 2м → 4м → 8м → далее каждые 10м.
// Каждая попытка — обычный startCore с ПОЛНЫМ перебором validated каналов и
// обязательным protected probe: ослабления FR-001 нет (probe-гейт на каждом
// канале), «самоддо» нет (пауза растёт, сам probe стоит ~минуту — нагрузка
// мала). Время следующей попытки публикуется в state (NextRetryAt) —
// Dashboard-баннер показывает честный отсчёт.
//
// Жизненный цикл счётчика эпизода: attempt растёт через ВСЕ авто-попытки
// одного эпизода fail-closed (fire сохраняет счётчик, failCore планирует
// следующую с удвоенной паузой) и сбрасывается при УСПЕШНОМ старте или любом
// ручном действии (Start/Stop/закрытие окна) — новый эпизод начинается с 30с.
// Гонок «двойного запуска» нет: startCore сам guarded через startBusy, а
// будильники (таймер и netwatch, V2-054) сериализованы begin-гвардом.
//
// Network wake использует до двух внеочередных запросов на fail-closed эпизод,
// не более одного на original path source. После лимита остаётся backoff.
// Каждый запрос — новый startCore + probe, без переноса старого ctx/report;
// serial/intent/done защищают от stale timer и гонки wake с callback.

import (
	"fmt"
	"time"
)

const (
	// retryBaseDelay — пауза перед первой авто-попыткой. Волна деградации
	// редко отпускает раньше минуты (V2-045: даже повторная серия внутри
	// startCore ждёт 20с), поэтому первая авто-попытка не раньше 30с.
	retryBaseDelay = 30 * time.Second
	// retryMaxDelay — потолок backoff: волна может длиться часами, но
	// бесконечно растущая пауза бессмысленна; 10м — компромисс «заметить
	// восстановление» vs «не дёргать сеть поминутно».
	retryMaxDelay = 10 * time.Minute
)

// retryTimer — планировщик авто-повтора. Все поля под a.mu (мутируются
// только в schedule/cancel/fire/wake, которые берут a.mu сами). Нулевое
// значение готово к использованию.
type retryTimer struct {
	timer         *time.Timer
	attempt       int       // номер следующей авто-попытки эпизода (1..); 0 = нет эпизода
	due           time.Time // нулевое = ничего не запланировано
	done          bool      // попытка уже начата (begin); страховка от двойного запуска
	serial        uint64    // callback identity, never reset/reused after manual actions
	intent        uint64    // exact operation that registered this timer
	networkIntent uint64
	networkBursts int
	networkPath   *networkPathSource
}

// failClosedState — true, если текущее состояние = fail-closed без живого
// туннеля (обе фактические формы). Требует удержания a.mu.
func (a *App) failClosedState() bool {
	if a.ctx == nil || a.ctx.Err() != nil || a.profileTransition || a.trustBlocked || a.proxyConflict || a.restartCommit != nil || a.releaseCommit != nil || a.installedImageChanged || a.state.BlockedReason == "cleanup_unconfirmed" || a.state.BlockedReason == "restart_failed" {
		return false // no old-profile retry or network wakeup over unresolved local custody
	}
	if a.state.State == "error" {
		return true
	}
	return a.state.State == "stopped" && a.state.BlockedReason == "all_channels_failed"
}

// retryDelay — пауза для попытки n (1-based): 30с, 1м, 2м, 4м, 8м, 10м, 10м…
func retryDelay(attempt int) time.Duration {
	d := retryBaseDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= retryMaxDelay {
			return retryMaxDelay
		}
	}
	return d
}

// scheduleFailClosedRetry — запланировать следующую авто-попытку, если мы в
// fail-closed. Вызывается ПОСЛЕ фиксации error/BLOCKED (failCore, сторож).
// Требует удержания a.mu (все вызывающие уже держат мьютекс). Проекцию
// NextRetryAt/RetryAttempt в state делает pushStateLocked.
func (a *App) scheduleFailClosedRetryLocked(now time.Time) {
	if a.trustBlocked || !a.protectionDesired || a.closing.Load() || !a.failClosedState() {
		return
	}
	a.retry.attempt++
	d := retryDelay(a.retry.attempt)
	attempt := a.retry.attempt
	if !a.armFailClosedRetryLocked(now, d, nil) {
		a.pushStateLocked()
		return
	}
	a.pushStateLocked()
	a.appendLogLocked("info", fmt.Sprintf("fail-closed: авто-повтор запуска №%d через %s (backoff; probe обязателен на каждой попытке, FR-001)", attempt, d))
}

// a.mu is held. Timer.Stop is not a join; serial+intent revoke a callback
// that already entered or is waiting for this mutex. Fast path is value-only.
func (a *App) armFailClosedRetryLocked(now time.Time, delay time.Duration, path *networkPathSource) bool {
	a.stopRetryTimerLocked()
	if a.retry.serial == ^uint64(0) {
		a.retry.due, a.retry.done = time.Time{}, true
		return false // saturate instead of reusing a callback identity
	}
	a.retry.intent = a.operationID
	a.retry.done = false
	a.retry.due = now.Add(delay)
	attempt, serial, intent := a.retry.attempt, a.retry.serial, a.retry.intent
	a.retry.timer = time.AfterFunc(delay, func() { a.fireFailClosedRetryForSchedule(attempt, serial, intent, path) })
	return true
}

// beginFailClosedRetryLocked — взять право на попытку ровно один раз:
// таймер/наблюдатель (V2-054), сработавшие одновременно, дают одну попытку.
// false = попытка уже начата другим будильником. Требует удержания a.mu.
func (a *App) beginFailClosedRetryLocked() bool {
	if a.retry.done {
		return false
	}
	a.retry.done = true
	a.stopRetryTimerLocked()
	a.retry.due = time.Time{} // state-проекция очистит NextRetryAt
	return true
}

// beginRetryLaunchLocked — общая точка после begin: перевод state в starting
// и выбор точки запуска (тесты подменяют retryLaunch). Требует a.mu.
func (a *App) beginRetryLaunchLocked() func() {
	previous := a.operationID
	if err := a.advanceOperationLocked(); err != nil {
		return nil
	}
	id := a.operationID // capture before unlocking: delayed launch cannot adopt a newer Start
	if a.retry.networkIntent != previous {
		a.retry.networkBursts, a.retry.networkPath = 0, nil
	}
	a.retry.networkIntent = id // carry the bounded episode across automatic IDs
	a.state.State = "starting"
	a.state.Error = ""
	a.state.BlockedReason = ""
	a.pushStateLocked()
	launch := a.retryLaunch
	if launch == nil {
		launch = func() { go a.startCoreGeneration(id) }
	}
	return launch
}

// Keep the existing private current-schedule entry point. Actual asynchronous
// callbacks must carry the registration they captured, never this late snapshot.
func (a *App) fireFailClosedRetry(attempt int) {
	a.mu.Lock()
	serial, intent := a.retry.serial, a.retry.intent
	a.mu.Unlock()
	a.fireFailClosedRetryForSchedule(attempt, serial, intent, nil)
}

// fireFailClosedRetryForSchedule — сработавший таймер: честный переход в starting и
// обычный startCore (полный перебор каналов с probe-гейтом). Exact serial/intent
// не дают старому callback совпасть с повторно использованным attempt. Счётчик эпизода
// СОХРАНЯЕТСЯ: следующий schedule (после провала startCore) возьмёт attempt+1
// — backoff растёт через весь эпизод, а не сбрасывается на каждый прогон.
func (a *App) fireFailClosedRetryForSchedule(attempt int, serial, intent uint64, path *networkPathSource) {
	a.mu.Lock()
	if a.retry.timer == nil || attempt != a.retry.attempt || serial != a.retry.serial || intent != a.retry.intent || intent != a.operationID {
		a.mu.Unlock()
		return
	}
	if a.trustBlocked || a.closing.Load() || !a.protectionDesired || !a.failClosedState() {
		a.cancelFailClosedRetryLocked() // only this exact callback owns the obsolete schedule
		a.pushStateLocked()
		a.mu.Unlock()
		return
	}
	if path != nil && !path.current.Load() {
		a.armFailClosedRetryLocked(time.Now(), retryDelay(attempt), nil)
		a.pushStateLocked()
		a.mu.Unlock()
		return // this fast-path source was revoked; keep ordinary backoff
	}
	if !a.beginFailClosedRetryLocked() {
		a.mu.Unlock()
		return // попытка уже начата (например, wake от netwatch)
	}
	launch := a.beginRetryLaunchLocked()
	a.mu.Unlock()
	if launch == nil {
		return
	}
	a.appendLog("info", "fail-closed: сеть могла восстановиться — авто-повтор запуска (полный перебор каналов, probe обязателен)")
	launch()
}

// wakeFailClosedRetry — сигнал наблюдателя сети: новый source может ускорить
// попытку в пределах общего network budget; после лимита backoff сохраняется.
// Та же семантика, что fire: счётчик эпизода сохраняется, attempt — обычный
// startCore с ПОЛНЫМ перебором каналов и обязательным probe (FR-001 не
// ослаблен; дебаунс наблюдателя не даёт шторму превратить это в самоддо).
// exactly-once: если таймер уже начал попытку — это no-op.
func (a *App) wakeFailClosedRetry() {
	a.wakeFailClosedRetryFor(nil)
}

func (a *App) wakeFailClosedRetryFor(change *serviceDNSChange) {
	if change != nil {
		runtime_.mu.Lock()
		if runtime_.cc != nil || runtime_.startBusy {
			runtime_.mu.Unlock()
			return // actor-less event cannot act on a later Start/session
		}
		a.mu.Lock()
		runtime_.mu.Unlock()
		if change.kind == "down" || !change.intentCurrentLocked(a) || !change.current() {
			a.mu.Unlock()
			return
		}
	} else {
		a.mu.Lock()
	}
	if a.trustBlocked || a.closing.Load() || !a.protectionDesired || !a.failClosedState() || a.retry.timer == nil || a.retry.attempt == 0 {
		a.mu.Unlock()
		return
	}
	path := netEpoch.pathSource()
	if change != nil {
		path = change.path
	}
	if a.retry.intent != a.operationID || !a.takeNetworkRetryBurstLocked(path) {
		a.mu.Unlock()
		return // retain backoff after two fast attempts or a repeated source
	}
	if !a.beginFailClosedRetryLocked() {
		a.mu.Unlock()
		return
	}
	launch := a.beginRetryLaunchLocked()
	a.mu.Unlock()
	if launch == nil {
		return
	}
	a.appendLog("info", "fail-closed: сеть сменилась — авто-повтор вне очереди (полный перебор каналов, probe обязателен, FR-001)")
	launch()
}

// cancelFailClosedRetry — полный сброс: таймер, счётчик эпизода, расписание.
// Ручное действие (Start/Stop/закрытие окна) или успешный старт — новый
// эпизод начнётся с retryBaseDelay. State-поля чистятся сразу (прямой
// GetState после ручного действия не должен видеть протухшее расписание);
// последующий push проекцией подтвердит то же. Требует удержания a.mu.
func (a *App) cancelFailClosedRetryLocked() {
	a.cancelPinActivationWakeLocked()
	a.stopRetryTimerLocked()
	a.retry.attempt = 0
	a.retry.due = time.Time{}
	a.retry.done = false
	a.retry.intent = 0
	a.retry.networkIntent, a.retry.networkBursts, a.retry.networkPath = 0, 0, nil
	a.state.NextRetryAt = ""
	a.state.RetryAttempt = 0
}

// Stop requests timer cancellation and revokes callback serial/intent;
// attempt/due are left to the caller. It never claims the callback has joined.
func (a *App) stopRetryTimerLocked() {
	if a.retry.timer != nil {
		a.retry.timer.Stop()
		a.retry.timer = nil
	}
	a.retry.intent = 0
	if a.retry.serial != ^uint64(0) {
		a.retry.serial++
	}
}
