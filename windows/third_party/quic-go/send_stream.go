package quic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

// A SendStream is a unidirectional Send Stream.
type SendStream struct {
	mutex sync.Mutex

	numOutstandingFrames int64 // outstanding STREAM and RESET_STREAM frames
	retransmissionQueue  []*sentStreamData

	ctx       context.Context
	ctxCancel context.CancelCauseFunc

	streamID   protocol.StreamID
	sender     streamSender
	callBudget *streamReceiveBudget
	callCounts [2]int // protected by callBudget.calls.access

	// reliableSize is the portion of the stream that needs to be transmitted reliably,
	// even if the stream is cancelled.
	// This requires the peer to support RESET_STREAM_AT.
	// This value should not be accessed directly, but only through the reliableOffset method.
	// This method returns 0 if the peer doesn't support the RESET_STREAM_AT extension.
	reliableSize protocol.ByteCount
	writeOffset  protocol.ByteCount

	shutdownErr            error
	shutdown               streamShutdownReceipt
	writeUncertain         bool // sticky; an original write did not return normally
	resetErr               *StreamError
	queuedResetStreamFrame *wire.ResetStreamFrame

	supportsResetStreamAt bool
	finishedWriting       bool // set once Close() is called
	finSent               bool // set when a STREAM_FRAME with FIN bit has been sent
	// Set when the application knows about the cancellation.
	// This can happen because the application called CancelWrite,
	// or because Write returned the error (for remote cancellations).
	cancellationFlagged bool
	completed           bool // set when this stream has been reported to the streamSender as completed

	dataForWriting []byte // during a Write() call, this slice is the part of p that still needs to be sent out
	nextFrame      *sentStreamData

	writeChan chan struct{}
	writeOnce chan struct{}
	deadline  monotime.Time

	flowController flowcontrol.StreamFlowController
}

var (
	_ streamControlFrameGetter = &SendStream{}
	_ outgoingStream           = &SendStream{}
	_ sendStreamFrameHandler   = &SendStream{}
)

var errSendStreamWriteUncertain = errors.New("quic: original stream write did not return normally")

func newSendStream(
	ctx context.Context,
	streamID protocol.StreamID,
	sender streamSender,
	flowController flowcontrol.StreamFlowController,
	supportsResetStreamAt bool,
	callBudget *streamReceiveBudget,
	construction *streamConstruction,
) *SendStream {
	construction.require(streamID)
	if construction.send != nil {
		panic(errStreamMapUncertain)
	}
	construction.stage = constructionSend
	s := &SendStream{
		streamID:              streamID,
		sender:                sender,
		flowController:        flowController,
		callBudget:            callBudget,
		supportsResetStreamAt: supportsResetStreamAt,
	}
	construction.send = s // before channel allocation or parent-context calls
	s.writeChan = make(chan struct{}, 1)
	s.writeOnce = make(chan struct{}, 1) // cap: 1, to protect concurrent Write
	construction.stage = constructionContext
	s.ctx, s.ctxCancel = context.WithCancelCause(ctx)
	construction.stage = constructionContextReady
	return s
}

// StreamID returns the stream ID.
func (s *SendStream) StreamID() StreamID {
	return s.streamID // same for receiveStream and sendStream
}

// Write writes data to the stream.
// Write can be made to time out using [SendStream.SetWriteDeadline].
// If the stream was canceled, the error is a [StreamError].
func (s *SendStream) Write(p []byte) (int, error) {
	call, err := s.beginSendCall(sendWriteCall)
	if err != nil {
		return 0, err
	}
	returned := false
	defer call.finish(&returned)
	// Concurrent use of Write is not permitted (and doesn't make any sense),
	// but sometimes people do it anyway.
	// Make sure that we only execute one call at any given time to avoid hard to debug failures.
	select {
	case s.writeOnce <- struct{}{}:
	case <-call.owner.closing:
		returned = true
		return 0, call.owner.stoppedError()
	}
	defer func() { <-s.writeOnce }()
	mutexHeld := false
	defer s.finishSendCall(&mutexHeld, &returned)

	isNewlyCompleted, n, err := s.write(p)
	if isNewlyCompleted {
		s.sender.onStreamCompleted(s.streamID)
	}
	returned = true
	return n, err
}

