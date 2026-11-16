package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
)

type serverResponseKind uint8

const (
	serverVersionResponse serverResponseKind = iota
	serverInvalidTokenResponse
	serverRefusedResponse
	serverRetryResponse
)

// The single existing response worker owns all these original views. Failure
// retains them and never authorizes a second write or speculative pool return.
type serverPacketOutputState struct {
	current, draining rejectedPacket
	buffer            *packetBuffer
	backing           []byte // immutable original pool span, even if Append returns nil
	data, oob         []byte
	conn              rawConn
	active, failed    bool
}

func (s *baseServer) responsePacketBuffer() (*packetBuffer, error) {
	o := &s.packetOutput
	if o.buffer != nil {
		o.failed = true
		return nil, errPacketOutputUncertain
	}
	o.buffer = getPacketBuffer()
	o.backing = o.buffer.Data[:cap(o.buffer.Data)]
	return o.buffer, nil
}

func (s *baseServer) writeServerResponse(data []byte, p receivedPacket) error {
	o := &s.packetOutput
	o.data, o.conn = data, s.conn
	o.oob = p.info.OOB()
	n, err := o.conn.WritePacket(o.data, p.remoteAddr, o.oob, 0, protocol.ECNUnsupported)
	err = packetWriteResult(n, len(o.data), err)
	if errors.Is(err, errPacketOutputUncertain) {
		o.failed = true
	}
	return err
}

func (s *baseServer) processServerResponse(kind serverResponseKind, p rejectedPacket) bool {
	o := &s.packetOutput
	o.current, o.active = p, true
	returned := false
	defer s.finishServerResponse(&returned)
	switch kind {
	case serverVersionResponse:
		s.maybeSendVersionNegotiationPacket(p.receivedPacket)
	case serverInvalidTokenResponse:
		s.maybeSendInvalidToken(p)
	case serverRefusedResponse:
		s.sendConnectionRefused(p)
	case serverRetryResponse:
		s.sendRetry(p)
	default:
		o.failed = true
	}
	returned = true
	return !o.failed
}

func (s *baseServer) finishServerResponse(returned *bool) {
	cleanupReturned := false
	defer func() {
		if !*returned || !cleanupReturned {
			s.packetOutput.failed = true
			s.failServerPacketInput()
		}
	}()
	o := &s.packetOutput
	if !*returned || o.failed {
		return
	}
	if o.buffer != nil {
		o.buffer.Data = o.backing[:0]
		o.buffer.Release()
		o.buffer = nil
		o.backing = nil
	}
	o.data, o.oob, o.conn = nil, nil, nil
	releaseClosedPacket(&o.current.receivedPacket)
	o.current, o.active = rejectedPacket{}, false
	cleanupReturned = true
}

func (s *baseServer) finishServerResponses(returned *bool) {
	cleanupReturned := false
	defer func() {
		if !*returned || !cleanupReturned {
			s.packetOutput.failed = true
			s.failServerPacketInput()
		}
	}()
	if !*returned || s.packetOutput.active || s.packetOutput.failed {
		return
	}
	// No response producer remains after the actual receive task done channel.
	<-s.running
	for {
		select {
		case p := <-s.versionNegotiationQueue:
			s.packetOutput.draining = rejectedPacket{receivedPacket: p}
			releaseClosedPacket(&s.packetOutput.draining.receivedPacket)
			s.packetOutput.draining = rejectedPacket{}
		default:
			goto otherQueues
		}
	}
otherQueues:
	for _, queue := range [3]chan rejectedPacket{s.invalidTokenQueue, s.connectionRefusedQueue, s.retryQueue} {
		s.drainServerResponseQueue(queue)
	}
	s.sendReturned = true
	cleanupReturned = true
}

func (s *baseServer) drainServerResponseQueue(queue chan rejectedPacket) {
	for {
		select {
		case p := <-queue:
			s.packetOutput.draining = p
			releaseClosedPacket(&s.packetOutput.draining.receivedPacket)
			s.packetOutput.draining = rejectedPacket{}
		default:
			return
		}
	}
}
