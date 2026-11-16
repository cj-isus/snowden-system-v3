package core

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio/deadline"
	N "github.com/sagernet/sing/common/network"
)

// Original mixed TCP socket custody is independent of forwarding admission:
// the local SOCKS/HTTP handshake must also serve the narrowly owned probe while
// the ordinary gate is closed. This owner never permits an outbound connection.
const maxMixedInputs = maxForwardingRoutes

type mixedInputKey struct{}

type mixedIngress struct {
	gate                   *forwardingAdmission
	raw                    net.Conn
	local, remote          net.Addr // captured once before SDK entry; usable after Close without raw calls
	ctx                    context.Context
	cancel                 context.CancelFunc
	packet                 *mixedPacket                // one original UDP bind attempt; never replaced after error
	prefetch               *mixedPrefetch              // one original CONNECT parser remainder; no late replacement
	httpPipes              map[*mixedHTTPPipe]struct{} // original pairs retained before acquisition/Router
	httpPipesSealed        bool
	httpBodies             map[*mixedHTTPBody]struct{}
	httpBodiesSealed       bool
	httpDials              map[*mixedHTTPDialScope]struct{}
	httpDialsSealed        bool
	handshakeBudgets       map[*mixedHandshakeBudget]struct{}
	handshakeBudgetsSealed bool
	socksReply             *mixedSOCKSConnection
	socksReplySealed       bool

	mu, controlMu                  sync.Mutex
	closing, failed                bool
	ioPending                      int
	ioDoneClosed                   bool
	closeStarted, closeReturned    bool
	closeErr                       error
	handlerReturned                bool
	callbackStarted                atomic.Bool
	callbackErr                    error
	callbackReturned               bool
	onClose                        N.CloseHandlerFunc
	closeWait, ioDone, handlerDone chan struct{}
	callbackDone, done             chan struct{}
}

// Native Accept consumed one intake turn before acquiring raw. Injected inputs
// enter through registerMixedInput and use the same bucket/rejection custody.
// This registration still precedes metadata, scheduling and all socket APIs.
func (gate *forwardingAdmission) registerAcceptedMixedInput(ctx context.Context, raw net.Conn, callback N.CloseHandlerFunc) (*mixedIngress, bool) {
	gate.mu.Lock()
	if gate.rejectionsSealed {
		gate.routeFault = true
		gate.revokeRegistrationLocked()
		worker := gate.workerCustody
		gate.mu.Unlock()
		stopUntrackedIngress(worker) // original process root owns the untrackable accepted socket
	}
	if len(gate.mixedInputs) >= maxMixedInputs {
		gate.mu.Unlock()
		// Pressure alone need not kill healthy flows. Reuse the bounded original
		// rejection lane, synchronously applying Accept backpressure if Close
		// blocks. Only an untrackable/full rejection lane invokes worker fail-stop.
		_ = (admissionRouter{admission: gate}).rejectIngress(raw, callback, ErrForwardingNotAdmitted)
		return nil, false
	}
	if gate.mixedInputs == nil {
		gate.mixedInputs = make(map[*mixedIngress]struct{})
	}
	lifetime, cancel := context.WithCancel(ctx)
	owner := &mixedIngress{gate: gate, raw: raw, ctx: lifetime, cancel: cancel, onClose: callback,
		closeWait: make(chan struct{}), ioDone: make(chan struct{}), handlerDone: make(chan struct{}),
		callbackDone: make(chan struct{}), done: make(chan struct{})}
	owner.ctx = context.WithValue(lifetime, mixedInputKey{}, owner)
	gate.mixedInputs[owner] = struct{}{} // BEFORE metadata/goroutine/SDK/raw work
	allowed := !gate.closed && !gate.routeFault && ctx.Err() == nil
	gate.mu.Unlock()
	context.AfterFunc(lifetime, owner.closeTask) // original task retained; never stop/replace it
	return owner, allowed
}

