package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	mixedHandshakeTimeout    = 30 * time.Second // initial local protocol/header/control budget
	maxMixedHandshakeBudgets = 8                // pending/failed timer receipts per input, not a process quota
	maxMixedHTTPAuthFailures = 4
)

var errMixedHandshakeTimeout = errors.New("mixed handshake time limit exceeded")

// The pinned SOCKS response codecs check errors but discard Write's count.
// Keep the original single write; an incomplete reply must not admit a peer.
type mixedHandshakeWriter struct{ conn net.Conn }

func (writer mixedHandshakeWriter) Write(buffer []byte) (int, error) {
	n, err := writer.conn.Write(buffer)
	if n < 0 || n > len(buffer) {
		if original, ok := writer.conn.(*mixedIngressConnection); ok {
			original.owner.gate.noteRouteFault()
		}
		return 0, errors.Join(ErrEngineCleanupFailed, err)
	}
	if n != len(buffer) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err // no retry or guessed completion of a partially sent response
}

// A phase can cancel only its original input. Release disarms that authority
// before Stop, then observes the original callback if Stop says it has fired.
// No native socket deadline is overwritten or guessed/restored at handoff.
type mixedHandshakeBudget struct {
	input *mixedIngress
	until time.Time
	mu    sync.Mutex // values only; no timer API/cancel/join while held

	timer                     *time.Timer
	installReturned           bool
	released, expired, failed bool
	installed                 chan struct{}

	callbackStarted, callbackReturned bool
	actionStarted, actionReturned     bool
	callbackDone                      chan struct{}

	stopStarted, stopReturned, stopped bool
	closeStarted, closeReturned        bool
	closeErr                           error
	closeWait                          chan struct{}
}

func newMixedHandshakeBudget(ctx context.Context, raw net.Conn, until time.Time) (*mixedHandshakeBudget, error) {
	conn, ok := raw.(*mixedIngressConnection)
	if !ok || conn == nil || conn.owner == nil || conn.owner.gate == nil || ctx == nil || ctx.Err() != nil || ctx.Value(mixedInputKey{}) != conn.owner {
		return nil, ErrForwardingNotAdmitted
	}
	deadline := time.Now().Add(mixedHandshakeTimeout)
	if !until.IsZero() && until.Before(deadline) {
		deadline = until
	}
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	if !time.Now().Before(deadline) {
		return nil, errMixedHandshakeTimeout
	}
	input, gate := conn.owner, conn.owner.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	input.mu.Lock()
	if !owned || gate.closed || gate.routeFault || gate.rejectionsSealed || input.closing || input.handshakeBudgetsSealed || input.ctx.Err() != nil || ctx.Err() != nil || len(input.handshakeBudgets) >= maxMixedHandshakeBudgets {
		input.mu.Unlock()
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	if input.handshakeBudgets == nil {
		input.handshakeBudgets = make(map[*mixedHandshakeBudget]struct{})
	}
	owner := &mixedHandshakeBudget{input: input, until: deadline, installed: make(chan struct{}), callbackDone: make(chan struct{}), closeWait: make(chan struct{})}
	input.handshakeBudgets[owner] = struct{}{} // BEFORE AfterFunc construction/callback
	input.mu.Unlock()
	gate.mu.Unlock()
	return owner, owner.install()
}

func (owner *mixedHandshakeBudget) install() (finalErr error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.installReturned = returned
		owner.failed = owner.failed || fault || finalErr != nil
		owner.mu.Unlock()
		close(owner.installed)
		if fault || finalErr != nil {
			owner.input.gate.noteRouteFault()
		}
	}()
	owner.timer = time.AfterFunc(time.Until(owner.until), owner.expire)
	returned = true
	if owner.timer == nil {
		return ErrEngineCleanupFailed
	}
	return nil
}

func (owner *mixedHandshakeBudget) expire() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		owner.callbackReturned = returned
		owner.failed = owner.failed || fault
		owner.mu.Unlock()
		close(owner.callbackDone) // actual callback frame, never fabricated by Stop(true)
		if fault {
			owner.input.gate.noteRouteFault()
		}
	}()
	owner.mu.Lock()
	owner.callbackStarted = true
	if owner.released {
		owner.mu.Unlock()
		returned = true
		return // old phase has no authority after release
	}
	owner.expired, owner.actionStarted = true, true
	owner.mu.Unlock()
	owner.input.cancel() // requests original TCP/IO cleanup; not proof it has returned
	owner.mu.Lock()
	owner.actionReturned = true
	owner.mu.Unlock()
	returned = true
}

