package core

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/pipe"
)

// Initial per-input receipt budget, including pending/failed pairs. This is not
// a whole-process byte/RSS quota. Exhaustion rejects before creating a pipe.
const maxMixedHTTPPipes = 32

type mixedHTTPPipe struct {
	input   *mixedIngress
	ctx     context.Context
	cancel  context.CancelFunc
	onClose N.CloseHandlerFunc
	mu      sync.Mutex // values only; never held during pipe IO/Close or SDK callbacks

	raw                 [2]net.Conn
	ends                [2]*mixedHTTPPipeEnd
	local, remote       [2]net.Addr
	constructorReturned bool
	acquisitionReturned bool

	handlerStarted, handlerReturned bool
	closing, failed                 bool
	ioPending                       int
	ioDoneClosed                    bool

	rawCloseStarted, rawCloseReturned [2]bool
	rawCloseErr                       [2]error
	closeReturned                     bool
	closeErr                          error // immutable after original closeWait
	callbackStarted                   atomic.Bool
	callbackReturned                  bool

	acquired, closeWait, ioDone     chan struct{}
	handlerDone, callbackDone, done chan struct{}
}

func openMixedHTTPPipe(ctx context.Context, handler N.TCPConnectionHandlerEx, source, destination M.Socksaddr, callback N.CloseHandlerFunc) (*mixedHTTPPipe, error) {
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
	if !owned || gate.closed || gate.routeFault || gate.rejectionsSealed {
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	input.mu.Lock()
	if input.closing || input.httpPipesSealed || input.ctx.Err() != nil || ctx.Err() != nil || len(input.httpPipes) >= maxMixedHTTPPipes {
		input.mu.Unlock()
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	if input.httpPipes == nil {
		input.httpPipes = make(map[*mixedHTTPPipe]struct{})
	}
	lifetime, cancel := context.WithCancel(ctx)
	owner := &mixedHTTPPipe{input: input, ctx: lifetime, cancel: cancel, onClose: callback,
		acquired: make(chan struct{}), closeWait: make(chan struct{}), ioDone: make(chan struct{}),
		handlerDone: make(chan struct{}), callbackDone: make(chan struct{}), done: make(chan struct{})}
	input.httpPipes[owner] = struct{}{} // BEFORE pipe constructor, metadata, Go or Router
	input.mu.Unlock()
	gate.mu.Unlock()
	context.AfterFunc(lifetime, owner.closeTask) // exact cancellation task, never replaced/stopped
	if err := owner.acquire(); err != nil {
		owner.cancel()
		return owner, err
	}
	owner.mu.Lock()
	if owner.closing || owner.failed || owner.ctx.Err() != nil {
		owner.mu.Unlock()
		owner.cancel()
		return owner, ErrForwardingNotAdmitted
	}
	owner.handlerStarted = true // serialize the original task claim with Close sealing
	owner.mu.Unlock()
	go owner.run(handler, source, destination)
	return owner, nil
}

func (owner *mixedHTTPPipe) acquire() (finalErr error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.acquisitionReturned = returned
		owner.failed = owner.failed || fault || finalErr != nil
		owner.mu.Unlock()
		close(owner.acquired)
		if fault || finalErr != nil {
			owner.input.gate.noteRouteFault()
		}
	}()
	owner.raw[0], owner.raw[1] = pipe.Pipe()
	owner.constructorReturned = true
	for index, raw := range owner.raw {
		if raw == nil {
			returned = true
			return ErrEngineCleanupFailed
		}
		owner.local[index], owner.remote[index] = raw.LocalAddr(), raw.RemoteAddr()
		owner.ends[index] = &mixedHTTPPipeEnd{owner: owner, index: index}
	}
	returned = true
	return nil
}

func (owner *mixedHTTPPipe) run(handler N.TCPConnectionHandlerEx, source, destination M.Socksaddr) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		owner.handlerReturned = returned
		owner.failed = owner.failed || fault
		owner.mu.Unlock()
		close(owner.handlerDone)
		if fault {
			owner.cancel()
			owner.input.gate.noteRouteFault()
		}
	}()
	if err := contextError(owner.ctx); err != nil {
		owner.callback(err) // known skipped delegation, actual local callback invocation
	} else {
		handler.NewConnectionEx(owner.ctx, owner.ends[1], source, destination, owner.callback)
	}
	returned = true // a returned handler does not prove its asynchronous flow retired
}

