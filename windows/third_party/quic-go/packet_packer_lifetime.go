package quic

import (
	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
)

// The serialized packer owns partial output until an ordinary result transfer.
// AppendPacket borrows the Conn's existing buffer; it never returns that buffer
// to the pool. DATAGRAM history credits remain the original queue entries.
type packetPackerIntake struct {
	original, refused                   *packetBuffer
	datagrams                           [maxPacketOutputDatagrams]*datagramSendEntry
	refusedDatagram                     *datagramSendEntry
	count                               int
	active, borrowed, releasing, failed bool
}

func (s *packetPackerIntake) begin(buf *packetBuffer, borrowed bool) error {
	if s.active || s.failed || s.releasing || s.original != nil || borrowed && buf == nil {
		if s.refused == nil && buf != s.original {
			s.refused = buf
		}
		s.failed = true
		return errPacketOutputUncertain
	}
	s.active, s.borrowed, s.original = true, borrowed, buf
	return nil
}

func (s *packetPackerIntake) acquire() *packetBuffer {
	if !s.active || s.failed || s.borrowed || s.releasing || s.original != nil {
		s.failed = true
		panic(errPacketOutputUncertain)
	}
	s.original = getPacketBuffer()
	return s.original
}

func (s *packetPackerIntake) retain(entry *datagramSendEntry) {
	for _, previous := range s.datagrams[:s.count] {
		if previous == entry {
			return
		}
	}
	if !s.active || s.failed || s.releasing || s.count == len(s.datagrams) {
		if s.refusedDatagram == nil {
			s.refusedDatagram = entry
		}
		s.failed = true
		panic(errPacketOutputUncertain)
	}
	s.datagrams[s.count] = entry
	s.count++
}

func (s *packetPackerIntake) retainFrames(frames []ackhandler.Frame) {
	for _, frame := range frames {
		if entry, ok := frame.Handler.(*datagramSendEntry); ok {
			s.retain(entry)
		}
	}
}

func (s *packetPackerIntake) guard(returned *bool) {
	if !*returned {
		s.failed = true // retain original partial buffer/credits; no speculative cleanup
	}
}

func (s *packetPackerIntake) finish(transferred bool) error {
	if !s.active || s.failed || s.releasing {
		s.failed = true
		return errPacketOutputUncertain
	}
	if !transferred {
		s.releasing = true
		for i := 0; i < s.count; i++ {
			entry := s.datagrams[i]
			entry.retireHistory(&entry.frame)
			s.datagrams[i] = nil
		}
		if !s.borrowed && s.original != nil {
			s.original.Release()
		}
		s.releasing = false
	}
	// No raw-buffer reads after transfer or after the actual pool return.
	s.original = nil
	clear(s.datagrams[:s.count])
	s.count = 0
	s.active, s.borrowed = false, false
	return nil
}

func (c *Conn) packetPackerQuiescent() bool {
	p, ok := c.packer.(*packetPacker)
	return ok && !p.buffers.active && !p.buffers.failed && !p.buffers.releasing && p.buffers.original == nil && p.buffers.count == 0
}

func (p *packetPacker) PackCoalescedPacket(onlyAck bool, maxSize protocol.ByteCount, now monotime.Time, v protocol.Version) (*coalescedPacket, error) {
	if err := p.buffers.begin(nil, false); err != nil {
		return nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, err := p.packCoalescedPacket(onlyAck, maxSize, now, v)
	if finishErr := p.buffers.finish(err == nil && packet != nil); finishErr != nil {
		packet, err = nil, finishErr
	}
	returned = true
	return packet, err
}

func (p *packetPacker) packConnectionClose(isApplicationError bool, errorCode, frameType uint64, reason string, maxSize protocol.ByteCount, v protocol.Version) (*coalescedPacket, error) {
	if err := p.buffers.begin(nil, false); err != nil {
		return nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, err := p.packConnectionCloseIntoBuffer(isApplicationError, errorCode, frameType, reason, maxSize, v)
	if finishErr := p.buffers.finish(err == nil && packet != nil); finishErr != nil {
		packet, err = nil, finishErr
	}
	returned = true
	return packet, err
}

func (p *packetPacker) PackAckOnlyPacket(maxSize protocol.ByteCount, now monotime.Time, v protocol.Version) (shortHeaderPacket, *packetBuffer, error) {
	if err := p.buffers.begin(nil, false); err != nil {
		return shortHeaderPacket{}, nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, buf, err := p.packAckOnlyPacket(maxSize, now, v)
	if finishErr := p.buffers.finish(err == nil); finishErr != nil {
		err = finishErr
	}
	if err != nil {
		packet, buf = shortHeaderPacket{}, nil
	}
	returned = true
	return packet, buf, err
}

// AppendPacket borrows an existing Conn/GSO buffer through actual packing return.
func (p *packetPacker) AppendPacket(buf *packetBuffer, maxSize protocol.ByteCount, now monotime.Time, v protocol.Version) (shortHeaderPacket, error) {
	if err := p.buffers.begin(buf, true); err != nil {
		return shortHeaderPacket{}, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, err := p.appendPacket(buf, false, maxSize, now, v)
	if finishErr := p.buffers.finish(err == nil); finishErr != nil {
		packet, err = shortHeaderPacket{}, finishErr
	}
	returned = true
	return packet, err
}

func (p *packetPacker) PackPTOProbePacket(encLevel protocol.EncryptionLevel, maxSize protocol.ByteCount, addPingIfEmpty bool, now monotime.Time, v protocol.Version) (*coalescedPacket, error) {
	if err := p.buffers.begin(nil, false); err != nil {
		return nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, err := p.packPTOProbePacket(encLevel, maxSize, addPingIfEmpty, now, v)
	if finishErr := p.buffers.finish(err == nil && packet != nil); finishErr != nil {
		packet, err = nil, finishErr
	}
	returned = true
	return packet, err
}

func (p *packetPacker) PackMTUProbePacket(ping ackhandler.Frame, size protocol.ByteCount, v protocol.Version) (shortHeaderPacket, *packetBuffer, error) {
	if err := p.buffers.begin(nil, false); err != nil {
		return shortHeaderPacket{}, nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	packet, buf, err := p.packMTUProbePacket(ping, size, v)
	if finishErr := p.buffers.finish(err == nil); finishErr != nil {
		err = finishErr
	}
	if err != nil {
		packet, buf = shortHeaderPacket{}, nil
	}
	returned = true
	return packet, buf, err
}

func (p *packetPacker) PackPathProbePacket(connID protocol.ConnectionID, frames []ackhandler.Frame, v protocol.Version) (shortHeaderPacket, *packetBuffer, error) {
	return p.PackPathProbePacketWithLimit(connID, frames, v, protocol.MinInitialPacketSize)
}

func (p *packetPacker) PackPathProbePacketWithLimit(connID protocol.ConnectionID, frames []ackhandler.Frame, v protocol.Version, limit protocol.ByteCount) (shortHeaderPacket, *packetBuffer, error) {
	if limit <= 0 || limit > protocol.MinInitialPacketSize {
		return shortHeaderPacket{}, nil, errPathProbeCredit
	}
	if err := p.buffers.begin(nil, false); err != nil {
		return shortHeaderPacket{}, nil, err
	}
	returned := false
	defer p.buffers.guard(&returned)
	p.buffers.retainFrames(frames)
	packet, buf, err := p.packPathProbePacket(connID, frames, v, limit)
	if finishErr := p.buffers.finish(err == nil); finishErr != nil {
		err = finishErr
	}
	if err != nil {
		packet, buf = shortHeaderPacket{}, nil
	}
	returned = true
	return packet, buf, err
}
