package http3

import (
	"context"
	"errors"

	"github.com/sagernet/quic-go"
)

type streamRegistryEntry struct {
	id     quic.StreamID
	stream *stateTrackingStream
}

type streamRegistryStage uint8

const (
	registryNativeJoining streamRegistryStage = iota + 1
	registryNativeSnapshot
	registryNativeSides
	registryRegisteredJoin
	registryRetirementDone
)

// One external finalizer and retained prefix per original rawConn, not per Wait
// caller. Result publication uses streamMx. Only that finalizer accesses its
// fixed entries/stage/index before done; no worker or cancellation receipt.
type streamRegistryRetirement struct {
	original     *quic.Conn
	done         chan struct{}
	ended        bool
	result       error
	nativeReady  bool
	nativeCause  error
	stage        streamRegistryStage
	entries      [maxTrackedStreams]streamRegistryEntry
	count, index int
}

func (c *rawConn) registryRetirementCleanLocked(r *streamRegistryRetirement) bool {
	return c.streamRetirement == r && r.original != nil && c.conn == r.original && r.nativeReady &&
		c.streamAdmissionClosed && c.streams != nil && len(c.streams) == 0 &&
		c.streamClearOriginal == nil && c.streamRegistryFailure == nil
}

func (c *rawConn) retireStreamRegistry() (result error) {
	c.streamMx.Lock()
	c.streamAdmissionClosed = true
	if r := c.streamRetirement; r != nil {
		done := r.done
		c.streamMx.Unlock()
		<-done // join the original finalizer, never repeat its native/metadata work
		c.streamMx.Lock()
		defer c.streamMx.Unlock()
		if !r.ended || r.result == nil && (!c.registryRetirementCleanLocked(r) ||
			r.stage != registryRetirementDone || r.count != 0 || r.index != 0 ||
			r.nativeCause != nil ||
			r.entries != [maxTrackedStreams]streamRegistryEntry{}) {
			c.failStreamRegistryLocked()
			return errors.Join(errStreamTrackingUncertain, r.result)
		}
		return r.result
	}
	r := &streamRegistryRetirement{original: c.conn, done: make(chan struct{}), stage: registryNativeJoining}
	c.streamRetirement = r // original owner exists before the native join
	c.streamMx.Unlock()
	returned := false
	defer func() {
		c.streamMx.Lock()
		defer c.streamMx.Unlock()
		if returned && result == nil && !c.registryRetirementCleanLocked(r) {
			result = errStreamTrackingUncertain
		}
		if !returned || result != nil {
			c.failStreamRegistryLocked()
			result = errors.Join(errStreamTrackingUncertain, result)
		} else {
			r.entries = [maxTrackedStreams]streamRegistryEntry{}
			r.nativeCause = nil
			r.count, r.index, r.stage = 0, 0, registryRetirementDone
		}
		r.result, r.ended = result, true
		close(r.done)
	}()
	if r.original == nil {
		returned = true
		return errStreamTrackingUncertain
	}
	// This is the same original aggregate used by HY2's native close owner.
	// It joins real native map/call/data receipts, not just Closing or Context.
	if err := r.original.WaitForStreamMapRetirement(); err != nil {
		returned = true
		return errors.Join(errStreamTrackingUncertain, err)
	}
	// Preserve the original terminal cause. Context inspection stays outside
	// metadata locks, and a missing cause cannot authorize local completion.
	r.nativeCause = context.Cause(r.original.Context())
	if r.nativeCause == nil {
		returned = true
		return errStreamTrackingUncertain
	}
	if err := c.captureNativeStreamRegistry(r); err != nil {
		returned = true
		return err
	}
	r.stage = registryNativeSides
	for r.index < r.count {
		if err := c.retireNativeStreamEntry(r, r.entries[r.index]); err != nil {
			returned = true
			return err
		}
		r.index++
	}
	r.stage = registryRegisteredJoin
	c.qloggerWG.Wait()
	c.streamMx.Lock()
	clean := c.registryRetirementCleanLocked(r)
	c.streamMx.Unlock()
	returned = true
	if !clean {
		return errStreamTrackingUncertain
	}
	return nil
}

func (c *rawConn) captureNativeStreamRegistry(r *streamRegistryRetirement) error {
	c.streamMx.Lock()
	defer c.streamMx.Unlock()
	if c.streamRetirement != r || r.original != c.conn || !c.streamAdmissionClosed ||
		r.ended || r.nativeReady || r.stage != registryNativeJoining || r.count != 0 || r.index != 0 ||
		r.nativeCause == nil ||
		c.streams == nil || len(c.streams) > maxTrackedStreams ||
		c.streamClearOriginal != nil || c.streamRegistryFailure != nil {
		return errStreamTrackingUncertain
	}
	r.stage = registryNativeSnapshot
	for id, s := range c.streams {
		if s == nil {
			return errStreamTrackingUncertain
		}
		for i := 0; i < r.count; i++ {
			if r.entries[i].stream == s {
				return errStreamTrackingUncertain
			}
		}
		r.entries[r.count] = streamRegistryEntry{id: id, stream: s}
		r.count++
	}
	r.nativeReady = true // published only after original receipt + full snapshot
	return nil
}

func (c *rawConn) retireNativeStreamEntry(r *streamRegistryRetirement, entry streamRegistryEntry) error {
	s := entry.stream
	// Preserve the established stream -> parent lock order. No native/provider,
	// Context, onStreamsEmpty or payload/pool operation runs under these locks.
	s.mx.Lock()
	defer s.mx.Unlock()
	c.streamMx.Lock()
	defer c.streamMx.Unlock()
	if c.streamRetirement != r || !r.nativeReady || r.ended || r.original != c.conn ||
		r.nativeCause == nil ||
		!c.streamAdmissionClosed || c.streamRegistryFailure != nil || c.streamClearOriginal != nil ||
		r.index < 0 || r.index >= r.count || r.entries[r.index] != entry ||
		s.clearer != c || s.Stream == nil {
		return errStreamTrackingUncertain
	}
	current, exists := c.streams[entry.id]
	if !exists {
		if s.sendErr == nil || s.recvErr == nil {
			return errStreamTrackingUncertain
		}
		return nil // original ordinary clear already removed this exact wrapper
	}
	if current != s {
		return errStreamTrackingUncertain
	}
	if s.sendErr == nil {
		s.sendErr = r.nativeCause
	}
	if s.recvErr == nil {
		s.recvErr = r.nativeCause
		if s.receiveDone != nil {
			close(s.receiveDone)
		}
		s.signalHasDatagram()
	}
	// The underlying owner actually retired. Preserve queued data for external
	// drain; remove only this map reference/WG charge. No GOAWAY re-close or idle
	// timer callback is needed after terminal native completion.
	delete(c.streams, entry.id)
	c.qloggerWG.Done()
	return nil
}