// Caller holds gate.mu; the same flag that seals rejection records seals these
// inputs after Box.Close. No forgotten post-snapshot registration can succeed.
func (gate *forwardingAdmission) mixedInputsLocked() []*mixedIngress {
	inputs := make([]*mixedIngress, 0, len(gate.mixedInputs))
	for original := range gate.mixedInputs {
		inputs = append(inputs, original)
	}
	return inputs
}

func (owner *mixedIngress) run(call func(net.Conn)) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		owner.handlerReturned = returned
		owner.failed = owner.failed || fault
		close(owner.handlerDone)
		owner.mu.Unlock()
		if fault {
			owner.gate.noteRouteFault()
			owner.cancel()
		}
	}()
	if call != nil && owner.ctx.Err() == nil {
		var local, remote net.Addr
		if err := owner.perform(func() error { local, remote = owner.raw.LocalAddr(), owner.raw.RemoteAddr(); return nil }); err != nil {
			if owner.onClose != nil {
				owner.callback(ErrForwardingNotAdmitted)
			}
			returned = true
			return // canceled before metadata acquisition; no SDK producer exists
		}
		owner.mu.Lock()
		owner.local, owner.remote = local, remote
		owner.mu.Unlock()
		call(&mixedIngressConnection{owner})
	} else if owner.onClose != nil {
		owner.callback(ErrForwardingNotAdmitted) // no SDK handler was entered
	}
	returned = true // returning a delegated handler does NOT close its socket
}

func (owner *mixedIngress) callback(err error) {
	if !owner.callbackStarted.CompareAndSwap(false, true) {
		return
	}
	owner.mu.Lock()
	owner.callbackErr = err
	owner.mu.Unlock()
	close(owner.callbackDone)
	// Stock mixed may report nil when its handler returns after delegation.
	// This notification never closes an input or grants cleanup/forwarding proof.
}

func (owner *mixedIngress) closeOriginal() (finalErr error) {
	owner.mu.Lock()
	owner.closing = true
	if owner.ioPending == 0 && !owner.ioDoneClosed {
		owner.ioDoneClosed = true
		close(owner.ioDone)
	}
	if owner.closeStarted {
		err := owner.closeErr
		if !owner.closeReturned {
			err = ErrEngineCleanupFailed
		}
		owner.mu.Unlock()
		return err // pending/reentrant is not a second native attempt
	}
	owner.closeStarted, owner.closeErr = true, ErrEngineCleanupFailed
	owner.mu.Unlock()
	owner.cancel()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.closeReturned, owner.closeErr = returned, finalErr
		owner.failed = owner.failed || fault || finalErr != nil
		close(owner.closeWait)
		owner.mu.Unlock()
		if fault || finalErr != nil {
			owner.gate.noteRouteFault()
		}
	}()
	owner.controlMu.Lock()
	defer owner.controlMu.Unlock()
	finalErr = closeForwardingIngress(owner.gate, owner.raw, nil)
	returned = true
	return finalErr
}

