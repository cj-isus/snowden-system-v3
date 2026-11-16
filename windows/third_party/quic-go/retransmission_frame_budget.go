package quic

import "github.com/sagernet/quic-go/internal/wire"

// Shared immutable pending-view credit; framer and retransmission retain their
// own admission, operation and retirement owners.
type queuedFrameEntry struct {
	frame wire.Frame
	bytes int // immutable credit; cleanup never re-reads a pooled frame
}

// Admission inspects only known native frame layouts, never a caller-supplied
// Frame.Length method. Fixed fields are covered by the entry-count quota.
// Payload credit is a referenced view, not unique physical backing or RSS.
func queuedFramePayloadBytes(frame wire.Frame) (int, error) {
	switch f := frame.(type) {
	case *wire.CryptoFrame:
		if f != nil {
			return cap(f.Data), nil
		}
	case *wire.NewTokenFrame:
		if f != nil {
			return cap(f.Token), nil
		}
	case *wire.ConnectionCloseFrame:
		if f != nil {
			return len(f.ReasonPhrase), nil
		}
	case *wire.PingFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.HandshakeDoneFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.MaxDataFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.MaxStreamDataFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.MaxStreamsFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.StreamDataBlockedFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.StreamsBlockedFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.DataBlockedFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.RetireConnectionIDFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.NewConnectionIDFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.ResetStreamFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.StopSendingFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.AckFrequencyFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.ImmediateAckFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.PathChallengeFrame:
		if f != nil {
			return 0, nil
		}
	case *wire.PathResponseFrame:
		if f != nil {
			return 0, nil
		}
	}
	// STREAM/DATAGRAM have their own original handlers; ACK is never retransmitted.
	return 0, errRetransmissionState
}
