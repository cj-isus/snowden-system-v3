package quic

import (
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
)

func (h *sendQueue) notifyLocked() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
	select {
	case h.available <- struct{}{}:
	default:
	}
}

func (h *sendQueue) failLocked() {
	h.failed, h.sealed = true, true
	h.notifyLocked()
}

func (h *sendQueue) fail() {
	h.access.Lock()
	h.failLocked()
	h.access.Unlock()
}

func (h *sendQueue) stop(cause error) {
	h.access.Lock()
	if h.cause == nil && cause != nil {
		h.cause = cause
	}
	h.sealed = true
	h.notifyLocked()
	h.access.Unlock()
}

func validPacketBatch(batch packetSendBatch) bool {
	if batch.original == nil || batch.count < 0 || batch.count > len(batch.datagrams) {
		return false
	}
	capacity := cap(batch.original.Data)
	if capacity != protocol.MaxPacketBufferSize && capacity != protocol.MaxLargePacketBufferSize {
		return false
	}
	for _, entry := range batch.datagrams[:batch.count] {
		if entry == nil {
			return false
		}
	}
	return true
}

// Caller owns the input before this fence. Fixed slots are reserved before copy;
// no provider, pool call or foreign error method runs with access held.
func (h *sendQueue) admit(batch packetSendBatch, gso uint16, ecn protocol.ECN, probe bool) (*queueEntry, error) {
	h.access.Lock()
	defer h.access.Unlock()
	if !validPacketBatch(batch) {
		h.failLocked()
		return nil, errPacketOutputUncertain
	}
	if h.failed {
		return nil, errPacketOutputUncertain
	}
	if h.sealed {
		return nil, net.ErrClosed
	}
	var e *queueEntry
	if probe {
		if h.probeRunning || h.probe.used {
			return nil, errSendQueueCapacity
		}
		e = &h.probe
	} else {
		if h.pending == sendQueueCapacity {
			return nil, errSendQueueCapacity
		}
		for i := range h.entries {
			if !h.entries[i].used {
				e = &h.entries[i]
				break
			}
		}
		if e == nil {
			h.failLocked()
			return nil, errPacketOutputUncertain
		}
	}
	e.used = true
	e.packetSendBatch, e.gsoSize, e.ecn = batch, gso, ecn
	if probe {
		h.probeRunning = true
		return e, nil
	}
	tail := (h.head + h.pending) % sendQueueCapacity
	if h.queue[tail] != nil {
		h.failLocked() // keep the old ring original and newly captured input
		return nil, errPacketOutputUncertain
	}
	h.queue[tail] = e
	h.pending++
	select {
	case h.wake <- struct{}{}:
	default:
	}
	if h.pending == sendQueueCapacity {
		select {
		case <-h.available:
		default:
		}
	}
	return e, nil
}

func (h *sendQueue) start() error {
	h.access.Lock()
	defer h.access.Unlock()
	if h.started {
		if h.settled {
			return net.ErrClosed
		}
		h.failLocked()
		return errPacketOutputUncertain
	}
	h.started = true
	return nil
}

func (h *sendQueue) take() (*queueEntry, bool, bool, error) {
	h.access.Lock()
	defer h.access.Unlock()
	if h.failed {
		return nil, false, true, errPacketOutputUncertain
	}
	if h.pending == 0 {
		return nil, false, h.sealed, h.cause
	}
	e := h.queue[h.head]
	if e == nil || !e.used || h.active != nil {
		h.failLocked()
		return nil, false, true, errPacketOutputUncertain
	}
	h.active = e // original root precedes removal from the ring
	h.queue[h.head] = nil
	h.head = (h.head + 1) % sendQueueCapacity
	h.pending--
	h.notifyLocked()
	return e, h.cause != nil, false, nil
}

// Only the original worker/probe owns e until its actual pool call returns.
func (h *sendQueue) release(e *queueEntry, unsent bool) error {
	h.access.Lock()
	if e == nil || !e.used || e.releasing || h.failed {
		h.failLocked()
		h.access.Unlock()
		return errPacketOutputUncertain
	}
	e.releasing = true
	h.access.Unlock()
	if unsent {
		for i := 0; i < e.count; i++ {
			entry := e.datagrams[i]
			entry.retireHistory(&entry.frame)
			e.datagrams[i] = nil
		}
	}
	e.original.Release()
	h.access.Lock()
	if h.active == e {
		h.active = nil
	}
	*e = queueEntry{} // no raw reads after pool return; free slot last
	h.notifyLocked()
	h.access.Unlock()
	return nil
}

func (h *sendQueue) settleLocked() {
	if h.settled || !h.workerFinished || h.probeRunning {
		return
	}
	h.retired = !h.failed && h.active == nil && h.pending == 0 && !h.probe.used
	for i := range h.entries {
		h.retired = h.retired && !h.entries[i].used
	}
	h.result = h.cause
	if !h.retired {
		h.result = errPacketOutputUncertain
		if h.cause != nil {
			h.result = errors.Join(h.result, h.cause)
		}
	}
	h.settled = true
	close(h.done)
}

func (h *sendQueue) finishWorker(returned *bool) {
	h.access.Lock()
	if !*returned || h.active != nil {
		h.failLocked()
	}
	h.workerFinished = true
	h.sealed = true
	h.notifyLocked()
	h.settleLocked()
	h.access.Unlock()
}

func (h *sendQueue) finishProbe(returned *bool) {
	h.access.Lock()
	if !*returned || h.probe.used {
		h.failLocked()
	}
	h.probeRunning = false
	h.settleLocked()
	h.access.Unlock()
}

// A failed original queue must not authorize internal connection recreation.
func queueCloseError(previous, queueErr error) error {
	if queueErr == nil {
		return previous
	}
	var recreate *errCloseForRecreating
	if errors.As(previous, &recreate) {
		return queueErr
	}
	return errors.Join(previous, queueErr)
}
