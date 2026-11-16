package quic

import (
	"context"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/qlog"
	"github.com/sagernet/quic-go/qlogwriter"
)

// closeMx may precede access. access never precedes closeMx/Transport.mutex or
// callbacks/pool/native work. Only the original receive task owns current,
// held, handoff, candidate and draining. No cross-task packet ref is added:
// packetBuffer's reference counter is deliberately not concurrent.
type serverPacketInputState struct {
	once                                     sync.Once
	access                                   sync.Mutex
	sealed, failed, settled, failureSignaled bool
	calls                                    [maxConnPacketCalls]connPacketCall
	count, bytes                             int
	callsDone, failure                       chan struct{}
	current, handoff, draining               receivedPacket
	header                                   *wire.Header
	rejected                                 rejectedPacket // original handoff before channel commit
	held                                     bool
	candidate                                *wrappedConn
	opening                                  serverConnectionOpening
}

// Only the original serialized server receive task mutates this prefix. Keep
// partial construction attached before callbacks, even when they never return.
type serverConnectionOpening struct {
	active                   bool
	conn                     sendConn
	sendOpening              sendConnOpening
	context, callbackContext context.Context
	cancel1, cancel2         context.CancelCauseFunc
	trace                    qlogwriter.Trace
	connection               *Conn
}

func (s *baseServer) finishConnectionOpening(returned *bool) {
	if !*returned {
		s.failServerPacketInput()
		return // unknown context / trace / partial Conn remains original-owned
	}
	s.packetInput.opening = serverConnectionOpening{}
}

func (o *serverPacketInputState) init() {
	o.once.Do(func() {
		o.callsDone = make(chan struct{})
		o.failure = make(chan struct{})
	})
}

func (o *serverPacketInputState) settleLocked() {
	if o.failed && !o.failureSignaled {
		o.failureSignaled = true
		close(o.failure)
	}
	if o.sealed && !o.settled && (o.failed || o.count == 0) {
		o.settled = true
		close(o.callsDone)
	}
}

func (s *baseServer) packetInputFailed() bool {
	o := &s.packetInput
	o.access.Lock()
	defer o.access.Unlock()
	return o.failed
}

func (s *baseServer) sealPacketInput() {
	o := &s.packetInput
	o.init()
	o.access.Lock()
	o.sealed = true
	o.settleLocked()
	o.access.Unlock()
}

func (s *baseServer) failServerPacketInput() {
	o := &s.packetInput
	o.init()
	o.access.Lock()
	o.failed, o.sealed = true, true
	o.settleLocked()
	o.access.Unlock()
	// A server task cannot synchronously join its own server.close.
	t := (*Transport)(s.tr)
	t.failTransportInput()
	t.abortTransport()
}

func (s *baseServer) reserveServerPacket(p receivedPacket) *connPacketCall {
	o := &s.packetInput
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
		o.bytes += size
		*call = connPacketCall{packet: p, size: size, active: true}
		return call
	}
	return nil
}

func (s *baseServer) finishServerPacketCall(call *connPacketCall, returned *bool) {
	o := &s.packetInput
	o.access.Lock()
	failed := !*returned || call.packet.buffer != nil || len(call.packet.data) != 0
	if failed {
		o.failed, o.sealed = true, true
	} else {
		o.count--
		o.bytes -= call.size
		*call = connPacketCall{}
	}
	o.settleLocked()
	o.access.Unlock()
	if failed {
		s.failServerPacketInput()
	}
}

func (s *baseServer) handlePacket(p receivedPacket) {
	call := s.reserveServerPacket(p)
	if call == nil {
		releaseClosedPacket(&p) // original upstream task still owns this refusal
		return
	}
	returned := false
	defer s.finishServerPacketCall(call, &returned)
	o := &s.packetInput
	o.access.Lock()
	queued, full := false, false
	if !o.sealed && !o.failed {
		select {
		case s.receivedPackets <- call.packet:
			call.packet = receivedPacket{}
			queued = true
		default:
			full = true
		}
	}
	o.access.Unlock()
	if !queued {
		if full {
			s.logger.Debugf("Dropping packet from %s (%d bytes). Server receive queue full.", call.packet.remoteAddr, call.packet.Size())
			if s.qlogger != nil {
				s.qlogger.RecordEvent(qlog.PacketDropped{Raw: qlog.RawInfo{Length: int(call.packet.Size())}, Trigger: qlog.PacketDropDOSPrevention})
			}
		}
		releaseClosedPacket(&call.packet)
	}
	returned = true
}

