package quic

import (
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
)

const (
	maxIncomingStreamRows     = 4096
	incomingStreamRowCharge   = 4 << 10
	maxIncomingStreamRowBytes = 16 << 20
)

var errIncomingMapUncertain = errors.New("quic: original incoming stream map operation is uncertain")

// One original batch, protected by factoryMutex. Reservation and publication
// also take the metadata mutex. Row charge is logical metadata credit, not RSS
// or stream payload backing (which has its own receive/send budgets).
type incomingStreamCreation[T incomingStream] struct {
	active       bool
	id           protocol.StreamID
	end          protocol.StreamID
	remaining    int
	factory      func(protocol.StreamID) T
	candidate    T
	result       T
	construction streamConstruction
}

func (m *incomingStreamsMap[T]) stateErrorLocked() error {
	if m.uncertain {
		return errIncomingMapUncertain
	}
	if m.closeStarted {
		return m.closeErr
	}
	return nil
}

func (m *incomingStreamsMap[T]) sealLocked(cause error) {
	if !m.closeStarted {
		if cause == nil {
			cause = net.ErrClosed
		}
		m.closeErr = cause
		m.closeStarted = true
		close(m.newStreamChan)
	}
}

func (m *incomingStreamsMap[T]) streamIfOpenedLocked(id protocol.StreamID) (T, bool) {
	if id >= m.nextStreamToOpen {
		return *new(T), false
	}
	if entry, ok := m.streams[id]; ok && !entry.shouldDelete {
		return entry.stream, true
	}
	return *new(T), true
}

// Caller owns both mutexes. Count the entire skipped-ID batch before the first
// factory, channel, sorter, flow-controller or map insertion can allocate.
func (m *incomingStreamsMap[T]) reserveCreationLocked(id protocol.StreamID) error {
	if m.creation.active || m.rowCount != len(m.streams) || m.rowCount < 0 || m.rowCount > maxIncomingStreamRows || m.rowBytes != m.rowCount*incomingStreamRowCharge {
		m.uncertain = true
		m.sealLocked(errIncomingMapUncertain)
		return errIncomingMapUncertain
	}
	if id < m.nextStreamToOpen || id > protocol.MaxStreamID || (id-m.nextStreamToOpen)%4 != 0 {
		return &qerr.TransportError{ErrorCode: qerr.StreamStateError, ErrorMessage: "invalid incoming stream ID"}
	}
	required := uint64((id-m.nextStreamToOpen)/4) + 1
	if required > uint64(maxIncomingStreamRows-m.rowCount) || required > uint64((maxIncomingStreamRowBytes-m.rowBytes)/incomingStreamRowCharge) {
		// A valid peer ID can exceed local resources without violating its
		// advertised stream limit. Do not misreport STREAM_LIMIT_ERROR.
		return &qerr.TransportError{ErrorCode: qerr.InternalError, ErrorMessage: "local incoming stream metadata capacity exceeded"}
	}
	m.rowCount += int(required)
	m.rowBytes += int(required) * incomingStreamRowCharge
	m.creation = incomingStreamCreation[T]{
		active: true, id: m.nextStreamToOpen, end: id,
		remaining: int(required), factory: m.newStream,
	}
	return nil
}

// factoryMutex remains held. The original candidate is retained before any
// map insertion; abnormal publication cannot lose the factory's known result.
func (m *incomingStreamsMap[T]) publishCreatedStream() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	c := &m.creation
	m.streams[c.id] = incomingStreamEntry[T]{stream: c.candidate}
	// The completed stream now owns its flow, context, halves and sorter.
	// Clear only after successful insertion, before the next batch item.
	c.construction = streamConstruction{}
	c.remaining--
	m.nextStreamToOpen = c.id + 4
	c.id += 4
	c.candidate = *new(T)
	if err := m.stateErrorLocked(); err != nil {
		return err // still map-owned for the original native close
	}
	select {
	case m.newStreamChan <- struct{}{}:
	default:
	}
	return nil
}

func (m *incomingStreamsMap[T]) finishCreation(returned *bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*returned {
		m.uncertain = true
		m.sealLocked(errIncomingMapUncertain)
		return // original batch, candidate and unspent credits stay held
	}
	// Only a never-created suffix can be refunded after a known early close.
	m.rowCount -= m.creation.remaining
	m.rowBytes -= m.creation.remaining * incomingStreamRowCharge
	m.creation = incomingStreamCreation[T]{}
}

func (m *incomingStreamsMap[T]) finishClose(returned *bool, result *error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*returned {
		m.uncertain = true
		m.closeFailure = *result
		if m.closeFailure == nil {
			m.closeFailure = errIncomingMapUncertain
		}
		return // retain the original retiring stream and close reservation
	}
	m.closeReturned = true
}

func (m *incomingStreamsMap[T]) CloseWithError(err error) (result error) {
	m.mutex.Lock()
	if m.closeStarted {
		known := m.closeReturned
		failure := m.closeFailure
		m.mutex.Unlock()
		if !known {
			if failure != nil {
				return failure
			}
			return errIncomingMapUncertain // no duplicate cleanup or false success
		}
		return nil
	}
	m.sealLocked(err) // fence writers and wake Accept before a factory/close wait
	cause := m.closeErr
	m.mutex.Unlock()
	if !m.joinControl() {
		m.mutex.Lock()
		m.uncertain, m.closeFailure = true, errIncomingMapUncertain
		m.mutex.Unlock()
		return errIncomingMapUncertain
	}

	// Never wait for this original operation gate under the metadata mutex.
	// An opaque factory can still block; no new worker or forced deadline exists.
	m.factoryMutex.Lock()
	defer m.factoryMutex.Unlock()
	returned := false
	defer m.finishClose(&returned, &result)
	m.mutex.RLock()
	uncertain := m.uncertain || m.creation.active
	m.mutex.RUnlock()
	if uncertain {
		return errIncomingMapUncertain
	}
	// Close has fenced Get/Accept/Delete. The original factory gate is held,
	// so there are no map writers and no unbounded snapshot is needed.
	for _, entry := range m.streams {
		m.mutex.Lock()
		m.retiring, m.retiringActive = entry.stream, true
		m.mutex.Unlock()
		if shutdownErr := m.retiring.closeForShutdown(cause); shutdownErr != nil {
			return shutdownErr // original retiring stream and rows stay retained
		}
		m.mutex.Lock()
		m.retiring, m.retiringActive = *new(T), false
		m.mutex.Unlock()
	}
	returned = true
	return nil
}
