package quic

import (
	"errors"
	"net"
	"sync"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
)

var errPacketInputUncertain = errors.New("quic: packet input completion unconfirmed")

// One original basic reader is reserved before pool/provider work. access only
// guards this metadata; native I/O and pool operations never run under it.
// A malformed count or abnormal return keeps the original references attached.
type packetReadState struct {
	access         sync.Mutex
	active, failed bool
	source         net.PacketConn
	buffer         *packetBuffer
	address        net.Addr
}

func (c *basicConn) ReadPacket() (receivedPacket, error) {
	s := &c.read
	s.access.Lock()
	if s.failed {
		s.access.Unlock()
		return receivedPacket{}, errPacketInputUncertain
	}
	if s.active {
		s.access.Unlock()
		return receivedPacket{}, errTransportCallCapacity
	}
	s.active, s.source = true, c.PacketConn
	s.access.Unlock()
	returned := false
	defer func() {
		s.access.Lock()
		if returned {
			s.active = false
			s.source, s.buffer, s.address = nil, nil, nil
		} else {
			s.failed = true
		}
		s.access.Unlock()
	}()
	s.buffer = getPacketBuffer()
	s.buffer.Data = s.buffer.Data[:protocol.MaxPacketBufferSize]
	n, addr, err := s.source.ReadFrom(s.buffer.Data)
	s.address = addr
	if n < 0 || n > len(s.buffer.Data) {
		return receivedPacket{}, errPacketInputUncertain
	}
	if err != nil {
		s.buffer.Release() // actual ReadFrom returned; no input was transferred
		returned = true
		return receivedPacket{}, err
	}
	p := receivedPacket{
		remoteAddr: s.address,
		rcvTime:    monotime.Now(),
		data:       s.buffer.Data[:n],
		buffer:     s.buffer,
	}
	returned = true // the caller now owns this one original reference
	return p, nil
}