func (s *baseServer) waitServerPacketCalls() bool {
	o := &s.packetInput
	<-o.callsDone
	o.access.Lock()
	defer o.access.Unlock()
	return !o.failed && o.count == 0
}

func (s *baseServer) processServerPacket(p receivedPacket) {
	o := &s.packetInput
	o.current, o.held = p, true
	returned := false
	defer func() {
		if !returned {
			s.failServerPacketInput()
		}
	}()
	s.handlePacketImpl(p)
	// Normal errors that never transferred the input still belong to this task.
	if o.held {
		s.releaseServerPacket()
	}
	if o.header != nil || !emptyRejectedPacketMetadata(o.rejected) {
		panic(errPacketInputUncertain) // do not overwrite an unfinished header/handoff
	}
	o.current = receivedPacket{}
	returned = true
}

// Only the original receive task mutates its parsed Header. A response message
// carries a value copy, so this cannot change the receiving actor's metadata.
func (s *baseServer) clearServerHeader() {
	if s.packetInput.header != nil {
		s.packetInput.header.Token = nil // references only, before pool return/handoff
		s.packetInput.header = nil
	}
}

func (s *baseServer) releaseServerPacket() {
	s.clearServerHeader()
	releaseClosedPacket(&s.packetInput.current)
	s.packetInput.held = false
}

func (s *baseServer) transferServerPacket(handler packetHandler, p receivedPacket) {
	s.packetInput.held = false // private handler consumes the original ref at entry
	s.clearServerHeader()
	handler.handlePacket(p)
}

// The existing fixed-capacity channels bound metadata. Transfer the original
// raw reference once, without a concurrent packetBuffer.Split or payload clone.
func (s *baseServer) enqueueRejectedPacket(queue chan rejectedPacket, p receivedPacket, hdr *wire.Header) {
	o := &s.packetInput
	if !o.held || hdr == nil || o.header != hdr || p.buffer == nil || o.current.buffer != p.buffer ||
		len(p.data) == 0 || len(o.current.data) != len(p.data) || &o.current.data[0] != &p.data[0] ||
		!emptyRejectedPacketMetadata(o.rejected) {
		panic(errPacketInputUncertain) // original processing defer retains unknown input
	}
	o.rejected = rejectedPacket{receivedPacket: p, hdr: *hdr, hasHeader: true}
	select {
	case queue <- o.rejected:
		o.held = false // only metadata stores follow; receiver may already return the pool buffer
		s.clearServerHeader()
		o.rejected = rejectedPacket{}
	default:
		o.rejected = rejectedPacket{}
		s.releaseServerPacket()
	}
}

func (s *baseServer) discardZeroRTTQueue(id protocol.ConnectionID) {
	q, ok := s.zeroRTTQueues[id]
	if !ok {
		return
	}
	for i := range q.packets {
		releaseClosedPacket(&q.packets[i])
	}
	delete(s.zeroRTTQueues, id)
}

func (s *baseServer) transferZeroRTTQueue(id protocol.ConnectionID, handler packetHandler) {
	q, ok := s.zeroRTTQueues[id]
	if !ok {
		return
	}
	for i := range q.packets {
		s.packetInput.handoff = q.packets[i]
		q.packets[i] = receivedPacket{} // original handoff slot now owns this ref
		handler.handlePacket(s.packetInput.handoff)
		s.packetInput.handoff = receivedPacket{}
	}
	delete(s.zeroRTTQueues, id)
}

func (s *baseServer) finishServerReceive(returned *bool) {
	cleanupReturned := false
	defer func() {
		if !*returned || !cleanupReturned || !s.runReturned {
			s.failServerPacketInput()
		}
	}()
	s.sealPacketInput()
	if !*returned {
		return
	}
	o := &s.packetInput
	for {
		select {
		case p := <-s.receivedPackets:
			o.draining = p
			releaseClosedPacket(&o.draining)
		default:
			goto zeroRTT
		}
	}
zeroRTT:
	for id := range s.zeroRTTQueues {
		s.discardZeroRTTQueue(id)
	}
	o.access.Lock()
	failed := o.failed
	o.access.Unlock()
	s.runReturned = !failed && !o.held && emptyPathInputMetadata(o.current) &&
		emptyPathInputMetadata(o.handoff) && emptyPathInputMetadata(o.draining) && o.header == nil &&
		emptyRejectedPacketMetadata(o.rejected) && o.candidate == nil && !o.opening.active && s.rejectedConn == nil
	cleanupReturned = true
}
