package quic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/utils/ringbuffer"
	"github.com/sagernet/quic-go/internal/wire"
)

const (
	maxDatagramSendQueueLen = 32
	maxDatagramRcvQueueLen  = 128
	maxDatagramSendOwned    = 4096
	maxDatagramSendBytes    = 8 << 20
	maxDatagramReceiveBytes = 2 << 20
)

// Local retained-storage pressure, not peer delivery or connection failure.
// Refusal makes neither a payload copy nor an internal waiting task.
var ErrDatagramSendCapacity = errors.New("QUIC DATAGRAM retained storage limit")

type datagramQueue struct {
	sendMx               sync.Mutex
	sendQueue            ringbuffer.RingBuffer[*datagramSendEntry]
	sent                 chan struct{} // used to notify Add that a datagram was dequeued
	sendOwned            int
	sendBytes            int
	sendPoisoned         bool
	sendLive             map[*datagramSendEntry]struct{}
	sendProducers        int
	sendProducersDone    chan struct{}
	sendProducersSettled bool
	sendRetirement       datagramSendRetirement
	scopeOwned           int // open and sealed scopes whose original calls still run
	scopePoisoned        bool
	peerParams           *atomic.Pointer[peerParameterSnapshot] // bound once before exposure
	peerChanged          chan struct{}                          // sendMx; broadcast every outbound-head change

	rcvMx    sync.Mutex
	rcvQueue ringbuffer.RingBuffer[[]byte]
	rcvBytes int
	rcvd     chan struct{} // used to notify Receive that a new datagram was received

	closeErr error
	closed   chan struct{}

	hasData func()

	logger utils.Logger
}

func newDatagramQueue(hasData func(), logger utils.Logger) *datagramQueue {
	h := &datagramQueue{
		hasData:           hasData,
		rcvd:              make(chan struct{}, 1),
		sent:              make(chan struct{}, 1),
		closed:            make(chan struct{}),
		logger:            logger,
		peerChanged:       make(chan struct{}),
		sendLive:          make(map[*datagramSendEntry]struct{}),
		sendProducersDone: make(chan struct{}),
		sendRetirement:    datagramSendRetirement{done: make(chan struct{})},
	}
	h.sendQueue.Init(maxDatagramSendQueueLen)
	h.rcvQueue.Init(maxDatagramRcvQueueLen)
	return h
}

func datagramContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return context.Canceled
}

// Only captured channels and internal fields are consulted under the lock;
// custom Context methods cannot reenter queue cleanup while metadata is held.
func (h *datagramQueue) sendErrorLocked(done <-chan struct{}) (error, bool) {
	select {
	case <-h.closed:
		return h.closeErr, false
	default:
	}
	select {
	case <-done:
		return nil, true
	default:
		return nil, false
	}
}

// AddCopy reserves count and bytes before cloning the caller's payload.
// Producer and queue/history hold separate refs; Pop never refunds history.
func (h *datagramQueue) AddCopy(ctx context.Context, data []byte) error {
	return h.addCopy(ctx, nil, data)
}

