package quic

import (
	"errors"
	"math/bits"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

const cryptoReceiveBytes = int(protocol.MaxCryptoStreamOffset)
const cryptoReceiveWords = (cryptoReceiveBytes + 63) / 64

var (
	errCryptoReceiveState   = errors.New("quic: original crypto receive storage is uncertain")
	errCryptoReceiveRetired = errors.New("quic: crypto receive storage retired")
)

// A single lazy block for the already bounded CRYPTO offset domain. Bitmap
// metadata never escapes; accepted payload bytes are written only once, so TLS
// may retain an earlier capped view while later unseen offsets are filled.
type cryptoReceiveBlock struct {
	data [cryptoReceiveBytes]byte
	seen [cryptoReceiveWords]uint64
}

type cryptoReceiveStorage struct {
	owner                   *baseCryptoStream
	block                   *cryptoReceiveBlock
	read, accepted, pending int
	active, retired         bool
	failure                 error
}

func (s *baseCryptoStream) cryptoReceiveStateError() error {
	if s == nil {
		return errCryptoReceiveState
	}
	r := &s.receiveStore
	if r.retired {
		return errCryptoReceiveRetired
	}
	if r.owner != s || r.active || r.failure != nil ||
		s.highestOffset < 0 || s.highestOffset > protocol.MaxCryptoStreamOffset ||
		r.read < 0 || r.read > cryptoReceiveBytes ||
		r.accepted < 0 || r.accepted > int(s.highestOffset) ||
		r.pending < 0 || r.pending > cryptoReceiveBytes ||
		r.accepted != r.read+r.pending ||
		(r.block == nil) != (r.accepted == 0) {
		r.failure = errCryptoReceiveState
		return r.failure
	}
	return nil
}

func (s *baseCryptoStream) admitCryptoFrame(frame *wire.CryptoFrame) (result error) {
	if err := s.cryptoReceiveStateError(); err != nil {
		return err
	}
	if frame == nil {
		s.receiveStore.failure = errCryptoReceiveState
		return errCryptoReceiveState
	}
	// Validate before integer addition, lazy block allocation or payload copy.
	if frame.Offset < 0 || frame.Offset > protocol.MaxCryptoStreamOffset ||
		len(frame.Data) > cryptoReceiveBytes-int(frame.Offset) {
		return &qerr.TransportError{ErrorCode: qerr.CryptoBufferExceeded, ErrorMessage: "received invalid crypto stream range"}
	}
	end := frame.Offset + protocol.ByteCount(len(frame.Data))
	if s.finished {
		if end > s.highestOffset {
			return &qerr.TransportError{ErrorCode: qerr.ProtocolViolation, ErrorMessage: "received crypto data after change of encryption level"}
		}
		return nil
	}
	if len(frame.Data) == 0 {
		s.highestOffset = max(s.highestOffset, end)
		return nil
	}
	r := &s.receiveStore
	returned := false
	r.active = true // original owned slot before lazy allocation, no raw view kept
	defer func() {
		if !returned {
			r.failure, result = errCryptoReceiveState, errCryptoReceiveState
		}
	}()
	if r.block == nil {
		r.block = &cryptoReceiveBlock{}
	}
	for i, value := range frame.Data {
		position := int(frame.Offset) + i
		mask := uint64(1) << uint(position&63)
		if r.block.seen[position>>6]&mask != 0 {
			continue // never rewrite an accepted byte or an earlier TLS view
		}
		r.block.data[position] = value
		r.block.seen[position>>6] |= mask
		r.accepted++
		r.pending++
	}
	s.highestOffset = max(s.highestOffset, end)
	r.active = false
	returned = true
	return nil
}

func (s *baseCryptoStream) receiveCryptoData() []byte {
	if s.cryptoReceiveStateError() != nil {
		return nil
	}
	r := &s.receiveStore
	if r.block == nil {
		return nil
	}
	start, end := r.read, r.read
	for end < int(s.highestOffset) && r.block.seen[end>>6]&(uint64(1)<<uint(end&63)) != 0 {
		end++
	}
	if start == end {
		return nil
	}
	if end-start > r.pending {
		r.failure = errCryptoReceiveState
		return nil
	}
	returned := false
	r.active = true
	defer func() {
		if !returned {
			r.failure = errCryptoReceiveState
		}
	}()
	data := r.block.data[start:end:end]
	r.read, r.pending = end, r.pending-(end-start)
	r.active = false
	returned = true
	return data // block stays owned through the original TLS/native close receipt
}

func (s *baseCryptoStream) cryptoReceiveMetadataReady() bool {
	if s.cryptoReceiveStateError() != nil {
		return false
	}
	r := &s.receiveStore
	if r.block == nil {
		return true
	}
	count := 0
	for i, word := range r.block.seen {
		start := i * 64
		prefix := min(max(r.read-start, 0), 64)
		prefixMask := ^uint64(0) >> uint(64-prefix)
		valid := min(max(int(s.highestOffset)-start, 0), 64)
		validMask := ^uint64(0) >> uint(64-valid)
		if word&prefixMask != prefixMask || word&^validMask != 0 {
			return false
		}
		count += bits.OnesCount64(word)
	}
	return count == r.accepted
}

// External aggregate has preflighted all original streams and joined native/
// history/TLS/event/key operations. Drop only this local block reference.
func (s *baseCryptoStream) retireCryptoReceiveStorage() {
	s.receiveStore.retired = true
	s.receiveStore.block = nil // counters retained; provider payload views are unchanged
}

func (c *Conn) cryptoReceiveStateError() error {
	if c == nil || c.initialStream == nil || c.handshakeStream == nil || c.oneRTTStream == nil {
		return errCryptoReceiveState
	}
	for _, stream := range [3]*baseCryptoStream{&c.initialStream.baseCryptoStream, &c.handshakeStream.baseCryptoStream, &c.oneRTTStream.baseCryptoStream} {
		if err := stream.cryptoReceiveStateError(); err != nil {
			return err
		}
	}
	return nil
}
