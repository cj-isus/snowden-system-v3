package hysteria2

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/packetaddr"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/pipe"
)

var udpMessagePool = sync.Pool{
	New: func() interface{} {
		return new(udpMessage)
	},
}

func allocMessage() *udpMessage {
	message := udpMessagePool.Get().(*udpMessage)
	message.referenced = true
	return message
}

func releaseMessages(messages []*udpMessage) {
	for _, message := range messages {
		if message != nil {
			message.releaseMessage()
		}
	}
}

type udpMessage struct {
	sessionID     uint32
	packetID      uint16
	fragmentID    uint8
	fragmentTotal uint8
	destination   string
	data          *buf.Buffer
	referenced    bool
	rxBudget      *udpReceiveBudget
	rxBytes       int
}

func (m *udpMessage) release() {
	if !m.referenced {
		return
	}
	budget, charge := m.rxBudget, m.rxBytes
	*m = udpMessage{}
	if budget != nil {
		budget.release(charge) // actual release or explicit reader buffer handoff
	}
	udpMessagePool.Put(m)
}

func (m *udpMessage) releaseMessage() {
	m.data.Release()
	m.release()
}

type udpPacketConn struct {
	ctx             context.Context
	cancel          common.ContextCancelCauseFunc
	initStage       udpPacketInitStage // retained constructor prefix; immutable after publication
	sessionID       uint32
	quicConn        *quic.Conn
	local           packetaddr.Snapshot // immutable constructor-time socket fact, not live path/egress
	localSource     net.Addr            // original constructor capture prefix, nil after known capture
	data            chan *udpMessage
	udpMTU          int
	packetId        atomic.Uint32
	closeOnce       sync.Once
	destroyErr      error
	shutdownOnce    sync.Once
	shutdownErr     error
	shutdownStage   udpShutdownStage
	shutdownCurrent *udpMessage // one original queue drain, retained on abnormal return
	defragger       *udpDefragger
	onDestroy       func()
	readWaitOptions N.ReadWaitOptions
	readDeadline    pipe.Deadline
	readSlots       [maxUDPReadSessionCalls]udpReadCall // budget.mu admits; original caller owns payload fields
	readCalls       int                                 // budget.mu; includes retained unknown slots
	readSealed      bool                                // budget.mu
	control         sync.Mutex                          // serialize setters and original close timer stop
	controlCalls    int                                 // budget.mu; distinct lane from packet reads
	controlFailed   bool                                // budget.mu; unknown timer/native setter result
	receiveAccess   sync.Mutex                          // serial feed/defrag/close; readers own dequeued messages
	receiveInput    *udpMessage                         // receiveAccess; retained through feed/release/queue handoff
	receiveOutput   *udpMessage                         // receiveAccess; original assembled output through handoff
	receiveFailed   bool
	sendBudget      *udpSendBudget
	sendScope       *quic.DatagramSendScope
}

func (c *udpPacketConn) LocalAddr() net.Addr {
	return c.local.Addr()
}

func (c *udpPacketConn) ReaderMTU() int {
	return protocol.MaxUDPSize
}

func (c *udpPacketConn) WriterMTU() int {
	return protocol.MaxUDPSize
}