func (s *SendStream) write(p []byte) (bool /* is newly completed */, int, error) {
	s.mutex.Lock()
	mutexHeld, returned := true, false
	defer func() {
		// The body deliberately unlocks around sender notifications and waits.
		// An abnormal callback must not unlock an already unlocked mutex or
		// let a subsequent writer replace its uncertain original input view.
		if !mutexHeld {
			s.mutex.Lock()
		}
		if !returned {
			s.writeUncertain = true
			s.callBudget.markUncertain()
		} else if !s.writeUncertain {
			// The packetizer borrows this view only under this same mutex.
			// Clear it on every ordinary result, including reset/shutdown errors,
			// before the caller is allowed to return its original backing.
			s.dataForWriting = nil
		}
		s.mutex.Unlock()
	}()
	completed, n, err := s.writeLocked(p, &mutexHeld)
	returned = true
	return completed, n, err
}

// Caller holds mutex. mutexHeld follows the intentional unlock/relock pairs so
// the original wrapper can retain and seal abnormal state before writeOnce ends.
func (s *SendStream) writeLocked(p []byte, mutexHeld *bool) (bool, int, error) {
	if s.writeUncertain {
		return false, 0, errSendStreamWriteUncertain
	}
	if err := s.callBudget.calls.stoppedError(); err != nil {
		return false, 0, err
	}
	if s.resetErr != nil {
		s.cancellationFlagged = true
		return s.isNewlyCompleted(), 0, s.resetErr
	}
	if s.shutdownErr != nil {
		return false, 0, s.shutdownErr
	}
	if s.finishedWriting {
		return false, 0, fmt.Errorf("write on closed stream %d", s.streamID)
	}
	if !s.deadline.IsZero() && !monotime.Now().Before(s.deadline) {
		return false, 0, errDeadline
	}
	if len(p) == 0 {
		return false, 0, nil
	}

	s.dataForWriting = p

	var (
		deadlineTimer  *time.Timer
		bytesWritten   int
		notifiedSender bool
	)
	for {
		bytesWritten = len(p) - len(s.dataForWriting)
		// Cancellation and shutdown must win before a short remaining tail
		// is copied into a fresh frame. Already accepted bytes keep their count.
		if s.shutdownErr != nil || s.resetErr != nil || s.dataForWriting == nil {
			break
		}
		if err := s.callBudget.calls.stoppedError(); err != nil {
			return false, bytesWritten, err
		}
		deadline := s.deadline
		if !deadline.IsZero() && !monotime.Now().Before(deadline) {
			return false, bytesWritten, errDeadline
		}
		var copied bool
		// As soon as dataForWriting becomes smaller than a certain size x, we copy all the data to a STREAM frame (s.nextFrame),
		// which can then be popped the next time we assemble a packet.
		// This allows us to return Write() when all data but x bytes have been sent out.
		// When the user now calls Close(), this is much more likely to happen before we popped that last STREAM frame,
		// allowing us to set the FIN bit on that frame (instead of sending an empty STREAM frame with FIN).
		if s.canBufferStreamFrame() && len(s.dataForWriting) > 0 {
			if s.nextFrame == nil {
				original, err := s.newSentFrame(true)
				if err != nil {
					return false, bytesWritten, err
				}
				s.nextFrame = original
				f := original.raw()
				f.Offset = s.writeOffset
				f.StreamID = s.streamID
				f.DataLenPresent = true
				f.Data = f.Data[:len(s.dataForWriting)]
				copy(f.Data, s.dataForWriting)
			} else {
				frame := s.nextFrame.raw()
				l := len(frame.Data)
				frame.Data = frame.Data[:l+len(s.dataForWriting)]
				copy(frame.Data[l:], s.dataForWriting)
			}
			s.dataForWriting = nil
			bytesWritten = len(p)
			copied = true
		} else {
			if !deadline.IsZero() {
				if deadlineTimer == nil {
					deadlineTimer = time.NewTimer(monotime.Until(deadline))
					defer deadlineTimer.Stop()
				} else {
					deadlineTimer.Reset(monotime.Until(deadline))
				}
			}
		}

		s.mutex.Unlock()
		*mutexHeld = false
		if !notifiedSender {
			s.sender.onHasStreamData(s.streamID, s) // must be called without holding the mutex
			notifiedSender = true
		}
		if copied {
			s.mutex.Lock()
			*mutexHeld = true
			break
		}
		if deadline.IsZero() {
			select {
			case <-s.writeChan:
			case <-s.callBudget.calls.closing:
			}
		} else {
			select {
			case <-s.writeChan:
			case <-s.callBudget.calls.closing:
			case <-deadlineTimer.C:
			}
		}
		s.mutex.Lock()
		*mutexHeld = true
	}

	if bytesWritten == len(p) {
		return false, bytesWritten, nil
	}
	if s.shutdownErr != nil {
		return false, bytesWritten, s.shutdownErr
	}
	if s.resetErr != nil {
		s.cancellationFlagged = true
		return s.isNewlyCompleted(), bytesWritten, s.resetErr
	}
	return false, bytesWritten, nil
}

