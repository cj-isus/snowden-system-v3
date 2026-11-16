package quic

import (
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/qlog"
)

const (
	maxConnPacketCalls     = 64
	maxConnPacketCallBytes = maxConnPacketCalls * protocol.MaxLargePacketBufferSize
)

// receivedPacketMx may precede access; access never precedes receivedPacketMx
// or a provider/trace/pool callback. The original connection task alone owns
// delivery/processing/draining/replay. Done is positive only after that task
// and all admitted intake calls returned normally; failure retains originals.
type connPacketInputState struct {
	once                                              sync.Once
	access                                            sync.Mutex
	sealed, failed, loopActive, settled, callsSettled bool
	done, callsDone                                   chan struct{}
	calls                                             [maxConnPacketCalls]connPacketCall
	count, bytes                                      int
	delivery, draining                                receivedPacket
	processing                                        *packetInputProcessing
	processingSlot                                    packetInputProcessing
	replay                                            []receivedPacketWithDatagramID
	replayPos                                         int
}

type connPacketCall struct {
	packet receivedPacket
	size   int
	active bool
}

type packetInputProcessing struct {
	packet          receivedPacket
	anchor, working bool
	pathCreditUsed  bool  // original datagram contributes to migration credit once
	serverPath      *path // original authenticated non-active route for frame handlers
}

func (o *connPacketInputState) init() {
	o.once.Do(func() {
		o.loopActive = true
		o.done = make(chan struct{})
		o.callsDone = make(chan struct{})
	})
}

func (o *connPacketInputState) settleLocked() {
	if o.sealed && !o.callsSettled && (o.failed || o.count == 0) {
		o.callsSettled = true
		close(o.callsDone)
	}
	if o.sealed && !o.settled && (o.failed || (!o.loopActive && o.count == 0)) {
		o.settled = true
		close(o.done) // failure is a negative result, never authority to reuse refs
	}
}

func (c *Conn) waitPacketIntakeCalls() error {
	o := &c.packetInput
	<-o.callsDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.count != 0 {
		return errPacketInputUncertain
	}
	return nil
}

// Only the original ending loop reads its delivery/processing/drain fields.
// Sealed zero-count intake excludes future queue commits; this lock is released
// before path retirement takes the framer mutex or clears any other metadata.
func (o *connPacketInputState) pathMetadataReady() bool {
	o.access.Lock()
	defer o.access.Unlock()
	if !o.sealed || o.failed || !o.loopActive || !o.callsSettled || o.count != 0 || o.bytes != 0 ||
		o.processing != nil || o.processingSlot.serverPath != nil ||
		!emptyPathInputMetadata(o.processingSlot.packet) || !emptyPathInputMetadata(o.delivery) ||
		!emptyPathInputMetadata(o.draining) || len(o.replay) != 0 {
		return false
	}
	for i := range o.calls {
		if call := &o.calls[i]; call.active || !emptyPathInputMetadata(call.packet) {
			return false
		}
	}
	return true
}

func emptyPathInputMetadata(packet receivedPacket) bool {
	return packet.buffer == nil && packet.data == nil && packet.transport == nil &&
		packet.remoteAddr == nil && packet.info == (packetInfo{})
}

func (o *connPacketInputState) seal() {
	o.init()
	o.access.Lock()
	o.sealed = true
	o.settleLocked()
	o.access.Unlock()
}

func (c *Conn) failPacketInput() {
	o := &c.packetInput
	o.init()
	o.access.Lock()
	o.failed, o.sealed = true, true
	o.settleLocked()
	o.access.Unlock()
	c.destroyImpl(errPacketInputUncertain) // nonwaiting, outside metadata locks
}

func (c *Conn) reservePacketInput(p receivedPacket) *connPacketCall {
	o := &c.packetInput
	o.init()
	if p.buffer == nil || len(p.data) == 0 {
		return nil
	}
	size := cap(p.buffer.Data)
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.sealed || o.count >= maxConnPacketCalls || size > protocol.MaxLargePacketBufferSize || len(p.data) > size || size > maxConnPacketCallBytes-o.bytes {
		return nil
	}
	for i := range o.calls {
		call := &o.calls[i]
		if call.active {
			continue
		}
		o.count++
		o.bytes += size // before attaching input and the receive queue mutex wait
		*call = connPacketCall{packet: p, size: size, active: true}
		return call
	}
	return nil
}

func (c *Conn) finishPacketInput(call *connPacketCall, returned *bool) {
	o := &c.packetInput
	o.access.Lock()
	failed := !*returned || call.packet.buffer != nil || len(call.packet.data) != 0
	if failed {
		o.failed, o.sealed = true, true
	} else {
		o.count--
		o.bytes -= call.size
		*call = connPacketCall{} // ordinary return alone makes this fixed cell reusable
	}
	o.settleLocked()
	o.access.Unlock()
	if failed {
		c.destroyImpl(errPacketInputUncertain)
	}
}

// A successful pre-seal admission commits before the terminal drain can take
// receivedPacketMx. A late call drops outside both metadata mutexes.
func (c *Conn) enqueuePacketInput(call *connPacketCall) (queued, full bool) {
	c.receivedPacketMx.Lock()
	defer c.receivedPacketMx.Unlock()
	o := &c.packetInput
	o.access.Lock()
	closed := o.sealed || o.failed || c.closeErr.Load() != nil
	o.access.Unlock()
	if closed {
		return false, false
	}
	if c.receivedPackets.Len() >= protocol.MaxConnUnprocessedPackets {
		return false, true
	}
	c.receivedPackets.PushBack(call.packet)
	call.packet = receivedPacket{} // queue now owns this original reference
	return true, false
}

