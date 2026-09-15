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
// V2-054 (wake): наблюдатель сети (netwatch.go) будит попытку НЕМЕДЛЕННО,
// когда сеть сменилась — ждать остаток backoff на старой сети бессмысленно
// (смена сети = смена return-пути = смена шансов). Попытка та же самая
// (startCore + probe), счётчик эпизода сохраняется, exactly-once через
// retry.done.

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
	timer   *time.Timer
	attempt int       // номер следующей авто-попытки эпизода (1..); 0 = нет эпизода
	due     time.Time // нулевое = ничего не запланировано
	done    bool      // попытка уже начата (begin); страховка от двойного запуска
}

// failClosedState — true, если текущее состояние = fail-closed без живого
// туннеля (обе фактические формы). Требует удержания a.mu.
func (a *App) failClosedState() bool {
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
	if !a.failClosedState() {
		return
	}
	a.stopRetryTimerLocked() // прошлый таймер не наследуется; счётчик эпизода растёт
	a.retry.attempt++
	a.retry.done = false // новый будильник — попытка ещё не начата
	d := retryDelay(a.retry.attempt)
	attempt := a.retry.attempt
	a.retry.due = now.Add(d)
	a.pushStateLocked()
	a.appendLogLocked("info", fmt.Sprintf("fail-closed: авто-повтор запуска №%d через %s (backoff; probe обязателен на каждой попытке, FR-001)", attempt, d))
	a.retry.timer = time.AfterFunc(d, func() { a.fireFailClosedRetry(attempt) })
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
	a.state.State = "starting"
	a.state.Error = ""
	a.state.BlockedReason = ""
	a.pushStateLocked()
	launch := a.retryLaunch
	if launch == nil {
		launch = func() { go a.startCore() }
	}
	return launch
}

// fireFailClosedRetry — сработавший таймер: честный переход в starting и
// обычный startCore (полный перебор каналов с probe-гейтом). attempt —
// токен-защита от устаревшего срабатывания: если планировщик уже переигрывали
// (cancel/новый schedule), старый таймер игнорируется. Счётчик эпизода
// СОХРАНЯЕТСЯ: следующий schedule (после провала startCore) возьмёт attempt+1
// — backoff растёт через весь эпизод, а не сбрасывается на каждый прогон.
func (a *App) fireFailClosedRetry(attempt int) {
	a.mu.Lock()
	if a.closing.Load() || attempt != a.retry.attempt {
		a.mu.Unlock()
		return
	}
	if !a.beginFailClosedRetryLocked() {
		a.mu.Unlock()
		return // попытка уже начата (например, wake от netwatch)
	}
	launch := a.beginRetryLaunchLocked()
	a.mu.Unlock()
	a.appendLog("info", "fail-closed: сеть могла восстановиться — авто-повтор запуска (полный перебор каналов, probe обязателен)")
	launch()
}

// wakeFailClosedRetry — сигнал наблюдателя сети (V2-054): сеть сменилась,
// ждать остаток backoff на старой сети бессмысленно — попытка немедленно.
// Та же семантика, что fire: счётчик эпизода сохраняется, attempt — обычный
// startCore с ПОЛНЫМ перебором каналов и обязательным probe (FR-001 не
// ослаблен; дебаунс наблюдателя не даёт шторму превратить это в самоддо).
// exactly-once: если таймер уже начал попытку — это no-op.
func (a *App) wakeFailClosedRetry() {
	a.mu.Lock()
	if a.closing.Load() || !a.failClosedState() || a.retry.timer == nil || a.retry.attempt == 0 {
		a.mu.Unlock()
		return
	}
	if !a.beginFailClosedRetryLocked() {
		a.mu.Unlock()
		return
	}
	launch := a.beginRetryLaunchLocked()
	a.mu.Unlock()
	a.appendLog("info", "fail-closed: сеть сменилась — авто-повтор вне очереди (полный перебор каналов, probe обязателен, FR-001)")
	launch()
}

// cancelFailClosedRetry — полный сброс: таймер, счётчик эпизода, расписание.
// Ручное действие (Start/Stop/закрытие окна) или успешный старт — новый
// эпизод начнётся с retryBaseDelay. State-поля чистятся сразу (прямой
// GetState после ручного действия не должен видеть протухшее расписание);
// последующий push проекцией подтвердит то же. Требует удержания a.mu.
func (a *App) cancelFailClosedRetryLocked() {
	a.stopRetryTimerLocked()
	a.retry.attempt = 0
	a.retry.due = time.Time{}
	a.retry.done = false
	a.state.NextRetryAt = ""
	a.state.RetryAttempt = 0
}

// stopRetryTimerLocked — остановить таймер (счётчик и due не трогает).
func (a *App) stopRetryTimerLocked() {
	if a.retry.timer != nil {
		a.retry.timer.Stop()
		a.retry.timer = nil
	}
}
