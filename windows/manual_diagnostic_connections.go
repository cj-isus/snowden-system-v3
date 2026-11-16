package main

import (
	"context"
	"errors"
	"net"
	"sync"
)

// The volume plan admits at most 8 sizes on two routes. Allow twice that many
// original dial callbacks in total, without a per-request retry guarantee.
// Exhaustion is local unavailable, not a network verdict or a wire/RSS bound.
const maxManualDiagnosticDials = 32

type manualDiagnosticDial struct {
	task     *manualDiagnosticTask
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	returned bool // immutable after done; ordinary callback return only
	mu       sync.Mutex
	conn     *manualDiagnosticConn
}

// Every protocol receives this wrapper, so watcher/SSH/HTTP Close share one
// original attempt. A pending/reentrant Close never waits on or retries itself.
type manualDiagnosticConn struct {
	net.Conn
	task          *manualDiagnosticTask
	mu            sync.Mutex
	closeStarted  bool
	closeReturned bool
	closeDone     chan struct{}
	closeErr      error // immutable after closeDone
}

func (conn *manualDiagnosticConn) Close() (result error) {
	conn.mu.Lock()
	if conn.closeStarted {
		conn.mu.Unlock()
		select {
		case <-conn.closeDone:
			return conn.closeErr
		default:
			return errSecretTestUnavailable // original pending attempt remains independently joinable
		}
	}
	conn.closeStarted = true
	conn.mu.Unlock()
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errSecretTestUnavailable
		}
		if result != nil {
			conn.task.cleanupFailed.Store(true)
		}
		conn.closeReturned, conn.closeErr = returned, result
		close(conn.closeDone)
	}()
	err := conn.Conn.Close()
	returned = true
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return errSecretTestUnavailable
	}
	return nil
}

func (task *manualDiagnosticTask) ownedDialer(call func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if ctx == nil || ctx.Err() != nil || task.currentError() != nil {
			return nil, errSecretTestUnavailable
		}
		task.mu.Lock()
		if task.sealed || task.currentError() != nil {
			task.mu.Unlock()
			return nil, errSecretTestUnavailable
		}
		if len(task.dials) >= maxManualDiagnosticDials {
			task.dialLimitReached.Store(true)
			task.mu.Unlock()
			return nil, errSecretTestUnavailable
		}
		original, cancel := context.WithCancel(ctx)
		dial := &manualDiagnosticDial{task: task, ctx: original, cancel: cancel, done: make(chan struct{})}
		task.dials = append(task.dials, dial) // before the original native/library dial callback
		task.mu.Unlock()
		return dial.run(call, network, address)
	}
}

func (dial *manualDiagnosticDial) run(call func(context.Context, string, string) (net.Conn, error), network, address string) (conn net.Conn, result error) {
	defer close(dial.done)
	defer dial.cancel()
	defer func() {
		if payload := recover(); payload != nil || !dial.returned {
			dial.task.cleanupFailed.Store(true)
			conn, result = nil, errSecretTestUnavailable
		}
	}()
	conn, result = dial.perform(call, network, address)
	dial.returned = true
	return
}

func (dial *manualDiagnosticDial) perform(call func(context.Context, string, string) (net.Conn, error), network, address string) (net.Conn, error) {
	raw, err := call(dial.ctx, network, address)
	var conn *manualDiagnosticConn
	if raw != nil {
		conn = &manualDiagnosticConn{Conn: raw, task: dial.task, closeDone: make(chan struct{})}
		dial.mu.Lock()
		dial.conn = conn // retain even a late successful socket after the watcher returned
		dial.mu.Unlock()
	}
	dial.task.mu.Lock()
	sealed := dial.task.sealed
	dial.task.mu.Unlock()
	if err != nil || raw == nil || sealed || dial.ctx.Err() != nil || dial.task.currentError() != nil {
		if conn != nil {
			_ = conn.Close()
		}
		if err != nil {
			return nil, err // existing HTTP observation classifiers retain their ordinary error shape
		}
		return nil, errSecretTestUnavailable
	}
	return conn, nil
}

func (dial *manualDiagnosticDial) connection() *manualDiagnosticConn {
	dial.mu.Lock()
	defer dial.mu.Unlock()
	return dial.conn
}

func (task *manualDiagnosticTask) sealDials() []*manualDiagnosticDial {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.sealed = true
	return append([]*manualDiagnosticDial(nil), task.dials...)
}

func cancelManualDials(dials []*manualDiagnosticDial) {
	for _, dial := range dials {
		dial.cancel() // request all cancellations before attempting any original Close
	}
}

func (task *manualDiagnosticTask) closeDials(dials []*manualDiagnosticDial) {
	sequence := &appCleanupSequence{}
	defer func() {
		if sequence.result != nil {
			task.cleanupFailed.Store(true)
		}
	}()
	for index := len(dials) - 1; index >= 0; index-- {
		dial := dials[index]
		defer appCleanupStep("close original diagnostic socket", sequence, func() error {
			if conn := dial.connection(); conn != nil {
				_ = conn.Close() // pending is not a fault; finish joins this original closeDone
			}
			return nil
		})
	}
}

func (task *manualDiagnosticTask) retireDials() {
	dials := task.sealDials()
	cancelManualDials(dials)
	task.closeDials(dials)
	for _, dial := range dials {
		<-dial.done // includes a late successful connection's rejection/Close
		if !dial.returned {
			task.cleanupFailed.Store(true)
		}
		if conn := dial.connection(); conn != nil {
			_ = conn.Close()
			<-conn.closeDone
			if !conn.closeReturned || conn.closeErr != nil {
				task.cleanupFailed.Store(true)
			}
		}
	}
}

func (task *manualDiagnosticTask) dialSSH(address string) (net.Conn, error) {
	return task.dialConnection("tcp", address)
}

func (task *manualDiagnosticTask) dialConnection(network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: testDialTimeout}
	conn, err := task.ownedDialer(dialer.DialContext)(task.ctx, network, address)
	if err != nil {
		return nil, errSecretTestUnavailable
	}
	deadline, bounded := task.ctx.Deadline()
	if task.currentError() != nil || !bounded || conn.SetDeadline(deadline) != nil {
		_ = conn.Close()
		return nil, errSecretTestUnavailable
	}
	return conn, nil
}
