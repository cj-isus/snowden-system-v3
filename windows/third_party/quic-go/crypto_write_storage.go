package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
)

const (
	maxCryptoWriteBytes   = 8 << 20
	maxCryptoWriteBacking = 16 << 20
	maxCryptoWriteCalls   = 4096
	maxCryptoWriteBuffers = 64
	minCryptoWriteBuffer  = 16 << 10
)

var (
	errCryptoWriteCapacity = errors.New("quic: crypto transmit storage limit")
	errCryptoWriteState    = errors.New("quic: original crypto transmit storage is uncertain")
	errCryptoWriteRetired  = errors.New("quic: crypto transmit storage retired")
)

// Original stream-owned backing, including buffers still aliased by packet
// frames/history after Pop. Lifetime credit is not returned on queue drain/ACK.
// This excludes copies made by retransmission splitting and native TLS storage.
type cryptoWriteStorage struct {
	owner                *baseCryptoStream
	buffers              [maxCryptoWriteBuffers][]byte
	allocations, backing int
	writes, written      int
	active, retired      bool
	failure              error
}

func (s *baseCryptoStream) cryptoWriteStateValid() bool {
	r := &s.writeStore
	return r.owner == s && !r.active && !r.retired &&
		r.allocations >= 0 && r.allocations <= maxCryptoWriteBuffers &&
		r.backing >= 0 && r.backing <= maxCryptoWriteBacking &&
		r.writes >= 0 && r.writes <= maxCryptoWriteCalls &&
		r.written >= r.writes && r.written <= maxCryptoWriteBytes &&
		(r.writes == 0) == (r.written == 0) &&
		(r.written == 0) == (r.allocations == 0) &&
		(r.allocations == 0) == (r.backing == 0) &&
		r.backing >= r.allocations*minCryptoWriteBuffer && r.written <= r.backing &&
		len(s.writeBuf) <= r.written && cap(s.writeBuf) <= maxCryptoWriteBytes &&
		s.writeOffset >= 0 && s.writeOffset <= protocol.ByteCount(r.written) &&
		s.cryptoWriteViewValid()
}

func (s *baseCryptoStream) cryptoWriteViewValid() bool {
	if cap(s.writeBuf) == 0 {
		return len(s.writeBuf) == 0
	}
	r := &s.writeStore
	if r.allocations <= 0 || r.allocations > maxCryptoWriteBuffers {
		return false
	}
	last := r.buffers[r.allocations-1]
	if len(last) == 0 || len(last) != cap(last) || cap(s.writeBuf) > len(last) {
		return false
	}
	view := s.writeBuf[:cap(s.writeBuf)]
	return &view[0] == &last[len(last)-cap(s.writeBuf)]
}

func (s *baseCryptoStream) writeCrypto(p []byte) (n int, result error) {
	r := &s.writeStore
	if r.retired {
		return 0, errCryptoWriteRetired
	}
	if !s.cryptoWriteStateValid() {
		r.failure = errCryptoWriteState
		return 0, r.failure
	}
	if r.failure != nil {
		return 0, r.failure
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.writes == maxCryptoWriteCalls || len(p) > maxCryptoWriteBytes-r.written {
		r.failure = errCryptoWriteCapacity
		return 0, r.failure
	}
	length := len(s.writeBuf) + len(p) // bounded by accepted lifetime bytes above
	capacity := 0
	if length > cap(s.writeBuf) {
		capacity = min(max(minCryptoWriteBuffer, length, 2*cap(s.writeBuf)), maxCryptoWriteBytes)
		if r.allocations == maxCryptoWriteBuffers || capacity > maxCryptoWriteBacking-r.backing {
			r.failure = errCryptoWriteCapacity
			return 0, r.failure
		}
		if r.buffers[r.allocations] != nil {
			r.failure = errCryptoWriteState
			return 0, r.failure
		}
	}
	returned := false
	r.active = true
	defer func() {
		if !returned {
			r.failure = errCryptoWriteState
			result = errCryptoWriteState // original active/prefix stays retained
		}
	}()
	r.writes++
	r.written += len(p) // reserve original lifetime credit before make/copy
	if capacity != 0 {
		index := r.allocations
		r.allocations++
		r.backing += capacity
		r.buffers[index] = make([]byte, capacity) // original slot before any copy
		copy(r.buffers[index], s.writeBuf)
		s.writeBuf = r.buffers[index][:len(s.writeBuf)]
	}
	previous := len(s.writeBuf)
	s.writeBuf = s.writeBuf[:length]
	copy(s.writeBuf[previous:], p)
	r.active = false
	returned = true
	return len(p), nil
}

func (s *baseCryptoStream) cryptoWriteReadable() bool {
	r := &s.writeStore
	return r.owner == s && !r.retired && !r.active &&
		(r.failure == nil || r.failure == errCryptoWriteCapacity)
}

// External preflight after native/public/data/history/TLS receipts. The caller
// also joins event/key retirement before clearing any stream references.
func (s *baseCryptoStream) cryptoWriteMetadataReady() bool {
	r := &s.writeStore
	if !s.cryptoWriteStateValid() || !s.cryptoWriteReadable() {
		return false
	}
	backing := 0
	for i, buffer := range r.buffers {
		if i >= r.allocations {
			if buffer != nil {
				return false
			}
			continue
		}
		if len(buffer) == 0 || len(buffer) != cap(buffer) ||
			len(buffer) > maxCryptoWriteBytes || len(buffer) > maxCryptoWriteBacking-backing {
			return false
		}
		for j := 0; j < i; j++ {
			if &buffer[0] == &r.buffers[j][0] {
				return false
			}
		}
		backing += len(buffer)
	}
	if backing != r.backing {
		return false
	}
	return true
}

// Caller has preflighted all original streams and joined their native aliases.
// Drop reference cells only; counters/failure remain diagnostic, bytes unchanged.
func (s *baseCryptoStream) retireCryptoWriteStorage() {
	s.writeStore.retired = true
	s.writeBuf = nil
	s.writeStore.buffers = [maxCryptoWriteBuffers][]byte{}
}
