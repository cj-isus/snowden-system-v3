package quic

import (
	"sync"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils/ringbuffer"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/quicvarint"
)

const (
	maxPathResponses = 256
	maxControlFrames = 16 << 10
)

// This is the largest possible size of a stream-related control frame
// (which is the RESET_STREAM frame).
const maxStreamControlFrameSize = 25

type streamFrameGetter interface {
	popStreamFrame(protocol.ByteCount, protocol.Version) (ackhandler.StreamFrame, *wire.StreamDataBlockedFrame, bool)
}

type streamControlFrameGetter interface {
	getControlFrame(monotime.Time) (_ ackhandler.Frame, ok, hasMore bool)
}

type framer struct {
	mutex sync.Mutex

	activeStreams            map[protocol.StreamID]streamFrameGetter
	streamQueue              ringbuffer.RingBuffer[protocol.StreamID]
	streamsWithControlFrames map[protocol.StreamID]streamControlFrameGetter
	// Written with controlFrameMutex and mutex held; readers hold either one.
	sealed, metadataRetired bool
	streamGeneration        atomic.Pointer[streamGeneration] // swaps under both metadata locks

	controlFrameMutex          sync.Mutex
	controlFrames              []queuedFrameEntry
	controlStore               framerControlStorage
	pathResponses              []queuedPathResponse
	pathResponsesSealed        bool // protected by controlFrameMutex
	connFlowController         flowcontrol.ConnectionFlowController
	queuedTooManyControlFrames atomic.Bool
}

func newFramer(connFlowController flowcontrol.ConnectionFlowController) *framer {
	f := &framer{}
	f.init(connFlowController, nil)
	return f
}

func (f *framer) init(controller flowcontrol.ConnectionFlowController, wake chan<- struct{}) {
	if f.controlStore.owner != nil || f.activeStreams != nil || f.streamsWithControlFrames != nil {
		panic(errFramerControlState)
	}
	f.controlStore.owner, f.controlStore.wake = f, wake
	f.connFlowController = controller
	f.activeStreams = make(map[protocol.StreamID]streamFrameGetter)
	f.streamsWithControlFrames = make(map[protocol.StreamID]streamControlFrameGetter)
}

func (f *framer) HasData() bool {
	if f.controlFailure() != nil {
		return false
	}
	f.mutex.Lock()
	if f.sealed {
		f.mutex.Unlock()
		return false
	}
	hasData := !f.streamQueue.Empty()
	f.mutex.Unlock()
	if hasData {
		return true
	}
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	return len(f.streamsWithControlFrames) > 0 || len(f.controlFrames) > 0 || len(f.pathResponses) > 0
}

func (f *framer) QueueControlFrame(frame wire.Frame) {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	if f.sealed || f.controlFailure() != nil {
		return
	}

	if pr, ok := frame.(*wire.PathResponseFrame); ok {
		if pr == nil {
			f.failControlLocked(framerControlUncertain)
			return
		}
		if f.pathResponsesSealed {
			return // terminal retirement cannot be refilled by a late legacy producer
		}
		// Only queue up to maxPathResponses PATH_RESPONSE frames.
		// This limit should be high enough to never be hit in practice,
		// unless the peer is doing something malicious.
		if len(f.pathResponses) >= maxPathResponses {
			return
		}
		f.pathResponses = append(f.pathResponses, queuedPathResponse{frame: pr})
		return
	}
	f.queueControlFrameLocked(frame)
}

