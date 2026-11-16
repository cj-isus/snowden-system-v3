package quic

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/internal/ackhandler"
)

var errCongestionCallUncertain = errors.New("quic: original congestion call retirement is uncertain")

type congestionCallStage uint8

const (
	congestionCallRegistered congestionCallStage = iota + 1
	congestionCallSetup
	congestionCallReturned
	congestionCallClosing
)

// One original public setter, including its outer close-request/defer path.
// Only that caller owns stage/result. The registry mutex guards admission and
// finish; it never invokes a controller, close request, error method or wait.
type connectionCongestionCall struct {
	active     bool
	stage      congestionCallStage
	controller congestion.CongestionControl
	handler    ackhandler.SentPacketHandler
	result     error
}

type connectionCongestionCalls struct {
	access                  sync.Mutex
	call                    connectionCongestionCall
	closeState              *atomic.Pointer[closeError]
	closing                 <-chan struct{}
	sealed, failed, settled bool
	done                    chan struct{}
}

// Original constructor only, before public exposure or controller callbacks.
func (r *connectionCongestionCalls) init(closeState *atomic.Pointer[closeError], closing <-chan struct{}) {
	if r.done != nil || r.sealed || closeState == nil || closing == nil {
		panic(errCongestionCallUncertain)
	}
	r.closeState, r.closing = closeState, closing
	r.done = make(chan struct{})
}

func (r *connectionCongestionCalls) settleLocked() {
	if r.done != nil && r.sealed && !r.settled && (r.failed || !r.call.active) {
		r.settled = true
		close(r.done) // failed settlement is negative, never a retirement proof
	}
}

func (r *connectionCongestionCalls) seal() {
	r.access.Lock()
	r.sealed = true
	r.settleLocked()
	r.access.Unlock()
}

func (r *connectionCongestionCalls) begin(cc congestion.CongestionControl, handler ackhandler.SentPacketHandler) (*connectionCongestionCall, error) {
	r.access.Lock()
	defer r.access.Unlock()
	if r.done == nil || r.closeState == nil || r.failed || handler == nil {
		return nil, errCongestionCallUncertain
	}
	if r.sealed || r.closeState.Load() != nil {
		return nil, net.ErrClosed
	}
	select {
	case <-r.closing:
		return nil, net.ErrClosed
	default:
	}
	if r.call.active {
		return nil, ackhandler.ErrCongestionChangeBusy
	}
	r.call = connectionCongestionCall{
		active: true, stage: congestionCallRegistered, controller: cc, handler: handler,
	}
	return &r.call, nil
}

func (r *connectionCongestionCalls) finish(call *connectionCongestionCall, returned *bool) {
	r.access.Lock()
	defer r.access.Unlock()
	if call != &r.call || !call.active || !*returned {
		r.failed, r.sealed = true, true
		// Original input/handler/stage/result remain retained on unknown return.
	} else {
		r.call = connectionCongestionCall{}
	}
	r.settleLocked()
}

func (r *connectionCongestionCalls) wait() error {
	r.access.Lock()
	if r.done == nil {
		r.access.Unlock()
		return errCongestionCallUncertain
	}
	r.sealed = true
	r.settleLocked()
	done := r.done
	r.access.Unlock()
	<-done // every observer joins this original receipt outside metadata locks
	r.access.Lock()
	defer r.access.Unlock()
	if r.failed || r.call.active || !r.settled {
		return errCongestionCallUncertain
	}
	return nil
}

// WaitForCongestionControlShutdown is an external-owner receipt after close.
// It joins original packet input/native-loop shutdown and the public setter
// (including its close-request path), then checks retained ACK congestion state.
// Never call it from a controller callback or a call that the receipt joins.
// It does not cancel nonreturning providers, retire all controller/native data,
// or join hidden tasks created by custom controllers.
func (c *Conn) WaitForCongestionControlShutdown() error {
	if c == nil || c.ctx == nil {
		return errCongestionCallUncertain
	}
	<-c.ctx.Done()
	inputErr := c.WaitForPacketInputShutdown() // Context cancellation alone is not loop retirement
	callErr := c.congestionCalls.wait()
	if c.sentPacketHandler == nil {
		return errors.Join(inputErr, callErr, errCongestionCallUncertain)
	}
	return errors.Join(inputErr, callErr, c.sentPacketHandler.CongestionStateError())
}
