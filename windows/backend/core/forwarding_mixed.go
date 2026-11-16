package core

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// The pinned SOCKS/HTTP parsers are reused by a local mixed dispatcher with a
// retained UDP PacketListener. This adapter owns the sole TCP listen/accept path
// and registers the original socket BEFORE metadata, scheduling or handshake.
// The public normalized Config has neither inbound TLS nor SDK system-proxy
// ownership. Reject those private options rather than silently omit a lifecycle.
type ownedMixedInbound struct {
	handler adapter.ConnectionHandlerEx
	tag     string
	listen  *listener.Listener
	options option.ListenOptions
	ctx     context.Context
	gate    *forwardingAdmission

	mu                            sync.Mutex
	startStarted, startReturned   bool
	startDone                     chan struct{}
	startErr                      error
	acceptStarted, acceptReturned bool
	acceptFailed                  bool
	acceptErr                     error
	acceptDone                    chan struct{}
	closing, closeStarted         bool
	closeDone, closeReturned      bool
	closeErr                      error
}

func newOwnedMixedInbound(ctx context.Context, router admissionRouter, logger log.ContextLogger, tag string, raw any) (adapter.Inbound, error) {
	options, ok := raw.(*option.HTTPMixedInboundOptions)
	if !ok || options == nil || options.TLS != nil || options.SetSystemProxy || router.admission == nil || !router.admission.mixedRates.configured() || contextError(ctx) != nil {
		return nil, ErrForwardingNotAdmitted
	}
	inbound := &ownedMixedInbound{tag: tag, ctx: ctx, gate: router.admission,
		options: options.ListenOptions, startDone: make(chan struct{}), acceptDone: make(chan struct{}),
		listen: listener.New(listener.Options{Context: ctx, Logger: logger, Network: []string{N.NetworkTCP}, Listen: options.ListenOptions})}
	inbound.handler = newOwnedMixedProcessor(router, logger, tag, *options, inbound)
	return inbound, nil
}

func (inbound *ownedMixedInbound) Type() string { return C.TypeMixed }
func (inbound *ownedMixedInbound) Tag() string  { return inbound.tag }

func (inbound *ownedMixedInbound) Start(stage adapter.StartStage) (finalErr error) {
	if stage != adapter.StartStateStart {
		return nil
	}
	inbound.gate.mu.Lock()
	closed := inbound.gate.closed || inbound.gate.routeFault
	inbound.gate.mu.Unlock()
	if closed || contextError(inbound.ctx) != nil {
		return ErrForwardingNotAdmitted
	}
	inbound.mu.Lock()
	if inbound.closing || inbound.startStarted {
		inbound.mu.Unlock()
		return ErrEngineCleanupFailed
	}
	inbound.startStarted = true // original Listen attempt belongs to this adapter
	inbound.mu.Unlock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		inbound.mu.Lock()
		inbound.startReturned, inbound.startErr = returned, finalErr
		close(inbound.startDone)
		inbound.mu.Unlock()
		if fault {
			inbound.gate.noteRouteFault()
		}
	}()
	original, err := inbound.listen.ListenTCP() // existing SDK socket options, no second bind
	if err != nil {
		returned = true
		return err
	}
	inbound.mu.Lock()
	if inbound.closing || original == nil {
		inbound.mu.Unlock()
		returned = true
		return ErrEngineCleanupFailed // original SDK listener retained; no replacement
	}
	inbound.acceptStarted = true // publish original task before scheduling it
	inbound.mu.Unlock()
	go inbound.accept(original)
	returned = true
	return nil
}

func (inbound *ownedMixedInbound) isClosing() bool {
	inbound.mu.Lock()
	defer inbound.mu.Unlock()
	return inbound.closing
}

