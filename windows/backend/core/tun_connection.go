package core

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	N "github.com/sagernet/sing/common/network"
)

// Deliberately no Upstream/replaceable or half-close interface: SDK copy
// optimizations must not unwrap the original once-close receipt. Name Conn
// explicitly so no native deadline method can bypass this owner's close seal.
// The flow owner joins in-flight I/O; a failed Close is sticky.
type tunScopedConnection struct {
	Conn         net.Conn
	mu           sync.Mutex
	closing      atomic.Bool
	controlFault atomic.Bool
	readBytes    atomic.Uint64
	writtenBytes atomic.Uint64
	closeStarted bool
	closeErr     error
}

var _ net.Conn = (*tunScopedConnection)(nil)

func (connection *tunScopedConnection) Read(buffer []byte) (int, error) {
	if connection.closing.Load() {
		return 0, net.ErrClosed
	}
	n, err := connection.Conn.Read(buffer)
	if n > 0 {
		connection.readBytes.Add(uint64(n))
	}
	return n, err
}

func (connection *tunScopedConnection) Write(buffer []byte) (int, error) {
	if connection.closing.Load() {
		return 0, net.ErrClosed
	}
	n, err := connection.Conn.Write(buffer)
	if n > 0 {
		connection.writtenBytes.Add(uint64(n))
	}
	return n, err
}

func (connection *tunScopedConnection) LocalAddr() net.Addr {
	return connection.Conn.LocalAddr()
}

func (connection *tunScopedConnection) RemoteAddr() net.Addr {
	return connection.Conn.RemoteAddr()
}

func (connection *tunScopedConnection) Close() error {
	connection.closing.Store(true)
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closeStarted {
		return connection.closeErr
	}
	connection.closeStarted, connection.closeErr = true, ErrNativeScopeUnavailable
	connection.closeErr = connection.Conn.Close()
	if connection.controlFault.Load() {
		connection.closeErr = errors.Join(ErrNativeScopeUnavailable, connection.closeErr)
	}
	return connection.closeErr
}

func (connection *tunScopedConnection) SetDeadline(deadline time.Time) error {
	return connection.setDeadline(deadline, true, true)
}

func (connection *tunScopedConnection) SetReadDeadline(deadline time.Time) error {
	return connection.setDeadline(deadline, true, false)
}

func (connection *tunScopedConnection) SetWriteDeadline(deadline time.Time) error {
	return connection.setDeadline(deadline, false, true)
}

// Pinned gVisor deadline/handshake calls can initialize a lazy TCP endpoint.
// Serialize all three deadline operations with the same original Close. Keep
// the native method and its ordinary error unchanged for each requested half.
func (connection *tunScopedConnection) setDeadline(deadline time.Time, read, write bool) error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closing.Load() {
		return net.ErrClosed
	}
	returned := false
	defer connection.finishControl(&returned) // before unlock, including panic/Goexit
	var err error
	switch {
	case read && write:
		err = connection.Conn.SetDeadline(deadline)
	case read:
		err = connection.Conn.SetReadDeadline(deadline)
	default:
		err = connection.Conn.SetWriteDeadline(deadline)
	}
	returned = true
	return err
}

func (connection *tunScopedConnection) handshake(peer net.Conn) error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closing.Load() {
		return net.ErrClosed
	}
	returned := false
	defer connection.finishControl(&returned)
	err := N.ReportConnHandshakeSuccess(connection.Conn, peer)
	returned = true
	return err
}

// Called while mu is still held. An unknown native control result closes new
// admission before publishing its sticky fault. Preserve the original unwind;
// a later successful native Close cannot turn this result into positive proof.
func (connection *tunScopedConnection) finishControl(returned *bool) {
	if !*returned {
		connection.closing.Store(true)
		connection.controlFault.Store(true)
	}
}