func (s *SendStream) canBufferStreamFrame() bool {
	var l protocol.ByteCount
	if s.nextFrame != nil {
		l = s.nextFrame.raw().DataLen()
	}
	return l+protocol.ByteCount(len(s.dataForWriting)) <= protocol.MaxPacketBufferSize
}

// popStreamFrame returns the next STREAM frame that is supposed to be sent on this stream
// maxBytes is the maximum length this frame (including frame header) will have.
func (s *SendStream) popStreamFrame(maxBytes protocol.ByteCount, v protocol.Version) (_ ackhandler.StreamFrame, _ *wire.StreamDataBlockedFrame, hasMore bool) {
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		return ackhandler.StreamFrame{}, nil, false
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	original, blocked, hasMoreData := s.popNewOrRetransmittedStreamFrame(maxBytes, v)
	var attempt *streamSendAttempt
	if original != nil {
		attempt, err = original.newAttempt()
		if err != nil {
			returned = true
			return ackhandler.StreamFrame{}, blocked, false
		}
		s.numOutstandingFrames++
	}
	s.mutex.Unlock()
	mutexHeld = false

	if original == nil {
		returned = true
		return ackhandler.StreamFrame{}, blocked, hasMoreData
	}
	frame := original.raw()
	returned = true
	return ackhandler.StreamFrame{
		Frame:   frame,
		Handler: attempt,
	}, blocked, hasMoreData
}

func (s *SendStream) popNewOrRetransmittedStreamFrame(maxBytes protocol.ByteCount, v protocol.Version) (_ *sentStreamData, _ *wire.StreamDataBlockedFrame, hasMoreData bool) {
	if s.shutdownErr != nil || s.writeUncertain {
		return nil, nil, false
	}
	if s.resetErr != nil {
		reliableOffset := s.reliableOffset()
		if reliableOffset == 0 || (s.writeOffset >= reliableOffset && len(s.retransmissionQueue) == 0) {
			return nil, nil, false
		}
	}

	if len(s.retransmissionQueue) > 0 {
		f, hasMoreRetransmissions := s.maybeGetRetransmission(maxBytes, v)
		if f != nil || hasMoreRetransmissions {
			if f == nil {
				return nil, nil, true
			}
			// We always claim that we have more data to send.
			// This might be incorrect, in which case there'll be a spurious call to popStreamFrame in the future.
			return f, nil, true
		}
	}

	if len(s.dataForWriting) == 0 && s.nextFrame == nil {
		if s.finishedWriting && !s.finSent {
			original, err := s.newSentFrame(false)
			if err != nil {
				return nil, nil, false
			}
			s.finSent = true
			original.raw().Fin = true
			return original, nil, false
		}
		return nil, nil, false
	}

	maxDataLen := s.flowController.SendWindowSize()
	if maxDataLen == 0 {
		return nil, nil, true
	}

	// if the stream is canceled, only data up to the reliable size needs to be sent
	reliableOffset := s.reliableOffset()
	if s.resetErr != nil && reliableOffset > 0 {
		maxDataLen = min(maxDataLen, reliableOffset-s.writeOffset)
	}
	original, hasMoreData := s.popNewStreamFrame(maxBytes, maxDataLen, v)
	if original == nil {
		return nil, nil, hasMoreData
	}
	f := original.raw()
	if f.DataLen() > 0 {
		s.writeOffset += f.DataLen()
		s.flowController.AddBytesSent(f.DataLen())
	}
	if s.resetErr != nil && s.writeOffset >= reliableOffset {
		hasMoreData = false
	}
	var blocked *wire.StreamDataBlockedFrame
	// If the entire send window is used, the stream might have become blocked on stream-level flow control.
	// This is not guaranteed though, because the stream might also have been blocked on connection-level flow control.
	if f.DataLen() == maxDataLen && s.flowController.IsNewlyBlocked() {
		blocked = &wire.StreamDataBlockedFrame{StreamID: s.streamID, MaximumStreamData: s.writeOffset}
	}
	f.Fin = s.finishedWriting && s.dataForWriting == nil && s.nextFrame == nil && !s.finSent
	if f.Fin {
		s.finSent = true
	}
	return original, blocked, hasMoreData
}