func (c *Conn) handlePacket(p receivedPacket) {
	call := c.reservePacketInput(p)
	if call == nil {
		// The upstream listener/server still owns the rejected call's original.
		// Rejection allocates no token, starts no worker and invokes no trace.
		releaseClosedPacket(&p)
		return
	}
	returned := false
	defer c.finishPacketInput(call, &returned)
	queued, full := c.enqueuePacketInput(call)
	if !queued {
		if full && c.qlogger != nil {
			var datagramID qlog.DatagramID
			if len(call.packet.data) > 0 && wire.IsLongHeaderPacket(call.packet.data[0]) {
				datagramID = qlog.CalculateDatagramID(call.packet.data)
			}
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Raw:        qlog.RawInfo{Length: int(call.packet.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropDOSPrevention,
			})
		}
		releaseClosedPacket(&call.packet)
		returned = true
		return
	}
	select {
	case c.notifyReceivedPacket <- struct{}{}:
	default:
	}
	returned = true
}

// Anchor the complete original datagram before any crypto or tracing callback.
// Segment handlers consume or queue their own working reference. The anchor
// prevents a deferred decrement from recycling storage during abnormal unwind.
func (c *Conn) handleOnePacket(rp receivedPacket, datagramID qlog.DatagramID) (bool, error) {
	o := &c.packetInput
	if o.processing != nil || rp.buffer == nil {
		o.delivery = rp
		c.failPacketInput()
		return false, errPacketInputUncertain
	}
	owner := &o.processingSlot
	*owner = packetInputProcessing{packet: rp, working: true}
	o.processing, o.delivery = owner, receivedPacket{}
	returned := false
	defer c.finishPacketProcessing(owner, &returned)
	rp.buffer.Split()
	owner.anchor = true
	if len(rp.data) == 0 {
		returned = true
		return false, nil
	}
	processed, err := c.handleOnePacketImpl(rp, datagramID)
	returned = true // ordinary error is still an actual segment-handler return
	return processed, err
}

func (c *Conn) finishPacketProcessing(owner *packetInputProcessing, returned *bool) {
	cleanupReturned := false
	defer func() {
		if !*returned || !cleanupReturned {
			c.failPacketInput()
		}
	}()
	if !*returned {
		// Keep the original anchor; no speculative pool return.
		return
	}
	if owner.working {
		owner.packet.buffer.Decrement()
		owner.working = false
	}
	if owner.anchor {
		owner.packet.buffer.Decrement()
		owner.packet.buffer.MaybeRelease()
		owner.anchor = false
	}
	owner.packet = receivedPacket{}
	owner.serverPath = nil
	c.packetInput.processing = nil
	cleanupReturned = true
}

func (c *Conn) pendingUndecryptablePackets() int {
	o := &c.packetInput
	return len(c.undecryptablePackets) + len(c.undecryptablePacketsToProcess) + len(o.replay) - o.replayPos
}

func discardQueuedPacketInputs(packets *[]receivedPacketWithDatagramID) {
	for i := range *packets {
		releaseClosedPacket(&(*packets)[i].receivedPacket)
		(*packets)[i] = receivedPacketWithDatagramID{}
	}
	*packets = nil
}

func (c *Conn) popPacketInputForDrain() bool {
	c.receivedPacketMx.Lock()
	defer c.receivedPacketMx.Unlock()
	if c.receivedPackets.Empty() {
		return false
	}
	c.packetInput.draining = c.receivedPackets.PopFront()
	return true
}

func (c *Conn) drainPacketInputs(returned *bool) {
	o := &c.packetInput
	o.seal() // all previously admitted queue commits finish before this drain
	completed := false
	defer func() {
		if !completed {
			c.failPacketInput()
		}
	}()
	for c.popPacketInputForDrain() {
		releaseClosedPacket(&o.draining)
	}
	discardQueuedPacketInputs(&c.undecryptablePackets)
	discardQueuedPacketInputs(&c.undecryptablePacketsToProcess)
	discardQueuedPacketInputs(&o.replay)
	o.replayPos = 0
	*returned = o.processing == nil && o.delivery.buffer == nil && len(o.delivery.data) == 0
	completed = true
}

func (o *connPacketInputState) finishLoop(returned *bool) {
	o.init()
	o.access.Lock()
	if !*returned || o.processing != nil || o.delivery.buffer != nil || o.draining.buffer != nil || len(o.replay) != 0 {
		o.failed = true
	}
	o.sealed, o.loopActive = true, false
	if o.failed {
		*returned = false
	}
	o.settleLocked()
	o.access.Unlock()
}

// WaitForPacketInputShutdown joins the original connection input task and
// registered intake / path-probe calls, and the loop's sealed path-response
// queue/server-path metadata. It is not a receipt for server/custom raw
// providers, all crypto/ACK state, global retiring transports or native OS retirement.
func (c *Conn) WaitForPacketInputShutdown() error {
	<-c.ctx.Done()
	o := &c.packetInput
	o.init()
	<-o.done
	o.access.Lock()
	uncertain := o.failed || o.loopActive || o.count != 0 || !c.pathMetadataRetired || c.pathMetadataRetiring
	o.access.Unlock()
	if uncertain {
		return errPacketInputUncertain
	}
	if pm := c.pathManagerOutgoing.Load(); pm != nil {
		return pm.waitProbeCalls() // no packet metadata lock across the join
	}
	return nil
}
