package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

const (
	maxRetransmissionEntries    = 4096
	maxRetransmissionBytes      = 16 << 20
	maxRetransmissionFrameBytes = 1 << 20
	maxRetransmissionCopies     = 4096
	maxRetransmissionCopyBytes  = 16 << 20
)

var (
	errRetransmissionCapacity = errors.New("quic: retransmission storage limit")
	errRetransmissionState    = errors.New("quic: original retransmission storage is uncertain")
	errRetransmissionRetired  = errors.New("quic: retransmission storage retired")
)

type retransmissionStage uint8

const (
	retransmissionIdle retransmissionStage = iota
	retransmissionEnqueue
	retransmissionRead
	retransmissionSplit
	retransmissionDrop
	retransmissionRetire
)

// One original serialized operation. Abnormal exit retains every published
// candidate/head here; it cannot be retried or mistaken for ordinary pressure.
type retransmissionOperation struct {
	stage         retransmissionStage
	input         wire.Frame
	output        *wire.CryptoFrame
	beforeData    []byte
	growing       []queuedFrameEntry
	growingCopies [][]byte
	spaces        [3]framesToRetransmit
	copies        [][]byte
}

type retransmissionStorage struct {
	owner                *retransmissionQueue
	entries, bytes       int
	copyCount, copyBytes int // lifetime; no ACK/pop/space-drop refund
	copies               [][]byte
	operation            retransmissionOperation
	failure              error
}

func (q *retransmissionQueue) stateError() error {
	if q == nil {
		return errRetransmissionState
	}
	r := &q.storage
	if r.owner != q || r.operation.stage != retransmissionIdle ||
		r.failure != nil && r.failure != errRetransmissionCapacity ||
		r.entries < 0 || r.entries > maxRetransmissionEntries ||
		r.bytes < 0 || r.bytes > maxRetransmissionBytes ||
		r.copyCount < 0 || r.copyCount > maxRetransmissionCopies ||
		r.copyBytes < 0 || r.copyBytes > maxRetransmissionCopyBytes ||
		len(r.copies) != r.copyCount || cap(r.copies) > maxRetransmissionCopies {
		return errRetransmissionState
	}
	if q.initial != nil && (q.initial == q.handshake || q.initial == &q.appData) ||
		q.handshake == &q.appData {
		return errRetransmissionState
	}
	entries := 0
	for _, space := range [3]*framesToRetransmit{q.initial, q.handshake, &q.appData} {
		if space == nil {
			continue
		}
		for _, queue := range [2][]queuedFrameEntry{space.crypto, space.other} {
			if len(queue) > maxRetransmissionEntries || cap(queue) > maxRetransmissionEntries ||
				len(queue) > maxRetransmissionEntries-entries {
				return errRetransmissionState
			}
			entries += len(queue)
		}
	}
	if entries != r.entries {
		return errRetransmissionState
	}
	return nil
}

func (q *retransmissionQueue) failureError() error {
	if q != nil && q.retired {
		return errRetransmissionRetired
	}
	if err := q.stateError(); err != nil {
		if q != nil {
			q.storage.failure = errRetransmissionState
		}
		return err
	}
	return q.storage.failure
}

func (q *retransmissionQueue) failureAfter(err error) error {
	if queueErr := q.failureError(); queueErr != nil {
		return errors.Join(err, queueErr)
	}
	return err // preserve the original error identity on the ordinary path
}

func (q *retransmissionQueue) guardOperation(returned *bool) {
	if !*returned {
		q.storage.failure = errRetransmissionState
	}
}

