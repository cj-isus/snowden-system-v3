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

// Only the original loopMessages task writes this slot. Its task join precedes
// the carrier's final inspection. No per-packet worker, queue or allocation.
type clientUDPReceive struct {
	active   bool
	failed   bool
	data     []byte
	message  *udpMessage
	reserved bool
	charge   int
}

func (r *clientUDPReceive) idle() bool {
	return !r.active && !r.failed && r.data == nil && r.message == nil && !r.reserved && r.charge == 0
}

func (c *Client) handleMessage(conn *clientQUICConnection, data []byte) (err error) {
	original := &conn.udpReceive
	if !original.idle() {
		original.failed = true
		conn.receiveBudget.markUncertain()
		return errClientUncertain
	}
	original.active, original.data = true, data
	returned := false
	defer func() {
		if !returned || conn.receiveBudget.uncertain() {
			// Keep even an ambiguous pool/handoff prefix without dereferencing or
			// releasing it again. Notification occurs outside carrier/RX locks.
			original.failed = true
			conn.receiveBudget.markUncertain()
			if returned {
				err = errors.Join(err, errClientUncertain)
			}
		} else {
			*original = clientUDPReceive{}
		}
	}()
	message, err := receiveUDPMessageOwned(data, &conn.receiveBudget, original)
	if err != nil {
		returned = true
		if errors.Is(err, errUDPReceiveCapacity) {
			return nil // nonfatal overload; preserve neighboring sessions/carrier
		}
		return E.Cause(err, "decode UDP message")
	}
	conn.handleUDPMessage(message)
	returned = true
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