func (owner *mixedIngress) joinClose() error {
	owner.mu.Lock()
	started := owner.closeStarted
	owner.mu.Unlock()
	if !started {
		_ = owner.closeOriginal()
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	if !mixedOriginalDone(ctx, owner.closeWait) {
		return ErrEngineCleanupFailed
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.closeReturned || owner.closeErr != nil {
		return errors.Join(ErrEngineCleanupFailed, owner.closeErr)
	}
	return nil
}

func (owner *mixedIngress) closeTask() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		owner.mu.Lock()
		owner.failed = owner.failed || fault || !owner.closeReturned || owner.closeErr != nil || !owner.handlerReturned || owner.ioPending != 0 || !owner.callbackReturned
		failed := owner.failed
		owner.mu.Unlock()
		if failed {
			owner.gate.noteRouteFault()
		}
		close(owner.done) // this original task; not opaque SDK/UDP/whole Job retirement
		if !failed {
			owner.gate.mu.Lock()
			delete(owner.gate.mixedInputs, owner)
			owner.gate.mu.Unlock()
		}
	}()
	socksReply := owner.sealSOCKSReply()
	handshakeBudgets := owner.sealHandshakeBudgets()
	httpDials := owner.sealHTTPDials() // original request cancellation before pipe/native/handler waits
	httpPipes := owner.sealHTTPPipes() // stop both pipe ends before native/handler waits
	httpBodies := owner.sealHTTPBodies()
	_ = owner.closeOriginal()
	<-owner.closeWait
	for _, original := range handshakeBudgets {
		if err := original.Close(); err != nil {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // timer/callback original retained; never pretend it is retired
		}
	}
	owner.mu.Lock()
	confirmed := owner.closeReturned && owner.closeErr == nil
	owner.mu.Unlock()
	if !confirmed {
		returned = true
		return // failed owner retained, no invented callback/handler/raw-I/O receipt
	}
	if socksReply != nil {
		if err := socksReply.retire(); err != nil {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // retain original reply/peer/phase receipts; no fabricated handoff or retry
		}
	}
	for _, original := range httpBodies {
		_ = original.Close() // native TCP/pipe cancellation precedes possibly draining Body.Close
	}
	<-owner.ioDone
	<-owner.handlerDone
	owner.mu.Lock()
	failed := owner.failed
	packet := owner.packet // handler has returned and closing forbids a new bind
	prefetch := owner.prefetch
	owner.mu.Unlock()
	if failed {
		returned = true
		return // a failed handler cannot fabricate its missing SDK callback
	}
	if prefetch != nil {
		if err := prefetch.retire(); err != nil {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // keep the exact original input and uncertain prefix owner
		}
	}
	if packet != nil {
		<-packet.done
		packet.mu.Lock()
		packetFailed := packet.failed
		packet.mu.Unlock()
		if packetFailed {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // retain the exact original TCP/UDP owner pair
		}
	}
	for _, original := range httpPipes {
		<-original.done
		original.mu.Lock()
		pipeFailed := original.failed
		original.mu.Unlock()
		if pipeFailed {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // original failed pair stays attached; no fabricated task receipt
		}
	}
	for _, original := range httpBodies {
		<-original.done
		original.mu.Lock()
		bodyFailed := original.failed
		original.mu.Unlock()
		if bodyFailed {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // retain the exact original failed/unknown body
		}
	}
	for _, original := range httpDials {
		if err := original.Close(); err != nil {
			owner.mu.Lock()
			owner.failed = true
			owner.mu.Unlock()
			returned = true
			return // the exact scope and its pending/failed original dial records stay attached
		}
	}
	if owner.onClose != nil {
		<-owner.callbackDone
	}
	owner.mu.Lock()
	confirmed = !owner.failed && owner.handlerReturned
	err := owner.callbackErr
	owner.mu.Unlock()
	if confirmed {
		if owner.onClose != nil {
			owner.onClose(err) // actual return is included in the original task fence
		}
		owner.mu.Lock()
		owner.callbackReturned = true
		owner.mu.Unlock()
	}
	returned = true
}

func (owner *mixedIngress) perform(call func() error) (finalErr error) {
	owner.mu.Lock()
	if owner.closing || owner.ctx.Err() != nil {
		owner.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	owner.ioPending++
	owner.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.ioPending--
		owner.failed = owner.failed || fault
		if owner.closing && owner.ioPending == 0 && !owner.ioDoneClosed {
			owner.ioDoneClosed = true
			close(owner.ioDone)
		}
		owner.mu.Unlock()
		if fault {
			owner.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // do not invent a raw partial-I/O result
		}
	}()
	finalErr = call()
	returned = true
	return finalErr
}

func (owner *mixedIngress) control(call func() error) error {
	owner.controlMu.Lock()
	defer owner.controlMu.Unlock()
	return owner.perform(call)
}

