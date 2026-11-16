package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
)

// The original buffer stays Conn-owned through the actual write and all closed
// cache-copy callbacks. It is released by handleCloseError after their return.
func (c *Conn) writeConnectionClosePacket(packet *coalescedPacket) ([]byte, error) {
	if packet == nil {
		c.packetOutput.failed = true
		c.receiveFrames.markUncertain()
		return nil, errPacketOutputUncertain
	}
	if err := c.beginPacketOutput(packet.buffer); err != nil {
		return nil, err
	}
	if size := len(packet.buffer.Data); size == 0 || size > protocol.MaxPacketBufferSize {
		return nil, c.rejectPacketOutput(errPacketOutputCapacity)
	}
	ecn := c.sentPacketHandler.ECNMode(packet.IsOnlyShortHeaderPacket())
	c.logCoalescedPacket(packet, ecn)
	c.packetOutput.borrowing = true
	err := c.conn.Write(packet.buffer.Data, 0, ecn)
	c.packetOutput.borrowing = false
	if errors.Is(err, errPacketOutputUncertain) {
		return nil, c.rejectPacketOutput(err)
	}
	return packet.buffer.Data, err
}
