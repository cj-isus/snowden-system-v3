package http3

import (
	"errors"
	"sync"

	"github.com/sagernet/quic-go"
)

const maxClientTasks = 64

var (
	errClientTasksStopped  = errors.New("http3: client task admission closed")
	errClientTaskCapacity  = errors.New("http3: client task capacity exceeded")
	errClientTaskUncertain = errors.New("http3: client task did not return normally")
)

// This owner precedes constructor workers. Admission and seal share one mutex;
// the finite count covers synchronous API calls and original internal workers,
// not external body/stream borrowers or a process-wide memory/native quota.
type clientTaskOwner struct {
	conn    *quic.Conn
	closing <-chan struct{}
	mu      sync.Mutex
	active  int
	sealed  bool
	settled bool
	err     error
	cause   error // known refusal is distinct from uncertain cleanup
	drained chan struct{}
}

func newClientTaskOwner(conn *quic.Conn) *clientTaskOwner {
	return &clientTaskOwner{conn: conn, closing: conn.Closing(), drained: make(chan struct{})}
}

func (t *clientTaskOwner) settleLocked() {
	if t.sealed && t.active == 0 && !t.settled {
		t.settled = true
		close(t.drained)
	}
}

func (t *clientTaskOwner) begin() bool {
	t.mu.Lock()
	select {
	case <-t.closing:
		t.sealed = true
	default:
	}
	if t.sealed {
		t.settleLocked()
		t.mu.Unlock()
		return false
	}
	if t.active >= maxClientTasks {
		t.cause = errClientTaskCapacity
		t.sealed = true
		t.mu.Unlock()
		// The requesting constructor/call/worker retains this original operation.
		// No capacity wait-list, fallback goroutine or foreign call under mu.
		t.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeExcessiveLoad), "")
		return false
	}
	t.active++
	t.mu.Unlock()
	return true
}

func (t *clientTaskOwner) finish(returned bool) {
	if !returned {
		t.mu.Lock()
		if t.err == nil {
			t.err = errClientTaskUncertain
		}
		t.sealed = true
		t.mu.Unlock()
		// Keep the original worker registered through this close call too.
		t.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeInternalError), "")
	}
	t.mu.Lock()
	t.active--
	t.settleLocked()
	t.mu.Unlock()
}

func (t *clientTaskOwner) run(work func()) bool {
	if !t.begin() {
		return false
	}
	go func() {
		returned := false
		defer func() {
			if !returned {
				_ = recover()
			}
			t.finish(returned)
		}()
		work()
		returned = true
	}()
	return true
}

func (t *clientTaskOwner) wait() error {
	<-t.closing
	t.mu.Lock()
	t.sealed = true
	t.settleLocked()
	t.mu.Unlock()
	<-t.drained
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

func (t *clientTaskOwner) admissionError() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cause != nil {
		return t.cause
	}
	return errClientTasksStopped
}

func (c *ClientConn) acceptUnidirectionalStreams() {
	c.tasks.run(func() {
		for {
			str, err := c.conn.AcceptUniStream(c.conn.Context())
			if err != nil {
				return
			}
			if !c.tasks.run(func() { c.handleUnidirectionalStream(str) }) {
				str.CancelRead(quic.StreamErrorCode(ErrCodeRequestCanceled))
				return
			}
		}
	})
}

// WaitForShutdown joins this client's registered calls/workers and its original
// qlogger completion after the underlying QUIC connection begins closing. It
// does not initiate/retry Close or prove native/global retirement. Do not call
// it from one of those workers. External body/stream borrowers remain separate.
func (c *ClientConn) WaitForShutdown() error {
	err := c.tasks.wait()
	<-c.rawConn.qloggerDone
	return errors.Join(err, c.rawConn.qloggerErr)
}