func (inbound *ownedMixedInbound) accept(original net.Listener) {
	returned := false
	failed := false
	var result error
	var retryDelay time.Duration
	defer func() {
		fault := recover() != nil || !returned || failed
		inbound.mu.Lock()
		inbound.acceptReturned, inbound.acceptFailed, inbound.acceptErr = returned, fault, result
		close(inbound.acceptDone)
		inbound.mu.Unlock()
		if fault {
			inbound.gate.noteRouteFault()
		}
	}()
	for !inbound.isClosing() && inbound.ctx.Err() == nil {
		if !inbound.waitMixedIntake() {
			returned = true
			return
		}
		raw, err := original.Accept()
		if raw != nil {
			owner, allowed := inbound.gate.registerAcceptedMixedInput(inbound.ctx, raw, nil)
			if owner == nil {
				// The existing rejection lane owns the over-budget socket.
			} else if err != nil || !allowed {
				owner.run(nil)
				owner.cancel()
			} else {
				go owner.run(func(conn net.Conn) {
					metadata := adapter.InboundContext{InboundDetour: inbound.options.Detour,
						Source: M.SocksaddrFromNet(conn.RemoteAddr()).Unwrap(), OriginDestination: M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()}
					inbound.handler.NewConnectionEx(log.ContextWithNewID(owner.ctx), conn, metadata, nil)
				})
			}
		}
		if err != nil || raw == nil {
			result = err
			if err != nil && !inbound.isClosing() {
				if temporary, ok := err.(net.Error); ok && temporary.Temporary() {
					if retryDelay == 0 {
						retryDelay = 5 * time.Millisecond
					} else if retryDelay < time.Second {
						retryDelay *= 2
						if retryDelay > time.Second {
							retryDelay = time.Second
						}
					}
					if waitMixedAcceptRetry(inbound.ctx, retryDelay) {
						continue // retained Accept task, no busy loop or replacement listener
					}
					returned = true
					return
				}
			}
			if !inbound.isClosing() {
				failed = true
				if result == nil {
					result = ErrEngineCleanupFailed
				}
				inbound.gate.noteRouteFault() // terminal original Accept failure, no error busy loop
			}
			returned = true
			return
		}
		retryDelay, result = 0, nil
	}
	returned = true
}

func waitMixedAcceptRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err() == nil // retry scheduling, never a cleanup receipt
	case <-ctx.Done():
		return false
	}
}

func (inbound *ownedMixedInbound) NewConnectionEx(ctx context.Context, raw net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if raw == nil || ctx == nil {
		_ = (admissionRouter{admission: inbound.gate}).rejectIngress(raw, onClose, ErrForwardingNotAdmitted)
		return
	}
	owner, allowed := inbound.gate.registerMixedInput(ctx, raw, onClose)
	if owner == nil {
		return // rejection lane already owns Close/callback, including uncertainty
	}
	if !allowed {
		owner.run(nil)
		owner.cancel()
		return
	}
	var callback N.CloseHandlerFunc
	if onClose != nil {
		callback = owner.callback
	}
	owner.run(func(conn net.Conn) { inbound.handler.NewConnectionEx(owner.ctx, conn, metadata, callback) })
}

func (inbound *ownedMixedInbound) Close() (finalErr error) {
	inbound.mu.Lock()
	inbound.closing = true
	if inbound.closeStarted {
		err := inbound.closeErr
		if !inbound.closeDone || !inbound.closeReturned {
			err = ErrEngineCleanupFailed
		}
		inbound.mu.Unlock()
		return err
	}
	inbound.closeStarted, inbound.closeErr = true, ErrEngineCleanupFailed
	startStarted := inbound.startStarted
	inbound.mu.Unlock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		inbound.mu.Lock()
		inbound.closeDone, inbound.closeReturned, inbound.closeErr = true, returned, finalErr
		inbound.mu.Unlock()
		if fault || finalErr != nil {
			inbound.gate.noteRouteFault()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	if startStarted && !mixedOriginalDone(ctx, inbound.startDone) {
		returned = true
		return ErrEngineCleanupFailed // pending Listen retains original native owner
	}
	listenErr := cleanupCall("close original mixed listener", inbound.listen.Close)
	inbound.mu.Lock()
	acceptStarted, startReturned := inbound.acceptStarted, inbound.startReturned
	inbound.mu.Unlock()
	var taskErr error
	if startStarted && !startReturned {
		taskErr = ErrEngineCleanupFailed
	}
	if acceptStarted {
		if !mixedOriginalDone(ctx, inbound.acceptDone) {
			taskErr = ErrEngineCleanupFailed
		} else {
			inbound.mu.Lock()
			if !inbound.acceptReturned || inbound.acceptFailed {
				taskErr = ErrEngineCleanupFailed
			}
			inbound.mu.Unlock()
		}
	}
	returned = true
	return errors.Join(listenErr, taskErr)
}

var _ adapter.TCPInjectableInbound = (*ownedMixedInbound)(nil)