func (q *retransmissionQueue) enqueue(space *framesToRetransmit, frame wire.Frame) {
	if q.retired || space == nil || q.failureError() != nil {
		return
	}
	r := &q.storage
	r.operation = retransmissionOperation{stage: retransmissionEnqueue, input: frame}
	returned := false
	defer q.guardOperation(&returned)
	bytes, err := queuedFramePayloadBytes(frame)
	if err != nil {
		return
	}
	if bytes > maxRetransmissionFrameBytes || r.entries == maxRetransmissionEntries ||
		bytes > maxRetransmissionBytes-r.bytes {
		r.failure = errRetransmissionCapacity
		r.operation = retransmissionOperation{}
		returned = true
		return
	}
	queue := &space.other
	if f, ok := frame.(*wire.CryptoFrame); ok {
		if f.Offset < 0 || f.Offset > protocol.MaxByteCount-protocol.ByteCount(len(f.Data)) {
			return
		}
		queue = &space.crypto
	}
	if len(*queue) > r.entries || len(*queue) >= maxRetransmissionEntries || cap(*queue) > maxRetransmissionEntries {
		return
	}
	r.entries++
	r.bytes += bytes // original credit before metadata allocation/adoption
	if len(*queue) == cap(*queue) {
		capacity := min(max(16, 2*cap(*queue)), maxRetransmissionEntries)
		r.operation.growing = make([]queuedFrameEntry, len(*queue), capacity)
		copy(r.operation.growing, *queue)
		*queue = r.operation.growing
		r.operation.growing = nil
	}
	*queue = append(*queue, queuedFrameEntry{frame: frame, bytes: bytes})
	r.operation = retransmissionOperation{}
	returned = true
}

func (q *retransmissionQueue) pop(queue *[]queuedFrameEntry) wire.Frame {
	r := &q.storage
	entry := (*queue)[0]
	if r.entries <= 0 || entry.bytes < 0 || entry.bytes > r.bytes {
		r.failure = errRetransmissionState
		return nil
	}
	(*queue)[0] = queuedFrameEntry{}
	*queue = (*queue)[1:]
	if len(*queue) == 0 {
		*queue = nil
	}
	r.entries--
	r.bytes -= entry.bytes // pending reference credit only, never split backing
	return entry.frame
}

func (q *retransmissionQueue) getCryptoFrame(space *framesToRetransmit, maxLen protocol.ByteCount, v protocol.Version) *wire.CryptoFrame {
	r := &q.storage
	entry := space.crypto[0]
	r.operation = retransmissionOperation{stage: retransmissionRead, input: entry.frame}
	returned := false
	defer q.guardOperation(&returned)
	f, ok := entry.frame.(*wire.CryptoFrame)
	if !ok || f == nil || entry.bytes != cap(f.Data) || entry.bytes > maxRetransmissionFrameBytes ||
		f.Offset < 0 || f.Offset > protocol.MaxByteCount-protocol.ByteCount(len(f.Data)) {
		return nil
	}
	if f.Length(v) <= maxLen {
		if q.pop(&space.crypto) == nil {
			return nil
		}
		r.operation = retransmissionOperation{}
		returned = true
		return f
	}
	n := int(f.MaxDataLen(maxLen))
	if n == 0 {
		r.operation = retransmissionOperation{}
		returned = true
		return nil
	}
	if n < 0 || n >= len(f.Data) || r.entries <= 0 || entry.bytes > r.bytes {
		return nil
	}
	tail := len(f.Data) - n
	if r.copyCount == maxRetransmissionCopies || tail > maxRetransmissionCopyBytes-r.copyBytes {
		r.failure = errRetransmissionCapacity
		r.operation = retransmissionOperation{}
		returned = true
		return nil
	}
	// Preserve legacy tail-copy semantics without an unowned allocation hidden
	// inside wire.MaybeSplitOffFrame. The wire API itself is unchanged.
	r.operation.stage, r.operation.beforeData = retransmissionSplit, f.Data
	r.operation.copies = r.copies
	r.copyCount++
	r.copyBytes += tail
	if len(r.copies) == cap(r.copies) {
		capacity := min(max(16, 2*cap(r.copies)), maxRetransmissionCopies)
		r.operation.growingCopies = make([][]byte, len(r.copies), capacity)
		copy(r.operation.growingCopies, r.copies)
		r.copies = r.operation.growingCopies
		r.operation.growingCopies = nil
	}
	index := len(r.copies)
	r.copies = append(r.copies, nil)
	r.operation.output = &wire.CryptoFrame{Offset: f.Offset, Data: f.Data[:n:n]}
	r.copies[index] = make([]byte, tail) // original slot before the payload copy
	copy(r.copies[index], f.Data[n:])
	f.Data, f.Offset = r.copies[index], f.Offset+protocol.ByteCount(n)
	r.bytes -= entry.bytes - tail
	space.crypto[0].bytes = tail
	output := r.operation.output
	r.operation = retransmissionOperation{}
	returned = true
	return output
}

