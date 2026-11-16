package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

// Exactly one native diagnostic, with both original streams and the actual
// selected adapter captured from the live box. No Router/rules, listener, DNS,
// public dialer or direct fallback can retarget this one-flow operation.
type nativeTUNFlow struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	ingress  *tunScopedConnection
	upstream *tunScopedConnection
	selector *admissionSelector
	selected string
	outbound adapter.Outbound
	metadata adapter.InboundContext
	onClose  N.CloseHandlerFunc
	bound    bool
	fault    bool
	err      error
}

// This owner's code does not acquire admission.mu. Its external callback may
// reenter SDK routing, so retirement waits for I/O/callback outside that lock.
func (flow *nativeTUNFlow) run() {
	var result error = ErrNativeScopeUnavailable
	complete := false
	onClose, cancel := flow.onClose, flow.cancel // original callbacks, never a later replacement
	defer func() {
		fault := recover() != nil || !complete
		sequence := &lifecycleCleanup{}
		defer func() {
			sequence.drained = true // frame drainage alone is not a native retirement receipt
			flow.mu.Lock()
			flow.fault = fault || sequence.result != nil
			flow.err = errors.Join(result, sequence.result)
			if flow.fault {
				flow.err = errors.Join(ErrNativeScopeUnavailable, flow.err) // local custody fault, never a remote-path diagnosis
			}
			flow.mu.Unlock()
			close(flow.done) // a faulted terminal frame is observable, never proven/retired success
		}()
		// Register every original step before any callback: Goexit in cancel or
		// stream Close must still attempt later cleanup and publish a failed end.
		defer lifecycleCleanupStep("retire native TCP callback", sequence, func() error {
			if onClose != nil {
				if result != nil || sequence.result != nil || fault {
					onClose(ErrNativeScopeUnavailable) // finite/redacted failure only
				} else {
					onClose(nil)
				}
			}
			return nil
		})
		defer lifecycleCleanupStep("retire native TCP streams", sequence, flow.closeStreams)
		defer lifecycleCleanupStep("cancel native TCP flow", sequence, func() error { cancel(); return nil })
	}()
	result = flow.transfer()
	complete = true
}

func (flow *nativeTUNFlow) transfer() error {
	if contextError(flow.ctx) != nil || flow.selector == nil || flow.selector.ctx == nil || flow.selector.Now() != flow.selected || flow.outbound == nil || flow.outbound.Tag() != flow.selected {
		return ErrNativeScopeUnavailable
	}
	ctx := adapter.WithContext(workerOutboundContext{Context: flow.ctx, services: flow.selector.ctx}, &flow.metadata)
	ctx = context.WithValue(ctx, initialBootstrapProbeKey{}, flow.selector.admission)
	connection, dialErr := flow.outbound.DialContext(ctx, "tcp", flow.metadata.Destination)
	flow.mu.Lock()
	if connection != nil {
		flow.upstream = &tunScopedConnection{Conn: connection} // BEFORE checking partial/error/cancel
	}
	upstream := flow.upstream
	flow.mu.Unlock()
	if dialErr != nil || upstream == nil || contextError(flow.ctx) != nil || flow.selector.Now() != flow.selected {
		return ErrNativeScopeUnavailable
	}
	deadline, ok := flow.ctx.Deadline()
	if !ok || flow.ingress.SetDeadline(deadline) != nil || upstream.SetDeadline(deadline) != nil || contextError(flow.ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	if flow.ingress.handshake(upstream) != nil || contextError(flow.ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	flow.mu.Lock()
	flow.bound = true
	flow.mu.Unlock()
	// Pinned sing CopyConn joins BOTH copy tasks, including cancellation cleanup.
	// Merely queuing SDK NewConnectionEx/onClose would not be an I/O receipt.
	copyErr := bufio.CopyConn(flow.ctx, flow.ingress, upstream)
	if flow.selector.Now() != flow.selected {
		flow.mu.Lock()
		flow.bound = false
		flow.mu.Unlock()
		return ErrNativeScopeUnavailable
	}
	if copyErr != nil {
		return ErrNativeScopeUnavailable // HTTP/body verification remains in the original helper
	}
	return nil
}

func (flow *nativeTUNFlow) closeStreams() (result error) {
	flow.mu.Lock()
	ingress, upstream := flow.ingress, flow.upstream
	flow.mu.Unlock()
	sequence := &lifecycleCleanup{}
	defer func() { sequence.drained, result = true, sequence.result }()
	defer lifecycleCleanupStep("close original native upstream", sequence, func() error {
		if upstream != nil {
			return upstream.Close()
		}
		return nil
	})
	defer lifecycleCleanupStep("close original native ingress", sequence, func() error {
		if ingress != nil {
			return ingress.Close()
		}
		return nil
	})
	return nil
}

// Timeout bounds the done wait, not an arbitrary native Close syscall. A late
// Dial return is retained/closed by run before done. On uncertainty the scope
// stays owned and the parent retires the same original engine Job.
func (flow *nativeTUNFlow) retire() error {
	flow.cancel()
	closeErr := flow.closeStreams()
	timer := time.NewTimer(workerCloseTimeout)
	defer timer.Stop()
	select {
	case <-flow.done:
	case <-timer.C:
		return errors.Join(ErrNativeScopeUnavailable, closeErr)
	}
	flow.mu.Lock()
	fault := flow.fault
	flow.mu.Unlock()
	if fault {
		return ErrNativeScopeUnavailable
	}
	return errors.Join(closeErr, flow.closeStreams())
}

func (flow *nativeTUNFlow) wait(ctx context.Context) error {
	select {
	case <-flow.done:
		flow.mu.Lock()
		defer flow.mu.Unlock()
		return flow.err
	case <-ctx.Done():
		return ctx.Err() // scope still owns original flow until explicit retirement
	}
}

// Not a forwarding permit. The private close ACK additionally requires live
// scope/actor/epoch/deadline; parent accepts HTTP only after full helper retirement.
func (flow *nativeTUNFlow) proven() bool {
	select {
	case <-flow.done:
	default:
		return false
	}
	flow.mu.Lock()
	defer flow.mu.Unlock()
	return flow.bound && !flow.fault && flow.ingress != nil && flow.upstream != nil &&
		!flow.ingress.controlFault.Load() && !flow.upstream.controlFault.Load() &&
		flow.ingress.readBytes.Load() > 0 && flow.ingress.writtenBytes.Load() > 0 && flow.upstream.readBytes.Load() > 0 && flow.upstream.writtenBytes.Load() > 0
}