func (owner *mixedHandshakeBudget) Close() (finalErr error) {
	owner.mu.Lock()
	if owner.closeStarted {
		owner.mu.Unlock()
		return owner.joinClose()
	}
	owner.closeStarted, owner.released = true, true
	owner.mu.Unlock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.failed = owner.failed || fault || finalErr != nil
		failed := owner.failed
		if failed && finalErr == nil {
			finalErr = ErrEngineCleanupFailed
		}
		owner.closeReturned, owner.closeErr = returned, finalErr
		owner.mu.Unlock()
		if failed {
			owner.input.gate.noteRouteFault()
		}
		close(owner.closeWait)
		if !failed {
			owner.input.mu.Lock()
			delete(owner.input.handshakeBudgets, owner)
			owner.input.mu.Unlock()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	if !mixedOriginalDone(ctx, owner.installed) {
		returned = true
		return ErrEngineCleanupFailed
	}
	if owner.timer == nil {
		returned = true
		return ErrEngineCleanupFailed
	}
	owner.mu.Lock()
	owner.stopStarted = true
	owner.mu.Unlock()
	stopped := owner.timer.Stop() // exactly one original attempt, no Reset or replacement
	owner.mu.Lock()
	owner.stopReturned, owner.stopped = true, stopped
	owner.mu.Unlock()
	if !stopped && !mixedOriginalDone(ctx, owner.callbackDone) {
		returned = true
		return ErrEngineCleanupFailed
	}
	owner.mu.Lock()
	stoppedBeforeCallback := stopped && !owner.callbackStarted && !owner.actionStarted
	callbackFinished := !stopped && owner.callbackStarted && owner.callbackReturned && (!owner.actionStarted || owner.actionReturned)
	confirmed := !owner.failed && owner.installReturned && owner.stopStarted && owner.stopReturned && (stoppedBeforeCallback || callbackFinished)
	owner.mu.Unlock()
	returned = true
	if !confirmed {
		return ErrEngineCleanupFailed
	}
	return nil // stopped-before-callback OR original callback/action return, not socket/worker retirement
}

func (owner *mixedHandshakeBudget) joinClose() error {
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	_ = mixedOriginalDone(ctx, owner.closeWait)
	owner.mu.Lock()
	returned, err := owner.closeReturned, owner.closeErr
	unknown := !returned || err != nil || owner.failed
	owner.failed = owner.failed || unknown
	owner.mu.Unlock()
	if unknown {
		owner.input.gate.noteRouteFault()
		return errors.Join(ErrEngineCleanupFailed, err)
	}
	return nil
}

func (owner *mixedHandshakeBudget) finish() error {
	completedAt := time.Now() // phase end, before a potentially delayed Stop/callback observation
	if err := owner.Close(); err != nil {
		return err
	}
	owner.mu.Lock()
	expired := owner.expired || !completedAt.Before(owner.until)
	owner.mu.Unlock()
	if expired {
		owner.input.cancel()
		return errMixedHandshakeTimeout
	}
	return nil
}

func withMixedHandshakeBudget(ctx context.Context, conn net.Conn, until time.Time, call func() error) (finalErr error) {
	if call == nil {
		return ErrForwardingNotAdmitted
	}
	owner, err := newMixedHandshakeBudget(ctx, conn, until)
	if err != nil {
		return err
	} // any partial original owner stays with the input
	defer func() { finalErr = errors.Join(finalErr, owner.finish()) }()
	return call()
}

func (input *mixedIngress) sealHandshakeBudgets() []*mixedHandshakeBudget {
	input.mu.Lock()
	input.handshakeBudgetsSealed = true
	owners := make([]*mixedHandshakeBudget, 0, len(input.handshakeBudgets))
	for original := range input.handshakeBudgets {
		owners = append(owners, original)
	}
	input.mu.Unlock()
	for _, original := range owners {
		original.mu.Lock()
		original.released = true // disarm action; original Stop/callback still must be observed
		original.mu.Unlock()
	}
	return owners
}
