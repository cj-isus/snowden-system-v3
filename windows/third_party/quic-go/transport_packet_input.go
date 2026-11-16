package quic

import (
	"context"
	"net"

	"github.com/sagernet/quic-go/packetaddr"
)

// Lifecycle.access guards admission and settlement. Only the original listener
// uses current/source; only the original drain uses draining. Read tokens are
// private to their actual call until done. Other fields are observed after the
// corresponding original done channel, never by optimistic cleanup retries.
type transportInputState struct {
	current              receivedPacket
	source               rawConn
	reading              bool
	failed, sealed       bool
	closing              chan struct{}
	reads                map[*transportReadCall]struct{}
	readCount, readBytes int
	readsDone            chan struct{}
	readsSettled         bool
	drain                transportCloseCall
	draining             receivedPacket
}

type transportReadCall struct {
	context context.Context
	buffer  []byte
	packet  receivedPacket
}

// Production active handlers expose a nonwaiting original close request. Do not
// create a destroy goroutine for every duplicate reset datagram. Private custom
// handlers execute in the original listener task and retain its failure guard.
func requestStatelessReset(handler packetHandler) {
	err := &StatelessResetError{}
	switch value := handler.(type) {
	case *Conn:
		value.destroyImpl(err)
	case *wrappedConn:
		value.Conn.destroyImpl(err)
	default:
		handler.destroy(err)
	}
}

func (o *transportLifecycle) settleCallsLocked() {
	o.settleWritesLocked()
	o.settleClientOpeningsLocked()
	o.settleServerOpeningLocked()
	o.settlePathOpeningsLocked()
	s := &o.inputs
	if (o.sealed || o.failed) && !s.sealed {
		s.sealed = true
		close(s.closing)
	}
	if o.failed {
		s.failed = true
	}
	if s.sealed && !s.readsSettled && (s.failed || s.readCount == 0) {
		s.readsSettled = true
		close(s.readsDone) // failed is a negative result, not a retirement receipt
	}
}

func (t *Transport) failTransportInput() {
	o := t.lifecycle()
	o.access.Lock()
	o.failed, o.sealed, o.inputs.failed = true, true, true
	o.settleCallsLocked()
	o.access.Unlock()
}

func (t *Transport) readTransportPacket(conn rawConn) (receivedPacket, error) {
	o := t.lifecycle()
	o.inputs.source, o.inputs.reading = conn, true
	p, err := conn.ReadPacket()
	p.transport = t      // original reader identity, never a provider-supplied hint
	o.inputs.current = p // before error methods, tracer or handler callbacks
	o.inputs.reading = false
	return p, err
}

func (t *Transport) clearTransportPacket() {
	o := t.lifecycle()
	o.inputs.current, o.inputs.source = receivedPacket{}, nil
}

func (t *Transport) discardTransportPacket() {
	o := t.lifecycle()
	releaseClosedPacket(&o.inputs.current)
	t.clearTransportPacket()
}

func (t *Transport) reserveTransportRead(ctx context.Context, b []byte) (*transportReadCall, error) {
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	s := &o.inputs
	if s.failed || o.failed {
		return nil, errPacketInputUncertain
	}
	if s.sealed || o.sealed {
		return nil, net.ErrClosed
	}
	if s.readCount >= maxTransportCalls || len(b) > maxTransportWriteBytes-s.readBytes {
		return nil, errTransportCallCapacity
	}
	s.readCount++
	s.readBytes += len(b) // before token/map allocation or initializer/context waits
	if s.reads == nil {
		s.reads = make(map[*transportReadCall]struct{})
	}
	call := &transportReadCall{context: ctx, buffer: b}
	s.reads[call] = struct{}{}
	return call, nil
}

func (t *Transport) finishTransportRead(call *transportReadCall, returned *bool) {
	o := t.lifecycle()
	o.access.Lock()
	s := &o.inputs
	if *returned {
		s.readCount--
		s.readBytes -= len(call.buffer)
		call.context, call.buffer, call.packet = nil, nil, receivedPacket{}
		delete(s.reads, call)
	} else {
		o.failed, o.sealed, s.failed = true, true, true
	}
	o.settleCallsLocked()
	o.access.Unlock()
}

// Preserve opaque concrete addresses required by custom PacketConn providers.
// Standard metadata is detached before returning its associated input buffer.
func receivedPacketAddress(addr net.Addr) (net.Addr, error) {
	switch addr.(type) {
	case *net.UDPAddr, *net.TCPAddr, *net.IPAddr, *net.UnixAddr:
		snapshot, err := packetaddr.Capture(addr)
		if err != nil {
			return nil, err
		}
		return snapshot.Addr(), nil
	default:
		return addr, nil
	}
}

// ReadNonQUICPacket reads one non-QUIC datagram. Detection requires both high
// bits to be zero (stricter than RFC 9443). The queue is created once by the
// original initializer. Closing wakes registered readers without a timer.
func (t *Transport) ReadNonQUICPacket(ctx context.Context, b []byte) (int, net.Addr, error) {
	call, err := t.reserveTransportRead(ctx, b)
	if err != nil {
		return 0, nil, err
	}
	returned := false
	defer t.finishTransportRead(call, &returned)
	if err := t.init(false); err != nil {
		returned = true
		return 0, nil, err
	}
	t.readingNonQUICPackets.Store(true)
	o := t.lifecycle()
	select {
	case <-o.inputs.closing:
		returned = true
		return 0, nil, net.ErrClosed
	case <-call.context.Done():
		err := call.context.Err()
		returned = true
		return 0, nil, err
	case p := <-t.nonQUICPackets:
		call.packet = p // before copy, address preparation or pool-return callbacks
		n := copy(call.buffer, call.packet.data)
		addr, err := receivedPacketAddress(call.packet.remoteAddr)
		releaseClosedPacket(&call.packet)
		returned = true
		return n, addr, err
	}
}

func (t *Transport) waitTransportReads() error {
	o := t.lifecycle()
	<-o.inputs.readsDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.inputs.failed || o.inputs.readCount != 0 {
		return errPacketInputUncertain
	}
	return nil
}

// Only after actual listener/read-call completion. Both natural listener exit
// and public Close share this original drain operation; no second pool release.
func (t *Transport) retireTransportInputs() (result error) {
	o := t.lifecycle()
	o.access.Lock()
	if o.inputs.drain.started {
		o.access.Unlock()
		return t.waitTransportCall(&o.inputs.drain)
	}
	o.access.Unlock()
	readErr := t.waitTransportReads()
	o.access.Lock()
	if o.inputs.drain.started {
		o.access.Unlock()
		return t.waitTransportCall(&o.inputs.drain)
	}
	if readErr != nil {
		o.access.Unlock()
		return readErr
	}
	if o.failed || (o.loopsScheduled && !o.listenerReturned) || o.inputs.reading || o.inputs.current.buffer != nil || o.inputs.source != nil {
		o.access.Unlock()
		return errPacketInputUncertain
	}
	o.inputs.drain.started = true
	o.access.Unlock()
	returned := false
	defer t.finishTransportCall(&o.inputs.drain, &returned, &result)
	for {
		select {
		case p := <-t.nonQUICPackets:
			o.inputs.draining = p
			releaseClosedPacket(&o.inputs.draining)
		default:
			returned = true
			return nil
		}
	}
}
