package hysteria2

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/packetaddr"
	qtls "github.com/sagernet/sing-quic"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

// Core net.Conn operations and Peek are registered on the original carrier.
// The embedded advanced QUIC control surface remains compatible; it is not a
// proof that arbitrary external control/stream borrowers have been retired.
type clientConn struct {
	*quic.Stream
	owner              *clientQUICConnection
	destination        M.Socksaddr
	address            string
	local              packetaddr.Snapshot
	read               sync.Mutex
	write              sync.Mutex
	control            sync.Mutex
	requestWritten     atomic.Bool
	responseRead       atomic.Bool
	readClosed         atomic.Bool
	writeClosed        atomic.Bool
	writeCloseComplete atomic.Bool // publishes completion, never just close intent
	writeCloseErr      error
	responseErr        error
	response           protocol.TCPResponseReader
	header             *buf.Buffer // write mutex, then close after admitted calls drain
	headerOffset       int
	stage              *buf.Buffer // retained on abnormal original write return
	closeOnce          sync.Once
	opened             chan struct{}
	idle               chan struct{}
	done               chan struct{}
	closed             chan struct{}
	opening            bool // remaining fields guarded by owner.lifeAccess
	closing            bool
	poisoned           bool
	idleReported       bool
	calls              int
	controls           int
	closeErr           error
}

func (c *clientConn) NeedHandshake() bool { return c.NeedHandshakeForWrite() }

func (c *clientConn) readResponse() error {
	if c.responseRead.Load() {
		return nil
	}
	if c.responseErr != nil {
		return c.responseErr
	}
	ok, message, err := c.response.Read(c.Stream)
	if err != nil {
		return qtls.WrapError(err)
	}
	if !ok {
		c.responseErr = E.New("remote error: ", message)
		c.response = protocol.TCPResponseReader{}
		return c.responseErr
	}
	c.responseRead.Store(true)
	c.response = protocol.TCPResponseReader{}
	return nil
}

func (c *clientConn) readPayload(p []byte, peek bool) (int, error) {
	if err := c.beginCall(false); err != nil {
		return 0, err
	}
	returned := false
	locked := false
	defer func() {
		if !returned {
			c.poisonBeforeUnlock()
		}
		if locked {
			c.read.Unlock()
		}
		c.endCall(false, returned)
	}()
	c.read.Lock()
	locked = true
	if c.isClosed() || c.readClosed.Load() {
		returned = true
		return 0, net.ErrClosed
	}
	if len(p) == 0 {
		returned = true
		return 0, nil
	}
	if err := c.readResponse(); err != nil {
		returned = true
		return 0, err
	}
	var n int
	var err error
	if peek {
		n, err = c.Stream.Peek(p)
	} else {
		n, err = c.Stream.Read(p)
	}
	wrappedErr := qtls.WrapError(err)
	returned = true
	return n, wrappedErr
}

func (c *clientConn) Read(p []byte) (int, error) { return c.readPayload(p, false) }
func (c *clientConn) Peek(p []byte) (int, error) { return c.readPayload(p, true) }

// Caller holds write. Header-only flushes use their existing bounded stream
// storage; an empty upload must not compete for payload coalescing permits.
func (c *clientConn) ensureRequestHeader() error {
	if c.header != nil {
		return nil
	}
	var err error
	c.header, err = protocol.WriteTCPRequestHeader(c.address)
	return err
}

func (c *clientConn) advanceRequestHeader(n int) {
	c.headerOffset += n
	if c.headerOffset == c.header.Len() {
		c.header.Release()
		c.header = nil
		c.requestWritten.Store(true)
	}
}

func (c *clientConn) writeEmptyRequest() (int, error) {
	if err := c.ensureRequestHeader(); err != nil {
		return 0, err
	}
	remaining := c.header.Bytes()[c.headerOffset:]
	n, err := c.Stream.Write(remaining)
	if n < 0 || n > len(remaining) {
		c.owner.markUncertain()
		return 0, errTCPWriteSize
	}
	c.advanceRequestHeader(n)
	if err == nil && n != len(remaining) {
		err = io.ErrShortWrite
	}
	return 0, qtls.WrapError(err)
}

