package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/quicvarint"
)

type peerParameterPhase uint8

const (
	peerParametersRestored peerParameterPhase = iota + 1
	peerParametersReceived
	peerParametersApplied
	peerParametersRejected0RTT
)

var (
	errPeerParameterState    = errors.New("QUIC peer parameters unavailable")
	errPeerDatagramsDisabled = errors.New("datagram support disabled")
)

// Immutable scalar copies only. The native wire object belongs to Conn.run;
// receiving a fresh value does not yet authorize using it for outbound 0-RTT.
type peerParameterSnapshot struct {
	phase                  peerParameterPhase
	maxDatagramFrameSize   protocol.ByteCount
	maxDatagramPayloadSize protocol.ByteCount
	enableResetStreamAt    bool
	disableActiveMigration bool
}

func snapshotPeerParameters(params *wire.TransportParameters, phase peerParameterPhase) *peerParameterSnapshot {
	snapshot := &peerParameterSnapshot{
		phase:                  phase,
		maxDatagramFrameSize:   params.MaxDatagramFrameSize,
		enableResetStreamAt:    params.EnableResetStreamAt,
		disableActiveMigration: params.DisableActiveMigration,
	}
	// Type and LEN are both present in our queued encoding. Start with the
	// one-byte LEN allowance and correct for its actual width (at most 8).
	if size := snapshot.maxDatagramFrameSize; size >= 2 && size <= protocol.ByteCount(quicvarint.Max) {
		payload := size - 2
		for 1+protocol.ByteCount(quicvarint.Len(uint64(payload))) > size-payload {
			payload--
		}
		snapshot.maxDatagramPayloadSize = payload
	}
	return snapshot
}

func (s *peerParameterSnapshot) datagramError(size int) error {
	if s == nil {
		return errPeerDatagramsDisabled
	}
	if s.phase == peerParametersRejected0RTT {
		return Err0RTTRejected
	}
	if s.phase != peerParametersRestored && s.phase != peerParametersApplied {
		return errPeerParameterState
	}
	if s.maxDatagramFrameSize <= 0 {
		return errPeerDatagramsDisabled
	}
	if size < 0 || s.maxDatagramFrameSize > protocol.ByteCount(quicvarint.Max) {
		return errPeerParameterState
	}
	// Check before converting to a varint. Subtraction avoids overflow; the
	// zero-payload case still needs its two-byte header (RFC 9221 sections 3/4).
	payload := protocol.ByteCount(size)
	if payload > s.maxDatagramFrameSize ||
		1+protocol.ByteCount(quicvarint.Len(uint64(size))) > s.maxDatagramFrameSize-payload {
		return &DatagramTooLargeError{MaxDatagramPayloadSize: int64(s.maxDatagramPayloadSize)}
	}
	return nil
}

// Conn.run is the only writer. Actual preSetup binds the queue to this atomic
// head before exposure. Publication and queue admission/history transfer share
// sendMx; no custom context or scheduling callback runs under that mutex.
func (c *Conn) publishPeerSendParameters(snapshot *peerParameterSnapshot) {
	h := c.datagramQueue
	if h == nil { // private construction seam, before a production queue exists
		c.peerSendParams.Store(snapshot)
		return
	}
	changed := make(chan struct{})
	h.sendMx.Lock()
	c.peerSendParams.Store(snapshot)
	close(h.peerChanged)
	h.peerChanged = changed
	wake := !h.sendQueue.Empty()
	h.sendMx.Unlock()
	if wake {
		h.hasData()
	}
}

// The caller holds sendMx (and, for admission, the original scope lock).
// Unbound private queues retain their original generic AddCopy contract.
func (h *datagramQueue) peerDatagramErrorLocked(size int) error {
	if h.peerParams == nil {
		return nil
	}
	return h.peerParams.Load().datagramError(size)
}
