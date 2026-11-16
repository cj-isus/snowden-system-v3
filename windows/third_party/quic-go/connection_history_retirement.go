package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/ackhandler"
)

// WaitForPacketHistoryRetirement joins the original native packet loop/public
// congestion setter, DATAGRAM send data, and stream-map/data/all-call/framer
// receipts before retiring sent and received-history reference metadata. It is
// an external-owner aggregate: never call it from an operation/callback it joins.
// Crypto/control/retransmission holders, controller hidden tasks, raw providers
// and physical native/OS/GC/RSS retirement remain separate.
func (c *Conn) WaitForPacketHistoryRetirement() error {
	if c == nil || c.ctx == nil || c.sentPacketHandler == nil {
		return ackhandler.ErrSentHistoryRetirement
	}
	if c.receivedPacketHandler == nil {
		return ackhandler.ErrReceivedHistoryRetirement
	}
	if err := c.WaitForCongestionControlShutdown(); err != nil {
		return errors.Join(ackhandler.ErrSentHistoryRetirement, err)
	}
	if err := c.WaitForDatagramSendDataShutdown(); err != nil {
		return errors.Join(ackhandler.ErrSentHistoryRetirement, err)
	}
	if err := c.WaitForStreamMapRetirement(); err != nil {
		return errors.Join(ackhandler.ErrSentHistoryRetirement, err)
	}
	if err := c.sentPacketHandler.RetireHistoryMetadata(); err != nil {
		return err
	}
	return c.receivedPacketHandler.RetireHistoryMetadata()
}
