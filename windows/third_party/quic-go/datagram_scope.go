package quic

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sagernet/quic-go/internal/monotime"
)

const (
	maxDatagramSendScopes = 256
	maxDatagramScopeCalls = 8
)

// DatagramSendScope owns one session's enqueue seal and write deadline, not a
// socket or the lifetime of already transferred frames. It must not be copied;
// copied/zero values fail original-identity checks instead of sharing authority.
type DatagramSendScope struct {
	self  *DatagramSendScope
	conn  *Conn
	queue *datagramQueue
	mu    sync.Mutex

	closed    bool
	uncertain bool
	returned  bool
	calls     int
	deadline  monotime.Time
	changed   chan struct{}
}

// NewDatagramSendScope binds one original queue before publication. This is not
// peer capability, forwarding or delivery evidence. Open and retiring scopes
// share a bound; Close cannot refund a slot while its Send calls still run.
func (c *Conn) NewDatagramSendScope() (*DatagramSendScope, error) {
	if c == nil || c.datagramQueue == nil {
		return nil, errors.New("datagram support disabled")
	}
	h := c.datagramQueue
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	select {
	case <-h.closed:
		return nil, h.closeErr
	default:
	}
	if h.scopePoisoned || h.sendPoisoned || h.scopeOwned >= maxDatagramSendScopes {
		return nil, ErrDatagramSendCapacity
	}
	scope := &DatagramSendScope{conn: c, queue: h, changed: make(chan struct{})}
	scope.self = scope
	h.scopeOwned++
	return scope, nil
}

func (s *DatagramSendScope) original() bool {
	return s != nil && s.self == s && s.conn != nil && s.queue != nil && s.conn.datagramQueue == s.queue
}

func (s *DatagramSendScope) errorLocked() error {
	if s.closed || s.uncertain || s.returned {
		return net.ErrClosed
	}
	if !s.deadline.IsZero() && !monotime.Now().Before(s.deadline) {
		return errDeadline
	}
	return nil
}

// Err is an observation, not an enqueue permit. Send repeats the check under
// the same fence as the actual queue transfer. A deadline error may be reset.
func (s *DatagramSendScope) Err() error {
	if !s.original() {
		return net.ErrClosed
	}
	select {
	case <-s.queue.closed:
		return net.ErrClosed
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errorLocked()
}

func (s *DatagramSendScope) Send(ctx context.Context, data []byte) error {
	if !s.original() {
		return net.ErrClosed
	}
	if ctx == nil {
		return errors.New("nil DATAGRAM context")
	}
	select {
	case <-s.queue.closed:
		return net.ErrClosed
	default:
	}
	s.mu.Lock()
	if err := s.errorLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.calls >= maxDatagramScopeCalls {
		s.mu.Unlock()
		return ErrDatagramSendCapacity
	}
	s.calls++
	s.mu.Unlock()
	defer s.leaveCall()
	return s.conn.sendDatagram(ctx, s, data)
}

func (s *DatagramSendScope) returnableLocked() bool {
	if !s.closed || s.uncertain || s.returned || s.calls != 0 {
		return false
	}
	s.returned = true
	return true
}

func (h *datagramQueue) returnScope() {
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	if h.scopeOwned <= 0 {
		h.scopePoisoned = true
		return
	}
	h.scopeOwned--
}

func (s *DatagramSendScope) leaveCall() {
	s.mu.Lock()
	if s.calls <= 0 {
		s.uncertain = true
		if !s.closed {
			s.closed = true
			close(s.changed)
		}
		s.mu.Unlock()
		s.queue.sendMx.Lock()
		s.queue.scopePoisoned = true
		s.queue.sendMx.Unlock()
		return
	}
	s.calls--
	returnSlot := s.returnableLocked()
	s.mu.Unlock()
	if returnSlot {
		s.queue.returnScope() // never acquire queue while holding scope.mu
	}
}

// Close seals new enqueue under the same mutex used by queue transfer and wakes
// pending calls. It does not await those calls, revoke admitted frames or claim
// carrier/socket retirement. The quota slot remains held until actual return.
func (s *DatagramSendScope) Close() error {
	if !s.original() {
		return net.ErrClosed
	}
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.changed)
	}
	returnSlot := s.returnableLocked()
	s.mu.Unlock()
	if returnSlot {
		s.queue.returnScope()
	}
	return nil
}

// Anchor the standard time.Time deadline at one current wall/monotonic pair.
// This avoids treating a wall-only input relative to an old wall-clock epoch.
// Finite saturated dates and the zero-sentinel collision remain finite values.
func datagramDeadline(t time.Time) monotime.Time {
	if t.IsZero() {
		return 0
	}
	reference := time.Now()
	base := int64(monotime.FromTime(reference))
	delta := int64(t.Sub(reference))
	const maximum int64 = 1<<63 - 1
	const minimum int64 = -maximum - 1
	var value int64
	switch {
	case delta > 0 && base > maximum-delta:
		value = maximum
	case delta < 0 && base < minimum-delta:
		value = minimum
	default:
		value = base + delta
	}
	if value == 0 {
		value = 1 // finite, already expired; only time.Time{} means unlimited
	}
	return monotime.Time(value)
}

// SetWriteDeadline updates future and waiting sends in this scope only. A
// changed deadline wakes every waiter; a stale timer is only a reason to recheck.
func (s *DatagramSendScope) SetWriteDeadline(t time.Time) error {
	if !s.original() {
		return net.ErrClosed
	}
	select {
	case <-s.queue.closed:
		return net.ErrClosed
	default:
	}
	deadline := datagramDeadline(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.uncertain || s.returned {
		return net.ErrClosed
	}
	if s.deadline == deadline {
		return nil
	}
	s.deadline = deadline
	close(s.changed)
	s.changed = make(chan struct{})
	return nil
}

type datagramAdmissionState struct {
	deadline monotime.Time
	changed  <-chan struct{}
}

// Success holds queue.sendMx -> scope.mu through the caller's reserve/transfer.
// Failure holds neither lock. Context methods remain outside both mutexes.
func (h *datagramQueue) lockAdmission(done <-chan struct{}, scope *DatagramSendScope) (state datagramAdmissionState, err error, revoked bool) {
	h.sendMx.Lock()
	if err, revoked = h.sendErrorLocked(done); err != nil || revoked {
		h.sendMx.Unlock()
		return
	}
	if h.sendPoisoned {
		h.sendMx.Unlock()
		return state, ErrDatagramSendCapacity, false
	}
	if scope == nil {
		return
	}
	if !scope.original() || scope.queue != h {
		h.sendMx.Unlock()
		return state, net.ErrClosed, false
	}
	scope.mu.Lock()
	err = scope.errorLocked()
	if err == nil && scope.calls <= 0 {
		err = net.ErrClosed
	}
	if err != nil {
		scope.mu.Unlock()
		h.sendMx.Unlock()
		return
	}
	state.deadline, state.changed = scope.deadline, scope.changed
	return
}

func (h *datagramQueue) unlockAdmission(scope *DatagramSendScope) {
	if scope != nil {
		scope.mu.Unlock()
	}
	h.sendMx.Unlock()
}
