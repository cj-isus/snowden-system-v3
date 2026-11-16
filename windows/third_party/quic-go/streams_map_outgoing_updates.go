package quic

import (
	"unsafe"

	"github.com/sagernet/quic-go/internal/protocol"
)

const maxOutgoingUpdateBytes = 64 << 10

type outgoingUpdateKind uint8

const (
	outgoingWindowUpdate outgoingUpdateKind = iota
	outgoingResetUpdate
)

// One original prefix under factoryMutex, including snapshot allocation and
// actual callback return. No callback executes under the map metadata mutex.
type outgoingStreamUpdate[T outgoingStream] struct {
	active  bool
	kind    outgoingUpdateKind
	limit   protocol.ByteCount
	count   int
	charge  int
	streams []T
	next    int
	current T
}

func (m *outgoingStreamsMap[T]) updateStreams(kind outgoingUpdateKind, limit protocol.ByteCount) {
	if kind != outgoingWindowUpdate && kind != outgoingResetUpdate {
		panic(errOutgoingMapUncertain)
	}
	// The same original gate orders updates with factories and native close.
	// No metadata lock is held while waiting for it.
	m.factoryMutex.Lock()
	defer m.factoryMutex.Unlock()
	if !m.reserveUpdate(kind, limit) {
		return
	}
	returned := false
	defer m.finishUpdate(&returned)
	u := &m.updating
	u.streams = make([]T, u.count) // count and actual backing charge already reserved
	if !m.captureUpdateSnapshot() {
		returned = true // closed before any callback; the whole tail is uncalled
		return
	}
	for u.next < len(u.streams) {
		m.mutex.RLock()
		stopped := m.stateErrorLocked() != nil
		m.mutex.RUnlock()
		if stopped {
			break
		}
		u.current = u.streams[u.next]
		if u.kind == outgoingWindowUpdate {
			u.current.updateSendWindow(u.limit)
		} else {
			u.current.enableResetStreamAt()
		}
		// Clear only after the actual original callback returned normally.
		u.streams[u.next], u.current = *new(T), *new(T)
		u.next++
	}
	returned = true
}

func (m *outgoingStreamsMap[T]) reserveUpdate(kind outgoingUpdateKind, limit protocol.ByteCount) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stateErrorLocked() != nil {
		return false
	}
	count := len(m.streams)
	if m.creation.active || m.updating.active || count > maxOutgoingStreamRows || m.rowCount != count || m.rowBytes != count*outgoingStreamRowCharge {
		m.uncertain = true
		m.sealLocked(errOutgoingMapUncertain)
		panic(errOutgoingMapUncertain)
	}
	// Bound the actual generic element backing, not an assumed pointer size.
	// Division precedes multiplication to avoid overflow before the limit check.
	elementSize := unsafe.Sizeof(*new(T))
	if count > 0 && elementSize > uintptr(maxOutgoingUpdateBytes)/uintptr(count) {
		m.uncertain = true
		m.sealLocked(errOutgoingMapUncertain)
		panic(errOutgoingMapUncertain)
	}
	m.updating = outgoingStreamUpdate[T]{
		active: true, kind: kind, limit: limit, count: count,
		charge: int(elementSize * uintptr(count)),
	}
	return true
}

func (m *outgoingStreamsMap[T]) captureUpdateSnapshot() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stateErrorLocked() != nil {
		return false
	}
	// factoryMutex prevents additions. Delete may only have removed rows since
	// reservation; capture their original identities into the existing capacity.
	filled := 0
	for _, str := range m.streams {
		if filled >= len(m.updating.streams) {
			panic(errOutgoingMapUncertain)
		}
		m.updating.streams[filled] = str
		filled++
	}
	m.updating.streams = m.updating.streams[:filled]
	return true
}

func (m *outgoingStreamsMap[T]) finishUpdate(returned *bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*returned {
		m.uncertain = true
		m.sealLocked(errOutgoingMapUncertain)
		return // retain original kind/limit/snapshot/current/index/charged backing
	}
	clear(m.updating.streams) // an uncalled suffix at known close has no callback borrower
	m.updating = outgoingStreamUpdate[T]{}
}
