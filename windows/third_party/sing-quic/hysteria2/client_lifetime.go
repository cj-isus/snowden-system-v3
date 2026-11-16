package hysteria2

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing/common"
)

var (
	errClientRetiring  = errors.New("HY2 original carrier is still retiring")
	errClientUncertain = errors.New("HY2 original carrier cleanup is uncertain")
	errClientNoSocket  = errors.New("HY2 dial returned no socket")
)

// One task/result per original close call. It does not retry an uncertain Close,
// and task return is not a native success receipt when the result is negative.
type clientCloseCall struct {
	once sync.Once
	done chan struct{}
	call func() error
	err  error
}

func newClientCloseCall(call func() error) *clientCloseCall {
	return &clientCloseCall{done: make(chan struct{}), call: call}
}

func (c *clientCloseCall) start() {
	c.once.Do(func() {
		go func() {
			returned := false
			defer func() {
				if !returned {
					_ = recover()
					c.err = errClientUncertain
				}
				close(c.done)
			}()
			c.err = c.call()
			returned = true
		}()
	})
}

func newClientQUICConnection(parent context.Context) *clientQUICConnection {
	ctx, cancel := common.ContextWithCancelCause(parent)
	conn := &clientQUICConnection{
		ctx: ctx, cancel: cancel, initializing: true,
		connDone: make(chan struct{}), initialDone: make(chan struct{}), retired: make(chan struct{}),
		udpConnMap: make(map[uint32]*udpPacketConn),
		tcpStreams: make(map[*clientConn]struct{}), tcpDone: make(chan struct{}),
	}
	conn.receiveBudget.onFailure = conn.markUncertain
	conn.sendBudget.onFailure = conn.markUncertain
	return conn
}

// Only the original initializer attaches these resources. The owner exists
// before it starts. A resource arriving after seal starts its original close
// immediately; initialDone prevents the coordinator from missing late attachments.
func (c *clientQUICConnection) attachRaw(raw io.Closer) {
	call := newClientCloseCall(raw.Close)
	c.lifeAccess.Lock()
	c.rawClose = call
	closing := c.closing
	c.lifeAccess.Unlock()
	if closing {
		call.start()
	}
}

func (c *clientQUICConnection) attachTransport(transport *quic.Transport) {
	call := newClientCloseCall(transport.Close)
	c.lifeAccess.Lock()
	c.transportClose = call
	closing := c.closing
	c.lifeAccess.Unlock()
	if closing {
		call.start()
	}
}

func (c *clientQUICConnection) attachQUIC(conn *quic.Conn) {
	call := newClientCloseCall(func() error {
		closeErr := conn.CloseWithError(0, "")
		return errors.Join(closeErr, conn.WaitForCryptoQueueRetirement())
	})
	c.lifeAccess.Lock()
	c.quicConn, c.quicClose = conn, call
	closing := c.closing
	c.lifeAccess.Unlock()
	if closing {
		call.start()
	}
	// Observe early QUIC closure before constructing/authenticating HTTP/3.
	// Otherwise a failed SETTINGS task could wait on a blocked raw write while
	// the abort watcher had not yet been registered by the initializer.
	c.runTask(func() {
		select {
		case <-conn.Closing():
			c.closeWithError(net.ErrClosed)
		case <-conn.Context().Done():
			c.closeWithError(context.Cause(conn.Context()))
		case <-c.ctx.Done():
			c.closeWithError(context.Cause(c.ctx))
		}
	})
}

func (c *clientQUICConnection) attachHTTP3(conn *http3.ClientConn) {
	call := newClientCloseCall(conn.WaitForShutdown)
	c.lifeAccess.Lock()
	c.http3Wait = call
	closing := c.closing
	c.lifeAccess.Unlock()
	if closing {
		call.start()
	}
}

func (c *clientQUICConnection) markUncertain() {
	c.closeWithError(errClientUncertain)
}

// Called by the one initializer before offer publication or cleanup join.
func (c *clientQUICConnection) finishInitialization() {
	c.lifeAccess.Lock()
	c.initializing = false
	close(c.initialDone)
	c.lifeAccess.Unlock()
}

// Registration ends before initialDone. Internal RX/watch tasks request closure
// without waiting for their own join; public Client.CloseWithError does the wait.
func (c *clientQUICConnection) runTask(work func()) {
	c.tasks.Add(1)
	go func() {
		returned := false
		defer func() {
			if !returned {
				_ = recover()
				c.markUncertain()
			}
			c.tasks.Done()
		}()
		work()
		returned = true
	}()
}

func (c *clientQUICConnection) closeWithError(cause error) {
	c.requestClose(cause, false)
}

func (c *clientQUICConnection) requestClose(cause error, initializingOnly bool) {
	c.lifeAccess.Lock()
	if cause == errClientUncertain {
		// Failure publication and seal share this fence, including a late
		// uncertain result after closure was already requested.
		c.failure = errClientUncertain
	}
	if c.closing || initializingOnly && !c.initializing {
		c.lifeAccess.Unlock()
		return
	}
	if cause == nil {
		cause = net.ErrClosed
	}
	c.closing = true
	c.connErr = cause
	close(c.connDone)
	calls := [5]*clientCloseCall{c.rawClose, c.quicClose, c.transportClose, c.openingCloseLocked(), c.http3Wait}
	tcpOriginals := c.sealTCPLocked()
	c.lifeAccess.Unlock()
	// Start every independent abort before any cancel callback, UDP teardown
	// or send-queue wait. There is no carrier mutex over an external Close.
	for _, call := range calls {
		if call != nil {
			call.start()
		}
	}
	for _, original := range tcpOriginals {
		if original != nil {
			original.startClose()
		}
	}
	c.sendBudget.seal()
	c.receiveBudget.seal()
	go c.finishClose()
	c.cancel(cause)
}