func (owner *mixedHTTPPipe) callback(cause error) {
	if !owner.callbackStarted.CompareAndSwap(false, true) {
		return
	}
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		owner.callbackReturned = returned
		owner.failed = owner.failed || fault
		owner.mu.Unlock()
		owner.cancel() // never wait for the handler/route that called this callback
		close(owner.callbackDone)
		if fault {
			owner.input.gate.noteRouteFault()
		}
	}()
	if owner.onClose != nil {
		owner.onClose(cause)
	}
	returned = true
}

// Only this original task calls raw Close. Ends merely cancel and observe this
// closeWait; neither callback nor Close waits for its own handler/IO retirement.
func (owner *mixedHTTPPipe) closeEnds() (finalErr error) {
	owner.mu.Lock()
	owner.closing = true
	if owner.ioPending == 0 && !owner.ioDoneClosed {
		owner.ioDoneClosed = true
		close(owner.ioDone)
	}
	owner.mu.Unlock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.closeReturned, owner.closeErr = returned, finalErr
		owner.failed = owner.failed || fault || finalErr != nil
		owner.mu.Unlock()
		close(owner.closeWait)
	}()
	for index, raw := range owner.raw {
		if raw != nil {
			owner.rawCloseStarted[index] = true
			owner.rawCloseErr[index] = raw.Close()
			owner.rawCloseReturned[index] = true
		}
	}
	returned = true
	if !owner.constructorReturned || !owner.acquisitionReturned {
		return errors.Join(ErrEngineCleanupFailed, owner.rawCloseErr[0], owner.rawCloseErr[1]) // close known ends once even if metadata/frame failed
	}
	return errors.Join(owner.rawCloseErr[0], owner.rawCloseErr[1])
}

func (owner *mixedHTTPPipe) closeTask() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		endsConfirmed := owner.raw[0] != nil && owner.raw[1] != nil && owner.rawCloseReturned[0] && owner.rawCloseReturned[1]
		handlerConfirmed := !owner.handlerStarted || owner.handlerReturned && owner.callbackReturned
		owner.failed = owner.failed || fault || !owner.acquisitionReturned || !owner.closeReturned || owner.closeErr != nil || owner.ioPending != 0 || !endsConfirmed || !handlerConfirmed
		failed := owner.failed
		owner.mu.Unlock()
		if failed {
			owner.input.gate.noteRouteFault()
		}
		close(owner.done)
		if !failed {
			owner.input.mu.Lock()
			delete(owner.input.httpPipes, owner)
			owner.input.mu.Unlock()
		} // pending/failed original pairs remain attached; no replacement/retry
	}()
	<-owner.acquired
	_ = owner.closeEnds() // BOTH pipe ends before original IO/handler/callback joins
	if !owner.closeReturned || owner.closeErr != nil {
		returned = true
		return
	}
	<-owner.ioDone
	owner.mu.Lock()
	started := owner.handlerStarted
	owner.mu.Unlock()
	if started {
		<-owner.handlerDone
		owner.mu.Lock()
		failed := owner.failed || !owner.handlerReturned
		owner.mu.Unlock()
		if failed {
			returned = true
			return // never fabricate a missing SDK callback after a failed frame
		}
		<-owner.callbackDone
	}
	returned = true
}

func (input *mixedIngress) sealHTTPPipes() []*mixedHTTPPipe {
	input.mu.Lock()
	input.httpPipesSealed = true
	owners := make([]*mixedHTTPPipe, 0, len(input.httpPipes))
	for owner := range input.httpPipes {
		owners = append(owners, owner)
	}
	input.mu.Unlock()
	for _, owner := range owners {
		owner.cancel()
	}
	return owners
}
