package http3

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/internal/utils/ringbuffer"
)

const (
	streamDatagramQueueLen   = 32
	streamDatagramQueueBytes = 1 << 20
)

type streamDatagramEntry struct {
	data          []byte
	retainedBytes int // immutable original backing-capacity charge, not len(data)
}

// stateTrackingStream is an implementation of quic.Stream that delegates
// to an underlying stream
// it takes care of proxying send and receive errors onto an implementation of
// the errorSetter interface (intended to be occupied by a datagrammer)
// it is also responsible for clearing its original stream registration from its
// parent connection, this is done through the streamClearer interface when
// both the send and receive sides are closed
type stateTrackingStream struct {
	*quic.Stream

	sendDatagram func([]byte) error
	hasData      chan struct{}
	queue        ringbuffer.RingBuffer[streamDatagramEntry]
	queueReady   bool // lazy fixed32 metadata; unused streams allocate no ring
	queueBytes   int
	receiveDone  chan struct{} // lazy broadcast for all original waiting receivers

	mx      sync.Mutex
	sendErr error
	recvErr error

	clearer streamClearer
}

var _ datagramStream = &stateTrackingStream{}

type streamClearer interface {
	clearStream(quic.StreamID, *stateTrackingStream)
}

func newStateTrackingStream(s *quic.Stream, clearer streamClearer, sendDatagram func([]byte) error, tasks *clientTaskOwner) *stateTrackingStream {
	t := newUnwatchedStateTrackingStream(s, clearer, sendDatagram)
	t.watchSendContext(tasks)
	return t
}

func newUnwatchedStateTrackingStream(s *quic.Stream, clearer streamClearer, sendDatagram func([]byte) error) *stateTrackingStream {
	return &stateTrackingStream{
		Stream:       s,
		clearer:      clearer,
		sendDatagram: sendDatagram,
		hasData:      make(chan struct{}, 1),
	}
}

func (t *stateTrackingStream) watchSendContext(tasks *clientTaskOwner) {
	ctx := t.Stream.Context()
	context.AfterFunc(ctx, func() {
		returned := false
		defer func() {
			if tasks != nil {
				if !returned {
					_ = recover()
				}
				tasks.finish(returned)
			}
		}()
		t.closeSend(context.Cause(ctx))
		returned = true
	})
}

func (s *stateTrackingStream) closeSend(e error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	// clear the stream the first time both the send
	// and receive are finished
	if s.sendErr == nil {
		if s.recvErr != nil {
			s.clearer.clearStream(s.StreamID(), s)
		}
		s.sendErr = e
	}
}

func (s *stateTrackingStream) closeReceive(e error) {
	if e == nil {
		e = errors.New("http3: receive closed")
	}
	s.mx.Lock()
	defer s.mx.Unlock()

	// clear the stream the first time both the send
	// and receive are finished
	if s.recvErr == nil {
		if s.sendErr != nil {
			s.clearer.clearStream(s.StreamID(), s)
		}
		s.recvErr = e
		if s.receiveDone != nil {
			close(s.receiveDone)
		}
		s.signalHasDatagram()
	}
}

func (s *stateTrackingStream) Close() error {
	s.closeSend(errors.New("write on closed stream"))
	return s.Stream.Close()
}

func (s *stateTrackingStream) CancelWrite(e quic.StreamErrorCode) {
	s.closeSend(&quic.StreamError{StreamID: s.StreamID(), ErrorCode: e})
	s.Stream.CancelWrite(e)
}

func (s *stateTrackingStream) Write(b []byte) (int, error) {
	n, err := s.Stream.Write(b)
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		s.closeSend(err)
	}
	return n, err
}

func (s *stateTrackingStream) CancelRead(e quic.StreamErrorCode) {
	s.closeReceive(&quic.StreamError{StreamID: s.StreamID(), ErrorCode: e})
	s.Stream.CancelRead(e)
}

func (s *stateTrackingStream) Read(b []byte) (int, error) {
	n, err := s.Stream.Read(b)
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		s.closeReceive(err)
	}
	return n, err
}

func (s *stateTrackingStream) SendDatagram(b []byte) error {
	s.mx.Lock()
	sendErr := s.sendErr
	s.mx.Unlock()
	if sendErr != nil {
		return sendErr
	}

	return s.sendDatagram(b)
}

func (s *stateTrackingStream) signalHasDatagram() {
	select {
	case s.hasData <- struct{}{}:
	default:
	}
}

func (s *stateTrackingStream) enqueueDatagram(data []byte) {
	// Private callers only expose this view. The native wire path supplies the
	// full lower-QUIC allocation charge, including its stripped stream prefix.
	s.enqueueDatagramWithSize(data, cap(data))
}

func (s *stateTrackingStream) enqueueDatagramWithSize(data []byte, retainedBytes int) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.recvErr != nil {
		return
	}
	if s.queue.Len() >= streamDatagramQueueLen || retainedBytes < cap(data) ||
		retainedBytes > streamDatagramQueueBytes-s.queueBytes {
		return
	}
	if !s.queueReady {
		s.queue.Init(streamDatagramQueueLen)
		s.queueReady = true
	}
	// Admission and the only Init are under mx: PushBack never grows this ring.
	s.queue.PushBack(streamDatagramEntry{data: data, retainedBytes: retainedBytes})
	s.queueBytes += retainedBytes
	s.signalHasDatagram()
}

func (s *stateTrackingStream) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("http3: nil DATAGRAM context")
	}
	done := ctx.Done() // capture the caller's channel outside the queue mutex
start:
	s.mx.Lock()
	select {
	case <-done:
		s.mx.Unlock()
		return nil, context.Cause(ctx)
	default:
	}
	if !s.queue.Empty() {
		entry := s.queue.PopFront() // clears the original metadata cell
		s.queueBytes -= entry.retainedBytes
		if !s.queue.Empty() {
			s.signalHasDatagram() // hand queued data to another original waiter
		}
		s.mx.Unlock()
		// Transfer the original payload; external borrowers have their own scope.
		return entry.data, nil
	}
	if receiveErr := s.recvErr; receiveErr != nil {
		s.mx.Unlock()
		return nil, receiveErr
	}
	if s.receiveDone == nil {
		s.receiveDone = make(chan struct{})
	}
	receiveDone := s.receiveDone
	s.mx.Unlock()

	select {
	case <-done:
		return nil, context.Cause(ctx)
	case <-s.hasData:
	case <-receiveDone:
	}
	goto start
}

func (s *stateTrackingStream) QUICStream() *quic.Stream {
	return s.Stream
}
