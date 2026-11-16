package hysteria2

import (
	"errors"

	E "github.com/sagernet/sing/common/exceptions"
)

func (c *Client) loopMessages(conn *clientQUICConnection) {
	for {
		message, err := conn.quicConn.ReceiveDatagram(conn.ctx)
		if err != nil {
			conn.closeWithError(E.Cause(err, "receive message"))
			return
		}
		// One receive owner per carrier; inputPacket never waits on a reader.
		if hErr := c.handleMessage(conn, message); hErr != nil {
			conn.closeWithError(E.Cause(hErr, "handle message"))
			return
		}
	}
}

func (c *Client) handleMessage(conn *clientQUICConnection, data []byte) error {
	message, err := receiveUDPMessage(data, &conn.receiveBudget)
	if err != nil {
		if errors.Is(err, errUDPReceiveCapacity) {
			return nil // nonfatal overload; preserve neighboring sessions/carrier
		}
		return E.Cause(err, "decode UDP message")
	}
	conn.handleUDPMessage(message)
	return nil
}

func (c *clientQUICConnection) handleUDPMessage(message *udpMessage) {
	c.udpAccess.RLock()
	udpConn, loaded := c.udpConnMap[message.sessionID]
	c.udpAccess.RUnlock()
	if !loaded {
		message.releaseMessage()
		return
	}
	select {
	case <-udpConn.ctx.Done():
		message.releaseMessage()
		return
	default:
	}
	udpConn.inputPacket(message)
}
