package quic

import (
	"errors"
	"net"
	"sync"

	"github.com/sagernet/quic-go/internal/wire"
)

const (
	maxStreamReceiveOwners = 32768
	maxStreamReceiveBytes  = 32 << 20
)

var (
	errStreamReceiveCapacity  = errors.New("quic: connection STREAM receive capacity exceeded")
	errStreamReceiveUncertain = errors.New("quic: original STREAM receive retirement is uncertain")
)

// One original connection owner covers parser admission, queued/current data
// and trim-copy overlap. Mutex-protected metadata never invokes pool/provider
// callbacks. A negative settlement retains original references and credits.
type streamReceiveBudget struct {
	access       sync.Mutex
	live         map[*receivedStreamData]struct{}
	count        int
	bytes        int
	parserActive bool
	pending      *receivedStreamData
	sealed       bool
	failure      error
	settled      bool
	done         chan struct{}
	onFailure    func()
	calls        *streamCalls
	sent         *streamSendBudget
}

type receivedStreamData struct {
	owner    *streamReceiveBudget
	frame    *wire.StreamFrame
	data     []byte
	charge   int
	started  bool
	returned bool
}

func newStreamReceiveBudget(requestClose func(error), closing <-chan struct{}) *streamReceiveBudget {
	b := &streamReceiveBudget{live: make(map[*receivedStreamData]struct{}), done: make(chan struct{})}
	b.onFailure = func() { requestClose(errStreamReceiveUncertain) }
	b.calls = newStreamCalls(b.markUncertain, func() { requestClose(errStreamCallCapacity) }, closing)
	b.sent = newStreamSendBudget(requestClose, b.markUncertain, closing)
	return b
}

func (b *streamReceiveBudget) settleLocked() {
	if b.sealed && !b.settled && (b.failure != nil || b.count == 0 && !b.parserActive) {
		b.settled = true
		close(b.done)
	}
}

func (b *streamReceiveBudget) seal() {
	b.access.Lock()
	b.sealed = true
	b.settleLocked()
	b.access.Unlock()
}

func (b *streamReceiveBudget) markUncertain() {
	b.access.Lock()
	first := b.failure == nil
	if first {
		b.failure = errStreamReceiveUncertain
	}
	b.sealed = true
	b.settleLocked()
	b.access.Unlock()
	if b.sent != nil {
		b.sent.fail()
	}
	if first && b.onFailure != nil {
		b.onFailure() // existing Conn close notification, outside this metadata lock
	}
}

func (b *streamReceiveBudget) acquire(charge int) (*receivedStreamData, error) {
	b.access.Lock()
	if b.failure != nil {
		b.access.Unlock()
		return nil, errStreamReceiveUncertain
	}
	if b.sealed {
		b.access.Unlock()
		return nil, net.ErrClosed
	}
	if charge < 0 || b.count >= maxStreamReceiveOwners || charge > maxStreamReceiveBytes-b.bytes {
		b.access.Unlock()
		return nil, errStreamReceiveCapacity
	}
	// Opening metadata is counted before object/map/backing allocation. Seal
	// cannot report a positive empty owner while this reservation is opening.
	b.count++
	b.bytes += charge
	b.access.Unlock()
	original := &receivedStreamData{owner: b, charge: charge}
	b.access.Lock()
	b.live[original] = struct{}{}
	b.access.Unlock()
	return original, nil
}

func (b *streamReceiveBudget) ownsFrame(original *receivedStreamData) bool {
	if original == nil || original.owner != b {
		return false
	}
	b.access.Lock()
	_, owned := b.live[original]
	valid := owned && !original.started && original.frame != nil
	b.access.Unlock()
	return valid
}

func (original *receivedStreamData) frameView() *wire.StreamFrame {
	b := original.owner
	b.access.Lock()
	_, owned := b.live[original]
	frame := original.frame
	valid := owned && !original.started && frame != nil
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		panic(errStreamReceiveUncertain)
	}
	return frame // borrowed by the sole current intake owner until transfer
}

func (original *receivedStreamData) release() {
	b := original.owner
	b.access.Lock()
	if original.returned {
		b.access.Unlock()
		return
	}
	_, owned := b.live[original]
	if original.started || !owned {
		b.access.Unlock()
		b.markUncertain()
		panic(errStreamReceiveUncertain)
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
	original.frame, original.data = nil, nil
	original.returned = true
	delete(b.live, original)
	b.count--
	b.bytes -= original.charge
	b.settleLocked()
	b.access.Unlock()
	returned = true
}

func (b *streamReceiveBudget) allocateCopy(size int) (original *receivedStreamData, err error) {
	returned := false
	defer func() {
		if !returned {
			b.markUncertain()
		}
	}()
	original, err = b.acquire(size)
	if err != nil {
		returned = true
		return nil, err
	}
	data := make([]byte, size)
	b.access.Lock()
	original.data = data
	b.access.Unlock()
	returned = true
	return original, nil
}

func (original *receivedStreamData) bufferView() []byte {
	b := original.owner
	b.access.Lock()
	_, owned := b.live[original]
	data, valid := original.data, owned && !original.started && original.frame == nil
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		panic(errStreamReceiveUncertain)
	}
	return data
}

// This joins receive data and the original STREAM parser/dispatch producer.
// It is not a receipt for arbitrary public control callers or whole QUIC/OS
// retirement. HY2 also joins its existing TCP and HTTP/3 call/task owners.
func (c *Conn) WaitForStreamReceiveDataShutdown() error {
	<-c.ctx.Done()
	b := c.receiveFrames
	if b == nil {
		return errStreamReceiveUncertain
	}
	b.seal()
	<-b.done
	b.access.Lock()
	err := b.failure
	if err == nil && (b.count != 0 || b.parserActive || b.pending != nil) {
		err = errStreamReceiveUncertain
	}
	b.access.Unlock()
	return err
}
