package quic

import (
	"context"
	"fmt"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

type incomingStream interface {
	closeForShutdown(error) error
}

// When a stream is deleted before it was accepted, we can't delete it from the map immediately.
// We need to wait until the application accepts it, and delete it then.
type incomingStreamEntry[T incomingStream] struct {
	stream       T
	shouldDelete bool
}

type incomingStreamsMap[T incomingStream] struct {
	mutex            sync.RWMutex
	newStreamChan    chan struct{}
	factoryMutex     sync.Mutex // original factory/native-close gate; never waited under mutex
	creation         incomingStreamCreation[T]
	controlMutex     sync.Mutex
	control          streamMapControl
	rowCount         int // published rows plus original reserved batch suffix
	rowBytes         int // accounted metadata credits, not backing/RSS
	uncertain        bool
	closeStarted     bool
	closeReturned    bool
	closeFailure     error
	retiring         T
	retiringActive   bool
	metadataRetiring bool
	metadataRetired  bool

	streamType protocol.StreamType
	streams    map[protocol.StreamID]incomingStreamEntry[T]

	nextStreamToAccept protocol.StreamID // the next stream that will be returned by AcceptStream()
	nextStreamToOpen   protocol.StreamID // the highest stream that the peer opened
	maxStream          protocol.StreamID // the highest stream that the peer is allowed to open
	maxNumStreams      uint64            // maximum number of streams

	newStream        func(protocol.StreamID) T
	queueMaxStreamID func(*wire.MaxStreamsFrame)

	closeErr error
}

func newIncomingStreamsMap[T incomingStream](
	streamType protocol.StreamType,
	newStream func(protocol.StreamID) T,
	maxStreams uint64,
	queueControlFrame func(wire.Frame),
	pers protocol.Perspective,
) *incomingStreamsMap[T] {
	m := &incomingStreamsMap[T]{}
	m.init(streamType, newStream, maxStreams, queueControlFrame, pers)
	return m
}

func (m *incomingStreamsMap[T]) init(
	streamType protocol.StreamType,
	newStream func(protocol.StreamID) T,
	maxStreams uint64,
	queueControlFrame func(wire.Frame),
	pers protocol.Perspective,
) {
	if m == nil || m.streams != nil || m.newStream != nil || m.newStreamChan != nil {
		panic(errStreamMapUncertain)
	}
	var nextStreamToAccept protocol.StreamID
	switch {
	case streamType == protocol.StreamTypeBidi && pers == protocol.PerspectiveServer:
		nextStreamToAccept = protocol.FirstIncomingBidiStreamServer
	case streamType == protocol.StreamTypeBidi && pers == protocol.PerspectiveClient:
		nextStreamToAccept = protocol.FirstIncomingBidiStreamClient
	case streamType == protocol.StreamTypeUni && pers == protocol.PerspectiveServer:
		nextStreamToAccept = protocol.FirstIncomingUniStreamServer
	case streamType == protocol.StreamTypeUni && pers == protocol.PerspectiveClient:
		nextStreamToAccept = protocol.FirstIncomingUniStreamClient
	}
	m.streamType, m.maxNumStreams = streamType, maxStreams
	m.maxStream = protocol.StreamNum(maxStreams).StreamID(streamType, pers.Opposite())
	m.newStream = newStream
	m.nextStreamToOpen, m.nextStreamToAccept = nextStreamToAccept, nextStreamToAccept
	m.queueMaxStreamID = func(f *wire.MaxStreamsFrame) { queueControlFrame(f) }
	m.newStreamChan = make(chan struct{}, 1)
	m.streams = make(map[protocol.StreamID]incomingStreamEntry[T])
}

func (m *incomingStreamsMap[T]) AcceptStream(ctx context.Context) (T, error) {
	if ctx == nil {
		return *new(T), errStreamMapContext
	}
	// drain the newStreamChan, so we don't check the map twice if the stream doesn't exist
	select {
	case <-m.newStreamChan:
	default:
	}

	m.mutex.Lock()
	mutexHeld := true
	defer func() {
		if mutexHeld {
			m.mutex.Unlock()
		}
	}()

	var id protocol.StreamID
	var entry incomingStreamEntry[T]
	var done <-chan struct{}
	doneLoaded := false
	for {
		id = m.nextStreamToAccept
		if err := m.stateErrorLocked(); err != nil {
			return *new(T), err
		}
		var ok bool
		entry, ok = m.streams[id]
		if ok {
			break
		}
		m.mutex.Unlock()
		mutexHeld = false
		if !doneLoaded {
			done = ctx.Done()
			doneLoaded = true
		}
		select {
		case <-done:
			err := ctx.Err()
			if err == nil {
				err = context.Canceled
			}
			return *new(T), err
		case <-m.newStreamChan:
		}
		m.mutex.Lock()
		mutexHeld = true
	}
	m.nextStreamToAccept += 4
	// If this stream was completed before being accepted, we can delete it now.
	if entry.shouldDelete {
		if err := m.deleteStream(id); err != nil {
			return *new(T), err
		}
	}
	m.mutex.Unlock()
	mutexHeld = false
	m.drainMaxStreamsControl()
	return entry.stream, nil
}

func (m *incomingStreamsMap[T]) GetOrOpenStream(id protocol.StreamID) (T, error) {
	m.mutex.RLock()
	if err := m.stateErrorLocked(); err != nil {
		m.mutex.RUnlock()
		return *new(T), err
	}
	if id > m.maxStream {
		maxStream := m.maxStream
		m.mutex.RUnlock()
		return *new(T), &qerr.TransportError{
			ErrorCode:    qerr.StreamLimitError,
			ErrorMessage: fmt.Sprintf("peer tried to open stream %d (current limit: %d)", id, maxStream),
		}
	}
	if s, opened := m.streamIfOpenedLocked(id); opened {
		m.mutex.RUnlock()
		return s, nil
	}
	m.mutex.RUnlock()

	m.factoryMutex.Lock()
	defer m.factoryMutex.Unlock()
	m.mutex.Lock()
	if err := m.stateErrorLocked(); err != nil {
		m.mutex.Unlock()
		return *new(T), err
	}
	if s, opened := m.streamIfOpenedLocked(id); opened {
		m.mutex.Unlock()
		return s, nil
	}
	if err := m.reserveCreationLocked(id); err != nil {
		m.mutex.Unlock()
		return *new(T), err
	}
	m.mutex.Unlock()
	returned := false
	defer m.finishCreation(&returned)
	for m.creation.remaining > 0 {
		m.mutex.RLock()
		err := m.stateErrorLocked()
		m.mutex.RUnlock()
		if err != nil {
			returned = true
			return *new(T), err
		}
		// No map metadata mutex is held across the factory. The original
		// operation gate and batch record retain this invocation and result.
		m.creation.candidate = m.creation.factory(m.creation.id)
		if m.creation.id == m.creation.end {
			m.creation.result = m.creation.candidate
		}
		if err := m.publishCreatedStream(); err != nil {
			returned = true
			return *new(T), err
		}
	}
	m.mutex.RLock()
	err := m.stateErrorLocked()
	m.mutex.RUnlock()
	returned = true
	if err != nil {
		return *new(T), err
	}
	return m.creation.result, nil
}

func (m *incomingStreamsMap[T]) DeleteStream(id protocol.StreamID) error {
	m.mutex.Lock()
	mutexHeld := true
	defer func() {
		if mutexHeld {
			m.mutex.Unlock()
		}
	}()
	if m.closeStarted {
		return nil // retain sealed rows for the original native-close traversal
	}

	err := m.deleteStream(id)
	m.mutex.Unlock()
	mutexHeld = false
	if err != nil {
		return &qerr.TransportError{
			ErrorCode:    qerr.StreamStateError,
			ErrorMessage: err.Error(),
		}
	}
	m.drainMaxStreamsControl()
	return nil
}

func (m *incomingStreamsMap[T]) deleteStream(id protocol.StreamID) error {
	if _, ok := m.streams[id]; !ok {
		return fmt.Errorf("tried to delete unknown incoming stream %d", id)
	}

	// Don't delete this stream yet, if it was not yet accepted.
	// Just save it to streamsToDelete map, to make sure it is deleted as soon as it gets accepted.
	if id >= m.nextStreamToAccept {
		entry, ok := m.streams[id]
		if ok && entry.shouldDelete {
			return fmt.Errorf("tried to delete incoming stream %d multiple times", id)
		}
		entry.shouldDelete = true
		m.streams[id] = entry // can't assign to struct in map, so we need to reassign
		return nil
	}

	delete(m.streams, id)
	m.rowCount--
	m.rowBytes -= incomingStreamRowCharge
	// queue a MAX_STREAM_ID frame, giving the peer the option to open a new stream
	if m.maxNumStreams > uint64(len(m.streams)) {
		remaining := m.maxNumStreams - uint64(len(m.streams))
		base := uint64(m.nextStreamToOpen / 4)
		// Bound the count before multiplication/addition, including the
		// exhausted-ID boundary. Signed overflow cannot wrap a new wire limit.
		if base <= uint64(protocol.MaxStreamCount) && remaining <= uint64(protocol.MaxStreamCount)-base {
			maxStream := m.nextStreamToOpen + 4*protocol.StreamID(remaining-1)
			if maxStream > m.maxStream {
				m.maxStream = maxStream
				m.control.requestLocked(maxStream.StreamNum())
			}
		}
	}
	return nil
}
