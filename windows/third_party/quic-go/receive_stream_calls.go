package quic

import (
	"errors"
	"net"
	"sync"
)

const (
	maxStreamDataCalls         = 512
	maxStreamControlCalls      = 64
	maxStreamDirectionCalls    = 8
	maxStreamDirectionControls = 2
)

var errStreamCallCapacity = errors.New("quic: stream operation capacity exceeded")

type streamCallKind uint8

const (
	receiveReadCall streamCallKind = iota
	receiveControlCall
	receiveNativeCall
	sendWriteCall
	sendControlCall
	sendNativeCall
)

func (kind streamCallKind) native() bool { return kind == receiveNativeCall || kind == sendNativeCall }
func (kind streamCallKind) control() bool {
	return kind == receiveControlCall || kind == sendControlCall
}
func (kind streamCallKind) slot() int {
	if kind.control() {
		return 1
	}
	return 0
}

// This original owner precedes the connection loop and public stream exposure.
// One registry covers both directions with separate admission and receipts.
// Its mutex protects call metadata only, including each stream's callCounts;
// it never waits for a stream mutex, read gate or foreign callback.
type streamCalls struct {
	access      sync.Mutex
	live        map[*streamCall]struct{}
	counts      [6]int
	running     [6]int
	loopActive  bool
	sealed      bool
	settled     bool
	allSettled  bool
	failure     error
	cause       error
	closing     chan struct{}
	connClosing <-chan struct{}
	done        chan struct{}
	allDone     chan struct{}
	onFailure   func()
	onCapacity  func()

	framer         *framer // actual original, bound before Conn exposure
	framerRetiring bool
	framerRetired  bool
	framerFailure  error // separate from the receive-only registry result
}

type streamCall struct {
	owner    *streamCalls
	stream   any // original *ReceiveStream or *SendStream, retained on abnormal return
	counts   *[2]int
	kind     streamCallKind
	ended    bool
	returned bool
}

func newStreamCalls(onFailure, onCapacity func(), connClosing <-chan struct{}) *streamCalls {
	return &streamCalls{
		live: make(map[*streamCall]struct{}), loopActive: true,
		closing: make(chan struct{}), done: make(chan struct{}), allDone: make(chan struct{}), connClosing: connClosing,
		onFailure: onFailure, onCapacity: onCapacity,
	}
}

func (o *streamCalls) settleLocked() {
	if o.sealed && !o.loopActive {
		if !o.hasRunning(3) && !o.settled {
			o.settled = true
			close(o.done)
		}
		if !o.hasRunning(6) && !o.allSettled && (o.failure != nil || o.framerFailure != nil || o.framerRetired) {
			o.allSettled = true
			close(o.allDone)
		}
	}
}

// Caller holds access. The first three classes preserve the receive-only API.
func (o *streamCalls) hasRunning(end int) bool {
	for _, count := range o.running[:end] {
		if count != 0 {
			return true
		}
	}
	return false
}

func (o *streamCalls) sealLocked(cause error) {
	if o.cause == nil {
		o.cause = cause
	}
	if !o.sealed {
		o.sealed = true
		close(o.closing)
	}
	o.settleLocked()
}

func (o *streamCalls) seal(cause error) {
	o.access.Lock()
	o.sealLocked(cause)
	o.access.Unlock()
}

func (o *streamCalls) stoppedErrorLocked() error {
	// First-close publication precedes its metadata seal. A public caller must
	// not enter that gap merely because the closing goroutine was preempted.
	select {
	case <-o.connClosing:
		o.sealLocked(nil)
	default:
	}
	if o.failure != nil {
		return errStreamReceiveUncertain
	}
	if o.sealed {
		if o.cause != nil {
			return o.cause
		}
		return net.ErrClosed
	}
	return nil
}

func (o *streamCalls) stoppedError() error {
	o.access.Lock()
	err := o.stoppedErrorLocked()
	o.access.Unlock()
	return err
}

func (s *ReceiveStream) beginReceiveCall(kind streamCallKind) (*streamCall, error) {
	b := s.frameIntake.budget
	if kind > receiveNativeCall {
		return nil, errStreamReceiveUncertain
	}
	return b.beginStreamCall(s, &s.callCounts, kind)
}

