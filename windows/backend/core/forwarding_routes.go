package core

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	N "github.com/sagernet/sing/common/network"
)

// Initial per-box receipt budget, including pending/failed retirement. This is
// not a byte/RSS or throughput guarantee; exhaustion never bypasses the guard.
const maxForwardingRoutes = 4096

type forwardingClose struct {
	admission *forwardingAdmission
	routes    []*forwardingRoute
	inputs    []*mixedIngress // original pre-SDK cancellation cohort; post-SDK drain takes its own sealed view
	scope     *tunDiagnosticScope
}

type forwardingRoute struct {
	gate                     *forwardingAdmission
	epoch                    uint64
	subject, selector        string
	ctx                      context.Context
	cancel                   context.CancelFunc
	stop                     <-chan struct{}
	raw                      io.Closer
	onClose                  N.CloseHandlerFunc
	done, callbackDone       chan struct{}
	closeWait, ioDone        chan struct{}
	callbackStarted          atomic.Bool
	controlMu                sync.Mutex // serialize lazy control/handshake with original Close
	mu                       sync.Mutex // values only; never held during SDK/native/callback work
	closing, terminal, fault bool
	ioPending                int
	ioUnconfirmed            bool
	ioDoneClosed             bool
	routeDone, routeReturned bool
	callbackFinished         bool
	callbackReturned         bool
	sdkCallbackReturned      bool
	closeStarted, closeDone  bool
	closeReturned            bool
	closeTaskDone            bool
	callbackErr, closeErr    error
	result                   error // immutable after done
	dns                      *forwardingDNSScope
	dnsSealed                bool
}

// Publish original ingress/context before SDK routing or cancellation work.
// The admission mutex serializes this claim with permanent gate revocation.
func (router admissionRouter) registerRoute(ctx context.Context, raw io.Closer, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) (*forwardingRoute, error) {
	gate := router.admission
	if ctx == nil || contextError(ctx) != nil || raw == nil || gate == nil || metadata.Inbound != router.tag || metadata.InboundType != router.inboundType {
		return nil, ErrForwardingNotAdmitted
	}
	gate.mu.Lock()
	if gate.closed || gate.routeFault || !gate.open.Load() || gate.epoch == 0 || !workerSubject(gate.subject) || !validWorkerTag(gate.selector) || len(gate.routes) >= maxForwardingRoutes {
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	if gate.routes == nil {
		gate.routes = make(map[*forwardingRoute]struct{})
	}
	if gate.routeStop == nil {
		gate.routeStop = make(chan struct{})
	}
	lifetime, cancel := context.WithCancel(ctx)
	route := &forwardingRoute{gate: gate, epoch: gate.epoch, subject: gate.subject, selector: gate.selector,
		ctx: lifetime, cancel: cancel, stop: gate.routeStop, raw: raw, onClose: onClose,
		done: make(chan struct{}), callbackDone: make(chan struct{}), closeWait: make(chan struct{}), ioDone: make(chan struct{})}
	gate.routes[route] = struct{}{}
	gate.mu.Unlock()
	// Deliberately never stop/replace this callback. Even a pre-install cancel
	// must run the same Close task; its actual return is an independent fence.
	context.AfterFunc(lifetime, route.closeTask)
	return route, nil
}

func (route *forwardingRoute) current() bool {
	gate := route.gate
	gate.mu.Lock()
	_, owned := gate.routes[route]
	valid := owned && !gate.closed && !gate.routeFault && gate.open.Load() && gate.epoch == route.epoch && gate.subject == route.subject && gate.selector == route.selector
	gate.mu.Unlock()
	return valid && contextError(route.ctx) == nil
}

// Call the existing Ex API for both public entry points. Deprecated SDK route
// return/box cancellation is not a receipt for its original callback or I/O.
func (route *forwardingRoute) delegate(call func(context.Context, N.CloseHandlerFunc)) {
	returned := false
	defer func() {
		route.mu.Lock()
		route.routeDone, route.routeReturned = true, returned
		route.mu.Unlock()
		route.tryFinish()
	}()
	defer func() {
		recovered := recover()
		if recovered != nil || !returned {
			route.markFault()
			route.callback(ErrEngineCleanupFailed)
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // fail the original worker, no partial-I/O success
		}
	}()
	if !route.current() {
		route.callback(ErrForwardingNotAdmitted)
		returned = true
		return
	}
	call(route.ctx, route.callback)
	returned = true
}

func (route *forwardingRoute) callback(err error) {
	if !route.callbackStarted.CompareAndSwap(false, true) {
		return
	}
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		route.mu.Lock()
		route.callbackErr, route.sdkCallbackReturned = err, returned
		route.fault = route.fault || fault
		route.mu.Unlock()
		route.cancel()
		if fault {
			route.markFault()
		}
		close(route.callbackDone)
	}()
	// Do not Close or deliver the upstream callback synchronously here: raw
	// Close may invoke this SDK callback itself. The retained cancellation
	// task waits for it and for the original Close/I/O before delivery.
	returned = true
}

