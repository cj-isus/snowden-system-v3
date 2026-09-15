package main

// Тесты планировщика авто-повтора после fail-closed (V2-053).
//
// Проверяются фактические контракты:
//  - retryDelay: рост 30с→1м→2м→4м→8м→10м и потолок;
//  - scheduleFailClosedRetryLocked: планирование ТОЛЬКО в fail-closed,
//    публикация NextRetryAt/RetryAttempt в state, рост attempt внутри
//    эпизода (fire не сбрасывает счётчик — backoff растёт через попытки);
//  - отмена: Start/Stop-хуки (cancelFailClosedRetryLocked) сбрасывают
//    эпизод; просроченный таймер (cancel после schedule) не стреляет;
//  - pushStateLocked-проекция: пустые поля при attempt==0 и валидный
//    RFC3339 при активном расписании (в т.ч. after fire: due==zero).

import (
	"testing"
	"time"
)

func TestRetryDelayGrowth(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 30 * time.Second},
		{2, time.Minute},
		{3, 2 * time.Minute},
		{4, 4 * time.Minute},
		{5, 8 * time.Minute},
		{6, 10 * time.Minute}, // потолок
		{15, 10 * time.Minute},
	}
	for _, c := range cases {
		if got := retryDelay(c.attempt); got != c.want {
			t.Fatalf("retryDelay(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

// failClosedApp — App в форме №1 (error после исчерпания failover-on-start).
func failClosedApp() *App {
	a := &App{}
	a.state.State = "error"
	a.state.Error = "запуск: все каналы не прошли probe — fail-closed (FR-001)"
	return a
}

func TestScheduleOnlyInFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		state   string
		blocked string
		want    bool
	}{
		{"error", "error", "", true},
		{"watchdog blocked", "stopped", "all_channels_failed", true},
		{"plain stopped", "stopped", "", false},
		{"running", "running", "", false},
		{"starting", "starting", "", false},
	}
	for _, c := range cases {
		a := &App{}
		a.state.State = c.state
		a.state.BlockedReason = c.blocked
		a.scheduleFailClosedRetryLocked(time.Now())
		if got := a.retry.timer != nil; got != c.want {
			t.Fatalf("%s: scheduled = %v, want %v", c.name, got, c.want)
		}
		if c.want && a.retry.attempt != 1 {
			t.Fatalf("%s: attempt = %d, want 1", c.name, a.retry.attempt)
		}
		// Не оставить живой таймер в другие подтесты.
		a.mu.Lock()
		a.cancelFailClosedRetryLocked()
		a.mu.Unlock()
	}
}

func TestSchedulePublishesState(t *testing.T) {
	a := failClosedApp()
	a.mu.Lock()
	before := time.Now()
	a.scheduleFailClosedRetryLocked(before)
	after := time.Now()
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()

	if a.state.RetryAttempt != 1 {
		t.Fatalf("RetryAttempt = %d, want 1", a.state.RetryAttempt)
	}
	if a.state.NextRetryAt == "" {
		t.Fatal("NextRetryAt пуст — UI не увидит расписание")
	}
	due, err := time.Parse(time.RFC3339, a.state.NextRetryAt)
	if err != nil {
		t.Fatalf("NextRetryAt не RFC3339: %v", err)
	}
	if due.Before(before.Add(retryBaseDelay-time.Second)) || due.After(after.Add(retryBaseDelay+time.Second)) {
		t.Fatalf("due = %v, ожидался ~%v (±1с)", due, before.Add(retryBaseDelay))
	}
}

// Ключевой инвариант эпизода: провал авто-попытки (fire → startCore → failCore
// → schedule) должен ПРИРАЩАТЬ счётчик, а не сбрасывать на 1 — иначе backoff
// навсегда застрял бы на 30с и авто-ретрай превратился бы в самоддо.
func TestBackoffPersistsAcrossAutoAttempts(t *testing.T) {
	a := failClosedApp()
	launched := make(chan struct{}, 4)
	a.retryLaunch = func() { launched <- struct{}{} } // без реальной сети
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now()) // попытка 1 → 30с
	if a.retry.attempt != 1 {
		t.Fatalf("первое планирование: attempt = %d, want 1", a.retry.attempt)
	}
	a.mu.Unlock()
	// fire НЕ сбрасывает счётчик (только таймер и due) и запускает попытку
	// через подменённую точку запуска.
	a.fireFailClosedRetry(1)
	select {
	case <-launched:
	case <-time.After(time.Second):
		t.Fatal("fire не запустил попытку")
	}
	a.mu.Lock()
	still := a.retry.attempt
	a.mu.Unlock()
	if still != 1 {
		t.Fatalf("после fire attempt = %d, want 1 (счётчик эпизода живёт)", still)
	}
	// Провал авто-попытки → failCore → schedule: попытка 2 с удвоенной паузой.
	a.mu.Lock()
	a.state.State = "error"
	a.scheduleFailClosedRetryLocked(time.Now())
	want := retryDelay(2)
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()
	if a.retry.attempt != 2 {
		t.Fatalf("после провала attempt = %d, want 2", a.retry.attempt)
	}
	due, _ := time.Parse(time.RFC3339, a.state.NextRetryAt)
	if got := time.Until(due); got < want-(5*time.Second) || got > want+5*time.Second {
		t.Fatalf("пауза второй попытки = %v, want ~%v", got, want)
	}
}

