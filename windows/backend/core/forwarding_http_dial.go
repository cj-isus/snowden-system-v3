package core

import (
	"context"
	"errors"
	"net"
	"sync"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const maxMixedHTTPDialScopes = maxMixedHTTPPipes // initial per-input receipt budget

// One ordinary HTTP request retains its own cancellable source context. Go's
// Transport may detach its DialContext argument from request cancellation; that
// argument must not become the lifetime/authority of our original pipe owner.
type mixedHTTPDialScope struct {
	input  *mixedIngress
	ctx    context.Context
	cancel context.CancelFunc

	handler  N.TCPConnectionHandlerEx
	source   M.Socksaddr
	callback N.CloseHandlerFunc

	mu    sync.Mutex // values only, no SDK/pipe/Close work
	calls map[*mixedHTTPDial]struct{}

	closeStarted  bool
	closeReturned bool
	failed        bool
	closeErr      error
	closeWait     chan struct{}
}

type mixedHTTPDial struct {
	pipe             *mixedHTTPPipe
	returned, failed bool
	done             chan struct{} // fields immutable after original dial-frame completion
}

func newMixedHTTPDialScope(ctx context.Context, handler N.TCPConnectionHandlerEx, source M.Socksaddr, callback N.CloseHandlerFunc) (*mixedHTTPDialScope, error) {
	if ctx == nil || handler == nil || ctx.Err() != nil {
		return nil, ErrForwardingNotAdmitted
	}
	input, ok := ctx.Value(mixedInputKey{}).(*mixedIngress)
	if !ok || input == nil || input.gate == nil {
		return nil, ErrForwardingNotAdmitted
	}
	gate := input.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	input.mu.Lock()
	if !owned || gate.closed || gate.routeFault || gate.rejectionsSealed || input.closing || input.httpDialsSealed || input.ctx.Err() != nil || ctx.Err() != nil || len(input.httpDials) >= maxMixedHTTPDialScopes {
		input.mu.Unlock()
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	if input.httpDials == nil {
		input.httpDials = make(map[*mixedHTTPDialScope]struct{})
	}
	lifetime, cancel := context.WithCancel(ctx)
	owner := &mixedHTTPDialScope{input: input, ctx: lifetime, cancel: cancel, handler: handler, source: source, callback: callback,
		calls: make(map[*mixedHTTPDial]struct{}), closeWait: make(chan struct{})}
	input.httpDials[owner] = struct{}{} // BEFORE Client/Transport/Do or any DialContext invocation
	input.mu.Unlock()
	gate.mu.Unlock()
	return owner, nil
}

func (owner *mixedHTTPDialScope) dial(ctx context.Context, network, address string) (conn net.Conn, finalErr error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrForwardingNotAdmitted
	}
	owner.mu.Lock()
	if owner.closeStarted || owner.failed || owner.ctx.Err() != nil || len(owner.calls) >= maxMixedHTTPPipes {
		owner.mu.Unlock()
		return nil, ErrForwardingNotAdmitted // no pipe/Go acquisition after seal or pressure
	}
	call := &mixedHTTPDial{done: make(chan struct{})}
	owner.calls[call] = struct{}{} // BEFORE address parsing, pipe acquisition or handler Go
	owner.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		call.returned, call.failed = returned, call.failed || recovered != nil || !returned
		if call.failed {
			finalErr = ErrEngineCleanupFailed
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			owner.cancel()
			owner.input.gate.noteRouteFault()
		}
		close(call.done)
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	pipe, err := openMixedHTTPPipe(owner.ctx, owner.handler, owner.source, M.ParseSocksaddr(address).Unwrap(), owner.callback)
	call.pipe = pipe // keep even a failed/partial/late original result
	owner.mu.Lock()
	current := !owner.closeStarted && !owner.failed && owner.ctx.Err() == nil
	owner.mu.Unlock()
	if err != nil {
		returned = true
		return nil, err
	}
	if !current || ctx.Err() != nil {
		if pipe != nil {
			pipe.cancel()
		}
		returned = true
		return nil, ErrForwardingNotAdmitted // no late publication/replacement
	}
	if pipe == nil || pipe.ends[0] == nil {
		call.failed, returned = true, true
		return nil, ErrEngineCleanupFailed
	}
	returned = true
	return pipe.ends[0], nil // same guarded end; a return is not an SDK borrower ACK
}

// The request helper/root calls Close, never the dial/callback that it joins.
// Cancellation precedes observation of every original frame and original pair.
func (owner *mixedHTTPDialScope) Close() (finalErr error) {
	owner.mu.Lock()
	if owner.closeStarted {
		owner.mu.Unlock()
		return owner.joinClose()
	}
	owner.closeStarted = true
	calls := make([]*mixedHTTPDial, 0, len(owner.calls))
	for original := range owner.calls {
		calls = append(calls, original)
	}
	owner.mu.Unlock()
	owner.cancel()
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
			delete(owner.input.httpDials, owner)
			owner.input.mu.Unlock()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	unconfirmed := false
	for _, original := range calls {
		if !mixedOriginalDone(ctx, original.done) {
			unconfirmed = true
			continue
		}
		unconfirmed = unconfirmed || original.failed || !original.returned
		if original.pipe != nil {
			original.pipe.cancel()
			if !mixedOriginalDone(ctx, original.pipe.done) {
				unconfirmed = true
				continue
			}
			original.pipe.mu.Lock()
			unconfirmed = unconfirmed || original.pipe.failed
			original.pipe.mu.Unlock()
		}
	}
	owner.mu.Lock()
	unconfirmed = unconfirmed || owner.failed
	owner.mu.Unlock()
	returned = true
	if unconfirmed {
		return ErrEngineCleanupFailed
	}
	return nil // only our dial/pipe API receipts, not Transport goroutine/timer retirement
}

func (owner *mixedHTTPDialScope) joinClose() error {
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

func (input *mixedIngress) sealHTTPDials() []*mixedHTTPDialScope {
	input.mu.Lock()
	input.httpDialsSealed = true
	owners := make([]*mixedHTTPDialScope, 0, len(input.httpDials))
	for original := range input.httpDials {
		owners = append(owners, original)
	}
	input.mu.Unlock()
	for _, original := range owners {
		original.cancel()
	}
	return owners
}
