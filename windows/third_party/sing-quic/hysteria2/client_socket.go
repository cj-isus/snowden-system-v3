package hysteria2

import (
	"context"
	"errors"
	"net"
	"syscall"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
)

// One reservation covers the original dial and wrapper construction. The one
// initial/Hop dial task is serial; another reservation never waits or overwrites
// an unresolved socket. Close can abort a late result before RemoteAddr or any
// vectorised-writer constructor has returned.
type clientSocketDial struct {
	socket *clientOwnedSocket // published under the carrier's lifeAccess
	done   chan struct{}      // original callback returned, not native success
}

type clientOwnedSocket struct {
	net.Conn
	closeCall *clientCloseCall
}

func (c *clientOwnedSocket) Close() error {
	c.closeCall.start()
	<-c.closeCall.done
	return c.closeCall.err
}

func (c *clientOwnedSocket) Upstream() any { return c.Conn }

// Preserve only the original socket's explicit syscall capability. Do not
// manufacture a raw path by unwrapping a proxy/packet adapter here.
func (c *clientOwnedSocket) wrappedConn() net.Conn {
	if raw, ok := c.Conn.(syscall.Conn); ok {
		return &clientSyscallSocket{clientOwnedSocket: c, raw: raw}
	}
	return c
}

type clientSyscallSocket struct {
	*clientOwnedSocket
	raw syscall.Conn
}

func (c *clientSyscallSocket) SyscallConn() (syscall.RawConn, error) {
	return c.raw.SyscallConn()
}

func (c *clientQUICConnection) reserveSocket() (*clientSocketDial, error) {
	c.lifeAccess.Lock()
	defer c.lifeAccess.Unlock()
	if c.closing {
		return nil, net.ErrClosed
	}
	if c.opening != nil {
		return nil, errClientRetiring
	}
	opening := &clientSocketDial{done: make(chan struct{})}
	c.opening = opening
	return opening, nil
}

func (c *clientQUICConnection) attachSocket(opening *clientSocketDial, raw net.Conn) *clientOwnedSocket {
	socket := &clientOwnedSocket{Conn: raw, closeCall: newClientCloseCall(raw.Close)}
	c.lifeAccess.Lock()
	opening.socket = socket
	closing := c.closing
	c.lifeAccess.Unlock()
	if closing {
		socket.closeCall.start()
	}
	return socket
}

// Called with lifeAccess held. The slot remains through uncertain cleanup.
func (c *clientQUICConnection) openingCloseLocked() *clientCloseCall {
	if c.opening != nil && c.opening.socket != nil {
		return c.opening.socket.closeCall
	}
	return nil
}

func (c *clientQUICConnection) transferSocket(opening *clientSocketDial) bool {
	c.lifeAccess.Lock()
	defer c.lifeAccess.Unlock()
	if c.closing || c.opening != opening {
		return false
	}
	// The original initializer/Hop task retains the returned wrapper until its
	// next owner is installed. Their original task joins cover this handoff.
	c.opening = nil
	return true
}

func (c *clientQUICConnection) discardSocket(opening *clientSocketDial) error {
	var err error
	if opening.socket != nil {
		err = opening.socket.Close()
	}
	if err != nil {
		c.markUncertain() // preserve opening and its original negative close result
		return err
	}
	c.lifeAccess.Lock()
	if c.opening == opening {
		c.opening = nil // no socket or confirmed original Close, never a timeout
	}
	c.lifeAccess.Unlock()
	return nil
}

func (c *Client) dialPacket(ctx context.Context, owner *clientQUICConnection, address M.Socksaddr) (net.PacketConn, error) {
	opening, err := owner.reserveSocket()
	if err != nil {
		return nil, err
	}
	returned := false
	defer func() {
		if !returned {
			// The outer initializer/Hop task contains panic / Goexit. Start the
			// original abort here while retaining this socket and negative slot.
			owner.markUncertain()
		}
		close(opening.done)
	}()
	raw, err := c.dialer.DialContext(ctx, "udp", address)
	if raw == nil {
		if err == nil {
			err = errClientNoSocket
		}
		_ = owner.discardSocket(opening)
		returned = true
		return nil, err
	}
	socket := owner.attachSocket(opening, raw)
	if err != nil {
		cleanupErr := owner.discardSocket(opening)
		returned = true
		return nil, errors.Join(err, cleanupErr)
	}
	var packet net.PacketConn = bufio.NewUnbindPacketConn(socket.wrappedConn())
	if c.salamanderPassword != "" {
		packet = NewSalamanderConn(packet, []byte(c.salamanderPassword))
	}
	if !owner.transferSocket(opening) {
		cleanupErr := owner.discardSocket(opening)
		returned = true
		return nil, errors.Join(net.ErrClosed, cleanupErr)
	}
	returned = true
	return packet, nil
}
