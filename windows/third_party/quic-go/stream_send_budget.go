package quic

import (
	"errors"
	"net"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

const (
	maxStreamSendOwners = 32768
	maxStreamSendBytes  = 32 << 20
)

var (
	errStreamSendCapacity  = errors.New("quic: connection STREAM send capacity exceeded")
	errStreamSendUncertain = errors.New("quic: original STREAM send retirement is uncertain")
)

type streamSendBudget struct {
	access                sync.Mutex
	live                  map[*sentStreamData]struct{}
	count, bytes          int
	sealed                bool
	failure               error
	closing               <-chan struct{}
	requestClose          func(error)
	onFailure             func()
	finalizing, finalized bool
	done                  chan struct{}
}

type sentStreamData struct {
	owner             *streamSendBudget
	stream            *SendStream
	frame             *wire.StreamFrame
	attempt           *streamSendAttempt
	charge            int
	started, returned bool
}

func newStreamSendBudget(requestClose func(error), onFailure func(), closing <-chan struct{}) *streamSendBudget {
	return &streamSendBudget{live: make(map[*sentStreamData]struct{}), closing: closing,
		requestClose: requestClose, onFailure: onFailure, done: make(chan struct{})}
}

func (b *streamSendBudget) seal() {
	b.access.Lock()
	b.sealed = true
	b.access.Unlock()
}

// Restriction-only propagation from the original Conn owner, without recursion.
func (b *streamSendBudget) fail() {
	b.access.Lock()
	b.failure, b.sealed = errStreamSendUncertain, true
	b.access.Unlock()
}

func (b *streamSendBudget) markUncertain() {
	b.access.Lock()
	first := b.failure == nil
	b.failure, b.sealed = errStreamSendUncertain, true
	b.access.Unlock()
	if first {
		b.onFailure()
	}
}

func (b *streamSendBudget) allocate(stream *SendStream, pooled bool) (*sentStreamData, error) {
	charge := 0
	if pooled {
		charge = protocol.MaxPacketBufferSize
	}
	b.access.Lock()
	select {
	case <-b.closing:
		b.sealed = true
	default:
	}
	if b.failure != nil {
		b.access.Unlock()
		return nil, errStreamSendUncertain
	}
	if b.sealed {
		b.access.Unlock()
		return nil, net.ErrClosed
	}
	if b.count >= maxStreamSendOwners || charge > maxStreamSendBytes-b.bytes {
		b.access.Unlock()
		b.requestClose(errStreamSendCapacity)
		return nil, errStreamSendCapacity
	}
	// Reserve before holder/map/frame/pool allocation, including split overlap.
	b.count++
	b.bytes += charge
	b.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			b.markUncertain()
		}
	}()
	original := &sentStreamData{owner: b, stream: stream, charge: charge}
	b.access.Lock()
	b.live[original] = struct{}{}
	b.access.Unlock()
	var frame *wire.StreamFrame
	if pooled {
		frame = wire.GetStreamFrame()
	} else {
		frame = &wire.StreamFrame{}
	}
	b.access.Lock()
	original.frame = frame
	b.access.Unlock()
	frame.StreamID, frame.Offset = stream.streamID, stream.writeOffset
	frame.Fin, frame.DataLenPresent = false, true
	frame.Data = frame.Data[:0]
	returned = true
	return original, nil
}

func (original *sentStreamData) raw() *wire.StreamFrame {
	b := original.owner
	b.access.Lock()
	_, owned := b.live[original]
	frame := original.frame
	valid := owned && !original.started && frame != nil
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		panic(errStreamSendUncertain)
	}
	return frame
}

func (original *sentStreamData) release() { original.retire(false) }

func (original *sentStreamData) retire(quiescent bool) {
	b := original.owner
	b.access.Lock()
	if original.returned {
		b.access.Unlock()
		return
	}
	_, owned := b.live[original]
	if !owned || original.started || original.frame == nil || original.attempt != nil && !quiescent {
		b.access.Unlock()
		b.markUncertain()
		panic(errStreamSendUncertain)
	}
	if original.attempt != nil {
		original.attempt.closed = true
		original.attempt.loan = nil
		original.attempt = nil
	}
	original.started = true
	frame := original.frame
	b.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			b.markUncertain()
		}
	}()
	if frame != nil {
		frame.PutBack()
	}
	b.access.Lock()
	original.frame, original.stream = nil, nil
	original.returned = true
	delete(b.live, original)
	b.count--
	b.bytes -= original.charge
	b.access.Unlock()
	returned = true
}

// Only the external original owner invokes this after a positive loop+all-call
// quiescence receipt. Private packer/history aliases then have no producer left.
// On uncertainty the exact remaining originals are retained, never swept/retried.
func (b *streamSendBudget) finishQuiescent(callErr error) error {
	b.access.Lock()
	if b.finalizing || b.finalized {
		done := b.done
		b.access.Unlock()
		<-done
		b.access.Lock()
		err := b.failure
		b.access.Unlock()
		return err
	}
	b.finalizing, b.sealed = true, true
	if callErr != nil {
		b.failure = errStreamSendUncertain
	}
	b.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			b.markUncertain()
		}
		b.access.Lock()
		b.finalizing, b.finalized = false, true
		close(b.done)
		b.access.Unlock()
	}()
	b.access.Lock()
	if b.failure == nil && (b.count != len(b.live) || b.count > maxStreamSendOwners) {
		b.failure = errStreamSendUncertain
	}
	err := b.failure
	var originals []*sentStreamData
	if err == nil {
		// One bounded pointer snapshot avoids repeated sparse-map scans during
		// teardown. Payload remains with its original counted owner throughout.
		originals = make([]*sentStreamData, 0, len(b.live))
		for original := range b.live {
			originals = append(originals, original)
		}
	}
	b.access.Unlock()
	if err != nil {
		returned = true
		return err
	}
	for _, original := range originals {
		b.access.Lock()
		err = b.failure
		b.access.Unlock()
		if err != nil {
			returned = true
			return err
		}
		original.retire(true)
	}
	b.access.Lock()
	if b.failure == nil && (b.count != 0 || b.bytes != 0 || len(b.live) != 0) {
		b.failure = errStreamSendUncertain
	}
	err = b.failure
	b.access.Unlock()
	returned = true
	return err
}

// Includes residual current/queued/history/known-unsent STREAM data only after
// the original loop and all registered stream calls positively quiesced. This
// is not raw packet/ACK metadata/crypto/global/whole QUIC or OS retirement.
func (c *Conn) WaitForStreamSendDataShutdown() error {
	<-c.ctx.Done()
	if c.receiveFrames == nil || c.receiveFrames.sent == nil {
		return errStreamSendUncertain
	}
	err := c.WaitForStreamCallsShutdown()
	return errors.Join(err, c.receiveFrames.sent.finishQuiescent(err))
}