// popNewStreamFrame returns a new STREAM frame to send for this stream
// hasMoreData says if there's more data to send, *not* taking into account the reliable size
func (s *SendStream) popNewStreamFrame(maxBytes, maxDataLen protocol.ByteCount, v protocol.Version) (_ *sentStreamData, hasMoreData bool) {
	if s.nextFrame != nil {
		frame := s.nextFrame.raw()
		maxDataLen := min(maxDataLen, frame.MaxDataLen(maxBytes, v))
		if maxDataLen == 0 {
			return nil, true
		}
		nextFrame := s.nextFrame
		if frame.DataLen() > maxDataLen {
			rest, err := s.newSentFrame(true)
			if err != nil {
				return nil, true
			}
			output := rest.raw()
			output.Offset = s.writeOffset + maxDataLen
			output.Data = output.Data[:frame.DataLen()-maxDataLen]
			copy(output.Data, frame.Data[maxDataLen:])
			frame.Data = frame.Data[:maxDataLen]
			s.nextFrame = rest
		} else {
			s.nextFrame = nil
			s.signalWrite()
		}
		return nextFrame, s.nextFrame != nil || s.dataForWriting != nil
	}

	original, err := s.newSentFrame(true)
	if err != nil {
		return nil, false
	}
	f := original.raw()

	hasMoreData = s.popNewStreamFrameWithoutBuffer(f, maxBytes, maxDataLen, v)
	if len(f.Data) == 0 && !f.Fin {
		original.release()
		return nil, hasMoreData
	}
	return original, hasMoreData
}

func (s *SendStream) popNewStreamFrameWithoutBuffer(f *wire.StreamFrame, maxBytes, sendWindow protocol.ByteCount, v protocol.Version) bool {
	maxDataLen := f.MaxDataLen(maxBytes, v)
	if maxDataLen == 0 { // a STREAM frame must have at least one byte of data
		return s.dataForWriting != nil || s.nextFrame != nil || s.finishedWriting
	}
	s.getDataForWriting(f, min(maxDataLen, sendWindow))

	return s.dataForWriting != nil || s.nextFrame != nil || s.finishedWriting
}

func (s *SendStream) maybeGetRetransmission(maxBytes protocol.ByteCount, v protocol.Version) (*sentStreamData, bool /* has more retransmissions */) {
	f := s.retransmissionQueue[0]
	newFrame, needsSplit := s.splitRetransmission(f, maxBytes, v)
	if needsSplit {
		return newFrame, true
	}
	s.retransmissionQueue[0] = nil
	s.retransmissionQueue = s.retransmissionQueue[1:]
	return f, len(s.retransmissionQueue) > 0
}

func (s *SendStream) getDataForWriting(f *wire.StreamFrame, maxBytes protocol.ByteCount) {
	if protocol.ByteCount(len(s.dataForWriting)) <= maxBytes {
		f.Data = f.Data[:len(s.dataForWriting)]
		copy(f.Data, s.dataForWriting)
		s.dataForWriting = nil
		s.signalWrite()
		return
	}
	f.Data = f.Data[:maxBytes]
	copy(f.Data, s.dataForWriting)
	s.dataForWriting = s.dataForWriting[maxBytes:]
	if s.canBufferStreamFrame() {
		s.signalWrite()
	}
}

