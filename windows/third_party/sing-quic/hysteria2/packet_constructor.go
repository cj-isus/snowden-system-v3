package hysteria2

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/packetaddr"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/pipe"
)

type udpPacketInitStage uint8

const (
	udpPacketInitScope udpPacketInitStage = iota + 1
	udpPacketInitAddress
	udpPacketInitContext
	udpPacketInitQueue
	udpPacketInitFragments
	udpPacketInitDeadline
	udpPacketInitReady
)

type udpConstructionAbortStage uint8

const (
	udpConstructionAbortScope udpConstructionAbortStage = iota + 1
	udpConstructionAbortContext
	udpConstructionAbortReturned
)

// The client retains one original record before initialization starts. Once
// initialization relinquishes udpAccess, candidate/init failure are immutable.
// Only the original abort Once writes its stage/error; callers join that result.
type udpPacketConstruction struct {
	candidate            *udpPacketConn
	initializationFailed bool
	onFailure            func()
	failureNotified      bool
	abortOnce            sync.Once
	abortStage           udpConstructionAbortStage
	abortErr             error
}

func newUDPPacketConstruction(onFailure func()) *udpPacketConstruction {
	return &udpPacketConstruction{candidate: new(udpPacketConn), onFailure: onFailure}
}

func (o *udpPacketConstruction) notifyFailure() {
	if !o.failureNotified {
		o.failureNotified = true
		if o.onFailure != nil {
			o.onFailure()
		}
	}
}

// Only an unpublished candidate uses this abort. It never invokes onDestroy or
// replays a failed native Close/cancel. The client and retiring carrier share it.
func (o *udpPacketConstruction) abort() error {
	o.abortOnce.Do(func() {
		returned := false
		defer func() {
			if !returned || o.initializationFailed {
				o.abortErr = errors.Join(o.abortErr, errClientUncertain)
			}
			if o.abortErr != nil {
				o.notifyFailure() // no carrier/UDP/RX mutex held
			}
		}()
		if o.candidate == nil {
			o.abortErr = errClientUncertain
			returned = true
			return
		}
		o.abortStage = udpConstructionAbortScope
		if o.candidate.sendScope != nil {
			o.abortErr = o.candidate.sendScope.Close()
			if o.abortErr != nil {
				o.notifyFailure() // start independent carrier abort before cancel
			}
		}
		o.abortStage = udpConstructionAbortContext
		if o.candidate.cancel != nil {
			o.candidate.cancel(os.ErrClosed)
		}
		o.abortStage = udpConstructionAbortReturned
		returned = true
	})
	return o.abortErr
}

// The legacy service/private signature shares initialization, but its local
// record does not claim the client's persistent pending/retiring custody.
func newUDPPacketConn(ctx context.Context, quicConn *quic.Conn, budget *udpReceiveBudget, send *udpSendBudget, onDestroy func()) (result *udpPacketConn, err error) {
	original := newUDPPacketConstruction(nil)
	constructed, initReturned := false, false
	defer func() {
		if !constructed {
			original.initializationFailed = !initReturned
			if abortErr := original.abort(); abortErr != nil {
				err = errors.Join(err, abortErr)
			}
		}
	}()
	err = initializeUDPPacketConn(original.candidate, ctx, quicConn, budget, send, onDestroy)
	initReturned = true
	if err != nil {
		return nil, err
	}
	constructed = true
	return original.candidate, nil
}

func initializeUDPPacketConn(candidate *udpPacketConn, ctx context.Context, quicConn *quic.Conn, budget *udpReceiveBudget, send *udpSendBudget, onDestroy func()) error {
	candidate.quicConn, candidate.sendBudget, candidate.onDestroy = quicConn, send, onDestroy
	candidate.initStage = udpPacketInitScope
	var err error
	candidate.sendScope, err = quicConn.NewDatagramSendScope()
	if err != nil {
		return err
	}
	candidate.initStage = udpPacketInitAddress
	candidate.localSource = quicConn.LocalAddr()
	candidate.local, err = packetaddr.Capture(candidate.localSource)
	if err != nil {
		return err
	}
	candidate.localSource = nil
	candidate.initStage = udpPacketInitContext
	candidate.ctx, candidate.cancel = common.ContextWithCancelCause(ctx)
	candidate.initStage = udpPacketInitQueue
	candidate.data = make(chan *udpMessage, 64)
	candidate.udpMTU = initialUDPPacketBudget
	candidate.initStage = udpPacketInitFragments
	candidate.defragger = newUDPDefragger(budget)
	candidate.initStage = udpPacketInitDeadline
	candidate.readDeadline = pipe.MakeDeadline()
	candidate.initStage = udpPacketInitReady
	return nil
}