func mixedOriginalDone(ctx context.Context, done <-chan struct{}) bool {
	select {
	case <-done:
	case <-ctx.Done():
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func (gate *forwardingAdmission) drainMixedInputs(ctx context.Context, inputs []*mixedIngress) error {
	for _, input := range inputs {
		if !mixedOriginalDone(ctx, input.done) {
			return ErrEngineCleanupFailed
		}
		input.mu.Lock()
		failed := input.failed
		input.mu.Unlock()
		if failed {
			return ErrEngineCleanupFailed
		}
	}
	return nil
}

type mixedIngressConnection struct{ owner *mixedIngress }

func (conn *mixedIngressConnection) Read(buffer []byte) (n int, err error) {
	err = conn.owner.perform(func() error { n, err = conn.owner.raw.Read(buffer); return err })
	return
}

func (conn *mixedIngressConnection) Write(buffer []byte) (n int, err error) {
	err = conn.owner.perform(func() error { n, err = conn.owner.raw.Write(buffer); return err })
	return
}

func (conn *mixedIngressConnection) Close() error { return conn.owner.closeOriginal() }
func (conn *mixedIngressConnection) LocalAddr() (address net.Addr) {
	conn.owner.mu.Lock()
	defer conn.owner.mu.Unlock()
	return conn.owner.local
}
func (conn *mixedIngressConnection) RemoteAddr() (address net.Addr) {
	conn.owner.mu.Lock()
	defer conn.owner.mu.Unlock()
	return conn.owner.remote
}
func (conn *mixedIngressConnection) SetDeadline(value time.Time) error {
	return conn.owner.control(func() error { return conn.owner.raw.SetDeadline(value) })
}
func (conn *mixedIngressConnection) SetReadDeadline(value time.Time) error {
	return conn.owner.control(func() error { return conn.owner.raw.SetReadDeadline(value) })
}
func (conn *mixedIngressConnection) SetWriteDeadline(value time.Time) error {
	return conn.owner.control(func() error { return conn.owner.raw.SetWriteDeadline(value) })
}
func (conn *mixedIngressConnection) HandshakeSuccess() error {
	return conn.owner.control(func() error { return N.ReportHandshakeSuccess(conn.owner.raw) })
}
func (conn *mixedIngressConnection) ConnHandshakeSuccess(peer net.Conn) error {
	return conn.owner.control(func() error { return N.ReportConnHandshakeSuccess(conn.owner.raw, peer) })
}
func (conn *mixedIngressConnection) PacketConnHandshakeSuccess(peer net.PacketConn) error {
	return conn.owner.control(func() error { return N.ReportPacketConnHandshakeSuccess(conn.owner.raw, peer) })
}
func (conn *mixedIngressConnection) HandshakeFailure(cause error) error {
	return conn.owner.control(func() error {
		if reporter, ok := common.Cast[N.HandshakeFailure](conn.owner.raw); ok {
			return reporter.HandshakeFailure(cause)
		}
		return nil
	})
}
func (conn *mixedIngressConnection) NeedHandshakeForRead() (needed bool) {
	_ = conn.owner.control(func() error { needed = N.NeedHandshakeForRead(conn.owner.raw); return nil })
	return
}
func (conn *mixedIngressConnection) NeedHandshakeForWrite() (needed bool) {
	_ = conn.owner.control(func() error { needed = N.NeedHandshakeForWrite(conn.owner.raw); return nil })
	return
}
func (conn *mixedIngressConnection) NeedAdditionalReadDeadline() (needed bool) {
	_ = conn.owner.control(func() error { needed = deadline.NeedAdditionalReadDeadline(conn.owner.raw); return nil })
	return
}
func (*mixedIngressConnection) ReaderReplaceable() bool { return false }
func (*mixedIngressConnection) WriterReplaceable() bool { return false }

var _ net.Conn = (*mixedIngressConnection)(nil)
