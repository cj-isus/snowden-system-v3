package quic

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

// A ReceiveStream is a unidirectional Receive Stream.
type ReceiveStream struct {
	mutex sync.Mutex

	streamID protocol.StreamID

	sender streamSender

	frameQueue  *frameSorter
	finalOffset protocol.ByteCount

	currentFrame          []byte
	currentFrameDone      func()
	readPosInFrame        int
	currentFrameIsLast    bool // is the currentFrame the last frame on this stream
	currentFrameReleasing bool
	receiveUncertain      bool
	frameIntake           streamFrameIntake
	callCounts            [2]int // protected by frameIntake.budget.calls.access

	queuedStopSending   bool
	queuedMaxStreamData bool

	// Set once we read the io.EOF or the cancellation error.
	// Note that for local cancellations, this doesn't necessarily mean that we know the final offset yet.
	errorRead           bool
	completed           bool // set once we've called streamSender.onStreamCompleted
	cancelledRemotely   bool
	cancelledLocally    bool
	cancelErr           *StreamError
	closeForShutdownErr error
	shutdown            streamShutdownReceipt

	readPos      protocol.ByteCount
	reliableSize protocol.ByteCount

	readChan chan struct{}
	readOnce chan struct{} // cap: 1, to protect against concurrent use of Read
	deadline monotime.Time

	flowController flowcontrol.StreamFlowController
}

var (
	_ streamControlFrameGetter  = &ReceiveStream{}
	_ receiveStreamFrameHandler = &ReceiveStream{}
)

func newReceiveStream(
	streamID protocol.StreamID,
	sender streamSender,
	flowController flowcontrol.StreamFlowController,
	receiveBudget *streamReceiveBudget,
	construction *streamConstruction,
) *ReceiveStream {
	construction.require(streamID)
	if construction.receive != nil {
		panic(errStreamMapUncertain)
	}
	construction.stage = constructionReceive
	s := &ReceiveStream{
		streamID:       streamID,
		sender:         sender,
		flowController: flowController,
		frameIntake:    streamFrameIntake{budget: receiveBudget},
		finalOffset:    protocol.MaxByteCount,
	}
	construction.receive = s // before channels and sorter initialization
	s.readChan = make(chan struct{}, 1)
	s.readOnce = make(chan struct{}, 1)
	construction.stage = constructionSorter
	s.frameQueue = &frameSorter{}
	s.frameQueue.initReceiveFrameSorter(receiveBudget)
	return s
}

// StreamID returns the stream ID.
func (s *ReceiveStream) StreamID() protocol.StreamID {
	return s.streamID
}

// Read reads data from the stream.
// Read can be made to time out using [ReceiveStream.SetReadDeadline].
// If the stream was canceled, the error is a [StreamError].
func (s *ReceiveStream) Read(p []byte) (int, error) {
	call, err := s.beginReceiveCall(receiveReadCall)
	if err != nil {
		return 0, err
	}
	returned := false
	defer call.finish(&returned)
	// Concurrent use of Read is not permitted (and doesn't make any sense),
	// but sometimes people do it anyway.
	// Make sure that we only execute one call at any given time to avoid hard to debug failures.
	select {
	case s.readOnce <- struct{}{}:
	case <-call.owner.closing:
		returned = true
		return 0, call.owner.stoppedError()
	}
	defer func() { <-s.readOnce }()

	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	queuedStreamWindowUpdate, queuedConnWindowUpdate, n, err := s.readImpl(p, &mutexHeld)
	completed := s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false

	if completed {
		s.sender.onStreamCompleted(s.streamID)
	}
	if queuedStreamWindowUpdate {
		s.sender.onHasStreamControlFrame(s.streamID, s)
	}
	if queuedConnWindowUpdate {
		s.sender.onHasConnectionData()
	}
	returned = true
	return n, err
}

func (s *ReceiveStream) isNewlyCompleted() bool {
	if s.completed || s.retirementError() != nil {
		return false
	}
	// We need to know the final offset (either via FIN or RESET_STREAM) for flow control accounting.
	if s.finalOffset == protocol.MaxByteCount {
		return false
	}
	// We're done with the stream if it was cancelled locally...
	if s.cancelledLocally {
		s.completed = true
		return true
	}
	// ... or if the error (either io.EOF or the reset error) was read
	if s.errorRead {
		s.completed = true
		return true
	}
	return false
}