func TestManualActionCancelsEpisode(t *testing.T) {
	a := failClosedApp()
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	a.mu.Lock()
	a.cancelFailClosedRetryLocked() // как Start/Stop/OnBeforeClose
	a.mu.Unlock()
	if a.retry.timer != nil || a.retry.attempt != 0 || !a.retry.due.IsZero() {
		t.Fatal("cancel не сбросил планировщик полностью")
	}
	if a.state.NextRetryAt != "" || a.state.RetryAttempt != 0 {
		t.Fatalf("state не очищен: nextRetryAt=%q attempt=%d", a.state.NextRetryAt, a.state.RetryAttempt)
	}
	// Новый эпизод после отмены начинается с 30с, а не продолжает рост.
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()
	if a.retry.attempt != 1 {
		t.Fatalf("новый эпизод: attempt = %d, want 1", a.retry.attempt)
	}
}

// Отменённый таймер не должен стрелять (гонка cancel ↔ fire).
func TestCancelledTimerDoesNotFire(t *testing.T) {
	a := failClosedApp()
	a.retryLaunch = func() {}
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	a.mu.Lock()
	a.cancelFailClosedRetryLocked()
	a.mu.Unlock()
	// Имитация просроченного срабатывания отменённого таймера.
	a.fireFailClosedRetry(1)
	a.mu.Lock()
	fired := a.state.State == "starting"
	a.mu.Unlock()
	if fired {
		t.Fatal("просроченный таймер перевёл state в starting после cancel")
	}
}

func TestPushStateProjectsRetryFields(t *testing.T) {
	a := &App{}
	a.pushStateLocked() // attempt==0 → поля пустые
	if a.state.NextRetryAt != "" || a.state.RetryAttempt != 0 {
		t.Fatalf("attempt==0: state не пуст: %q/%d", a.state.NextRetryAt, a.state.RetryAttempt)
	}
	a.state.State = "error"
	a.mu.Lock()
	a.scheduleFailClosedRetryLocked(time.Now())
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.cancelFailClosedRetryLocked(); a.mu.Unlock() }()
	if a.state.NextRetryAt == "" || a.state.RetryAttempt != 1 {
		t.Fatalf("после schedule state не заполнен: %q/%d", a.state.NextRetryAt, a.state.RetryAttempt)
	}
	// Проецирование на произвольном push (не только при schedule).
	a.mu.Lock()
	a.state.NextRetryAt = "" // испортить — push обязан восстановить
	a.pushStateLocked()
	a.mu.Unlock()
	if a.state.NextRetryAt == "" {
		t.Fatal("pushState не восстановил NextRetryAt из планировщика")
	}
}