func (q *retransmissionQueue) getControlFrame(space *framesToRetransmit, maxLen protocol.ByteCount, v protocol.Version) wire.Frame {
	r := &q.storage
	entry := space.other[0]
	r.operation = retransmissionOperation{stage: retransmissionRead, input: entry.frame}
	returned := false
	defer q.guardOperation(&returned)
	bytes, err := queuedFramePayloadBytes(entry.frame)
	if err != nil || bytes != entry.bytes || bytes > maxRetransmissionFrameBytes {
		return nil
	}
	length := entry.frame.Length(v) // known native type, original input already held
	if length < 0 || length > maxRetransmissionFrameBytes+128 {
		return nil
	}
	if length > maxLen {
		r.operation = retransmissionOperation{}
		returned = true
		return nil
	}
	frame := q.pop(&space.other)
	if frame == nil {
		return nil
	}
	r.operation = retransmissionOperation{}
	returned = true
	return frame
}

// No frame dereferences, callbacks or pools: credit is immutable entry metadata.
func retransmissionSpaceCredit(space *framesToRetransmit) (int, int, bool) {
	if space == nil {
		return 0, 0, true
	}
	count, bytes := 0, 0
	for _, queue := range [2][]queuedFrameEntry{space.crypto, space.other} {
		if len(queue) > maxRetransmissionEntries || cap(queue) > maxRetransmissionEntries {
			return 0, 0, false
		}
		for i, entry := range queue[:cap(queue)] {
			if i >= len(queue) {
				if entry.frame != nil || entry.bytes != 0 {
					return 0, 0, false
				}
				continue
			}
			if entry.frame == nil || entry.bytes < 0 || entry.bytes > maxRetransmissionFrameBytes ||
				entry.bytes > maxRetransmissionBytes-bytes {
				return 0, 0, false
			}
			count++
			bytes += entry.bytes
		}
	}
	return count, bytes, true
}

func (q *retransmissionQueue) metadataReady() bool {
	if q.stateError() != nil || q.retired ||
		q.initial != nil && (q.initial == q.handshake || q.initial == &q.appData) ||
		q.handshake == &q.appData {
		return false
	}
	count, bytes := 0, 0
	for _, space := range [3]*framesToRetransmit{q.initial, q.handshake, &q.appData} {
		n, b, ok := retransmissionSpaceCredit(space)
		if !ok || n > maxRetransmissionEntries-count || b > maxRetransmissionBytes-bytes {
			return false
		}
		count, bytes = count+n, bytes+b
	}
	if count != q.storage.entries || bytes != q.storage.bytes {
		return false
	}
	backing := 0
	for i, buffer := range q.storage.copies[:cap(q.storage.copies)] {
		if i >= len(q.storage.copies) {
			if buffer != nil {
				return false
			}
			continue
		}
		if len(buffer) == 0 || cap(buffer) != len(buffer) || len(buffer) > maxRetransmissionFrameBytes ||
			len(buffer) > maxRetransmissionCopyBytes-backing {
			return false
		}
		backing += len(buffer)
	}
	return backing == q.storage.copyBytes
}

func (q *retransmissionQueue) snapshotSpaces() (spaces [3]framesToRetransmit) {
	if q.initial != nil {
		spaces[0] = *q.initial
	}
	if q.handshake != nil {
		spaces[1] = *q.handshake
	}
	spaces[2] = q.appData
	return
}

// External aggregate has preflighted original native/history/TLS/event/key
// completion. Keep the original heads on abnormal return; never wipe payloads.
func (q *retransmissionQueue) retireStorageReferences() {
	r := &q.storage
	r.operation = retransmissionOperation{stage: retransmissionRetire, spaces: q.snapshotSpaces(), copies: r.copies}
	returned := false
	defer q.guardOperation(&returned)
	q.retired = true
	q.initial, q.handshake, q.appData = nil, nil, framesToRetransmit{}
	r.entries, r.bytes, r.copies = 0, 0, nil
	r.operation = retransmissionOperation{}
	returned = true // lifetime copy counters and failure stay diagnostic
}
