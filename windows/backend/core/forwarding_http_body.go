package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

const maxMixedHTTPBodies = 2 * maxMixedHTTPPipes // initial receipt count, not a body-size/RSS limit

type mixedHTTPBody struct {
	input *mixedIngress
	raw   io.ReadCloser
	mu    sync.Mutex // values only; raw Read/Close run outside it

	remaining                 int64 // -1: EOF required; nonnegative: known content length still unread
	sawEOF                    bool
	readPending               int
	closing, failed, terminal bool

	closeStarted      bool
	closeFinished     bool
	closeReturned     bool
	priorCloseUnknown bool
	closeErr          error
	closeWait, done   chan struct{}
}

// Only call before lending a request body to the SDK, or immediately after the
// original Do returns a response. A body potentially closed before that return
// cannot use a second Close as proof of the missing original outcome.
func retainMixedHTTPBody(ctx context.Context, raw io.ReadCloser, length int64, priorCloseUnknown bool) (io.ReadCloser, error) {
	if raw == nil || raw == http.NoBody {
		return raw, nil // known no-resource; preserve nil/NoBody protocol semantics
	}
	if ctx == nil {
		panic(ErrEngineCleanupFailed) // invalid private adapter, no unowned Close fallback
	}
	input, ok := ctx.Value(mixedInputKey{}).(*mixedIngress)
	if !ok || input == nil || input.gate == nil {
		panic(ErrEngineCleanupFailed)
	}
	if body, ok := raw.(*mixedHTTPBody); ok && body.input == input {
		return body, nil // the exact original receipt, not another closer around it
	}
	gate := input.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	input.mu.Lock()
	allowed := owned && !gate.closed && !gate.routeFault && !gate.rejectionsSealed && !input.closing && !input.httpBodiesSealed && input.ctx.Err() == nil && ctx.Err() == nil && len(input.httpBodies) < maxMixedHTTPBodies
	if !allowed {
		input.mu.Unlock()
		gate.mu.Unlock()
		input.cancel() // raw Body.Close may otherwise drain the still-open input
		if priorCloseUnknown {
			stopUntrackedIngress(gate.workerCustody) // no safe original-close outcome to retry
		}
		return nil, (admissionRouter{admission: gate}).rejectIngress(raw, nil, ErrForwardingNotAdmitted)
	}
	if input.httpBodies == nil {
		input.httpBodies = make(map[*mixedHTTPBody]struct{})
	}
	if length < 0 {
		length = -1
	}
	body := &mixedHTTPBody{input: input, raw: raw, remaining: length, closeWait: make(chan struct{}), done: make(chan struct{})}
	if priorCloseUnknown {
		body.closing, body.failed, body.closeStarted, body.priorCloseUnknown = true, true, true, true
	}
	input.httpBodies[body] = struct{}{} // BEFORE first body method or SDK handoff
	input.mu.Unlock()
	gate.mu.Unlock()
	if priorCloseUnknown {
		input.gate.noteRouteFault()
		return body, ErrEngineCleanupFailed // retain pointer; never retry a possibly completed SDK Close
	}
	return body, nil
}

func (body *mixedHTTPBody) Read(buffer []byte) (n int, finalErr error) {
	body.mu.Lock()
	if body.closing || body.failed || body.input.ctx.Err() != nil {
		body.mu.Unlock()
		return 0, ErrForwardingNotAdmitted
	}
	body.readPending++
	body.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned || n < 0 || n > len(buffer)
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		body.mu.Lock()
		body.readPending--
		body.failed = body.failed || fault
		if !fault {
			body.sawEOF = body.sawEOF || finalErr == io.EOF
			if body.remaining >= 0 {
				if int64(n) >= body.remaining {
					body.remaining = 0
				} else {
					body.remaining -= int64(n)
				}
			}
		}
		body.mu.Unlock()
		if fault {
			body.input.gate.noteRouteFault()
		}
		body.tryFinish()
		if recovered != nil || n < 0 || n > len(buffer) {
			panic(ErrEngineCleanupFailed)
		}
	}()
	n, finalErr = body.raw.Read(buffer)
	returned = true
	return
}

func (body *mixedHTTPBody) Close() (finalErr error) {
	body.mu.Lock()
	if body.priorCloseUnknown {
		body.mu.Unlock()
		return ErrEngineCleanupFailed // no fabricated original Close/done receipt
	}
	if body.closeStarted {
		body.mu.Unlock()
		return body.joinClose() // original result only; never retry raw Close
	}
	body.closeStarted, body.closing = true, true
	incomplete := !body.sawEOF && body.remaining != 0 || body.readPending != 0
	body.mu.Unlock()
	if incomplete {
		body.input.cancel() // Close can read/drain: stop the original sockets before waiting
	}
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		body.mu.Lock()
		body.closeFinished, body.closeReturned, body.closeErr = true, returned, finalErr
		body.failed = body.failed || fault || finalErr != nil
		body.mu.Unlock()
		close(body.closeWait)
		if fault || finalErr != nil {
			finalErr = errors.Join(ErrEngineCleanupFailed, finalErr)
			body.input.gate.noteRouteFault()
		}
		body.tryFinish()
	}()
	finalErr = body.raw.Close()
	returned = true
	return
}

func (body *mixedHTTPBody) joinClose() error {
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	_ = mixedOriginalDone(ctx, body.closeWait)
	body.mu.Lock()
	returned, err := body.closeReturned, body.closeErr // fresh original fact after channel observation
	unknown := !returned || err != nil
	body.failed = body.failed || unknown
	body.mu.Unlock()
	if unknown {
		body.input.gate.noteRouteFault()
		body.tryFinish()
		return errors.Join(ErrEngineCleanupFailed, err)
	}
	return nil
}

func (body *mixedHTTPBody) tryFinish() {
	body.mu.Lock()
	if body.terminal || !body.closeFinished || body.readPending != 0 {
		body.mu.Unlock()
		return
	}
	body.terminal = true
	body.failed = body.failed || !body.closeReturned || body.closeErr != nil
	failed := body.failed
	close(body.done) // original raw Close AND all counted Read returns; not Transport retirement
	body.mu.Unlock()
	if failed {
		body.input.gate.noteRouteFault()
	} else {
		body.input.mu.Lock()
		delete(body.input.httpBodies, body)
		body.input.mu.Unlock()
	}
}

func (input *mixedIngress) sealHTTPBodies() []*mixedHTTPBody {
	input.mu.Lock()
	input.httpBodiesSealed = true
	owners := make([]*mixedHTTPBody, 0, len(input.httpBodies))
	for original := range input.httpBodies {
		owners = append(owners, original)
	}
	input.mu.Unlock()
	for _, original := range owners {
		original.mu.Lock()
		original.closing = true // stop new reads; original Close follows TCP/pipe closure
		original.mu.Unlock()
	}
	return owners
}

var _ io.ReadCloser = (*mixedHTTPBody)(nil)
