package quic

import (
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
)

// Both directions use the same explicit per-map accounting policy. These are
// logical metadata credits, not physical backing, aggregate generations or RSS.
const (
	maxOutgoingStreamRows     = maxIncomingStreamRows
	outgoingStreamRowCharge   = incomingStreamRowCharge
	maxOutgoingStreamRowBytes = maxIncomingStreamRowBytes
)

var (
	errOutgoingMapCapacity  = errors.New("quic: local outgoing stream metadata capacity exceeded")
	errOutgoingMapUncertain = errors.New("quic: original outgoing stream map operation is uncertain")
)

type outgoingStreamCreation[T outgoingStream] struct {
	active       bool
	inserted     bool
	id           protocol.StreamID
	factory      func(protocol.StreamID) T
	candidate    T
	waiter       chan struct{}
	construction streamConstruction
}

func (m *outgoingStreamsMap[T]) stateErrorLocked() error {
	if m.uncertain {
		return errOutgoingMapUncertain
	}
	if m.closeStarted {
		return m.closeErr
	}
	return nil
}

func (m *outgoingStreamsMap[T]) rowAvailableLocked() bool {
	return !m.creation.active && m.rowCount < maxOutgoingStreamRows && m.rowBytes <= maxOutgoingStreamRowBytes-outgoingStreamRowCharge
}

func (m *outgoingStreamsMap[T]) sealLocked(cause error) {
	if m.closeStarted {
		return
	}
	if cause == nil {
		cause = net.ErrClosed
	}
	m.closeErr, m.closeStarted = cause, true
	for _, waiter := range m.openQueue {
		if waiter != nil {
			close(waiter)
		}
	}
	m.openQueue = nil
}

// No metadata mutex is held on entry or across factory/gate waits. The exact
// head waiter owns admission; a non-waiting caller cannot overtake that queue.
// The bool reports actual admission pressure, never inferred from a close cause.
func (m *outgoingStreamsMap[T]) tryOpenStream(waiter chan struct{}) (T, bool, error) {
	defer m.drainBlockedControl() // after factory gate and metadata release
	m.factoryMutex.Lock()
	defer m.factoryMutex.Unlock()
	blocked, err := m.beginCreation(waiter)
	if err != nil {
		return *new(T), blocked, err
	}
	returned := false
	defer m.finishCreation(&returned)
	m.creation.candidate = m.creation.factory(m.creation.id)
	err = m.publishCreatedStream()
	returned = true
	if err != nil {
		return *new(T), false, err
	}
	return m.creation.candidate, false, nil
}

func (m *outgoingStreamsMap[T]) beginCreation(waiter chan struct{}) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.stateErrorLocked(); err != nil {
		return false, err
	}
	if (waiter == nil && len(m.openQueue) != 0) || (waiter != nil && (len(m.openQueue) == 0 || m.openQueue[0] != waiter)) {
		return true, &StreamLimitReachedError{}
	}
	if m.nextStream > m.maxStream || m.nextStream > protocol.MaxStreamID {
		m.maybeSendBlockedFrame()
		return true, &StreamLimitReachedError{}
	}
	if m.creation.active || m.rowCount != len(m.streams) || m.rowCount < 0 || m.rowCount > maxOutgoingStreamRows || m.rowBytes != m.rowCount*outgoingStreamRowCharge {
		m.uncertain = true
		m.sealLocked(errOutgoingMapUncertain)
		return false, errOutgoingMapUncertain
	}
	if !m.rowAvailableLocked() {
		return true, errOutgoingMapCapacity
	}
	m.rowCount++
	m.rowBytes += outgoingStreamRowCharge
	m.creation = outgoingStreamCreation[T]{active: true, id: m.nextStream, factory: m.newStream, waiter: waiter}
	if waiter != nil {
		m.openQueue[0] = nil
		m.openQueue = m.openQueue[1:]
	}
	return false, nil
}

func (m *outgoingStreamsMap[T]) publishCreatedStream() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.streams[m.creation.id] = m.creation.candidate
	m.creation.inserted = true
	m.nextStream = m.creation.id + 4
	// A candidate returning after seal remains in the original map for native
	// close; its caller receives an error, without a second cleanup attempt.
	return m.stateErrorLocked()
}

func (m *outgoingStreamsMap[T]) finishCreation(returned *bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*returned || !m.creation.inserted {
		m.uncertain = true
		m.sealLocked(errOutgoingMapUncertain)
		if *returned {
			panic(errOutgoingMapUncertain)
		}
		return // original factory/ID/candidate/waiter and row credit stay held
	}
	m.creation = outgoingStreamCreation[T]{}
	m.maybeUnblockOpenSync()
}

func (m *outgoingStreamsMap[T]) finishClose(returned *bool, result *error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*returned {
		m.uncertain = true
		m.closeFailure = *result
		if m.closeFailure == nil {
			m.closeFailure = errOutgoingMapUncertain
		}
		return
	}
	m.closeReturned = true
}

func (m *outgoingStreamsMap[T]) CloseWithError(err error) (result error) {
	m.mutex.Lock()
	if m.closeStarted {
		known := m.closeReturned
		failure := m.closeFailure
		m.mutex.Unlock()
		if !known {
			if failure != nil {
				return failure
			}
			return errOutgoingMapUncertain
		}
		return nil
	}
	m.sealLocked(err) // wake queued operations before waiting for the factory
	cause := m.closeErr
	m.mutex.Unlock()
	if !m.joinControl() {
		m.mutex.Lock()
		m.uncertain, m.closeFailure = true, errOutgoingMapUncertain
		m.mutex.Unlock()
		return errOutgoingMapUncertain
	}
	m.factoryMutex.Lock()
	defer m.factoryMutex.Unlock()
	returned := false
	defer m.finishClose(&returned, &result)
	m.mutex.RLock()
	uncertain := m.uncertain || m.creation.active || m.updating.active
	m.mutex.RUnlock()
	if uncertain {
		return errOutgoingMapUncertain
	}
	// Factory, Delete and update admission are fenced. This original map has
	// no writers; no extra snapshot or replacement cleanup worker is needed.
	for _, str := range m.streams {
		m.mutex.Lock()
		m.retiring, m.retiringActive = str, true
		m.mutex.Unlock()
		if shutdownErr := m.retiring.closeForShutdown(cause); shutdownErr != nil {
			return shutdownErr // retain exact original, rows and close reservation
		}
		m.mutex.Lock()
		m.retiring, m.retiringActive = *new(T), false
		m.mutex.Unlock()
	}
	returned = true // local cleanup receipts do not join all holders; rows remain held
	return nil
}
