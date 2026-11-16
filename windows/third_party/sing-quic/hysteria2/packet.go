package hysteria2

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
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
	sessionID       uint32
	quicConn        *quic.Conn
	data            chan *udpMessage
	udpMTU          int
	packetId        atomic.Uint32
	closeOnce       sync.Once
	defragger       *udpDefragger
	onDestroy       func()
	readWaitOptions N.ReadWaitOptions
	readDeadline    pipe.Deadline
	receiveAccess   sync.Mutex // serial feed/defrag/close; readers own dequeued messages
	sendBudget      *udpSendBudget
	sendScope       *quic.DatagramSendScope
}

func newUDPPacketConn(ctx context.Context, quicConn *quic.Conn, budget *udpReceiveBudget, send *udpSendBudget, onDestroy func()) (*udpPacketConn, error) {
	scope, err := quicConn.NewDatagramSendScope()
	if err != nil {
		return nil, err
	}
	constructed := false
	defer func() {
		if !constructed {
			_ = scope.Close()
		}
	}()
	ctx, cancel := common.ContextWithCancelCause(ctx)
	conn := &udpPacketConn{
		ctx:          ctx,
		cancel:       cancel,
		quicConn:     quicConn,
		data:         make(chan *udpMessage, 64),
		udpMTU:       initialUDPPacketBudget,
		defragger:    newUDPDefragger(budget),
		onDestroy:    onDestroy,
		readDeadline: pipe.MakeDeadline(),
		sendBudget:   send,
		sendScope:    scope,
	}
	constructed = true
	return conn, nil
}

func (c *udpPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case p := <-c.data:
		_, err = buffer.ReadOnceFrom(p.data)
		destination = M.ParseSocksaddr(p.destination).Unwrap()
		p.releaseMessage()
		return
	case <-c.ctx.Done():
		return M.Socksaddr{}, io.ErrClosedPipe
	case <-c.readDeadline.Wait():
		return M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (c *udpPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case pkt := <-c.data:
		n = copy(p, pkt.data.Bytes())
		destination := M.ParseSocksaddr(pkt.destination).Unwrap()
		if destination.IsFqdn() {
			addr = destination
		} else {
			addr = destination.UDPAddr()
		}
		pkt.releaseMessage()
		return n, addr, nil
	case <-c.ctx.Done():
		return 0, nil, io.ErrClosedPipe
	case <-c.readDeadline.Wait():
		return 0, nil, os.ErrDeadlineExceeded
	}
}

func (c *udpPacketConn) inputPacket(message *udpMessage) {
	c.receiveAccess.Lock()
	defer c.receiveAccess.Unlock()
	select {
	case <-c.ctx.Done():
		message.releaseMessage()
		return
	default:
	}
	if message.fragmentTotal <= 1 {
		select {
		case c.data <- message:
		default:
			message.releaseMessage()
		}
	} else {
		newMessage := c.defragger.feed(message)
		if newMessage != nil {
			select {
			case c.data <- newMessage:
			default:
				newMessage.releaseMessage()
			}
		}
	}
}

func (c *udpPacketConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeWithError(os.ErrClosed)
		c.onDestroy()
	})
	return nil
}

func (c *udpPacketConn) closeWithError(err error) {
	if c.sendScope != nil {
		_ = c.sendScope.Close() // seal enqueue before cancellation/RX retirement
	}
	c.cancel(err)
	c.receiveAccess.Lock()
	defer c.receiveAccess.Unlock()
	c.defragger.close()
	for {
		select {
		case message := <-c.data:
			message.releaseMessage()
		default:
			return
		}
	}
}

func (c *udpPacketConn) LocalAddr() net.Addr {
	return c.quicConn.LocalAddr()
}

func (c *udpPacketConn) SetDeadline(t time.Time) error {
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	c.readDeadline.Set(t)
	return nil
}

func (c *udpPacketConn) SetReadDeadline(t time.Time) error {
	c.readDeadline.Set(t)
	return nil
}

func (c *udpPacketConn) SetWriteDeadline(t time.Time) error {
	if c.sendScope == nil {
		return net.ErrClosed
	}
	return c.sendScope.SetWriteDeadline(t)
}

func (c *udpPacketConn) ReaderMTU() int {
	return protocol.MaxUDPSize
}

func (c *udpPacketConn) WriterMTU() int {
	return protocol.MaxUDPSize
}