func (s *ReceiveStream) readImpl(p []byte, mutexHeld *bool) (hasStreamWindowUpdate bool, hasConnWindowUpdate bool, _ int, _ error) {
	if err := s.frameIntake.budget.calls.stoppedError(); err != nil {
		return false, false, 0, err
	}
	if err := s.retirementError(); err != nil {
		return false, false, 0, err
	}
	if s.currentFrameIsLast && s.currentFrame == nil {
		if err := s.discardBufferedFrames(); err != nil {
			return false, false, 0, err
		}
		s.errorRead = true
		return false, false, 0, io.EOF
	}
	if s.cancelledLocally || s.isRemoteCancellationEffective() {
		if err := s.discardBufferedFrames(); err != nil {
			return false, false, 0, err
		}
		s.errorRead = true
		return false, false, 0, s.cancelErr
	}
	if s.closeForShutdownErr != nil {
		return false, false, 0, s.closeForShutdownErr
	}

	var bytesRead int
	var deadlineTimer *time.Timer
	for bytesRead < len(p) {
		if s.currentFrame == nil || s.readPosInFrame >= len(s.currentFrame) {
			if err := s.dequeueNextFrame(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
		}
		if s.currentFrame == nil && bytesRead > 0 {
			return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, s.closeForShutdownErr
		}

		for {
			// Stop waiting on errors
			if err := s.frameIntake.budget.calls.stoppedError(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
			if err := s.retirementError(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
			if s.closeForShutdownErr != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, s.closeForShutdownErr
			}
			if s.cancelledLocally || s.isRemoteCancellationEffective() {
				s.errorRead = true
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, s.cancelErr
			}

			deadline := s.deadline
			if !deadline.IsZero() && !monotime.Now().Before(deadline) {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, errDeadline
			}

			if s.currentFrame != nil || s.currentFrameIsLast {
				break
			}

			s.mutex.Unlock()
			*mutexHeld = false
			if deadline.IsZero() {
				select {
				case <-s.readChan:
				case <-s.frameIntake.budget.calls.closing:
				}
			} else {
				if deadlineTimer == nil {
					deadlineTimer = time.NewTimer(monotime.Until(deadline))
					defer deadlineTimer.Stop()
				} else {
					deadlineTimer.Reset(monotime.Until(deadline))
				}
				select {
				case <-s.readChan:
				case <-s.frameIntake.budget.calls.closing:
				case <-deadlineTimer.C:
				}
			}
			s.mutex.Lock()
			*mutexHeld = true
			if err := s.dequeueNextFrame(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
		}

		if bytesRead > len(p) {
			return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, fmt.Errorf("BUG: bytesRead (%d) > len(p) (%d) in stream.Read", bytesRead, len(p))
		}
		if s.readPosInFrame > len(s.currentFrame) {
			return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, fmt.Errorf("BUG: readPosInFrame (%d) > frame.DataLen (%d) in stream.Read", s.readPosInFrame, len(s.currentFrame))
		}
		available := s.currentFrame[s.readPosInFrame:]
		if s.cancelledRemotely {
			available = available[:min(protocol.ByteCount(len(available)), s.reliableSize-s.readPos)]
		}
		m := copy(p[bytesRead:], available)

		// when a RESET_STREAM was received, the flow controller was already
		// informed about the final offset for this stream
		if !s.isRemoteCancellationEffective() {
			hasStream, hasConn := s.flowController.AddBytesRead(protocol.ByteCount(m))
			if hasStream {
				s.queuedMaxStreamData = true
				hasStreamWindowUpdate = true
			}
			if hasConn {
				hasConnWindowUpdate = true
			}
		}

		s.readPosInFrame += m
		s.readPos += protocol.ByteCount(m)
		bytesRead += m

		if s.isRemoteCancellationEffective() {
			s.flowController.Abandon()
			s.currentFrameIsLast = false // a reset error must not become EOF after discard
			if err := s.discardBufferedFrames(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
			s.errorRead = true
			return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, s.cancelErr
		}

		if s.readPosInFrame >= len(s.currentFrame) && s.currentFrameIsLast {
			if err := s.discardBufferedFrames(); err != nil {
				return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, err
			}
			s.errorRead = true
			return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, io.EOF
		}
	}
	if s.isRemoteCancellationEffective() {
		s.errorRead = true
		return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, s.cancelErr
	}
	return hasStreamWindowUpdate, hasConnWindowUpdate, bytesRead, nil
}

// isRemoteCancellationEffective returns whether the stream was cancelled remotely
// and all reliable data has been read.
func (s *ReceiveStream) isRemoteCancellationEffective() bool {
	return s.cancelledRemotely && s.readPos >= s.reliableSize
}

// Peek fills b with stream data, without consuming the stream data.
// It blocks until len(b) bytes are available, or an error occurs.
// It respects the stream deadline set by SetReadDeadline.
// If the stream ends before len(b) bytes are available,
// it returns the number of bytes peeked along with io.EOF.
func (s *ReceiveStream) Peek(b []byte) (int, error) {
	call, err := s.beginReceiveCall(receiveReadCall)
	if err != nil {
		return 0, err
	}
	returned := false
	defer call.finish(&returned)
	if len(b) == 0 {
		returned = true
		return 0, nil
	}

	// prevent concurrent use with Read
	select {
	case s.readOnce <- struct{}{}:
	case <-call.owner.closing:
		returned = true
		return 0, call.owner.stoppedError()
	}
	defer func() { <-s.readOnce }()

	n, err := s.peekImpl(b)
	returned = true
	return n, err
}

func (s *ReceiveStream) peekImpl(b []byte) (int, error) {
	s.mutex.Lock()
	mutexHeld, returned := true, false
	defer s.finishReceiveCall(&mutexHeld, &returned)
	n, err := s.peekLocked(b, &mutexHeld)
	returned = true
	return n, err
}

func (s *ReceiveStream) peekLocked(b []byte, mutexHeld *bool) (int, error) {
	var deadlineTimer *time.Timer

	for {
		if err := s.frameIntake.budget.calls.stoppedError(); err != nil {
			return 0, err
		}
		if err := s.retirementError(); err != nil {
			return 0, err
		}
		if s.currentFrameIsLast && s.currentFrame == nil {
			return 0, io.EOF
		}
		if s.cancelledLocally || s.isRemoteCancellationEffective() {
			return 0, s.cancelErr
		}
		if s.closeForShutdownErr != nil {
			return 0, s.closeForShutdownErr
		}

		deadline := s.deadline
		if !deadline.IsZero() && !monotime.Now().Before(deadline) {
			return 0, errDeadline
		}

		if s.currentFrame == nil || s.readPosInFrame >= len(s.currentFrame) {
			if err := s.dequeueNextFrame(); err != nil {
				return 0, err
			}
		}

		if s.currentFrame != nil && s.readPosInFrame < len(s.currentFrame) {
			availableInCurrentFrame := len(s.currentFrame) - s.readPosInFrame
			offset := s.readPos + protocol.ByteCount(availableInCurrentFrame)
			// A reset's reliable prefix also limits a peek satisfied entirely
			// from the current frame. Peek does not consume/retire that prefix.
			if s.cancelledRemotely && protocol.ByteCount(len(b)) > s.reliableSize-s.readPos {
				total := int(s.reliableSize - s.readPos) // bounded by len(b) above
				fromCurrent := min(availableInCurrentFrame, total)
				if total <= availableInCurrentFrame || s.frameQueue.Peek(offset, b[fromCurrent:total]) == nil {
					copy(b[:fromCurrent], s.currentFrame[s.readPosInFrame:])
					return total, s.cancelErr
				}
			}

			if availableInCurrentFrame >= len(b) {
				copy(b, s.currentFrame[s.readPosInFrame:])
				return len(b), nil
			}

			// First peek, then copy.
			// This avoids copying data if there's not enough data in the queue.
			if err := s.frameQueue.Peek(offset, b[availableInCurrentFrame:]); err == nil {
				copy(b[:availableInCurrentFrame], s.currentFrame[s.readPosInFrame:])
				return len(b), nil
			}

			if s.currentFrameIsLast {
				copy(b[:availableInCurrentFrame], s.currentFrame[s.readPosInFrame:])
				return availableInCurrentFrame, io.EOF
			}

			// If the request extends beyond the stream's final offset,
			// return the data available with EOF (once it's all received).
			if s.readPos+protocol.ByteCount(len(b)) > s.finalOffset {
				total := int(s.finalOffset - s.readPos)
				needed := total - availableInCurrentFrame
				// only return once all available data is contiguous
				if needed <= 0 || s.frameQueue.Peek(offset, b[availableInCurrentFrame:total]) == nil {
					copy(b[:availableInCurrentFrame], s.currentFrame[s.readPosInFrame:])
					return total, io.EOF
				}
			}
		}

		if s.currentFrameIsLast || s.readPos >= s.finalOffset {
			return 0, io.EOF
		}

		s.mutex.Unlock()
		*mutexHeld = false
		if deadline.IsZero() {
			select {
			case <-s.readChan:
			case <-s.frameIntake.budget.calls.closing:
			}
		} else {
			if deadlineTimer == nil {
				deadlineTimer = time.NewTimer(monotime.Until(deadline))
				defer deadlineTimer.Stop()
			} else {
				deadlineTimer.Reset(monotime.Until(deadline))
			}
			select {
			case <-s.readChan:
			case <-s.frameIntake.budget.calls.closing:
			case <-deadlineTimer.C:
			}
		}
		s.mutex.Lock()
		*mutexHeld = true
		if s.currentFrame == nil || s.readPosInFrame >= len(s.currentFrame) {
			if err := s.dequeueNextFrame(); err != nil {
				return 0, err
			}
		}
	}
}

func (s *ReceiveStream) dequeueNextFrame() error {
	var offset protocol.ByteCount
	// We're done with the last frame. Release the buffer.
	if err := s.releaseCurrentFrame(); err != nil {
		return err
	}
	if s.frameQueue.closed {
		return nil // an empty retired queue must not manufacture a new FIN/EOF
	}
	offset, s.currentFrame, s.currentFrameDone = s.frameQueue.Pop()
	s.currentFrameIsLast = offset+protocol.ByteCount(len(s.currentFrame)) >= s.finalOffset && !s.cancelledRemotely
	s.readPosInFrame = 0
	return nil
}

// CancelRead aborts receiving on this stream.
// It instructs the peer to stop transmitting stream data.
// Read will unblock immediately, and future Read calls will fail.
// When called multiple times or after reading the io.EOF it is a no-op.
func (s *ReceiveStream) CancelRead(errorCode StreamErrorCode) {
	call, err := s.beginReceiveCall(receiveControlCall)
	if err != nil {
		return // known capacity requests Conn close; sealed close already wakes reads
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	queuedNewControlFrame := s.cancelReadImpl(errorCode)
	completed := s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false

	if queuedNewControlFrame {
		s.sender.onHasStreamControlFrame(s.streamID, s)
	}
	if completed {
		s.flowController.Abandon()
		s.sender.onStreamCompleted(s.streamID)
	}
	returned = true
}

func (s *ReceiveStream) cancelReadImpl(errorCode qerr.StreamErrorCode) (queuedNewControlFrame bool) {
	if s.cancelledLocally { // duplicate call to CancelRead
		return false
	}
	if s.closeForShutdownErr != nil {
		return false
	}
	s.cancelledLocally = true
	s.signalRead()
	if !s.errorRead {
		s.currentFrameIsLast = false
	}
	if err := s.discardBufferedFrames(); err != nil {
		s.receiveUncertain = true
		return false
	}
	if s.errorRead || s.cancelledRemotely {
		return false
	}
	s.queuedStopSending = true
	s.cancelErr = &StreamError{StreamID: s.streamID, ErrorCode: errorCode, Remote: false}
	s.signalRead()
	return true
}

func (s *ReceiveStream) handleStreamFrame(original *receivedStreamData, now monotime.Time) error {
	call, err := s.beginReceiveCall(receiveNativeCall)
	if err != nil {
		original.release() // this unentered native call never adopted the original
		return err
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	if err := s.frameIntake.begin(original); err != nil {
		returned = true
		return err
	}
	err = s.retirementError()
	if err == nil {
		err = s.handleStreamFrameImpl(original.frameView(), now)
	}
	// Consume normal refusals under the same receive mutex before publication.
	// The outer receive guard retains this original slot on abnormal return.
	s.frameIntake.release()
	completed := s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false

	if completed {
		s.flowController.Abandon()
		s.sender.onStreamCompleted(s.streamID)
	}
	returned = true
	return err
}

func (s *ReceiveStream) handleStreamFrameImpl(frame *wire.StreamFrame, now monotime.Time) error {
	maxOffset := frame.Offset + frame.DataLen()
	if err := s.flowController.UpdateHighestReceived(maxOffset, frame.Fin, now); err != nil {
		return err
	}
	if frame.Fin {
		s.finalOffset = maxOffset
	}
	if s.cancelledLocally || s.closeForShutdownErr != nil || s.errorRead || s.isRemoteCancellationEffective() {
		return nil
	}
	original := s.frameIntake.transfer()
	if err := s.frameQueue.Push(frame.Data, frame.Offset, original.release); err != nil {
		return err
	}
	s.signalRead()
	return nil
}

func (s *ReceiveStream) handleResetStreamFrame(frame *wire.ResetStreamFrame, now monotime.Time) error {
	call, err := s.beginReceiveCall(receiveNativeCall)
	if err != nil {
		return err
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	err = s.handleResetStreamFrameImpl(frame, now)
	completed := s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false

	if completed {
		s.sender.onStreamCompleted(s.streamID)
	}
	returned = true
	return err
}

func (s *ReceiveStream) handleResetStreamFrameImpl(frame *wire.ResetStreamFrame, now monotime.Time) error {
	if err := s.retirementError(); err != nil {
		return err
	}
	if s.closeForShutdownErr != nil {
		return nil
	}
	if err := s.flowController.UpdateHighestReceived(frame.FinalSize, true, now); err != nil {
		return err
	}
	s.finalOffset = frame.FinalSize

	// senders are allowed to reduce the reliable size, but frames might have been reordered
	if (!s.cancelledRemotely && s.reliableSize == 0) || frame.ReliableSize < s.reliableSize {
		s.reliableSize = frame.ReliableSize
	}
	if s.readPos >= s.reliableSize {
		// calling Abandon multiple times is a no-op
		s.flowController.Abandon()
		if !s.errorRead {
			s.currentFrameIsLast = false
		}
		if err := s.discardBufferedFrames(); err != nil {
			return err
		}
	}
	s.signalRead() // a reordered duplicate can reduce the pending reliable prefix
	// ignore duplicate RESET_STREAM frames for this stream (after checking their final offset)
	if s.cancelledRemotely {
		return nil
	}

	// don't save the error if the RESET_STREAM frames was received after CancelRead was called
	if s.cancelledLocally {
		return nil
	}
	s.cancelledRemotely = true
	s.cancelErr = &StreamError{StreamID: s.streamID, ErrorCode: frame.ErrorCode, Remote: true}
	s.signalRead()
	return nil
}

func (s *ReceiveStream) getControlFrame(now monotime.Time) (_ ackhandler.Frame, ok, hasMore bool) {
	call, err := s.beginReceiveCall(receiveNativeCall)
	if err != nil {
		return ackhandler.Frame{}, false, false
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	frame, ok, hasMore := s.getControlFrameLocked(now)
	returned = true
	return frame, ok, hasMore
}

func (s *ReceiveStream) getControlFrameLocked(now monotime.Time) (_ ackhandler.Frame, ok, hasMore bool) {
	if s.retirementError() != nil {
		return ackhandler.Frame{}, false, false
	}

	if !s.queuedStopSending && !s.queuedMaxStreamData {
		return ackhandler.Frame{}, false, false
	}
	if s.queuedStopSending {
		s.queuedStopSending = false
		return ackhandler.Frame{
			Frame: &wire.StopSendingFrame{StreamID: s.streamID, ErrorCode: s.cancelErr.ErrorCode},
		}, true, s.queuedMaxStreamData
	}

	s.queuedMaxStreamData = false
	return ackhandler.Frame{
		Frame: &wire.MaxStreamDataFrame{
			StreamID:          s.streamID,
			MaximumStreamData: s.flowController.GetWindowUpdate(now),
		},
	}, true, false
}

// SetReadDeadline sets the deadline for future Read calls and
// any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (s *ReceiveStream) SetReadDeadline(t time.Time) error {
	call, err := s.beginReceiveCall(receiveControlCall)
	if err != nil {
		return err
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishReceiveCall(&mutexHeld, &returned)
	if err := s.retirementError(); err != nil {
		returned = true
		return err
	}
	s.deadline = monotime.FromTime(t)
	s.mutex.Unlock()
	mutexHeld = false
	s.signalRead()
	returned = true
	return nil
}

// CloseForShutdown closes a stream abruptly.
// It makes Read unblock (and return the error) immediately.
// The peer will NOT be informed about this: the stream is closed without sending a FIN or RESET.
func (s *ReceiveStream) closeForShutdown(err error) (result error) {
	call, callErr := s.beginReceiveCall(receiveNativeCall)
	if callErr != nil {
		if s.frameIntake.budget != nil {
			s.frameIntake.budget.markUncertain() // refused teardown is not successful cleanup
		}
		s.signalRead()
		return errors.Join(errStreamShutdownUncertain, callErr)
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.signalRead()
	defer s.finishReceiveCall(&mutexHeld, &returned)
	defer func() {
		if returned {
			returned = false
			result = s.shutdown.complete(result, s.frameIntake.budget)
			returned = true
		}
	}()
	already, previous := s.shutdown.begin()
	if retirementErr := s.retirementError(); retirementErr != nil {
		returned = true
		return retirementErr
	}
	if already {
		returned = true
		return previous
	}
	s.closeForShutdownErr = streamShutdownCause(err)
	if !s.errorRead {
		s.currentFrameIsLast = false
	}
	if discardErr := s.discardBufferedFrames(); discardErr != nil {
		s.receiveUncertain = true
		returned = true
		return discardErr
	}
	returned = true
	return nil
}

// signalRead performs a non-blocking send on the readChan
func (s *ReceiveStream) signalRead() {
	select {
	case s.readChan <- struct{}{}:
	default:
	}
}