func (s *SendStream) isNewlyCompleted() bool {
	if s.completed || s.writeUncertain {
		return false
	}
	if s.nextFrame != nil && s.nextFrame.raw().DataLen() > 0 {
		return false
	}
	// We need to keep the stream around until all frames have been sent and acknowledged.
	if s.numOutstandingFrames > 0 || len(s.retransmissionQueue) > 0 || s.queuedResetStreamFrame != nil {
		return false
	}
	// The stream is completed if we sent the FIN.
	if s.finSent {
		s.completed = true
		return true
	}
	// The stream is also completed if:
	// 1. the application called CancelWrite, or
	// 2. we received a STOP_SENDING, and
	// 		* the application consumed the error via Write, or
	//		* the application called Close
	if s.resetErr != nil && (s.cancellationFlagged || s.finishedWriting) {
		s.completed = true
		return true
	}
	return false
}

// Close closes the write-direction of the stream.
// Future calls to Write are not permitted after calling Close.
// It must not be called concurrently with Write.
// It must not be called after calling CancelWrite.
func (s *SendStream) Close() error {
	call, err := s.beginSendCall(sendControlCall)
	if err != nil {
		return err
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if s.writeUncertain {
		returned = true
		return errSendStreamWriteUncertain
	}
	if s.shutdownErr != nil || s.finishedWriting {
		returned = true
		return nil
	}
	s.finishedWriting = true
	cancelled := s.resetErr != nil
	if cancelled {
		s.cancellationFlagged = true
	}
	completed := s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false

	if completed {
		s.sender.onStreamCompleted(s.streamID)
	}
	if cancelled {
		returned = true
		return fmt.Errorf("close called for canceled stream %d", s.streamID)
	}
	s.sender.onHasStreamData(s.streamID, s) // need to send the FIN, must be called without holding the mutex

	s.ctxCancel(nil)
	returned = true
	return nil
}

// SetReliableBoundary marks the data written to this stream so far as reliable.
// It is valid to call this function multiple times, thereby increasing the reliable size.
// It only has an effect if the peer enabled support for the RESET_STREAM_AT extension,
// otherwise, it is a no-op.
func (s *SendStream) SetReliableBoundary() {
	call, err := s.beginSendCall(sendControlCall)
	if err != nil {
		return
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if s.writeUncertain {
		returned = true
		return
	}

	s.reliableSize = s.writeOffset
	if s.nextFrame != nil {
		s.reliableSize += s.nextFrame.raw().DataLen()
	}
	returned = true
}

// returnFramesToPool returns all queued frames to the sync.Pool
func (s *SendStream) returnFramesToPool() {
	for i, original := range s.retransmissionQueue {
		original.release()
		s.retransmissionQueue[i] = nil
	}
	clear(s.retransmissionQueue)
	s.retransmissionQueue = nil
	if s.nextFrame != nil {
		s.nextFrame.release()
		s.nextFrame = nil
	}
}

// CancelWrite aborts sending on this stream.
// Data already written, but not yet delivered to the peer is not guaranteed to be delivered reliably.
// Write will unblock immediately, and future calls to Write will fail.
// When called multiple times it is a no-op.
// When called after Close, it aborts reliable delivery of outstanding stream data.
// Note that there is no guarantee if the peer will receive the FIN or the cancellation error first.
func (s *SendStream) CancelWrite(errorCode StreamErrorCode) {
	call, err := s.beginSendCall(sendControlCall)
	if err != nil {
		return
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if s.shutdownErr != nil || s.writeUncertain {
		returned = true
		return
	}

	s.cancellationFlagged = true

	if s.resetErr != nil {
		completed := s.isNewlyCompleted()
		s.mutex.Unlock()
		mutexHeld = false
		// The user has called CancelWrite. If the previous cancellation was because of a
		// STOP_SENDING, we don't need to flag the error to the user anymore.
		if completed {
			s.sender.onStreamCompleted(s.streamID)
		}
		returned = true
		return
	}
	s.resetErr = &StreamError{StreamID: s.streamID, ErrorCode: errorCode, Remote: false}
	s.ctxCancel(s.resetErr)

	reliableOffset := s.reliableOffset()
	if reliableOffset == 0 {
		s.numOutstandingFrames = 0
		s.returnFramesToPool()
	}
	s.queuedResetStreamFrame = &wire.ResetStreamFrame{
		StreamID:  s.streamID,
		FinalSize: max(s.writeOffset, reliableOffset),
		ErrorCode: errorCode,
		// if the peer doesn't support the extension, the reliable offset will always be 0
		ReliableSize: reliableOffset,
	}
	if reliableOffset > 0 {
		if s.nextFrame != nil {
			frame := s.nextFrame.raw()
			if frame.Offset >= reliableOffset {
				s.nextFrame.release()
				s.nextFrame = nil
			} else if frame.Offset+frame.DataLen() > reliableOffset {
				frame.Data = frame.Data[:reliableOffset-frame.Offset]
			}
		}
		if len(s.retransmissionQueue) > 0 {
			retransmissionQueue := make([]*sentStreamData, 0, len(s.retransmissionQueue))
			for _, original := range s.retransmissionQueue {
				f := original.raw()
				if f.Offset >= reliableOffset {
					original.release()
					continue
				}
				if f.Offset+f.DataLen() <= reliableOffset {
					retransmissionQueue = append(retransmissionQueue, original)
				} else {
					f.Data = f.Data[:reliableOffset-f.Offset]
					retransmissionQueue = append(retransmissionQueue, original)
				}
			}
			s.retransmissionQueue = retransmissionQueue
		}
	}
	s.mutex.Unlock()
	mutexHeld = false

	s.signalWrite()
	s.sender.onHasStreamControlFrame(s.streamID, s)
	returned = true
}

func (s *SendStream) enableResetStreamAt() {
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		return
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if !s.writeUncertain {
		s.supportsResetStreamAt = true
	}
	returned = true
}

func (s *SendStream) updateSendWindow(limit protocol.ByteCount) {
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		return
	}
	returned, mutexHeld := false, false
	defer call.finish(&returned)
	defer s.finishSendCall(&mutexHeld, &returned)
	updated := s.flowController.UpdateSendWindow(limit)
	if !updated { // duplicate or reordered MAX_STREAM_DATA frame
		returned = true
		return
	}
	s.mutex.Lock()
	mutexHeld = true
	hasStreamData := !s.writeUncertain && (s.dataForWriting != nil || s.nextFrame != nil)
	s.mutex.Unlock()
	mutexHeld = false
	if hasStreamData {
		s.sender.onHasStreamData(s.streamID, s)
	}
	returned = true
}

func (s *SendStream) handleStopSendingFrame(f *wire.StopSendingFrame) {
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		return
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if s.shutdownErr != nil || s.writeUncertain {
		returned = true
		return
	}

	// If the stream was already cancelled (either locally, or due to a previous STOP_SENDING frame),
	// there's nothing else to do.
	if s.resetErr != nil && s.reliableOffset() == 0 {
		returned = true
		return
	}
	// if the peer stopped reading from the stream, there's no need to transmit any data reliably
	s.reliableSize = 0
	s.numOutstandingFrames = 0
	s.returnFramesToPool()
	if s.resetErr == nil {
		s.resetErr = &StreamError{StreamID: s.streamID, ErrorCode: f.ErrorCode, Remote: true}
		s.ctxCancel(s.resetErr)
	}
	s.queuedResetStreamFrame = &wire.ResetStreamFrame{
		StreamID:  s.streamID,
		FinalSize: s.writeOffset,
		ErrorCode: s.resetErr.ErrorCode,
	}
	s.mutex.Unlock()
	mutexHeld = false

	s.signalWrite()
	s.sender.onHasStreamControlFrame(s.streamID, s)
	returned = true
}

func (s *SendStream) getControlFrame(monotime.Time) (_ ackhandler.Frame, ok, hasMore bool) {
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		return ackhandler.Frame{}, false, false
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)

	if s.queuedResetStreamFrame == nil || s.writeUncertain {
		returned = true
		return ackhandler.Frame{}, false, false
	}
	s.numOutstandingFrames++
	f := ackhandler.Frame{
		Frame:   s.queuedResetStreamFrame,
		Handler: (*sendStreamResetStreamHandler)(s),
	}
	s.queuedResetStreamFrame = nil
	returned = true
	return f, true, false
}

func (s *SendStream) reliableOffset() protocol.ByteCount {
	if !s.supportsResetStreamAt {
		return 0
	}
	return s.reliableSize
}

// The Context is canceled as soon as the write-side of the stream is closed.
// This happens when Close() or CancelWrite() is called, or when the peer
// cancels the read-side of their stream.
// The cancellation cause is set to the error that caused the stream to
// close, or `context.Canceled` in case the stream is closed without error.
func (s *SendStream) Context() context.Context {
	return s.ctx
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some data was successfully written.
// A zero value for t means Write will not time out.
func (s *SendStream) SetWriteDeadline(t time.Time) error {
	call, err := s.beginSendCall(sendControlCall)
	if err != nil {
		return err
	}
	returned := false
	defer call.finish(&returned)
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	if s.writeUncertain {
		returned = true
		return errSendStreamWriteUncertain
	}
	s.deadline = monotime.FromTime(t)
	s.mutex.Unlock()
	mutexHeld = false
	s.signalWrite()
	returned = true
	return nil
}

// CloseForShutdown closes a stream abruptly.
// It makes Write unblock (and return the error) immediately.
// The peer will NOT be informed about this: the stream is closed without sending a FIN or RST.
func (s *SendStream) closeForShutdown(err error) (result error) {
	call, callErr := s.beginSendCall(sendNativeCall)
	if callErr != nil {
		if s.callBudget != nil {
			s.callBudget.markUncertain()
		}
		s.signalWrite()
		return errors.Join(errStreamShutdownUncertain, callErr)
	}
	returned := false
	defer call.finish(&returned)
	defer s.signalWrite()
	s.mutex.Lock()
	mutexHeld := true
	defer s.finishSendCall(&mutexHeld, &returned)
	defer func() {
		if returned {
			returned = false // notification must return before any positive receipt
			result = s.shutdown.complete(result, s.callBudget)
			returned = true
		}
	}()
	already, previous := s.shutdown.begin()
	if s.writeUncertain {
		returned = true
		return errSendStreamWriteUncertain
	}
	if already {
		returned = true
		return previous
	}
	if s.shutdownErr == nil {
		s.shutdownErr = streamShutdownCause(err)
	}
	s.returnFramesToPool()
	if s.ctxCancel == nil {
		returned = true
		return errStreamShutdownUncertain
	}
	cancel, cause := s.ctxCancel, s.shutdownErr
	s.mutex.Unlock()
	mutexHeld = false
	s.signalWrite() // wake an original writer before a possibly opaque cancel call
	cancel(cause)   // exact original cancel function, outside the stream mutex
	s.mutex.Lock()
	mutexHeld = true
	if s.writeUncertain {
		returned = true
		return errSendStreamWriteUncertain
	}
	returned = true
	return nil
}

// signalWrite performs a non-blocking send on the writeChan
func (s *SendStream) signalWrite() {
	select {
	case s.writeChan <- struct{}{}:
	default:
	}
}

type sendStreamResetStreamHandler SendStream

var _ ackhandler.FrameHandler = &sendStreamResetStreamHandler{}

func (s *sendStreamResetStreamHandler) OnAcked(f wire.Frame) {
	rsf := f.(*wire.ResetStreamFrame)
	s.mutex.Lock()
	// If the peer sent a STOP_SENDING after we sent a RESET_STREAM_AT frame,
	// we sent 1. reduced the reliable size to 0 and 2. sent a RESET_STREAM frame.
	// In this case, we don't care about the acknowledgment of this frame.
	if rsf.ReliableSize != (*SendStream)(s).reliableOffset() {
		s.mutex.Unlock()
		return
	}
	s.numOutstandingFrames--
	if s.numOutstandingFrames < 0 {
		panic("numOutStandingFrames negative")
	}
	completed := (*SendStream)(s).isNewlyCompleted()
	s.mutex.Unlock()

	if completed {
		s.sender.onStreamCompleted(s.streamID)
	}
}

func (s *sendStreamResetStreamHandler) OnLost(f wire.Frame) {
	rsf := f.(*wire.ResetStreamFrame)
	s.mutex.Lock()
	// If the peer sent a STOP_SENDING after we sent a RESET_STREAM_AT frame,
	// we sent 1. reduced the reliable size to 0 and 2. sent a RESET_STREAM frame.
	// In this case, the loss of the RESET_STREAM_AT frame can be ignored.
	if rsf.ReliableSize != (*SendStream)(s).reliableOffset() {
		s.mutex.Unlock()
		return
	}
	s.queuedResetStreamFrame = rsf
	s.numOutstandingFrames--
	s.mutex.Unlock()
	s.sender.onHasStreamControlFrame(s.streamID, (*SendStream)(s))
}
