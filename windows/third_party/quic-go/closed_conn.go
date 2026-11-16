package quic

import (
	"math/bits"
	"net"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/packetaddr"
)

// A closedLocalConn is a connection that we closed locally.
// When receiving packets for such a connection, we need to retransmit the packet containing the CONNECTION_CLOSE frame,
// with an exponential backoff.
type closedLocalConn struct {
	owner   *closedCacheOwner
	counter atomic.Uint32
	logger  utils.Logger

	sendPacket func(net.Addr, packetInfo)
}

var _ packetHandler = &closedLocalConn{}

// newClosedLocalConn creates a handler dispatched by the transport listener.
func newClosedLocalConn(sendPacket func(net.Addr, packetInfo), logger utils.Logger) packetHandler {
	return &closedLocalConn{
		sendPacket: sendPacket,
		logger:     logger,
	}
}

func (c *closedLocalConn) handlePacket(p receivedPacket) {
	n := c.counter.Add(1)
	// exponential backoff
	// only send a CONNECTION_CLOSE for the 1st, 2nd, 4th, 8th, 16th, ... packet arriving
	if bits.OnesCount32(n) != 1 {
		releaseClosedPacket(&p)
		return
	}
	// Metadata can originate in a provider-owned receive view. Capture before
	// releasing that view, then keep only immutable facts through logging/retry.
	address, err := packetaddr.Capture(p.remoteAddr)
	info := p.info
	releaseClosedPacket(&p)
	if err != nil {
		return
	}
	c.logger.Debugf("Received %d packets after sending CONNECTION_CLOSE. Retransmitting.", n)
	c.sendPacket(address.Addr(), info)
}

func (c *closedLocalConn) destroy(error) {
	if c.owner != nil {
		c.owner.retire()
	}
}
func (c *closedLocalConn) closeWithTransportError(TransportErrorCode) { c.destroy(nil) }

// A closedRemoteConn is a connection that was closed remotely.
// For such a connection, we might receive reordered packets that were sent before the CONNECTION_CLOSE.
// We can just ignore those packets.
type closedRemoteConn struct {
	owner *closedCacheOwner
	// Nonzero storage gives concurrently retained handlers distinct identities.
	// Pointers to separate zero-sized values are allowed to compare equal.
	identity byte
}

var _ packetHandler = &closedRemoteConn{}

func newClosedRemoteConn() packetHandler {
	return &closedRemoteConn{}
}

func (c *closedRemoteConn) handlePacket(p receivedPacket) { releaseClosedPacket(&p) }
func (c *closedRemoteConn) destroy(error) {
	if c.owner != nil {
		c.owner.retire()
	}
}
func (c *closedRemoteConn) closeWithTransportError(TransportErrorCode) { c.destroy(nil) }

// The handler consumes one input reference, including shared/coalesced buffers.
// Clear the original view only after the actual pool-return operation returns.
func releaseClosedPacket(p *receivedPacket) {
	if p.buffer != nil {
		p.buffer.Decrement()
		p.buffer.MaybeRelease()
	}
	p.data, p.buffer = nil, nil
	p.transport, p.remoteAddr, p.info = nil, nil, packetInfo{}
}
