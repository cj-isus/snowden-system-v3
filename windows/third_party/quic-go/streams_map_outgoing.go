package quic

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

type outgoingStream interface {
	updateSendWindow(protocol.ByteCount)
	enableResetStreamAt()
	closeForShutdown(error) error
}

type outgoingStreamsMap[T outgoingStream] struct {
	mutex            sync.RWMutex
	factoryMutex     sync.Mutex // original factory/native-close/update gate, outside mutex waits
	creation         outgoingStreamCreation[T]
	updating         outgoingStreamUpdate[T]
	controlMutex     sync.Mutex
	control          streamMapControl
	rowCount         int
	rowBytes         int
	uncertain        bool
	closeStarted     bool
	closeReturned    bool
	closeFailure     error
	retiring         T
	retiringActive   bool
	metadataRetiring bool
	metadataRetired  bool

	streamType protocol.StreamType
	streams    map[protocol.StreamID]T

	openQueue []chan struct{}

	nextStream  protocol.StreamID // stream ID of the stream returned by OpenStream(Sync)
	maxStream   protocol.StreamID // the maximum stream ID we're allowed to open
	blockedSent bool              // was a STREAMS_BLOCKED sent for the current maxStream

	newStream            func(protocol.StreamID) T
	queueStreamIDBlocked func(*wire.StreamsBlockedFrame)

	closeErr error
}

func newOutgoingStreamsMap[T outgoingStream](
	streamType protocol.StreamType,
	newStream func(protocol.StreamID) T,
	queueControlFrame func(wire.Frame),
	pers protocol.Perspective,
) *outgoingStreamsMap[T] {
	m := &outgoingStreamsMap[T]{}
	m.init(streamType, newStream, queueControlFrame, pers)
	return m
}

func (m *outgoingStreamsMap[T]) init(
	streamType protocol.StreamType,
	newStream func(protocol.StreamID) T,
	queueControlFrame func(wire.Frame),
	pers protocol.Perspective,
) {
	if m == nil || m.streams != nil || m.newStream != nil {
		panic(errStreamMapUncertain)
	}
	var nextStream protocol.StreamID
	switch {
	case streamType == protocol.StreamTypeBidi && pers == protocol.PerspectiveServer:
		nextStream = protocol.FirstOutgoingBidiStreamServer
	case streamType == protocol.StreamTypeBidi && pers == protocol.PerspectiveClient:
		nextStream = protocol.FirstOutgoingBidiStreamClient
	case streamType == protocol.StreamTypeUni && pers == protocol.PerspectiveServer:
		nextStream = protocol.FirstOutgoingUniStreamServer
	case streamType == protocol.StreamTypeUni && pers == protocol.PerspectiveClient:
		nextStream = protocol.FirstOutgoingUniStreamClient
	}
	m.streamType, m.maxStream, m.nextStream = streamType, protocol.InvalidStreamNum, nextStream
	m.newStream = newStream
	m.queueStreamIDBlocked = func(f *wire.StreamsBlockedFrame) { queueControlFrame(f) }
	m.streams = make(map[protocol.StreamID]T)
}

func (m *outgoingStreamsMap[T]) OpenStream() (T, error) {
	str, _, err := m.tryOpenStream(nil)
	return str, err
}

func (m *outgoingStreamsMap[T]) OpenStreamSync(ctx context.Context) (T, error) {
	if ctx == nil {
		return *new(T), errStreamMapContext
	}
	// Context methods may be supplied by the caller. They cannot execute while
	// holding a map mutex needed by the original connection's close path.
	ctxErr := ctx.Err()
	if ctxErr != nil {
		m.mutex.RLock()
		closed := m.stateErrorLocked()
		m.mutex.RUnlock()
		if closed != nil {
			return *new(T), closed
		}
		return *new(T), ctxErr
	}
	str, blocked, err := m.tryOpenStream(nil)
	if !blocked {
		return str, err
	}
	m.mutex.Lock()
	if err := m.stateErrorLocked(); err != nil {
		m.mutex.Unlock()
		return *new(T), err
	}
	if len(m.openQueue) >= maxStreamMapCalls {
		m.mutex.Unlock()
		return *new(T), errStreamMapCapacity
	}

	waitChan := make(chan struct{}, 1)
	m.openQueue = append(m.openQueue, waitChan)
	m.maybeUnblockOpenSync() // credit may have arrived before enqueue
	m.mutex.Unlock()
	done := ctx.Done()

	for {
		select {
		case <-done:
			err := ctx.Err()
			if err == nil {
				err = context.Canceled
			}
			m.mutex.Lock()
			m.openQueue = slices.DeleteFunc(m.openQueue, func(c chan struct{}) bool {
				return c == waitChan
			})
			// If we just received a MAX_STREAMS frame, this might have been the next stream
			// that could be opened. Make sure we unblock the next OpenStreamSync call.
			m.maybeUnblockOpenSync()
			m.mutex.Unlock()
			return *new(T), err
		case <-waitChan:
		}

		str, blocked, err := m.tryOpenStream(waitChan)
		if !blocked {
			return str, err
		}
	}
}

