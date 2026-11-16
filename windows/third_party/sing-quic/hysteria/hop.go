package hysteria

import (
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/sagernet/quic-go/monotime"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	packetQueueSize    = 1024
	udpBufferSize      = 2048
	defaultHopInterval = 30 * time.Second
	maxHopPorts        = 65536
	maxHopCalls        = 16
	maxHopControls     = 2
	maxHopMetadata     = 2
	maxHopReceivers    = 3
)

type HopPacketConn struct {
	dialFunc          func(M.Socksaddr) (net.PacketConn, error)
	destination       M.Socksaddr
	ports             []uint16
	interval          time.Duration
	access            sync.Mutex
	control           sync.Mutex
	prevConn          *hopSocket
	currentConn       *hopSocket
	pendingConn       *hopSocket
	retiringConn      *hopSocket
	portIndex         int
	readBufferSize    int
	writeBufferSize   int
	readDeadline      monotime.Time
	writeDeadline     time.Time
	writeDeadlineSet  bool
	readChanged       chan struct{}
	packetReady       chan struct{}
	readFailure       error
	readFailureSocket *hopSocket
	receivers         int
	calls             int
	controls          int
	metadata          int
	receiveWG         sync.WaitGroup
	callWG            sync.WaitGroup
	packetChan        chan *buf.Buffer
	doneChan          chan struct{}
	hopDone           chan struct{}
	closeDone         chan struct{}
	closeErr          error
	failure           error
	done              bool
}

func NewHopPacketConn(
	dialFunc func(M.Socksaddr) (net.PacketConn, error),
	destination M.Socksaddr,
	ports []uint16,
	interval time.Duration,
) (*HopPacketConn, error) {
	if dialFunc == nil || len(ports) == 0 || len(ports) > maxHopPorts || interval < 0 {
		return nil, errors.New("hysteria: invalid UDP hopping parameters")
	}
	for _, port := range ports {
		if port == 0 {
			return nil, errors.New("hysteria: zero UDP hop port")
		}
	}
	if interval == 0 {
		interval = defaultHopInterval
	}
	hopConn := &HopPacketConn{
		dialFunc:    dialFunc,
		destination: destination,
		ports:       append([]uint16(nil), ports...),
		interval:    interval,
		packetChan:  make(chan *buf.Buffer, packetQueueSize),
		packetReady: make(chan struct{}, 1),
		readChanged: make(chan struct{}),
		doneChan:    make(chan struct{}),
		hopDone:     make(chan struct{}),
		closeDone:   make(chan struct{}),
	}
	currentConn, err := dialFunc(hopConn.nextAddr())
	if err != nil || currentConn == nil {
		if err == nil {
			err = errHopInvalidDial
		}
		if currentConn != nil {
			original := newHopSocket(currentConn)
			original.startClose()
			err = errors.Join(err, original.waitClosed())
		}
		return nil, err
	}
	hopConn.currentConn = newHopSocket(currentConn)
	hopConn.receivers = 1
	hopConn.receiveWG.Add(1)
	go hopConn.recvLoop(hopConn.currentConn)
	go hopConn.hopLoop()
	return hopConn, nil
}

func (c *HopPacketConn) nextAddr() M.Socksaddr {
	c.portIndex = rand.Intn(len(c.ports))
	return M.Socksaddr{
		Addr: c.destination.Addr,
		Fqdn: c.destination.Fqdn,
		Port: c.ports[c.portIndex],
	}
}

func (c *HopPacketConn) recvLoop(conn *hopSocket) {
	returned := false
	var buffer *buf.Buffer
	defer func() {
		if !returned {
			// Contain a foreign callback panic / Goexit without inventing a
			// successful native read or socket retirement receipt.
			_ = recover()
			c.beginClose(errHopCallUncertain)
		}
		if buffer != nil {
			buffer.Release()
		}
		c.access.Lock()
		c.receivers--
		c.access.Unlock()
		c.receiveWG.Done()
	}()
	for {
		select {
		case <-c.doneChan:
			returned = true
			return
		default:
		}
		buffer = buf.NewSize(udpBufferSize)
		n, _, err := conn.conn.ReadFrom(buffer.FreeBytes())
		if err != nil {
			buffer.Release()
			buffer = nil
			c.recordReadFailure(conn, err)
			returned = true
			return
		}
		if n < 0 || n > udpBufferSize {
			c.beginClose(errHopInvalidRead)
			returned = true
			return
		}
		buffer.Truncate(n)
		// Queue publication and closure share this fence. Close drains only
		// after all original receivers and public readers have returned.
		c.access.Lock()
		closed := c.done
		if !closed {
			select {
			case c.packetChan <- buffer:
				buffer = nil
				c.signalReadLocked()
			default:
			}
		}
		c.access.Unlock()
		if buffer != nil {
			buffer.Release()
			buffer = nil
		}
		if closed {
			returned = true
			return
		}
	}
}