func (route *forwardingRoute) closeTask() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		route.mu.Lock()
		route.closeTaskDone = true
		route.fault = route.fault || fault
		route.mu.Unlock()
		if fault {
			route.markFault()
		}
		route.tryFinish()
	}()
	dns := route.sealDNS()
	_ = route.closeOriginal()
	<-route.closeWait
	if dns != nil {
		if err := dns.retire(); err != nil {
			returned = true
			return
		}
	}
	<-route.ioDone
	<-route.callbackDone
	route.mu.Lock()
	confirmed := route.closeReturned && route.closeErr == nil && !route.ioUnconfirmed && route.sdkCallbackReturned
	err := route.callbackErr
	route.mu.Unlock()
	if confirmed {
		route.deliverCallback(err)
	} // unknown cleanup keeps the original callback/receipt; no alias Close retry
	returned = true
}

func (route *forwardingRoute) deliverCallback(err error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		route.mu.Lock()
		route.callbackFinished, route.callbackReturned = true, returned
		route.fault = route.fault || fault
		route.mu.Unlock()
		if fault {
			route.markFault()
		}
	}()
	if route.onClose != nil {
		route.onClose(err)
	}
	returned = true
}

// Only the first caller invokes raw Close. A concurrent/reentrant call returns
// pending instead of waiting on itself or turning a second ErrClosed into proof.
func (route *forwardingRoute) closeOriginal() (finalErr error) {
	route.mu.Lock()
	route.closing = true
	if route.ioPending == 0 && !route.ioDoneClosed {
		route.ioDoneClosed = true
		close(route.ioDone)
	}
	if route.closeStarted {
		err := route.closeErr
		if !route.closeDone {
			err = ErrEngineCleanupFailed
		}
		route.mu.Unlock()
		return err
	}
	route.closeStarted, route.closeErr = true, ErrEngineCleanupFailed
	route.mu.Unlock()
	route.cancel()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		route.mu.Lock()
		route.closeErr, route.closeDone, route.closeReturned = finalErr, true, returned
		route.fault = route.fault || fault || finalErr != nil
		close(route.closeWait)
		route.mu.Unlock()
		if fault || finalErr != nil {
			route.markFault()
		}
		route.tryFinish()
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	route.controlMu.Lock()
	defer route.controlMu.Unlock()
	finalErr = closeForwardingIngress(route.gate, route.raw, route)
	returned = true
	return finalErr
}

// Count actual raw calls, including controls. Revocation prevents new entry;
// already entered OS calls may still be returning and are never called retired
// merely because Close/cancel returned. Nothing holds gate.mu during I/O.
func (route *forwardingRoute) perform(call func() error) (finalErr error) {
	route.mu.Lock()
	if route.closing || contextError(route.ctx) != nil || !route.gate.open.Load() {
		route.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	route.ioPending++
	route.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		route.mu.Lock()
		route.ioPending--
		route.ioUnconfirmed = route.ioUnconfirmed || fault
		if route.closing && route.ioPending == 0 && !route.ioDoneClosed {
			route.ioDoneClosed = true
			close(route.ioDone)
		}
		route.mu.Unlock()
		if fault {
			route.markFault()
		}
		route.tryFinish()
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // do not return an invented raw-I/O result/buffer ownership
		}
	}()
	finalErr = call()
	returned = true
	return finalErr
}

func (route *forwardingRoute) control(call func() error) error {
	route.controlMu.Lock()
	defer route.controlMu.Unlock()
	return route.perform(call)
}