// maybeSendBlockedFrame queues a STREAMS_BLOCKED frame for the current stream offset,
// if we haven't sent one for this offset yet
func (m *outgoingStreamsMap[T]) maybeSendBlockedFrame() {
	if m.blockedSent {
		return
	}

	var streamLimit protocol.StreamNum
	if m.maxStream != protocol.InvalidStreamID {
		streamLimit = m.maxStream.StreamNum()
	}
	m.control.requestLocked(streamLimit)
	m.blockedSent = true
}

func (m *outgoingStreamsMap[T]) GetStream(id protocol.StreamID) (T, error) {
	m.mutex.RLock()
	if err := m.stateErrorLocked(); err != nil {
		m.mutex.RUnlock()
		return *new(T), err
	}
	if id >= m.nextStream {
		m.mutex.RUnlock()
		return *new(T), &qerr.TransportError{
			ErrorCode:    qerr.StreamStateError,
			ErrorMessage: fmt.Sprintf("peer attempted to open stream %d", id),
		}
	}
	s := m.streams[id]
	m.mutex.RUnlock()
	return s, nil
}

func (m *outgoingStreamsMap[T]) DeleteStream(id protocol.StreamID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.closeStarted {
		return nil // retain sealed rows for the original native close
	}

	if _, ok := m.streams[id]; !ok {
		return &qerr.TransportError{
			ErrorCode:    qerr.StreamStateError,
			ErrorMessage: fmt.Sprintf("tried to delete unknown outgoing stream %d", id),
		}
	}
	delete(m.streams, id)
	m.rowCount--
	m.rowBytes -= outgoingStreamRowCharge
	m.maybeUnblockOpenSync()
	return nil
}

func (m *outgoingStreamsMap[T]) SetMaxStream(id protocol.StreamID) {
	defer m.drainBlockedControl() // runs after the metadata unlock
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stateErrorLocked() != nil {
		return
	}

	if id <= m.maxStream {
		return
	}
	m.maxStream = id
	m.blockedSent = false
	if m.maxStream < m.nextStream-4+4*protocol.StreamID(len(m.openQueue)) {
		m.maybeSendBlockedFrame()
	}
	m.maybeUnblockOpenSync()
}

// UpdateSendWindow is called when the peer's transport parameters are received.
// Only in the case of a 0-RTT handshake will we have open streams at this point.
// We might need to update the send window, in case the server increased it.
func (m *outgoingStreamsMap[T]) UpdateSendWindow(limit protocol.ByteCount) {
	m.updateStreams(outgoingWindowUpdate, limit)
}

func (m *outgoingStreamsMap[T]) EnableResetStreamAt() {
	m.updateStreams(outgoingResetUpdate, 0)
}

// unblockOpenSync unblocks the next OpenStreamSync go-routine to open a new stream
func (m *outgoingStreamsMap[T]) maybeUnblockOpenSync() {
	if len(m.openQueue) == 0 || m.stateErrorLocked() != nil || !m.rowAvailableLocked() {
		return
	}
	if m.nextStream > m.maxStream || m.nextStream > protocol.MaxStreamID {
		return
	}
	// unblockOpenSync is called both from OpenStreamSync and from SetMaxStream.
	// It's sufficient to only unblock OpenStreamSync once.
	select {
	case m.openQueue[0] <- struct{}{}:
	default:
	}
}
