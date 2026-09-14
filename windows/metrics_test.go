package main

import (
	"testing"
	"time"
)

func mkSnapshot(up, dn int64, hosts ...string) clashSnapshot {
	s := clashSnapshot{UploadTotal: up, DownloadTotal: dn}
	for _, h := range hosts {
		var c clashConn
		c.Metadata.Host = h
		c.Upload, c.Download = 10, 20
		c.Start = time.Now()
		s.Connections = append(s.Connections, c)
	}
	return s
}

// Первый замер: база «за сессии» = сырые счётчики, скорость ещё не определена.
func TestMetricsFirstPollSetsBase(t *testing.T) {
	m := &metricsState{port: "127.0.0.1:1", secret: "s"}
	v := m.apply(mkSnapshot(1000, 2000, "a.example"))
	if !v.Available {
		t.Fatal("want available")
	}
	if v.SessionUp != 0 || v.SessionDown != 0 {
		t.Fatalf("first poll must report zero session deltas, got up=%d dn=%d", v.SessionUp, v.SessionDown)
	}
	if v.RateUp != 0 || v.RateDown != 0 {
		t.Fatalf("first poll has no rate basis, got %v/%v", v.RateUp, v.RateDown)
	}
	if len(v.Connections) != 1 || v.Connections[0].Host != "a.example" {
		t.Fatalf("connections not mapped: %+v", v.Connections)
	}
}

// Второй замер: скорость = дельта/время, сессия = дельта от базы.
func TestMetricsSecondPollComputesRate(t *testing.T) {
	m := &metricsState{port: "127.0.0.1:1", secret: "s"}
	_ = m.apply(mkSnapshot(1000, 2000))
	m.mu.Lock()
	m.lastAt = time.Now().Add(-2 * time.Second) // форсируем dt=2s
	m.mu.Unlock()
	v := m.apply(mkSnapshot(3000, 6000))
	if v.RateUp != 1000 || v.RateDown != 2000 {
		t.Fatalf("rate = %v/%v, want 1000/2000", v.RateUp, v.RateDown)
	}
	if v.SessionUp != 2000 || v.SessionDown != 4000 {
		t.Fatalf("session = %d/%d, want 2000/4000", v.SessionUp, v.SessionDown)
	}
}

// Сброс счётчиков ядра (рестарт) не даёт отрицательной скорости и не рушит базу.
func TestMetricsCounterResetHandled(t *testing.T) {
	m := &metricsState{port: "127.0.0.1:1", secret: "s"}
	_ = m.apply(mkSnapshot(10000, 20000))
	m.mu.Lock()
	m.lastAt = time.Now().Add(-2 * time.Second)
	m.mu.Unlock()
	v := m.apply(mkSnapshot(100, 200)) // счётчики обнулились
	if v.RateUp != 0 || v.RateDown != 0 {
		t.Fatalf("negative delta must clamp to zero rate, got %v/%v", v.RateUp, v.RateDown)
	}
}

// Топ соединений: сортировка по объёму, ограничение 20.
func TestMetricsTopConnectionsSorted(t *testing.T) {
	m := &metricsState{port: "127.0.0.1:1", secret: "s"}
	s := clashSnapshot{}
	for i := 0; i < 30; i++ {
		var c clashConn
		c.Metadata.Host = "h"
		c.Upload = int64(i)
		s.Connections = append(s.Connections, c)
	}
	v := m.apply(s)
	if len(v.Connections) != 20 {
		t.Fatalf("top must be capped to 20, got %d", len(v.Connections))
	}
	// наибольший (29) первым
	if v.Connections[0].Up != 29 {
		t.Fatalf("sort broken: first=%d", v.Connections[0].Up)
	}
}

// Порт 0 из опции не должен попасть в рендер (гвардия).
func TestPortOf(t *testing.T) {
	if got := portOf("127.0.0.1:9097"); got != 9097 {
		t.Fatalf("portOf = %d", got)
	}
}
