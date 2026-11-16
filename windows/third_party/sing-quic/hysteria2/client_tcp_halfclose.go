package hysteria2

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

var (
	_ N.ReadCloser  = (*clientConn)(nil)
	_ N.WriteCloser = (*clientConn)(nil)
	_ N.EarlyReader = (*clientConn)(nil)
	_ N.EarlyWriter = (*clientConn)(nil)
)

// These are framing facts, not an authentication, protected-path or close receipt.
// Keep the legacy writer predicate for existing callers as well.
func (c *clientConn) NeedHandshakeForRead() bool  { return !c.responseRead.Load() }
func (c *clientConn) NeedHandshakeForWrite() bool { return !c.requestWritten.Load() }

// CloseWrite sends FIN after the request header and all preceding core writes.
// The receive half remains available for a server's response after upload EOF.
func (c *clientConn) CloseWrite() error {
	if err := c.beginCall(false); err != nil {
		return err
	}
	returned, locked := false, false
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
	if c.isClosed() {
		returned = true
		return net.ErrClosed
	}
	if c.writeCloseComplete.Load() {
		returned = true
		return c.writeCloseErr
	}
	c.writeClosed.Store(true)
	if !c.requestWritten.Load() {
		_, err := c.writeRequest(nil)
		if err != nil {
			// No valid completed request can precede FIN. CopyConn may ignore a
			// CloseWrite error, so wake its reader through the existing full-close
			// owner instead of leaving it waiting on an incomplete request.
			c.writeCloseErr = err
			c.writeCloseComplete.Store(true)
			c.requestClose()
			returned = true
			return err
		}
	}
	var err error
	streamCtx := c.Stream.Context()
	select {
	case <-streamCtx.Done():
		// Close-after-CancelWrite is invalid. Keep the original reset cause;
		// do not tear down the receive half merely for a known write reset.
		err = context.Cause(streamCtx)
		if err == nil {
			err = net.ErrClosed
		}
	default:
		err = c.Stream.Close()
	}
	c.writeCloseErr = err
	c.writeCloseComplete.Store(true)
	if c.readClosed.Load() {
		// Both local directions were closed. Retire through the existing full
		// owner after this registered call returns, without another FIN attempt.
		c.requestClose()
	}
	returned = true
	return err
}

// Cancel the receive half without waiting for its read mutex or closing writes.
// The original carrier control lane tracks the actual native control return.
func (c *clientConn) CloseRead() error {
	if err := c.beginCall(true); err != nil {
		return err
	}
	returned, locked := false, false
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
	if !c.readClosed.Load() {
		c.readClosed.Store(true)
		c.Stream.CancelRead(0)
	}
	if c.writeCloseComplete.Load() {
		c.requestClose()
	}
	returned = true
	return nil
}
