package quic

import "github.com/sagernet/quic-go/internal/protocol"

func (s *SendStream) newSentFrame(pooled bool) (*sentStreamData, error) {
	return s.callBudget.sent.allocate(s, pooled)
}

// Both original and output remain charged before the split's pool allocation.
// Each keeps a full-size pool backing; only private stream-owned views are swapped.
func (s *SendStream) splitRetransmission(original *sentStreamData, maxSize protocol.ByteCount, version protocol.Version) (*sentStreamData, bool) {
	frame := original.raw()
	if maxSize >= frame.Length(version) {
		return nil, false
	}
	n := frame.MaxDataLen(maxSize, version)
	if n == 0 {
		return nil, true
	}
	if original.charge != protocol.MaxPacketBufferSize {
		original.owner.markUncertain()
		return nil, true
	}
	output, err := s.newSentFrame(true)
	if err != nil {
		return nil, true
	}
	part := output.raw()
	part.StreamID, part.Offset = frame.StreamID, frame.Offset
	part.Fin, part.DataLenPresent = false, frame.DataLenPresent
	part.Data, frame.Data = frame.Data, part.Data
	frame.Data = frame.Data[:protocol.ByteCount(len(part.Data))-n]
	copy(frame.Data, part.Data[n:])
	part.Data = part.Data[:n]
	frame.Offset += n
	return output, true
}