func (c *HopPacketConn) hopLoop() {
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			c.beginClose(errHopCallUncertain)
		}
		close(c.hopDone)
	}()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.hop()
		case <-c.doneChan:
			returned = true
			return
		}
	}
}

func (c *HopPacketConn) hop() {
	c.access.Lock()
	if c.done || c.receivers >= maxHopReceivers {
		c.access.Unlock()
		return
	}
	nextAddr := c.nextAddr()
	c.access.Unlock()

	newConn, err := c.dialFunc(nextAddr)
	if newConn == nil {
		if err == nil {
			c.beginClose(errHopInvalidDial)
		}
		return
	}
	original := newHopSocket(newConn)
	c.access.Lock()
	c.pendingConn = original
	closed := c.done
	c.access.Unlock()
	if err != nil || closed {
		c.discardHop(original)
		return
	}
	oldPrevConn, published := c.publishHop(original)
	if !published {
		c.discardHop(original)
		return
	}
	if oldPrevConn != nil {
		oldPrevConn.startClose()
		if closeErr := oldPrevConn.waitClosed(); closeErr != nil {
			c.beginClose(closeErr)
			return
		}
		c.access.Lock()
		if c.retiringConn == oldPrevConn {
			c.retiringConn = nil
		}
		c.access.Unlock()
	}
}

func (c *HopPacketConn) publishHop(original *hopSocket) (*hopSocket, bool) {
	// Only control operations serialize here. The lifetime mutex is never
	// held over a foreign socket callback; Close can abort this pending socket.
	c.control.Lock()
	defer c.control.Unlock()
	c.access.Lock()
	if c.done {
		c.access.Unlock()
		return nil, false
	}
	readBufferSize, writeBufferSize := c.readBufferSize, c.writeBufferSize
	writeDeadline, writeDeadlineSet := c.writeDeadline, c.writeDeadlineSet
	c.access.Unlock()
	var prepareErr error
	if readBufferSize > 0 {
		prepareErr = errors.Join(prepareErr, trySetReadBuffer(original.conn, readBufferSize))
	}
	if writeBufferSize > 0 {
		prepareErr = errors.Join(prepareErr, trySetWriteBuffer(original.conn, writeBufferSize))
	}
	if writeDeadlineSet {
		prepareErr = errors.Join(prepareErr, original.conn.SetWriteDeadline(writeDeadline))
	}
	c.access.Lock()
	if c.done || prepareErr != nil {
		c.access.Unlock()
		return nil, false
	}
	oldPrevConn := c.prevConn
	c.retiringConn = oldPrevConn
	c.prevConn = c.currentConn
	c.currentConn = original
	c.pendingConn = nil
	c.receivers++
	c.receiveWG.Add(1)
	c.access.Unlock()
	go c.recvLoop(original)
	return oldPrevConn, true
}

func (c *HopPacketConn) WriteTo(b []byte, _ net.Addr) (n int, err error) {
	original, err := c.beginCall()
	if err != nil {
		return 0, err
	}
	returned := false
	defer func() { c.endCall(returned) }()
	n, err = original.conn.WriteTo(b, (*hopFakeAddr)(nil))
	returned = true
	return n, err
}

func (c *HopPacketConn) Close() error {
	c.beginClose(nil)
	<-c.closeDone
	return c.closeErr
}

func (c *HopPacketConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *HopPacketConn) SetWriteDeadline(t time.Time) error {
	return c.setControl(hopSetWriteDeadline, t, 0)
}

func (c *HopPacketConn) SetReadBuffer(bytes int) error {
	return c.setControl(hopSetReadBuffer, time.Time{}, bytes)
}

func (c *HopPacketConn) SetWriteBuffer(bytes int) error {
	return c.setControl(hopSetWriteBuffer, time.Time{}, bytes)
}

func trySetReadBuffer(pc any, bytes int) error {
	udpConn, isUDPConn := common.Cast[interface {
		SetReadBuffer(bytes int) error
	}](pc)
	if !isUDPConn {
		return nil
	}
	return udpConn.SetReadBuffer(bytes)
}

func trySetWriteBuffer(pc any, bytes int) error {
	udpConn, isUDPConn := common.Cast[interface {
		SetWriteBuffer(bytes int) error
	}](pc)
	if !isUDPConn {
		return nil
	}
	return udpConn.SetWriteBuffer(bytes)
}

type hopFakeAddr struct{}

func (a *hopFakeAddr) Network() string {
	return "udphop"
}

func (a *hopFakeAddr) String() string {
	return "<udphop>"
}
