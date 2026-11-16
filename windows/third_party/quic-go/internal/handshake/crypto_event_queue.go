package handshake

import "errors"

const (
	maxCryptoQueuedEvents = 128
	maxCryptoEventBytes   = 8 << 20
)

var (
	ErrCryptoEventCapacity = errors.New("quic: pending crypto events storage limit")
	ErrCryptoEventState    = errors.New("quic: original crypto event queue is uncertain")
	errCryptoEventsClosed  = errors.New("quic: crypto event queue retired")
)

// The original Conn loop serializes producers/consumer. Credit covers queued
// copied payload only, not TLS internals or data already transferred to callers.
func (h *cryptoSetup) cryptoEventQueueError() error {
	if h.eventsSealed {
		return errCryptoEventsClosed
	}
	if h.eventFailure != nil {
		return h.eventFailure
	}
	if h.eventBytes < 0 || h.eventBytes > maxCryptoEventBytes ||
		len(h.events) > maxCryptoQueuedEvents || cap(h.events) > maxCryptoQueuedEvents {
		h.eventFailure = ErrCryptoEventState
	}
	return h.eventFailure
}

func (h *cryptoSetup) reserveCryptoEvent(bytes int) (int, bool) {
	if h.cryptoEventQueueError() != nil {
		return 0, false
	}
	if bytes < 0 {
		h.eventFailure = ErrCryptoEventState
		return 0, false
	}
	if len(h.events) == maxCryptoQueuedEvents || bytes > maxCryptoEventBytes-h.eventBytes {
		h.eventFailure = ErrCryptoEventCapacity
		return 0, false
	}
	if len(h.events) == cap(h.events) {
		capacity := min(max(16, 2*cap(h.events)), maxCryptoQueuedEvents)
		queue := make([]Event, len(h.events), capacity)
		copy(queue, h.events)
		h.events = queue
	}
	index := len(h.events)
	h.events = append(h.events, Event{}) // original slot before payload allocation
	h.eventBytes += bytes
	return index, true
}

func (h *cryptoSetup) queueCryptoEvent(event Event) {
	if event.Data != nil || event.Err != nil || event.Kind <= EventNoEvent ||
		event.Kind >= EventError || event.Kind == EventWriteInitialData || event.Kind == EventWriteHandshakeData {
		h.eventFailure = ErrCryptoEventState
		return
	}
	if index, ok := h.reserveCryptoEvent(0); ok {
		h.events[index] = event
	}
}

func (h *cryptoSetup) queueCryptoWrite(kind EventKind, data []byte) {
	if kind != EventWriteInitialData && kind != EventWriteHandshakeData {
		h.eventFailure = ErrCryptoEventState
		return
	}
	index, ok := h.reserveCryptoEvent(len(data))
	if !ok {
		return
	}
	h.events[index].Kind = kind
	h.events[index].Data = make([]byte, len(data))
	copy(h.events[index].Data, data) // before the next crypto/tls.NextEvent
}

func (h *cryptoSetup) copySessionTicket(data []byte) ([]byte, error) {
	if err := h.cryptoEventQueueError(); err != nil {
		return nil, err
	}
	if len(data) > maxCryptoEventBytes-h.eventBytes {
		h.eventFailure = ErrCryptoEventCapacity
		return nil, h.eventFailure
	}
	// One local copy while this serialized call owns it; return transfers it to
	// the caller. This is not a lifetime quota on caller-retained session tickets.
	ticket := make([]byte, len(data))
	copy(ticket, data)
	return ticket, nil
}

func (h *cryptoSetup) NextEvent() Event {
	if err := h.cryptoEventQueueError(); err != nil {
		return Event{Kind: EventError, Err: err}
	}
	if len(h.events) == 0 {
		if h.eventBytes != 0 {
			h.eventFailure = ErrCryptoEventState
			return Event{Kind: EventError, Err: h.eventFailure}
		}
		return Event{Kind: EventNoEvent}
	}
	event := h.events[0]
	if len(event.Data) > h.eventBytes || len(h.events) == 1 && len(event.Data) != h.eventBytes {
		h.eventFailure = ErrCryptoEventState
		return Event{Kind: EventError, Err: h.eventFailure}
	}
	h.events[0] = Event{} // returned values must not remain in the backing prefix
	h.events = h.events[1:]
	h.eventBytes -= len(event.Data)
	if len(h.events) == 0 {
		h.events = nil
	}
	return event
}

// RetireEventQueue is external-only after original Conn/native/data/history/TLS
// receipts and local crypto sorter preflight. Close's mutex protects one bounded
// metadata result; there is no wait, provider callback, byte/key wipe or pool call.
func (h *cryptoSetup) RetireEventQueue() (result error) {
	r := &h.closeState
	r.access.Lock()
	defer r.access.Unlock()
	if r.eventsStarted {
		return r.eventsResult
	}
	r.eventsStarted = true
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(ErrCryptoEventState, result)
		}
		r.eventsResult = result
	}()
	if r.done == nil || !r.started || !r.returned || h.eventsSealed ||
		h.eventFailure != nil && h.eventFailure != ErrCryptoEventCapacity ||
		h.eventBytes < 0 || h.eventBytes > maxCryptoEventBytes ||
		len(h.events) > maxCryptoQueuedEvents || cap(h.events) > maxCryptoQueuedEvents {
		returned = true
		return ErrCryptoEventState
	}
	bytes := 0
	for _, event := range h.events {
		write := event.Kind == EventWriteInitialData || event.Kind == EventWriteHandshakeData
		if event.Kind <= EventNoEvent || event.Kind >= EventError || event.Err != nil ||
			!write && event.Data != nil || cap(event.Data) != len(event.Data) ||
			len(event.Data) > maxCryptoEventBytes-bytes {
			returned = true
			return ErrCryptoEventState
		}
		bytes += len(event.Data)
	}
	if bytes != h.eventBytes {
		returned = true
		return ErrCryptoEventState
	}
	// Ordinary capacity refusal is quiescent; inconsistent credit was refused.
	h.eventsSealed = true
	clear(h.events[:cap(h.events)]) // pointer cells only, never payload contents
	h.events, h.eventBytes = nil, 0
	returned = true
	return nil
}