func (h *datagramQueue) addCopy(ctx context.Context, scope *DatagramSendScope, data []byte) error {
	if ctx == nil {
		return errors.New("nil DATAGRAM context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	done := ctx.Done()
	if _, err, revoked := h.lockAdmission(done, scope); err != nil || revoked {
		if revoked {
			return datagramContextError(ctx)
		}
		return err
	}
	if err := h.peerDatagramErrorLocked(len(data)); err != nil {
		h.unlockAdmission(scope)
		return err
	}
	entry, err := h.reserveSendProducerLocked(len(data))
	h.unlockAdmission(scope)
	if err != nil {
		return err
	}
	returned := false
	defer h.finishSendProducer(entry, &returned)
	err = h.enqueueCopy(ctx, done, scope, data, entry)
	returned = true
	return err
}

// The registered producer retains its original entry while native copy,
// Context/deadline and scheduling work execute outside metadata locks.
func (h *datagramQueue) enqueueCopy(ctx context.Context, done <-chan struct{}, scope *DatagramSendScope, data []byte, entry *datagramSendEntry) error {
	entry.frame.DataLenPresent = true
	entry.frame.SetOwnedData(make([]byte, len(data)))
	copy(entry.frame.Data, data)
	var deadlineTimer *time.Timer
	defer func() {
		if deadlineTimer != nil {
			deadlineTimer.Stop()
		}
	}()
	for {
		state, err, revoked := h.lockAdmission(done, scope)
		if err != nil || revoked {
			if revoked {
				return datagramContextError(ctx)
			}
			return err
		}
		if err := h.peerDatagramErrorLocked(entry.size); err != nil {
			h.unlockAdmission(scope)
			return err
		}
		if h.sendQueue.Len() < maxDatagramSendQueueLen {
			entry.refs++ // grant queue ownership; the producer keeps its own ref
			h.sendQueue.PushBack(entry)
			h.unlockAdmission(scope)
			h.hasData()
			return nil
		}
		select {
		case <-h.sent: // drain the queue so we don't loop immediately
		default:
		}
		peerChanged := h.peerChanged
		h.unlockAdmission(scope)
		var deadlineC <-chan time.Time
		if deadlineTimer != nil && !deadlineTimer.Stop() {
			select {
			case <-deadlineTimer.C:
			default:
			}
		}
		if !state.deadline.IsZero() {
			remaining := monotime.Until(state.deadline)
			if remaining <= 0 {
				continue // recheck the current deadline under the admission fence
			}
			if deadlineTimer == nil {
				deadlineTimer = time.NewTimer(remaining)
			} else {
				deadlineTimer.Reset(remaining)
			}
			deadlineC = deadlineTimer.C
		}
		select {
		case <-h.closed:
			return h.closeErr
		case <-done:
			return datagramContextError(ctx)
		case <-h.sent:
		case <-state.changed:
		case <-peerChanged:
		case <-deadlineC:
			// A deadline may have been extended/cleared; never return on a stale tick.
		}
	}
}

// Every Peek grants an explicit packer borrower, released with ReleasePeek.
// Queue Close cannot retire backing while that original borrower still reads it.
func (h *datagramQueue) Peek() *datagramSendEntry {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	select {
	case <-h.closed:
		return nil
	default:
	}
	for !h.sendQueue.Empty() {
		if h.peerParams != nil {
			params := h.peerParams.Load()
			if params == nil || params.phase == peerParametersRejected0RTT {
				return nil // retain unsent owners until actual fresh apply or Close
			}
		}
		entry := h.sendQueue.PeekFront()
		if h.peerDatagramErrorLocked(entry.size) == nil {
			entry.refs++
			return entry
		}
		// Only immutable size is read here; a previous packetizer borrower may
		// still own the frame's backing. Drop just this queue owner/credit.
		_ = h.sendQueue.PopFront()
		h.releaseEntryLocked(entry)
		select {
		case h.sent <- struct{}{}:
		default:
		}
	}
	return nil
}

func (h *datagramQueue) ReleasePeek(entry *datagramSendEntry) {
	h.sendMx.Lock()
	h.releaseEntryLocked(entry)
	h.sendMx.Unlock()
}

// Pop moves the queue owner to packet/history or drops it. It does not revoke
// the packer borrower, and a concurrent Close can make the transfer fail.
func (h *datagramQueue) Pop(entry *datagramSendEntry, transfer bool) bool {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	if h.sendQueue.Empty() || h.sendQueue.PeekFront() != entry {
		return false
	}
	if transfer && entry.history {
		h.poisonSendLocked()
		return false
	}
	if transfer && h.peerDatagramErrorLocked(entry.size) != nil {
		return false // preserve queue owner and the original Peek borrower
	}
	_ = h.sendQueue.PopFront()
	if transfer {
		entry.history = true
	} else {
		h.releaseEntryLocked(entry)
	}
	select {
	case h.sent <- struct{}{}:
	default:
	}
	return true
}

// HandleDatagramFrame handles a received DATAGRAM frame.
func (h *datagramQueue) HandleDatagramFrame(f *wire.DatagramFrame) {
	var queued bool
	h.rcvMx.Lock()
	select {
	case <-h.closed:
	default:
		if h.rcvQueue.Len() < maxDatagramRcvQueueLen && len(f.Data) <= maxDatagramReceiveBytes-h.rcvBytes {
			data := make([]byte, len(f.Data))
			copy(data, f.Data)
			h.rcvQueue.PushBack(data)
			h.rcvBytes += len(data)
			queued = true
			select {
			case h.rcvd <- struct{}{}:
			default:
			}
		}
	}
	h.rcvMx.Unlock()
	if !queued && h.logger != nil && h.logger.Debug() {
		h.logger.Debugf("Discarding received DATAGRAM frame (%d bytes payload)", len(f.Data))
	}
}

// Receive gets a received DATAGRAM frame.
func (h *datagramQueue) Receive(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("nil DATAGRAM context")
	}
	done := ctx.Done()
	for {
		h.rcvMx.Lock()
		select {
		case <-done:
			h.rcvMx.Unlock()
			return nil, datagramContextError(ctx)
		default:
		}
		if !h.rcvQueue.Empty() {
			data := h.rcvQueue.PopFront()
			h.rcvBytes -= len(data)
			h.rcvMx.Unlock()
			return data, nil
		}
		h.rcvMx.Unlock()
		select {
		case <-h.rcvd:
			continue
		case <-h.closed:
			return nil, h.closeErr
		case <-done:
			return nil, datagramContextError(ctx)
		}
	}
}

func (h *datagramQueue) CloseWithError(e error) {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	h.rcvMx.Lock()
	defer h.rcvMx.Unlock()
	select {
	case <-h.closed:
		return
	default:
	}
	if e == nil {
		e = errors.New("QUIC DATAGRAM queue closed")
	}
	h.closeErr = e
	close(h.closed)
	for !h.sendQueue.Empty() {
		h.releaseEntryLocked(h.sendQueue.PopFront())
	}
	h.rcvQueue.Clear()
	h.rcvBytes = 0
	h.settleSendProducersLocked()
	// Pending producers, packer borrowers and history keep their own credits.
	// This is not whole-connection/socket/history retirement.
}
