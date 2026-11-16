package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils/ringbuffer"
)

var errFramerRetirement = errors.New("quic: original framer metadata did not retire")

// The original loop or last registered stream caller reserves this one finalizer.
// Stream wrappers release their mutexes/gates before registry completion. No
// registry mutex, wait, provider callback or new worker crosses framer cleanup.
func (o *streamCalls) retireFramerIfReady() {
	o.access.Lock()
	if !o.sealed || o.loopActive || o.hasRunning(6) ||
		o.failure != nil || o.framerFailure != nil || o.framerRetiring || o.framerRetired {
		o.access.Unlock()
		return
	}
	if o.counts != [6]int{} || len(o.live) != 0 || o.framer == nil {
		// No actual producer remains to repair inconsistent retained metadata.
		// Report it instead of waiting forever or discarding original references.
		o.framerFailure = errFramerRetirement
		o.settleLocked()
		o.access.Unlock()
		return
	}
	o.framerRetiring = true // original pointer remains attached before metadata locks
	original := o.framer
	o.access.Unlock()
	returned := false
	defer o.finishFramerRetirement(&returned)
	returned = original.retireStreamMetadata()
}

func (o *streamCalls) finishFramerRetirement(returned *bool) {
	o.access.Lock()
	defer o.access.Unlock()
	if *returned {
		o.framer = nil
		o.framerRetired, o.framerRetiring = true, false
	} else {
		o.framerFailure = errFramerRetirement
		// Keep the original pointer and retiring marker; there is no retry.
	}
	o.settleLocked()
}

// controlFrameMutex -> mutex is also Append/0RTT's lock order. No stream,
// frame, flow-controller, native or external callback is invoked here.
func (f *framer) retireStreamMetadata() bool {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.metadataRetired {
		return true
	}
	if f.sealed || !f.pathResponsesSealed || f.pathResponses != nil {
		return false // prior path-response retirement must already have returned
	}
	f.sealed = true
	f.streamGeneration.Store(nil)
	clear(f.activeStreams)
	f.activeStreams = nil
	f.streamQueue = ringbuffer.RingBuffer[protocol.StreamID]{}
	clear(f.streamsWithControlFrames)
	f.streamsWithControlFrames = nil
	clear(f.controlFrames)
	f.controlFrames = nil
	f.connFlowController = nil
	f.metadataRetired = true
	return true
}
