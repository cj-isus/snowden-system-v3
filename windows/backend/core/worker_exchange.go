package core

import (
	"context"
	"errors"
	"time"
)

// A single exchange owns both pipes. Timeout or an interrupted parent callback
// invalidates the original protocol owner. No reply or ended waiter is a
// substitute for original I/O return and the existing process/Job retirement.
func (p *engineProcess) exchange(parent *workerRPCContext, request *workerRequest, limit time.Duration) (workerReply, error) {
	accepted, cancelReturned := false, false
	defer func() {
		if (!accepted || !cancelReturned) && p != nil {
			p.unusable = true // includes panic/Goexit before or after original I/O registration
		}
	}()
	if p == nil || parent == nil || parent.Context == nil || p.unusable || p.exited() {
		return workerReply{}, ErrEngineWorkerUnavailable
	}
	if err := p.ioRetirementReady(); err != nil {
		p.unusable = true
		return workerReply{}, errors.Join(errWorkerProtocol, err)
	}
	if err := contextError(parent); err != nil {
		p.unusable = true // a request may already have reserved its sequence
		return workerReply{}, err
	}
	// Revocation/Stop must remain possible after a root query fails. All other
	// commands, including hello and admission, require the original custody.
	positive := request == nil || request.Action != "stop" && request.Action != "dns_pause" && request.Action != "tun_scope_close"
	if positive {
		if err := p.ancestorScope(parent, true); err != nil {
			p.unusable = true
			return workerReply{}, err
		}
	}
	if request != nil && workerStateReplyAction(request.Action) && !request.replyState.matches(p, request) {
		p.unusable = true
		return workerReply{}, errWorkerProtocol // no unbound/stale state command reaches either pipe
	}
	ctx, cancel := context.WithTimeout(parent, limit)
	defer func() {
		cancel()
		cancelReturned = true // even deferred context cleanup must return normally
	}()
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	type workerExchange struct {
		*ownedPipeIO // finite header/effects only; no pointer back to the potentially large reply
		reply        workerReply
	}
	exchange := &workerExchange{ownedPipeIO: &ownedPipeIO{done: make(chan struct{}), effects: new(ownedPipeEffects)}}
	p.captureIO, p.ioDone = exchange.ownedPipeIO, exchange.done
	go func() {
		finished := false
		defer func() {
			if recovered := recover(); recovered != nil || !finished {
				exchange.err = errors.Join(exchange.err, errWorkerProtocol)
			}
			close(exchange.done) // goroutine end; original raw pending/return facts remain separate
		}()
		exchange.err = contextError(ctx) // the goroutine may start after intent/path revocation
		if exchange.err == nil && request != nil {
			exchange.err = writeWorkerFrame(ownedPipeWriter{ctx: ctx, writer: p.input, failure: errWorkerProtocol, effects: exchange.effects}, request, maxWorkerRequestBytes)
		}
		if exchange.err == nil {
			exchange.err = readWorkerFrame(ownedPipeReader{ctx: ctx, reader: p.output, failure: errWorkerProtocol, effects: exchange.effects}, &exchange.reply, maxWorkerReplyBytes)
		}
		if exchange.err != nil {
			if !errors.Is(exchange.err, context.Canceled) && !errors.Is(exchange.err, context.DeadlineExceeded) {
				exchange.err = errWorkerProtocol // omit raw pipe/codec values and paths
			}
		} else {
			expectedID := uint64(0)
			if request != nil {
				expectedID = request.ID
			}
			if exchange.reply.Version != workerProtocolVersion || exchange.reply.ID != expectedID || !validNativePreparedReply(request, exchange.reply) {
				exchange.err = errWorkerProtocol // retain correlation failure even if the caller is canceled
			} else {
				exchange.err = workerCommandReplyFailure(request, exchange.reply)
			}
		}
		finished = true
	}()
	var waitErr error
	select {
	case <-exchange.done:
	case <-intentDone:
		waitErr = context.Canceled
	case <-operationDone:
		waitErr = context.Canceled
	case <-dependencyDone:
		waitErr = context.Canceled
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	if p.captureIO != exchange.ownedPipeIO || p.ioDone != exchange.done {
		return workerReply{}, errWorkerProtocol // a replacement completion cannot publish this reply
	}
	if err := errors.Join(waitErr, contextError(ctx), exchange.completedFailure()); err != nil {
		p.unusable = true
		return workerReply{}, err // only original-owner retirement may follow; no stale reply escapes
	}
	if exchange.retirementReady() != nil {
		return workerReply{}, errWorkerProtocol // missing success fence, not a premature cleanup-failure claim
	}
	if positive {
		if err := p.ancestorScope(ctx, true); err != nil {
			p.unusable = true
			return workerReply{}, err // no Start/admission observation after original root custody failed
		}
	}
	if p.unusable || p.exited() || p.captureIO != exchange.ownedPipeIO || p.ioDone != exchange.done {
		return workerReply{}, ErrEngineWorkerUnavailable
	}
	accepted = true            // only ordinary accepted return makes this original process reusable
	return exchange.reply, nil // original done/raw-return and current source/ancestor facts only
}
