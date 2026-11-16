package hysteria

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sagernet/quic-go/packetaddr"
)

var (
	errHopCapacity       = errors.New("hysteria: UDP hop operation capacity")
	errHopInvalidDial    = errors.New("hysteria: UDP hop dial returned no socket")
	errHopInvalidRead    = errors.New("hysteria: UDP hop read returned invalid size")
	errHopCallUncertain  = errors.New("hysteria: UDP hop callback did not return normally")
	errHopCloseUncertain = errors.New("hysteria: original UDP socket close did not return normally")
)

// Each successful dial must transfer a fresh, exclusively owned PacketConn.
// A socket's original Close is invoked once. Its result is not replaced by a
// later ErrClosed or by a timeout. The Go socket object, not a numeric FD, stays
// reachable through outstanding I/O and this original close owner.
type hopSocket struct {
	conn      net.PacketConn
	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
	local     packetaddr.Snapshot // guarded by HopPacketConn.access
}

func newHopSocket(conn net.PacketConn) *hopSocket {
	return &hopSocket{conn: conn, closed: make(chan struct{})}
}

func (s *hopSocket) startClose() {
	s.closeOnce.Do(func() {
		// One close task per original socket, never one per waiter or timeout.
		// This independent path must not wait for the wrapper's control mutex.
		go func() {
			returned := false
			defer func() {
				if !returned {
					_ = recover()
					s.closeErr = errHopCloseUncertain
				}
				close(s.closed)
			}()
			s.closeErr = s.conn.Close()
			returned = true
		}()
	})
}

func (s *hopSocket) waitClosed() error {
	<-s.closed
	return s.closeErr
}

func (c *HopPacketConn) beginCall() (*hopSocket, error) {
	c.access.Lock()
	defer c.access.Unlock()
	if c.done {
		return nil, net.ErrClosed
	}
	if c.calls >= maxHopCalls {
		return nil, errHopCapacity
	}
	c.calls++
	c.callWG.Add(1)
	return c.currentConn, nil
}

func (c *HopPacketConn) endCall(returned bool) {
	if !returned {
		c.beginClose(errHopCallUncertain)
	}
	c.access.Lock()
	c.calls--
	c.access.Unlock()
	c.callWG.Done()
}

// A separate bounded control lane cannot be exhausted by waiting ReadFrom calls.
// Pure read-deadline metadata uses access alone and needs no foreign call slot.
func (c *HopPacketConn) beginControlCall() error {
	c.access.Lock()
	defer c.access.Unlock()
	if c.done {
		return net.ErrClosed
	}
	if c.controls >= maxHopControls {
		return errHopCapacity
	}
	c.controls++
	c.callWG.Add(1)
	return nil
}

func (c *HopPacketConn) endControlCall(returned bool) {
	if !returned {
		c.beginClose(errHopCallUncertain)
	}
	c.access.Lock()
	c.controls--
	c.access.Unlock()
	c.callWG.Done()
}

// Called only with access held. Pending and retiring sockets are retained even
// when a control callback, old Close or late dial has not yet returned.
func (c *HopPacketConn) socketsLocked() [4]*hopSocket {
	return [4]*hopSocket{c.prevConn, c.currentConn, c.pendingConn, c.retiringConn}
}

func (c *HopPacketConn) beginClose(cause error) {
	c.access.Lock()
	originals, started := c.sealCloseLocked(cause)
	c.access.Unlock()
	if started {
		c.launchClose(originals)
	}
}

// Caller holds access. Native read-fault selection and hop publication use the
// same fence, so an old socket error cannot close a newly published generation.
func (c *HopPacketConn) sealCloseLocked(cause error) ([4]*hopSocket, bool) {
	if cause != nil && c.failure == nil {
		c.failure = cause
	}
	if c.done {
		return [4]*hopSocket{}, false
	}
	c.done = true
	close(c.doneChan)
	return c.socketsLocked(), true
}

func (c *HopPacketConn) launchClose(originals [4]*hopSocket) {
	for _, original := range originals {
		if original != nil {
			original.startClose()
		}
	}
	go c.finishClose(originals)
}

func (c *HopPacketConn) finishClose(originals [4]*hopSocket) {
	var closeErr error
	for _, original := range originals {
		if original != nil {
			closeErr = errors.Join(closeErr, original.waitClosed())
		}
	}
	// A dial already in progress is owned by the one original hop task. It
	// closes a late/discarded result before returning, including partial errors.
	// The legacy dial callback cannot be forcibly canceled by this wrapper.
	<-c.hopDone
	c.receiveWG.Wait()
	c.callWG.Wait()
	for {
		select {
		case packet := <-c.packetChan:
			packet.Release()
		default:
			c.access.Lock()
			c.closeErr = errors.Join(closeErr, c.failure)
			c.access.Unlock()
			close(c.closeDone)
			return
		}
	}
}

func (c *HopPacketConn) discardHop(original *hopSocket) {
	original.startClose()
	closeErr := original.waitClosed()
	c.access.Lock()
	if c.pendingConn == original && closeErr == nil {
		c.pendingConn = nil
	}
	c.access.Unlock()
	if closeErr != nil {
		// Keep the original negative result and socket; deny further hops.
		c.beginClose(closeErr)
	}
}

type hopControl uint8

const (
	hopSetWriteDeadline hopControl = iota
	hopSetReadBuffer
	hopSetWriteBuffer
)

func (c *HopPacketConn) setControl(kind hopControl, deadline time.Time, size int) error {
	if err := c.beginControlCall(); err != nil {
		return err
	}
	returned := false
	defer func() { c.endControlCall(returned) }()
	// Lock order is control -> access. Admission takes access alone and releases
	// it before waiting for control. Original socket abort takes access alone.
	c.control.Lock()
	defer c.control.Unlock()
	c.access.Lock()
	if c.done {
		c.access.Unlock()
		returned = true
		return net.ErrClosed
	}
	switch kind {
	case hopSetWriteDeadline:
		c.writeDeadline, c.writeDeadlineSet = deadline, true
	case hopSetReadBuffer:
		c.readBufferSize = size
	case hopSetWriteBuffer:
		c.writeBufferSize = size
	}
	// A retiring socket already has an independent original Close. Do not
	// introduce spurious ErrClosed from it into a live control operation.
	// Preserve the upstream result contract: return the current socket's
	// outcome; previous/pending controls do not certify successful retirement.
	originals := [3]*hopSocket{c.prevConn, c.pendingConn, c.currentConn}
	c.access.Unlock()
	var err error
	for _, original := range originals {
		if original == nil {
			continue
		}
		var callErr error
		switch kind {
		case hopSetWriteDeadline:
			callErr = original.conn.SetWriteDeadline(deadline)
		case hopSetReadBuffer:
			callErr = trySetReadBuffer(original.conn, size)
		case hopSetWriteBuffer:
			callErr = trySetWriteBuffer(original.conn, size)
		}
		if original == originals[2] {
			err = callErr
		}
	}
	returned = true
	return err
}
