package quic

import (
	"net"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
)

// Immutable after publication. Public metadata methods load one original sender
// while the connection loop keeps its own conn/queue fields. The initial view
// is embedded in Conn; later views must outlive candidate-prefix reuse.
type connectionSendState struct {
	conn sendConn
}

func (c *Conn) currentSendConn() sendConn { return c.sendState.Load().conn }

// One original loop owns this candidate through provider calls and old-queue
// retirement. An uncertain transition keeps the exact originals for its owner.
type connectionPathSwitch struct {
	active        bool
	transport     *Transport
	raw           rawConn
	remote        net.Addr
	previousConn  sendConn
	previousQueue sender
	nextConn      sendConn
	nextQueue     sender
	nextState     *connectionSendState
	sendOpening   sendConnOpening
}

// Each existing queue task carries this notification guard. Run's own guard
// retains packet custody; an abnormal task also requests the original Conn close.
// Panic and Goexit keep their original behavior after the deferred notification.
func (c *Conn) runSendQueue(queue sender) {
	var cause error = errPacketOutputUncertain
	defer func() {
		if cause != nil {
			c.destroyImpl(cause)
		}
	}()
	cause = queue.Run()
}

func (c *Conn) failPathSwitch() {
	c.packetOutput.failed = true // connection-loop-owned metadata
	if c.receiveFrames != nil {
		c.receiveFrames.markUncertain()
	}
	c.destroyImpl(errPacketOutputUncertain)
}

func (c *Conn) finishPathSwitch(returned *bool) {
	if !*returned {
		c.failPathSwitch()
		return
	}
	c.pathSwitch = connectionPathSwitch{}
}

func (c *Conn) capturePathSwitch(call *connectionPathSwitch) error {
	t := call.transport
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed {
		return errPacketOutputUncertain
	}
	if o.sealed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if c.closeErr.Load() != nil {
		return net.ErrClosed
	}
	call.raw = t.conn // fixed original before LocalAddr / metadata providers
	return nil
}

// Publication and the existing worker start share the target's seal fence.
// All provider, old Close, migration-stat and PMTU calls precede these locks.
func (c *Conn) publishPathSwitch(call *connectionPathSwitch) error {
	t := call.transport
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed {
		return errPacketOutputUncertain
	}
	if o.sealed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if c.closeErr.Load() != nil {
		return net.ErrClosed
	}
	if call != &c.pathSwitch || !call.active || call.raw == nil || call.nextConn == nil || call.nextQueue == nil || call.nextState == nil {
		return errPacketOutputUncertain
	}
	c.conn, c.sendQueue = call.nextConn, call.nextQueue
	c.sendState.Store(call.nextState)
	go c.runSendQueue(call.nextQueue) // evaluate the original before prefix clearing
	return nil
}

func (c *Conn) switchToNewPath(tr *Transport, now monotime.Time) {
	if tr == nil || c.pathSwitch.active {
		c.failPathSwitch()
		return
	}
	if c.closeErr.Load() != nil {
		return
	}
	call := &c.pathSwitch
	*call = connectionPathSwitch{active: true, transport: tr, previousConn: c.conn, previousQueue: c.sendQueue}
	returned := false
	defer c.finishPathSwitch(&returned)
	if err := c.capturePathSwitch(call); err != nil {
		c.destroyImpl(err)
		returned = err != errPacketOutputUncertain
		return
	}
	call.remote = call.previousConn.RemoteAddr()
	nextConn, err := newSendConn(&call.sendOpening, call.raw, call.remote, packetInfo{}, utils.DefaultLogger)
	call.nextConn = nextConn
	if err != nil {
		c.destroyImpl(err)
		returned = true
		return
	}
	retired, queueErr := call.previousQueue.Close()
	if !retired || queueErr != nil {
		if queueErr == nil {
			queueErr = errPacketOutputUncertain
		}
		c.destroyImpl(queueErr)
		returned = retired
		return
	}
	initialPacketSize := protocol.ByteCount(c.config.InitialPacketSize)
	if err := c.sentPacketHandler.MigratedPath(now, initialPacketSize); err != nil {
		c.destroyImpl(err)
		return // preserve original partial path switch and negative receipt
	}
	maxPacketSize := protocol.ByteCount(protocol.MaxPacketBufferSize)
	if c.peerParams.MaxUDPPayloadSize > 0 && c.peerParams.MaxUDPPayloadSize < maxPacketSize {
		maxPacketSize = c.peerParams.MaxUDPPayloadSize
	}
	c.mtuDiscoverer.Reset(now, initialPacketSize, maxPacketSize, c.pathMTUDiscoveryEnabled(nextConn))
	call.nextQueue = newSendQueue(nextConn)
	call.nextState = &connectionSendState{conn: nextConn}
	if err := c.publishPathSwitch(call); err != nil {
		c.destroyImpl(err)
		returned = err != errPacketOutputUncertain
		return // an unstarted queue has no native writes or worker to join
	}
	returned = true
}