func (route *forwardingRoute) markFault() {
	route.mu.Lock()
	route.fault = true
	route.mu.Unlock()
	route.gate.noteRouteFault()
}

func (gate *forwardingAdmission) noteRouteFault() {
	gate.open.Store(false)
	gate.mu.Lock()
	if gate.routeFault {
		gate.mu.Unlock()
		return // one sweep, not one O(N) allocation for every failed stream
	}
	gate.routeFault = true
	routes := gate.sealRoutesLocked()
	inputs := gate.mixedInputsLocked()
	gate.clearPendingLocked()
	gate.nativeReceipt = nil
	gate.mu.Unlock()
	for _, original := range routes {
		original.cancel()
	}
	for _, original := range inputs {
		original.cancel()
	}
}

func (route *forwardingRoute) tryFinish() {
	route.mu.Lock()
	if route.terminal || !route.routeDone || !route.callbackFinished || !route.closeTaskDone || !route.closeDone || route.ioPending != 0 {
		route.mu.Unlock()
		return
	}
	route.fault = route.fault || !route.routeReturned || !route.callbackReturned || !route.closeReturned || route.ioUnconfirmed || route.closeErr != nil
	route.result = errors.Join(route.callbackErr, route.closeErr)
	if route.fault {
		route.result = errors.Join(route.result, ErrEngineCleanupFailed)
	}
	route.terminal = true
	fault := route.fault
	close(route.done) // local raw-call/Close/callback/frame fences, not opaque upstream proof
	route.mu.Unlock()
	if !fault {
		route.gate.mu.Lock()
		delete(route.gate.routes, route)
		route.gate.mu.Unlock()
	} // failed original receipts stay attached to this exact box
}

func (route *forwardingRoute) wait(ctx context.Context) error {
	select {
	case <-route.done:
		return route.result
	case <-ctx.Done():
		return route.pendingResult(ctx.Err())
	case <-route.stop:
		return route.pendingResult(ErrForwardingNotAdmitted)
	case <-route.callbackDone:
		// Streaming may live indefinitely, but callback completion starts one
		// bounded local drain. Timeout retains the same original record.
		cleanup, cancel := context.WithTimeout(ctx, workerControlTimeout)
		defer cancel()
		select {
		case <-route.done:
			return route.result
		case <-cleanup.Done():
			return route.pendingResult(cleanup.Err())
		case <-route.stop:
			return route.pendingResult(ErrForwardingNotAdmitted)
		}
	}
}

func (route *forwardingRoute) pendingResult(cause error) error {
	select {
	case <-route.done:
		return route.result
	default:
		return errors.Join(ErrEngineCleanupFailed, cause)
	}
}

// Caller holds gate.mu. This closes registration before capturing the exact
// original records; there is no reset or adoption after permanent revocation.
func (gate *forwardingAdmission) sealRoutesLocked() []*forwardingRoute {
	gate.revokeRegistrationLocked()
	routes := make([]*forwardingRoute, 0, len(gate.routes))
	for route := range gate.routes {
		routes = append(routes, route)
	}
	return routes
}

func (gate *forwardingAdmission) revokeRegistrationLocked() {
	gate.open.Store(false)
	if !gate.closed {
		gate.closed = true
		if gate.routeStop != nil {
			close(gate.routeStop)
		}
	}
}

func (gate *forwardingAdmission) drainRoutes(ctx context.Context, routes []*forwardingRoute) error {
	unconfirmed := false
	for _, route := range routes {
		select {
		case <-route.done:
		case <-ctx.Done():
		}
		select {
		case <-route.done:
			route.mu.Lock()
			unconfirmed = unconfirmed || route.fault
			route.mu.Unlock()
		default:
			unconfirmed = true
		}
	}
	gate.mu.Lock()
	unconfirmed = unconfirmed || gate.routeFault
	gate.mu.Unlock()
	if unconfirmed {
		return ErrEngineCleanupFailed
	}
	return nil
}

// Negative-only local custody fact, also consumed by the existing worker probe
// report path. A closed ingress by itself is not a network/channel diagnosis.
func (gate *forwardingAdmission) localRouteFault() error {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.routeFault {
		return ErrEngineCleanupFailed
	}
	return nil
}