func (b *streamReceiveBudget) beginStreamCall(stream any, counts *[2]int, kind streamCallKind) (*streamCall, error) {
	if b == nil || b.calls == nil || stream == nil || counts == nil || kind > sendNativeCall {
		return nil, errStreamReceiveUncertain
	}
	o := b.calls
	o.access.Lock()
	if kind.native() {
		// Only the original serialized connection loop owns native dispatch and
		// teardown. Public close must not prevent that loop from waking readers.
		if !o.loopActive {
			o.access.Unlock()
			return nil, net.ErrClosed
		}
	} else if err := o.stoppedErrorLocked(); err != nil {
		o.access.Unlock()
		return nil, err
	}
	limit := maxStreamDataCalls
	if kind.control() {
		limit = maxStreamControlCalls
	} else if kind.native() {
		limit = 1
	}
	full := o.counts[kind] >= limit
	if kind.native() {
		full = o.counts[receiveNativeCall]+o.counts[sendNativeCall] >= limit
	} else {
		streamLimit := maxStreamDirectionCalls
		if kind.control() {
			streamLimit = maxStreamDirectionControls
		}
		full = full || counts[kind.slot()] >= streamLimit
	}
	if full {
		o.sealLocked(errStreamCallCapacity)
		o.access.Unlock()
		if o.onCapacity != nil {
			o.onCapacity() // existing close notification; no stream lock or wait
		}
		return nil, errStreamCallCapacity
	}
	// Opening counts precede token/map allocation and both blocking gates.
	o.counts[kind]++
	o.running[kind]++
	if !kind.native() {
		counts[kind.slot()]++
	}
	o.access.Unlock()
	opened := false
	defer func() {
		if !opened {
			o.finishOpening(kind)
		}
	}()
	original := &streamCall{owner: o, stream: stream, counts: counts, kind: kind}
	o.access.Lock()
	o.live[original] = struct{}{}
	o.access.Unlock()
	opened = true
	return original, nil
}

// An abnormal opening retains its reserved credits. It is not a normal call
// return or authority to reuse the stream, even if no token could be published.
func (o *streamCalls) finishOpening(kind streamCallKind) {
	o.access.Lock()
	first := o.failure == nil
	o.failure = errStreamReceiveUncertain
	o.sealLocked(o.failure)
	o.access.Unlock()
	defer o.endRunning(kind)
	if first && o.onFailure != nil {
		o.onFailure()
	}
}

func (o *streamCalls) endRunning(kind streamCallKind) {
	o.access.Lock()
	o.running[kind]--
	o.settleLocked()
	o.access.Unlock()
}

func (call *streamCall) finish(returned *bool) {
	o := call.owner
	o.access.Lock()
	if call.ended {
		ok := call.returned
		o.access.Unlock()
		if !ok {
			panic(errStreamReceiveUncertain)
		}
		return
	}
	_, owned := o.live[call]
	if !owned {
		o.access.Unlock()
		panic(errStreamReceiveUncertain)
	}
	call.ended, call.returned = true, *returned
	firstFailure := !*returned && o.failure == nil
	if !*returned {
		o.failure = errStreamReceiveUncertain
		o.sealLocked(o.failure)
	}
	o.access.Unlock()
	// Keep the original invocation running through its failure notification.
	// Negative credits/references remain held after the actual call ends.
	defer func() {
		o.access.Lock()
		if call.returned {
			o.counts[call.kind]--
			if !call.kind.native() {
				call.counts[call.kind.slot()]--
			}
			call.counts = nil
			call.stream = nil
			delete(o.live, call)
		}
		o.running[call.kind]--
		o.settleLocked()
		retire := o.sealed && !o.loopActive && !o.hasRunning(6)
		o.access.Unlock()
		if retire {
			o.retireFramerIfReady() // no extra registry lock on ordinary active calls
		}
	}()
	if firstFailure && o.onFailure != nil {
		o.onFailure()
	}
}

func (o *streamCalls) finishLoop(returned *bool) {
	o.access.Lock()
	firstFailure := !*returned && o.failure == nil
	if !*returned {
		o.failure = errStreamReceiveUncertain
	}
	o.loopActive = false
	o.sealLocked(o.failure)
	o.access.Unlock()
	if firstFailure && o.onFailure != nil {
		o.onFailure()
	}
	o.retireFramerIfReady()
}

func (o *streamCalls) wait() error {
	<-o.done
	o.access.Lock()
	err := o.failure
	if err == nil && (o.hasRunning(3) || o.loopActive || o.counts[0] != 0 || o.counts[1] != 0 || o.counts[2] != 0) {
		err = errStreamReceiveUncertain
	}
	o.access.Unlock()
	return err
}

func (o *streamCalls) waitAll() error {
	<-o.allDone
	o.access.Lock()
	err := errors.Join(o.failure, o.framerFailure)
	if err == nil && (o.hasRunning(6) || o.loopActive || len(o.live) != 0 || o.counts != [6]int{} || !o.framerRetired || o.framerRetiring) {
		err = errStreamReceiveUncertain
	}
	o.access.Unlock()
	return err
}

// Original registered receive/send calls, their loop and sealed framer metadata
// are joined. Transmitted frame/ACK/loss/history and arbitrary providers remain separate.
func (c *Conn) WaitForStreamCallsShutdown() error {
	<-c.ctx.Done()
	if c.receiveFrames == nil || c.receiveFrames.calls == nil {
		return errStreamReceiveUncertain
	}
	return c.receiveFrames.calls.waitAll()
}

// This composes original receive data/parser and Read/Peek/CancelRead/deadline
// plus native receive/control/shutdown calls. It does not join send-side calls,
// raw/ACK/crypto/history storage, arbitrary providers or whole QUIC/OS work.
func (c *Conn) WaitForStreamReceiveShutdown() error {
	<-c.ctx.Done()
	b := c.receiveFrames
	if b == nil || b.calls == nil {
		return errStreamReceiveUncertain
	}
	return errors.Join(b.calls.wait(), c.WaitForStreamReceiveDataShutdown())
}
