package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Diagnostic admission bounds application work; these are resource ceilings,
// not measured latency/throughput SLOs. Payload bytes exclude TLS/HTTP headers,
// kernel buffers and retransmissions. Protected work has a reserved lane so
// optional diagnostics cannot spend its budget or occupy its concurrency.
type DiagnosticClass uint8

const (
	DiagnosticProtected DiagnosticClass = iota
	DiagnosticBackground
	DiagnosticVolume
	DiagnosticEndpoint
)

var ErrDiagnosticBudget = errors.New("diagnostic work budget unavailable")

type diagnosticReservation struct {
	at       time.Time
	requests int
	bytes    int64
}

type diagnosticLane struct {
	active int
	recent []diagnosticReservation
}

type diagnosticPool struct {
	mu        sync.Mutex
	protected diagnosticLane
	optional  diagnosticLane
}

var processDiagnostics diagnosticPool

// AdmitDiagnostic reserves the complete bounded request/body workload before
// network work starts. Context preparation precedes pool admission; rejection
// cancels that unused context and starts no application queue or network work.
// Reservations are not refunded on error/cancellation: rapid failed attempts
// still consume work. This is not a bound on context-library internals.
// The caller must defer release even when no I/O was performed.
func AdmitDiagnostic(parent context.Context, class DiagnosticClass, requests int, bodyBytes int64, duration time.Duration) (context.Context, func(), error) {
	if parent == nil || parent.Err() != nil || class > DiagnosticEndpoint || duration <= 0 || duration > 90*time.Second {
		return nil, nil, ErrDiagnosticBudget
	}
	maxRequests, maxBytes := 64, int64(2<<20)
	lane := &processDiagnostics.optional
	if class == DiagnosticProtected {
		maxRequests, maxBytes = 120, 1<<20
		lane = &processDiagnostics.protected
	}
	if requests < 1 || requests > maxRequests || bodyBytes < 0 || bodyBytes > maxBytes {
		return nil, nil, ErrDiagnosticBudget
	}
	// Context construction can call parent implementations. Do it before any
	// reservation and outside the pool mutex; an unreturned factory owns no slot.
	ctx, cancel := context.WithTimeout(parent, duration)
	handedOff := false
	defer func() {
		if !handedOff {
			cancel() // unused context only; no rolling reservation is refunded
		}
	}()
	var once sync.Once
	release := func() {
		once.Do(func() {
			cancel() // an interrupted original cancellation does not free/retry its slot
			processDiagnostics.mu.Lock()
			defer processDiagnostics.mu.Unlock()
			lane.active--
		})
	}
	processDiagnostics.mu.Lock()
	defer processDiagnostics.mu.Unlock()
	if ctx.Err() != nil {
		return nil, nil, ErrDiagnosticBudget // canceled during context construction or lock wait
	}
	now := time.Now()
	first := 0
	for first < len(lane.recent) && !lane.recent[first].at.After(now.Add(-time.Minute)) {
		first++
	}
	if first != 0 {
		copy(lane.recent, lane.recent[first:])
		lane.recent = lane.recent[:len(lane.recent)-first]
	}
	usedRequests, usedBytes := 0, int64(0)
	for _, reservation := range lane.recent {
		usedRequests += reservation.requests
		usedBytes += reservation.bytes
	}
	if lane.active >= 2 || requests > maxRequests-usedRequests || bodyBytes > maxBytes-usedBytes {
		return nil, nil, ErrDiagnosticBudget
	}
	lane.recent = append(lane.recent, diagnosticReservation{at: now, requests: requests, bytes: bodyBytes})
	lane.active++
	handedOff = true // no callbacks after committing this original reservation
	return ctx, release, nil
}
