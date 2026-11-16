package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
)

var (
	ErrPreflightCleanup        = errors.New("bootstrap network retirement unconfirmed")
	ErrPreflightBusy           = errors.New("bootstrap resolution already owned")
	ErrPreflightScope          = errors.New("bootstrap scope is no longer current")
	ErrPreflightAddressExpired = errors.New("bootstrap address hint expired before use")
)

const maxPreflightConnections = 10 // four UDP plus at most four TCP fallbacks, two HTTPS dials

// One original work domain survives newly constructed resolver values (the
// App's process domain or an original worker box's domain). Timeout is not
// retirement: all workers return and original Close outcomes are inspected
// before a successor starts. No App/epoch/value lock covers I/O.
type preflightWorkDomain struct {
	mu    sync.Mutex
	owner *preflightNetwork
}

var preflightWork preflightWorkDomain

type preflightNetwork struct {
	mu      sync.Mutex
	closing bool
	conns   []*preflightConn
	bodies  []*preflightBody
}

type preflightBody struct {
	io.ReadCloser
	mu       sync.Mutex
	closed   bool
	closeErr error
}

func (body *preflightBody) Close() (finalErr error) {
	body.mu.Lock()
	defer func() {
		_ = recover()
		if body.closeErr != nil {
			finalErr = body.closeErr
		}
		body.mu.Unlock()
	}()
	if body.closed {
		return body.closeErr
	}
	body.closed, body.closeErr = true, ErrPreflightCleanup
	if err := body.ReadCloser.Close(); err != nil {
		body.closeErr = errors.Join(ErrPreflightCleanup, err)
		return body.closeErr
	}
	body.closeErr = nil
	return nil
}

func (owner *preflightNetwork) body(raw io.ReadCloser) (*preflightBody, error) {
	if raw == nil {
		return nil, ErrPreflightScope
	}
	body := &preflightBody{ReadCloser: raw}
	owner.mu.Lock()
	owner.bodies = append(owner.bodies, body) // at most the two compiled HTTPS strategies
	closing := owner.closing
	owner.mu.Unlock()
	if closing {
		return body, errors.Join(ErrPreflightScope, body.Close())
	}
	return body, nil
}

// Close may be called by HTTP, cancellation and final retirement. Only the
// first actual Close is attempted; an error remains sticky on its original
// object. A repeated net.ErrClosed must not fabricate a successful receipt.
type preflightConn struct {
	net.Conn
	mu        sync.Mutex
	acquiring bool
	closed    bool
	closeErr  error
}

func (c *preflightConn) Close() (finalErr error) {
	c.mu.Lock()
	defer func() {
		_ = recover()
		if c.closeErr != nil {
			finalErr = c.closeErr
		}
		c.mu.Unlock()
	}()
	if c.acquiring {
		return ErrPreflightCleanup // dial owner will close after returning the original object
	}
	if c.closed {
		return c.closeErr
	}
	if c.Conn == nil {
		c.closed = true
		return nil // a completed failed dial returned no object
	}
	c.closed, c.closeErr = true, ErrPreflightCleanup
	if err := c.Conn.Close(); err != nil {
		c.closeErr = errors.Join(ErrPreflightCleanup, err)
		return c.closeErr
	}
	c.closeErr = nil // keep the published interface stable for concurrent Read/Write returns
	return nil
}

func (owner *preflightNetwork) dial(ctx context.Context, network, address string) (net.Conn, error) {
	return owner.dialWith(ctx, network, address, (&net.Dialer{}).DialContext)
}

// Keep the same reserve-before-acquire/sticky-Close owner when a platform SDK
// supplies the socket dialer. This does not create an unobserved dial task.
func (owner *preflightNetwork) dialWith(ctx context.Context, network, address string, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	owner.mu.Lock()
	if owner.closing || len(owner.conns) >= maxPreflightConnections {
		owner.mu.Unlock()
		return nil, ErrPreflightBusy
	}
	conn := &preflightConn{acquiring: true}
	owner.conns = append(owner.conns, conn) // reserve before the dial can acquire anything
	owner.mu.Unlock()
	raw, err := dial(ctx, network, address)
	conn.mu.Lock()
	conn.Conn, conn.acquiring = raw, false
	conn.mu.Unlock()
	owner.mu.Lock()
	closing := owner.closing
	owner.mu.Unlock()
	if closing {
		return nil, errors.Join(ErrPreflightScope, err, conn.Close())
	}
	if err != nil || raw == nil || contextError(ctx) != nil {
		if raw == nil && err == nil {
			err = ErrPreflightCleanup
		}
		return nil, errors.Join(err, contextError(ctx), conn.Close())
	}
	if deadline, bounded := ctx.Deadline(); bounded {
		if err := raw.SetDeadline(deadline); err != nil {
			return nil, errors.Join(err, conn.Close())
		}
	}
	return conn, nil
}

func (owner *preflightNetwork) close() error {
	owner.mu.Lock()
	owner.closing = true
	connections := append([]*preflightConn(nil), owner.conns...)
	bodies := append([]*preflightBody(nil), owner.bodies...)
	owner.mu.Unlock()
	var result error
	for _, conn := range connections {
		result = errors.Join(result, conn.Close())
	}
	for _, body := range bodies {
		result = errors.Join(result, body.Close())
	}
	return result
}

// Transfer exactly one completed acquisition without replacing its native
// interface. The caller must first stop/join all cancellation callbacks, then
// hold its original publication lock. No native operation runs under these locks.
func (owner *preflightNetwork) handoff(conn *preflightConn) (net.Conn, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closing || len(owner.conns) != 1 || owner.conns[0] != conn || len(owner.bodies) != 0 {
		return nil, ErrPreflightCleanup
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.acquiring || conn.closed || conn.closeErr != nil || conn.Conn == nil {
		return nil, ErrPreflightCleanup
	}
	owner.conns, owner.closing = nil, true
	return conn.Conn, nil // original SDK object and optional interfaces, now owned by its caller
}

// Metadata only. Never wait on a native Close mutex while the caller holds
// its publication lock; an in-progress acquisition/Close remains unconfirmed.
func (owner *preflightNetwork) retirementState() (bool, error) {
	if !owner.mu.TryLock() {
		return false, nil
	}
	defer owner.mu.Unlock()
	complete := true
	var result error
	for _, conn := range owner.conns {
		if !conn.mu.TryLock() {
			complete = false
			continue
		}
		complete = complete && !conn.acquiring && conn.closed
		result = errors.Join(result, conn.closeErr)
		conn.mu.Unlock()
	}
	return complete && len(owner.bodies) == 0, result // service-native owners never acquire HTTP bodies
}