func (f *framer) Append(
	frames []ackhandler.Frame,
	streamFrames []ackhandler.StreamFrame,
	maxLen protocol.ByteCount,
	now monotime.Time,
	v protocol.Version,
) ([]ackhandler.Frame, []ackhandler.StreamFrame, protocol.ByteCount) {
	f.controlFrameMutex.Lock()
	controlLocked := true
	defer func() {
		if controlLocked {
			f.controlFrameMutex.Unlock()
		}
	}()
	if f.sealed || f.controlFailure() != nil {
		return frames, streamFrames, 0
	}
	frames, controlFrameLen := f.appendControlFrames(frames, maxLen, now, v)
	maxLen -= controlFrameLen

	var lastFrame ackhandler.StreamFrame
	var streamFrameLen protocol.ByteCount
	f.mutex.Lock()
	streamsLocked := true
	defer func() {
		if streamsLocked {
			f.mutex.Unlock()
		}
	}()
	// pop STREAM frames, until less than 128 bytes are left in the packet
	numActiveStreams := f.streamQueue.Len()
	for i := 0; i < numActiveStreams; i++ {
		if protocol.MinStreamFrameSize > maxLen {
			break
		}
		sf, blocked := f.getNextStreamFrame(maxLen, v)
		if sf.Frame != nil {
			streamFrames = append(streamFrames, sf)
			maxLen -= sf.Frame.Length(v)
			lastFrame = sf
			streamFrameLen += sf.Frame.Length(v)
		}
		// If the stream just became blocked on stream flow control, attempt to pack the
		// STREAM_DATA_BLOCKED into the same packet.
		if blocked != nil {
			l := blocked.Length(v)
			// In case it doesn't fit, queue it for the next packet.
			if maxLen < l {
				f.queueControlFrameLocked(blocked)
				break
			}
			frames = append(frames, ackhandler.Frame{Frame: blocked})
			maxLen -= l
			controlFrameLen += l
		}
	}

	// The only way to become blocked on connection-level flow control is by sending STREAM frames.
	if isBlocked, offset := f.connFlowController.IsNewlyBlocked(); isBlocked {
		blocked := &wire.DataBlockedFrame{MaximumData: offset}
		l := blocked.Length(v)
		// In case it doesn't fit, queue it for the next packet.
		if maxLen >= l {
			frames = append(frames, ackhandler.Frame{Frame: blocked})
			controlFrameLen += l
		} else {
			f.queueControlFrameLocked(blocked)
		}
	}

	f.mutex.Unlock()
	streamsLocked = false
	f.controlFrameMutex.Unlock()
	controlLocked = false

	if lastFrame.Frame != nil {
		// account for the smaller size of the last STREAM frame
		streamFrameLen -= lastFrame.Frame.Length(v)
		lastFrame.Frame.DataLenPresent = false
		streamFrameLen += lastFrame.Frame.Length(v)
	}

	return frames, streamFrames, controlFrameLen + streamFrameLen
}

func (f *framer) appendControlFrames(
	frames []ackhandler.Frame,
	maxLen protocol.ByteCount,
	now monotime.Time,
	v protocol.Version,
) ([]ackhandler.Frame, protocol.ByteCount) {
	var length protocol.ByteCount
	// add a PATH_RESPONSE first, but only pack a single PATH_RESPONSE per packet
	if len(f.pathResponses) > 0 && f.pathResponses[0].transport == nil {
		frame := f.pathResponses[0].frame
		frameLen := frame.Length(v)
		if frameLen <= maxLen {
			frames = append(frames, ackhandler.Frame{Frame: frame})
			length += frameLen
			f.pathResponses[0] = queuedPathResponse{}
			f.pathResponses = f.pathResponses[1:]
		}
	}

	// add stream-related control frames
	for id, str := range f.streamsWithControlFrames {
	start:
		remainingLen := maxLen - length
		if remainingLen <= maxStreamControlFrameSize {
			break
		}
		fr, ok, hasMore := str.getControlFrame(now)
		if !hasMore {
			delete(f.streamsWithControlFrames, id)
		}
		if !ok {
			continue
		}
		frames = append(frames, fr)
		length += fr.Frame.Length(v)
		if hasMore {
			// It is rare that a stream has more than one control frame to queue.
			// We don't want to spawn another loop for just to cover that case.
			goto start
		}
	}

	for len(f.controlFrames) > 0 {
		next, frameLen, ok := f.appendQueuedControlLocked(frames, maxLen-length, v)
		if !ok {
			break
		}
		frames = next
		length += frameLen
	}

	return frames, length
}

