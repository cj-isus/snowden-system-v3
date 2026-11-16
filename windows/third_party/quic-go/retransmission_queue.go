package quic

import (
	"fmt"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

type framesToRetransmit struct {
	crypto []queuedFrameEntry
	other  []queuedFrameEntry
}

type retransmissionQueue struct {
	retired   bool // native loop, or external finalizer after positive producer receipts
	initial   *framesToRetransmit
	handshake *framesToRetransmit
	appData   framesToRetransmit
	storage   retransmissionStorage
}

func newRetransmissionQueue() *retransmissionQueue {
	q := &retransmissionQueue{}
	q.init()
	return q
}

func (q *retransmissionQueue) init() {
	if q.storage.owner != nil || q.initial != nil || q.handshake != nil {
		panic(errRetransmissionState)
	}
	q.storage.owner = q
	q.initial, q.handshake = &framesToRetransmit{}, &framesToRetransmit{}
}

func (q *retransmissionQueue) addInitial(f wire.Frame) {
	q.enqueue(q.initial, f)
}

func (q *retransmissionQueue) addHandshake(f wire.Frame) {
	q.enqueue(q.handshake, f)
}

func (q *retransmissionQueue) addAppData(f wire.Frame) {
	q.enqueue(&q.appData, f)
}

func (q *retransmissionQueue) HasData(encLevel protocol.EncryptionLevel) bool {
	if q.retired || q.failureError() != nil {
		return false
	}
	//nolint:exhaustive // 0-RTT data is retransmitted in 1-RTT packets.
	switch encLevel {
	case protocol.EncryptionInitial:
		return q.initial != nil &&
			(len(q.initial.crypto) > 0 || len(q.initial.other) > 0)
	case protocol.EncryptionHandshake:
		return q.handshake != nil &&
			(len(q.handshake.crypto) > 0 || len(q.handshake.other) > 0)
	case protocol.Encryption1RTT:
		return len(q.appData.crypto) > 0 || len(q.appData.other) > 0
	}
	return false
}

func (q *retransmissionQueue) GetFrame(encLevel protocol.EncryptionLevel, maxLen protocol.ByteCount, v protocol.Version) wire.Frame {
	if q.retired || q.failureError() != nil {
		return nil
	}
	var r *framesToRetransmit
	//nolint:exhaustive // 0-RTT data is retransmitted in 1-RTT packets.
	switch encLevel {
	case protocol.EncryptionInitial:
		r = q.initial
	case protocol.EncryptionHandshake:
		r = q.handshake
	case protocol.Encryption1RTT:
		r = &q.appData
	}
	if r == nil {
		return nil
	}

	if len(r.crypto) > 0 {
		if f := q.getCryptoFrame(r, maxLen, v); f != nil {
			return f
		}
	}
	if q.failureError() != nil || len(r.other) == 0 {
		return nil
	}
	return q.getControlFrame(r, maxLen, v)
}

func (q *retransmissionQueue) DropPackets(encLevel protocol.EncryptionLevel) {
	if q.retired {
		return
	}
	var target **framesToRetransmit
	//nolint:exhaustive // Can only drop Initial and Handshake packet number space.
	switch encLevel {
	case protocol.EncryptionInitial:
		target = &q.initial
	case protocol.EncryptionHandshake:
		target = &q.handshake
	default:
		panic(fmt.Sprintf("unexpected encryption level: %s", encLevel))
	}
	if !q.metadataReady() {
		q.storage.failure = errRetransmissionState
		return
	}
	if *target == nil {
		return
	}
	r := &q.storage
	r.operation = retransmissionOperation{stage: retransmissionDrop, spaces: q.snapshotSpaces()}
	returned := false
	defer q.guardOperation(&returned)
	n, bytes, ok := retransmissionSpaceCredit(*target)
	if !ok || n > r.entries || bytes > r.bytes {
		return
	}
	r.entries, r.bytes = r.entries-n, r.bytes-bytes
	*target = nil // original heads stay in operation until the ordinary return
	r.operation = retransmissionOperation{}
	returned = true
}

func (q *retransmissionQueue) AckHandler(encLevel protocol.EncryptionLevel) ackhandler.FrameHandler {
	switch encLevel {
	case protocol.EncryptionInitial:
		return (*retransmissionQueueInitialAckHandler)(q)
	case protocol.EncryptionHandshake:
		return (*retransmissionQueueHandshakeAckHandler)(q)
	case protocol.Encryption0RTT, protocol.Encryption1RTT:
		return (*retransmissionQueueAppDataAckHandler)(q)
	}
	return nil
}

type retransmissionQueueInitialAckHandler retransmissionQueue

func (q *retransmissionQueueInitialAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueInitialAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addInitial(f)
}

type retransmissionQueueHandshakeAckHandler retransmissionQueue

func (q *retransmissionQueueHandshakeAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueHandshakeAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addHandshake(f)
}

type retransmissionQueueAppDataAckHandler retransmissionQueue

func (q *retransmissionQueueAppDataAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueAppDataAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addAppData(f)
}