func (c *clientQUICConnection) finishClose() {
	var closeErr error
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			closeErr = errors.Join(closeErr, errClientUncertain)
		}
		c.lifeAccess.Lock()
		c.closeErr = errors.Join(closeErr, c.failure)
		if returned && c.closeErr == nil {
			// Handoff already ended all writes to udpConnMap. Only this finalizer
			// accesses retired fields; no UDP mutex is held over a life callback.
			if c.udpConnMap != nil || c.udpConstructing != nil || c.udpRetiringCurrent != nil {
				c.closeErr = errClientUncertain
			} else {
				c.udpRetiring = nil
				c.udpRetiringID = 0
				c.udpConstructionRetiring = nil
			}
		}
		c.lifeAccess.Unlock()
		close(c.retired)
	}()
	<-c.initialDone
	c.lifeAccess.Lock()
	calls := [4]*clientCloseCall{c.rawClose, c.quicClose, c.transportClose, c.http3Wait}
	opening := c.opening
	c.lifeAccess.Unlock()
	// Closing and publication share lifeAccess. An earlier constructor holds
	// udpAccess; hand off both the exact map and its unpublished original owner.
	func() {
		c.udpAccess.Lock()
		defer c.udpAccess.Unlock()
		if c.udpRetiring != nil || c.udpConstructionRetiring != nil || c.udpConnMap == nil || len(c.udpConnMap) > maxUDPReceiveSessions {
			panic(errClientUncertain)
		}
		c.udpRetiring, c.udpConnMap = c.udpConnMap, nil
		c.udpConstructionRetiring, c.udpConstructing = c.udpConstructing, nil
	}()
	if c.udpConstructionRetiring != nil {
		closeErr = errors.Join(closeErr, c.udpConstructionRetiring.abort())
	}
	for id, udpConn := range c.udpRetiring {
		c.udpRetiringID, c.udpRetiringCurrent = id, udpConn
		if udpConn == nil {
			panic(errClientUncertain)
		}
		closeErr = errors.Join(closeErr, udpConn.closeAndDestroy(c.connErr))
		c.udpRetiringCurrent = nil
		c.udpRetiringID = 0
	}
	for _, call := range calls {
		if call != nil {
			call.start() // same original task, including a late attachment
			<-call.done
			closeErr = errors.Join(closeErr, call.err)
		}
	}
	if opening != nil {
		<-opening.done
		if opening.socket != nil {
			call := opening.socket.closeCall
			call.start()
			<-call.done
			closeErr = errors.Join(closeErr, call.err)
		}
	}
	c.tasks.Wait()
	if !c.udpReceive.idle() {
		closeErr = errors.Join(closeErr, errClientUncertain)
	}
	<-c.tcpDone
	c.lifeAccess.Lock()
	closeErr = errors.Join(closeErr, c.tcpFailure)
	c.lifeAccess.Unlock()
	<-c.sendBudget.seal()
	<-c.receiveBudget.seal()
	closeErr = errors.Join(closeErr, c.sendBudget.retirementError(), c.receiveBudget.retirementError())
	returned = true
}

// A caller must observe retired before reading the immutable outcome. A negative
// outcome keeps this object in Client.conn / Client.pending, denying replacement.
func (c *clientQUICConnection) retirementError() error {
	<-c.retired
	return c.closeErr
}

func (c *Client) completeOffer(pending *clientOffer, offerCtx context.Context) {
	var err error
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			err = errClientUncertain
			pending.owner.markUncertain()
		}
		pending.owner.finishInitialization()
		pending.cancel(nil)
		c.finishOffer(pending, err)
	}()
	err = c.offerNew(offerCtx, pending.owner)
	returned = true
}

func (c *Client) finishOffer(pending *clientOffer, err error) {
	conn := pending.owner
	c.connAccess.Lock()
	if pending.discarded {
		if pending.cause != nil {
			err = pending.cause
		} else {
			err = net.ErrClosed
		}
	}
	if err == nil && conn.active() {
		pending.conn, c.conn = conn, conn
		if c.pending == pending {
			c.pending = nil
		}
		close(pending.done)
		c.connAccess.Unlock()
		return
	}
	if err == nil {
		err = net.ErrClosed
	}
	c.connAccess.Unlock()
	conn.closeWithError(err)
	cleanupErr := conn.retirementError()
	c.connAccess.Lock()
	pending.err = errors.Join(err, cleanupErr)
	// Failed/discarded offers remain the shared owner until original cleanup
	// returns. Uncertain cleanup retains even a completed pending result.
	if cleanupErr == nil && c.pending == pending {
		c.pending = nil
	}
	close(pending.done)
	c.connAccess.Unlock()
}
