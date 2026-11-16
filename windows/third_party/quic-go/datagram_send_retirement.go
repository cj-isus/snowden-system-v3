package quic

import "errors"

var errDatagramSendUncertain = errors.New("quic: original DATAGRAM send-data retirement is uncertain")

type datagramRetirementStage uint8

const (
	datagramRetirementProducers datagramRetirementStage = iota + 1
	datagramRetirementPreflight
	datagramRetirementReleasing
	datagramRetirementAccounting
)

// sendMx guards the result and original finalizer admission. Only that finalizer
// owns its bounded snapshot/current/index; failure retains it without retry.
type datagramSendRetirement struct {
	done           chan struct{}
	started, ended bool
	result         error
	stage          datagramRetirementStage
	entries        []*datagramSendEntry
	current        *datagramSendEntry
	index          int
}

// Admission already holds sendMx -> scope.mu. Reserve logical credit before
// cloning; the queue owns the shell before make/copy or any further callback.
func (h *datagramQueue) reserveSendProducerLocked(size int) (*datagramSendEntry, error) {
	if h.sendLive == nil || h.sendOwned != len(h.sendLive) || h.sendProducers < 0 ||
		h.sendProducers > h.sendOwned || h.sendBytes < 0 || h.sendBytes > maxDatagramSendBytes || size < 0 {
		h.poisonSendLocked()
		return nil, errDatagramSendUncertain
	}
	if h.sendPoisoned || h.sendOwned >= maxDatagramSendOwned || size > maxDatagramSendBytes-h.sendBytes {
		return nil, ErrDatagramSendCapacity
	}
	h.sendOwned++
	h.sendBytes += size
	entry := &datagramSendEntry{queue: h, refs: 1, size: size, producing: true}
	h.sendLive[entry] = struct{}{}
	h.sendProducers++
	return entry, nil
}

func (h *datagramQueue) poisonSendLocked() {
	h.sendPoisoned = true
	h.settleSendProducersLocked()
}

func (h *datagramQueue) settleSendProducersLocked() {
	if h.sendProducersDone == nil || h.sendProducersSettled {
		return
	}
	select {
	case <-h.closed:
		if h.sendPoisoned || h.sendProducers == 0 {
			h.sendProducersSettled = true
			close(h.sendProducersDone) // a poisoned result remains negative
		}
	default:
	}
}

// Last producer defer, after copy/enqueue/context/scheduling/timer work returned.
// Its reference is separate from queue/history and Peek borrower references.
func (h *datagramQueue) finishSendProducer(entry *datagramSendEntry, returned *bool) {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	_, owned := h.sendLive[entry]
	if !*returned || !owned || entry == nil || entry.queue != h || !entry.producing || h.sendProducers <= 0 {
		h.poisonSendLocked()
		return // keep the original entry, reference, data and producer credit
	}
	entry.producing = false
	h.sendProducers--
	h.releaseEntryLocked(entry)
	h.settleSendProducersLocked()
}

func (h *datagramQueue) waitSendProducers() error {
	h.sendMx.Lock()
	select {
	case <-h.closed:
	default:
		h.sendMx.Unlock()
		return errDatagramSendUncertain
	}
	if h.sendProducersDone == nil || h.sendPoisoned {
		h.sendMx.Unlock()
		return errDatagramSendUncertain
	}
	done := h.sendProducersDone
	h.sendMx.Unlock()
	<-done // no queue/scope mutex while joining original producers
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	if h.sendPoisoned || h.sendProducers != 0 || !h.sendProducersSettled {
		return errDatagramSendUncertain
	}
	return nil
}

// Only original owned data is inspected. No pooled STREAM frames, ACK scratch,
// foreign handlers, retransmission callback or pool operation is invoked here.
func (h *datagramQueue) retireSendHistoryData() error {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	r := &h.sendRetirement
	r.stage = datagramRetirementPreflight
	if h.sendPoisoned || h.sendProducers != 0 || !h.sendQueue.Empty() ||
		h.sendOwned != len(h.sendLive) || h.sendOwned > maxDatagramSendOwned ||
		h.sendBytes < 0 || h.sendBytes > maxDatagramSendBytes {
		return errDatagramSendUncertain
	}
	r.entries = make([]*datagramSendEntry, 0, len(h.sendLive))
	bytes := 0
	for entry := range h.sendLive {
		if entry == nil || entry.queue != h || entry.producing || !entry.history || entry.refs != 1 ||
			entry.size < 0 || entry.size > maxDatagramSendBytes-bytes ||
			len(entry.frame.Data) != entry.size || entry.frame.DataLen() != entry.size {
			return errDatagramSendUncertain
		}
		r.entries = append(r.entries, entry)
		bytes += entry.size
	}
	if bytes != h.sendBytes {
		return errDatagramSendUncertain
	}
	// Every entry passed preflight before any payload or history credit changes.
	r.stage = datagramRetirementReleasing
	for i, entry := range r.entries {
		r.index, r.current = i, entry
		entry.history = false
		h.releaseEntryLocked(entry)
		if h.sendPoisoned {
			return errDatagramSendUncertain
		}
		r.current = nil
	}
	r.stage = datagramRetirementAccounting
	if h.sendOwned != 0 || h.sendBytes != 0 || len(h.sendLive) != 0 || h.sendProducers != 0 {
		return errDatagramSendUncertain
	}
	h.sendLive = nil // closed admission cannot allocate another owned entry
	return nil
}

func (h *datagramQueue) finishSendData(inputErr error) (result error) {
	h.sendMx.Lock()
	r := &h.sendRetirement
	if r.done == nil {
		h.sendMx.Unlock()
		return errDatagramSendUncertain
	}
	if r.started {
		done := r.done
		h.sendMx.Unlock()
		<-done
		h.sendMx.Lock()
		result = r.result
		if !r.ended {
			result = errors.Join(errDatagramSendUncertain, result)
		}
		h.sendMx.Unlock()
		return result
	}
	r.started, r.stage = true, datagramRetirementProducers
	h.sendMx.Unlock()
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(errDatagramSendUncertain, result)
		}
		h.sendMx.Lock()
		defer h.sendMx.Unlock()
		r.result, r.ended = result, true
		if returned && result == nil {
			r.entries, r.current = nil, nil
		}
		close(r.done)
	}()
	if inputErr != nil {
		returned = true
		return errors.Join(errDatagramSendUncertain, inputErr)
	}
	if err := h.waitSendProducers(); err != nil {
		returned = true
		return err
	}
	result = h.retireSendHistoryData()
	returned = true
	return result
}

// WaitForDatagramSendDataShutdown is an external-owner, owned-data-only receipt.
// It joins original packet input/native output and admitted payload producers,
// then retires history payload credit. Never call it from a producer callback
// it joins. Caller-owned input before admission, scope/public-call completion,
// receive data, ACK metadata, hidden provider tasks and OS/GC/RSS are separate.
func (c *Conn) WaitForDatagramSendDataShutdown() error {
	if c == nil || c.ctx == nil || c.datagramQueue == nil {
		return errDatagramSendUncertain
	}
	<-c.ctx.Done()
	inputErr := c.WaitForPacketInputShutdown()
	return errors.Join(inputErr, c.datagramQueue.finishSendData(inputErr))
}