// QueuedTooManyControlFrames is the legacy capacity flag. Native Conn also
// observes the original typed controlFailure, including uncertain metadata.
func (f *framer) QueuedTooManyControlFrames() bool {
	return f.queuedTooManyControlFrames.Load()
}

func (f *framer) AddActiveStream(id protocol.StreamID, str streamFrameGetter) {
	f.addActiveStream(nil, id, str)
}

func (f *framer) addActiveStream(generation *streamGeneration, id protocol.StreamID, str streamFrameGetter) bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.sealed || (generation != nil && f.streamGeneration.Load() != generation) {
		return false
	}
	if _, ok := f.activeStreams[id]; !ok {
		f.streamQueue.PushBack(id)
		f.activeStreams[id] = str
	}
	return true
}

func (f *framer) AddStreamWithControlFrames(id protocol.StreamID, str streamControlFrameGetter) {
	f.addStreamWithControlFrames(nil, id, str)
}

func (f *framer) addStreamWithControlFrames(generation *streamGeneration, id protocol.StreamID, str streamControlFrameGetter) bool {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	if f.sealed || (generation != nil && f.streamGeneration.Load() != generation) {
		return false
	}
	if _, ok := f.streamsWithControlFrames[id]; !ok {
		f.streamsWithControlFrames[id] = str
	}
	return true
}

// RemoveActiveStream is called when a stream completes.
func (f *framer) RemoveActiveStream(id protocol.StreamID) {
	f.removeActiveStream(nil, id)
}

func (f *framer) removeActiveStream(generation *streamGeneration, id protocol.StreamID) bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.sealed || (generation != nil && f.streamGeneration.Load() != generation) {
		return false
	}
	delete(f.activeStreams, id)
	// We don't delete the stream from the streamQueue,
	// since we'd have to iterate over the ringbuffer.
	// Instead, we check if the stream is still in activeStreams when appending STREAM frames.
	return true
}

func (f *framer) getNextStreamFrame(maxLen protocol.ByteCount, v protocol.Version) (ackhandler.StreamFrame, *wire.StreamDataBlockedFrame) {
	id := f.streamQueue.PopFront()
	// This should never return an error. Better check it anyway.
	// The stream will only be in the streamQueue, if it enqueued itself there.
	str, ok := f.activeStreams[id]
	// The stream might have been removed after being enqueued.
	if !ok {
		return ackhandler.StreamFrame{}, nil
	}
	// For the last STREAM frame, we'll remove the DataLen field later.
	// Therefore, we can pretend to have more bytes available when popping
	// the STREAM frame (which will always have the DataLen set).
	maxLen += protocol.ByteCount(quicvarint.Len(uint64(maxLen)))
	frame, blocked, hasMoreData := str.popStreamFrame(maxLen, v)
	if hasMoreData { // put the stream back in the queue (at the end)
		f.streamQueue.PushBack(id)
	} else { // no more data to send. Stream is not active
		delete(f.activeStreams, id)
	}
	// Note that the frame.Frame can be nil:
	// * if the stream was canceled after it said it had data
	// * the remaining size doesn't allow us to add another STREAM frame
	return frame, blocked
}

func (f *framer) Handle0RTTRejection(original, generation *streamGeneration) error {
	if original == nil || generation == nil || original == generation ||
		original.framer != f || generation.framer != f {
		return errStreamMapUncertain
	}
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.sealed || f.metadataRetired || f.streamGeneration.Load() != original {
		return errStreamMapUncertain
	}
	if !f.filterControl0RTTLocked() {
		return errFramerControlState
	}

	f.streamQueue.Clear()
	for id := range f.activeStreams {
		delete(f.activeStreams, id)
	}
	clear(f.streamsWithControlFrames)
	f.streamGeneration.Store(generation)
	return nil
}
