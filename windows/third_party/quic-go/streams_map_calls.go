package quic

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

const maxStreamMapCalls = maxStreamDataCalls

var (
	errStreamMapCapacity  = errors.New("quic: stream map operation capacity exceeded")
	errStreamMapContext   = errors.New("quic: nil stream operation context")
	errStreamMapUncertain = errors.New("quic: stream map operation cleanup is uncertain")
)

type streamMapCallKind uint8

const (
	openBidiMapCall streamMapCallKind = iota
	openBidiSyncMapCall
	openUniMapCall
	openUniSyncMapCall
	acceptBidiMapCall
	acceptUniMapCall
)

// The original parent owns admission across child-map replacements. This mutex
// protects metadata only: no child lock, context method, factory or wait runs here.
type streamMapCalls struct {
	access     sync.Mutex
	calls      [maxStreamMapCalls]streamMapCall
	count      int
	running    int
	sealed     bool
	failed     bool
	loopActive bool
	settled    bool
	cause      error
	done       chan struct{}
	closeState *atomic.Pointer[closeError] // immutable original binding before exposure
	onFailure  func()
}

type streamMapCall struct {
	owner    *streamMapCalls
	active   bool
	ended    bool
	returned bool
	kind     streamMapCallKind
	ctx      context.Context
	target   any // actual child map, including one replaced after 0-RTT rejection
	result   any // original candidate remains map-owned if publication is refused
	err      error
}

func (o *streamMapCalls) init() {
	o.loopActive = true
	o.done = make(chan struct{})
}

func (o *streamMapCalls) settleLocked() {
	if o.sealed && !o.loopActive && o.running == 0 && !o.settled && o.done != nil {
		o.settled = true
		close(o.done)
	}
}

func (o *streamMapCalls) sealLocked(cause error) {
	if o.cause == nil {
		o.cause = cause
	}
	o.sealed = true
	o.settleLocked()
}

func (o *streamMapCalls) seal(cause error) {
	o.access.Lock()
	o.sealLocked(cause)
	o.access.Unlock()
}

func (o *streamMapCalls) stoppedErrorLocked() error {
	// The CAS precedes packet-input sealing and the Closing notification. Observe
	// that original record, including when the closing goroutine is preempted.
	if o.closeState != nil {
		if closed := o.closeState.Load(); closed != nil {
			o.sealLocked(closed.err)
		}
	}
	if o.failed || o.done == nil {
		return errStreamMapUncertain
	}
	if o.sealed || !o.loopActive {
		if o.cause != nil {
			return o.cause
		}
		return net.ErrClosed
	}
	return nil
}

func (o *streamMapCalls) begin(kind streamMapCallKind, ctx context.Context) (*streamMapCall, error) {
	if kind > acceptUniMapCall {
		return nil, errStreamMapUncertain
	}
	if ctx == nil && kind != openBidiMapCall && kind != openUniMapCall {
		return nil, errStreamMapContext
	}
	o.access.Lock()
	defer o.access.Unlock()
	if err := o.stoppedErrorLocked(); err != nil {
		return nil, err
	}
	if o.count >= len(o.calls) {
		return nil, errStreamMapCapacity // refuse only this extra caller
	}
	for i := range o.calls {
		call := &o.calls[i]
		if call.active {
			continue
		}
		*call = streamMapCall{owner: o, active: true, kind: kind, ctx: ctx}
		o.count++
		o.running++
		return call, nil
	}
	return nil, errStreamMapUncertain
}

// A late successful candidate is not handed to the caller or closed a second
// time here. Any still-live stream remains its original child map's shutdown
// responsibility; an already-completed incoming stream may have been removed.
func streamMapResult[T any](call *streamMapCall, candidate T, err error) (T, error) {
	call.result, call.err = candidate, err
	if err == errIncomingMapUncertain || err == errOutgoingMapUncertain {
		o := call.owner
		o.access.Lock()
		first := !o.failed
		o.failed = true
		o.sealLocked(err)
		o.access.Unlock()
		if first && o.onFailure != nil {
			o.onFailure() // actual invocation remains running through notification
		}
	}
	if err == nil {
		o := call.owner
		o.access.Lock()
		err = o.stoppedErrorLocked()
		o.access.Unlock()
		if err != nil {
			var zero T
			return zero, err
		}
	}
	return candidate, err
}

func (call *streamMapCall) finish(returned *bool) {
	o := call.owner
	o.access.Lock()
	if !call.active || call.ended {
		o.access.Unlock()
		panic(errStreamMapUncertain)
	}
	call.ended, call.returned = true, *returned
	firstFailure := !*returned && !o.failed
	if !*returned {
		o.failed = true
		o.sealLocked(errStreamMapUncertain)
	}
	o.access.Unlock()
	// Running includes the actual failure notification, even if it unwinds.
	// Unknown calls keep their original slot, context, target and result.
	defer func() {
		o.access.Lock()
		if call.returned {
			*call = streamMapCall{}
			o.count--
		}
		o.running--
		o.settleLocked()
		o.access.Unlock()
	}()
	if firstFailure && o.onFailure != nil {
		o.onFailure()
	}
}

func (o *streamMapCalls) finishLoop(returned *bool) {
	o.access.Lock()
	if !*returned {
		o.failed = true
	}
	o.loopActive = false
	o.sealLocked(nil)
	o.access.Unlock()
}

func (o *streamMapCalls) wait() error {
	o.access.Lock()
	done := o.done
	o.access.Unlock()
	if done == nil {
		return errStreamMapUncertain
	}
	<-done
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || !o.sealed || o.loopActive || o.running != 0 || o.count != 0 {
		return errStreamMapUncertain
	}
	return nil
}

// WaitForStreamMapShutdown joins the original loop and admitted Open/Accept
// invocations. Call only from an external owner after requesting connection close,
// never from a registered operation's own callback. It does not prove that all
// stream rows, factories, 0-RTT generations or native resources were retired.
func (c *Conn) WaitForStreamMapShutdown() error {
	<-c.ctx.Done()
	if c.streamsMap == nil {
		return errStreamMapUncertain
	}
	return c.streamsMap.calls.wait()
}
