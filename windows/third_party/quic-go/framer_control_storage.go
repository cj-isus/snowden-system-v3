package quic

import (
	"errors"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

const (
	maxFramerControlBytes      = 16 << 20
	maxFramerControlFrameBytes = 1 << 20
	framerControlCapacity      = uint32(1)
	framerControlUncertain     = uint32(2)
)

var (
	errFramerControlCapacity = errors.New("quic: pending control frame storage limit")
	errFramerControlState    = errors.New("quic: original control frame storage is uncertain")
)

// controlFrameMutex owns credit and the original operation prefix. Only failure
// is read atomically by native/readiness observers; wake is an immutable binding
// to the original Conn's buffered sending signal, not a callback or new task.
type framerControlStorage struct {
	owner     *framer
	wake      chan<- struct{}
	bytes     int
	failure   atomic.Uint32
	operation framerControlOperation
}

type framerControlOperation struct {
	active            bool
	input             wire.Frame
	original, growing []queuedFrameEntry
}

func (f *framer) controlFailure() error {
	if f == nil || f.controlStore.owner != f {
		return errFramerControlState
	}
	switch f.controlStore.failure.Load() {
	case 0:
		if f.queuedTooManyControlFrames.Load() {
			return errFramerControlCapacity
		}
		return nil
	case framerControlCapacity:
		return errFramerControlCapacity
	default:
		return errFramerControlState
	}
}

func (f *framer) controlNativeStateError() error {
	if err := f.controlFailure(); err != nil && err != errFramerControlCapacity {
		return err
	}
	return nil // ordinary pressure does not imply an unknown producer lifetime
}

// Caller holds controlFrameMutex. Nonblocking native notification also runs
// from an abnormal producer defer, before the original metadata lock is released.
func (f *framer) failControlLocked(code uint32) {
	r := &f.controlStore
	if code == framerControlUncertain || r.failure.Load() == 0 {
		r.failure.Store(code)
	}
	if code == framerControlCapacity {
		f.queuedTooManyControlFrames.Store(true)
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (f *framer) guardControlLocked(returned *bool) {
	if !*returned {
		f.failControlLocked(framerControlUncertain)
	}
}

func (f *framer) controlShapeReadyLocked() bool {
	r := &f.controlStore
	return r.owner == f && !r.operation.active && r.operation.input == nil &&
		r.operation.original == nil && r.operation.growing == nil &&
		r.bytes >= 0 && r.bytes <= maxFramerControlBytes &&
		len(f.controlFrames) <= maxControlFrames && cap(f.controlFrames) <= maxControlFrames &&
		(len(f.controlFrames) != 0 || r.bytes == 0) &&
		r.failure.Load() <= framerControlCapacity
}

// All generic queue producers, including Append's blocked-frame fallbacks,
// pass this single admission point while holding controlFrameMutex.
func (f *framer) queueControlFrameLocked(frame wire.Frame) bool {
	if f.sealed {
		return false
	}
	if !f.controlShapeReadyLocked() {
		f.failControlLocked(framerControlUncertain)
		return false
	}
	if f.controlFailure() != nil {
		return false
	}
	r := &f.controlStore
	r.operation = framerControlOperation{active: true, input: frame}
	returned := false
	defer f.guardControlLocked(&returned)
	bytes, err := queuedFramePayloadBytes(frame)
	if err != nil {
		return false
	}
	if len(f.controlFrames) == maxControlFrames || bytes > maxFramerControlFrameBytes || bytes > maxFramerControlBytes-r.bytes {
		f.failControlLocked(framerControlCapacity)
		r.operation = framerControlOperation{}
		returned = true
		return false
	}
	r.bytes += bytes // original credit before metadata allocation/adoption
	if len(f.controlFrames) == cap(f.controlFrames) {
		capacity := min(max(16, 2*cap(f.controlFrames)), maxControlFrames)
		r.operation.growing = make([]queuedFrameEntry, len(f.controlFrames), capacity)
		copy(r.operation.growing, f.controlFrames)
		f.controlFrames = r.operation.growing
		r.operation.growing = nil
	}
	f.controlFrames = append(f.controlFrames, queuedFrameEntry{frame: frame, bytes: bytes})
	r.operation = framerControlOperation{}
	returned = true
	return true
}

func (f *framer) appendQueuedControlLocked(frames []ackhandler.Frame, maxLen protocol.ByteCount, v protocol.Version) ([]ackhandler.Frame, protocol.ByteCount, bool) {
	if !f.controlShapeReadyLocked() {
		f.failControlLocked(framerControlUncertain)
		return frames, 0, false
	}
	if f.controlFailure() != nil || len(f.controlFrames) == 0 {
		return frames, 0, false
	}
	r := &f.controlStore
	index := len(f.controlFrames) - 1
	entry := f.controlFrames[index]
	r.operation = framerControlOperation{active: true, input: entry.frame}
	returned := false
	defer f.guardControlLocked(&returned)
	bytes, err := queuedFramePayloadBytes(entry.frame)
	if err != nil || bytes != entry.bytes || bytes > maxFramerControlFrameBytes || bytes > r.bytes {
		return frames, 0, false
	}
	length := entry.frame.Length(v) // known native type; original input held first
	if length < 0 || length > maxFramerControlFrameBytes+128 {
		return frames, 0, false
	}
	if length > maxLen {
		r.operation = framerControlOperation{}
		returned = true
		return frames, 0, false
	}
	frames = append(frames, ackhandler.Frame{Frame: entry.frame})
	f.controlFrames[index] = queuedFrameEntry{} // no hidden sent-frame references
	f.controlFrames = f.controlFrames[:index]
	if len(f.controlFrames) == 0 {
		f.controlFrames = nil
	}
	r.bytes -= entry.bytes
	r.operation = framerControlOperation{}
	returned = true
	return frames, length, true
}

// No payload dereference or frame method under cleanup: only stored credits.
func (f *framer) controlMetadataReadyLocked() bool {
	if !f.controlShapeReadyLocked() {
		return false
	}
	bytes := 0
	for i, entry := range f.controlFrames[:cap(f.controlFrames)] {
		if i >= len(f.controlFrames) {
			if entry.frame != nil || entry.bytes != 0 {
				return false
			}
			continue
		}
		if entry.frame == nil || entry.bytes < 0 || entry.bytes > maxFramerControlFrameBytes ||
			entry.bytes > maxFramerControlBytes-bytes {
			return false
		}
		bytes += entry.bytes
	}
	return bytes == f.controlStore.bytes
}

func keepControlAfter0RTT(frame wire.Frame) bool {
	switch frame.(type) {
	case *wire.MaxDataFrame, *wire.MaxStreamDataFrame, *wire.MaxStreamsFrame,
		*wire.DataBlockedFrame, *wire.StreamDataBlockedFrame, *wire.StreamsBlockedFrame:
		return false
	default:
		return true
	}
}

func (f *framer) filterControl0RTTLocked() bool {
	if !f.controlMetadataReadyLocked() {
		f.failControlLocked(framerControlUncertain)
		return false
	}
	count, bytes := 0, 0
	for _, entry := range f.controlFrames {
		if keepControlAfter0RTT(entry.frame) {
			count++
			bytes += entry.bytes
		}
	}
	if count == len(f.controlFrames) {
		return true
	}
	r := &f.controlStore
	r.operation = framerControlOperation{active: true, original: f.controlFrames}
	returned := false
	defer f.guardControlLocked(&returned)
	if count != 0 {
		r.operation.growing = make([]queuedFrameEntry, count, min(max(16, count), maxControlFrames))
		index := 0
		for _, entry := range r.operation.original {
			if keepControlAfter0RTT(entry.frame) {
				r.operation.growing[index] = entry
				index++
			}
		}
	}
	f.controlFrames = r.operation.growing
	r.bytes = bytes
	r.operation = framerControlOperation{}
	returned = true
	return true // failure latch is never reset by 0RTT filtering
}