// A bounded first prefix remains coalesced with the original header. The rest
// of a large caller payload is borrowed directly, never cloned in full. Header
// progress survives a partial write/deadline, so retries cannot repeat framing.
func (c *clientConn) writeRequest(p []byte) (int, error) {
	if len(p) == 0 {
		return c.writeEmptyRequest()
	}
	if err := c.reserveStaging(); err != nil {
		if err == errTCPStaging {
			// Coalescing is optional. Keep the same protected QUIC stream and
			// bounded original header, then let Write borrow the caller's payload.
			// Closed/uncertain admission failures never take this path.
			return c.writeEmptyRequest()
		}
		return 0, err
	}
	returned := false
	defer func() {
		if returned {
			if c.stage != nil {
				c.stage.Release()
				c.stage = nil
			}
			c.releaseStaging()
		}
	}()
	if err := c.ensureRequestHeader(); err != nil {
		returned = true
		return 0, err
	}
	remaining := c.header.Bytes()[c.headerOffset:]
	prefix := min(len(p), maxTCPFirstData)
	c.stage = buf.NewSize(len(remaining) + prefix)
	c.stage.Write(remaining)
	c.stage.Write(p[:prefix])
	n, err := c.Stream.Write(c.stage.Bytes())
	returned = true // only now may the original staging buffer/credit return
	if n < 0 || n > c.stage.Len() {
		c.owner.markUncertain()
		return 0, errTCPWriteSize
	}
	payloadWritten := max(0, n-len(remaining))
	c.advanceRequestHeader(min(n, len(remaining)))
	if err == nil && n != c.stage.Len() {
		err = io.ErrShortWrite
	}
	return payloadWritten, qtls.WrapError(err)
}

func (c *clientConn) Write(p []byte) (int, error) {
	if err := c.beginCall(false); err != nil {
		return 0, err
	}
	returned := false
	locked := false
	defer func() {
		if !returned {
			c.poisonBeforeUnlock()
		}
		if locked {
			c.write.Unlock()
		}
		c.endCall(false, returned)
	}()
	c.write.Lock()
	locked = true
	n, err := c.writePayload(p)
	returned = true
	return n, err
}

// The scalar and consuming vector APIs hold the same writer mutex and register
// one original core call. A vector keeps this lock across its entire sequence,
// so neither another payload nor CloseWrite can split its request/data framing.
func (c *clientConn) writePayload(p []byte) (int, error) {
	if c.isClosed() || c.writeClosed.Load() {
		return 0, net.ErrClosed
	}
	n := 0
	if !c.requestWritten.Load() {
		var err error
		n, err = c.writeRequest(p)
		if err != nil || n == len(p) {
			return n, err
		}
	}
	more, err := c.Stream.Write(p[n:])
	if more < 0 || more > len(p)-n {
		c.owner.markUncertain()
		return n, errTCPWriteSize
	}
	if err == nil && more != len(p)-n {
		err = io.ErrShortWrite
	}
	return n + more, qtls.WrapError(err)
}

func (c *clientConn) LocalAddr() net.Addr  { return c.local.Addr() }
func (c *clientConn) RemoteAddr() net.Addr { return c.destination }

func (c *clientConn) setDeadline(t time.Time, read, write bool) error {
	if err := c.beginCall(true); err != nil {
		return err
	}
	returned := false
	locked := false
	defer func() {
		if !returned {
			c.poisonBeforeUnlock()
		}
		if locked {
			c.control.Unlock()
		}
		c.endCall(true, returned)
	}()
	c.control.Lock()
	locked = true
	if c.isClosed() {
		returned = true
		return net.ErrClosed
	}
	var err error
	switch {
	case read && write:
		err = c.Stream.SetDeadline(t)
	case read:
		err = c.Stream.SetReadDeadline(t)
	case write:
		err = c.Stream.SetWriteDeadline(t)
	}
	returned = true
	return err
}

func (c *clientConn) SetDeadline(t time.Time) error      { return c.setDeadline(t, true, true) }
func (c *clientConn) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, true, false) }
func (c *clientConn) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, false, true) }

func (c *clientConn) Close() error {
	c.requestClose()
	<-c.closed
	return c.closeErr
}
