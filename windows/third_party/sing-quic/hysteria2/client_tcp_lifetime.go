package hysteria2

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/sagernet/quic-go/packetaddr"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	maxTCPConnections = 256
	// Allow the ordinary simultaneous Read+Write pair on every visible stream.
	maxTCPCalls       = maxTCPConnections * 2
	maxTCPStreamCalls = 8
	maxTCPControls    = 8
	maxTCPStaging     = 8
	maxTCPFirstData   = 16 << 10
	tcpStageCharge    = 64 << 10
	maxTCPStageBytes  = maxTCPStaging * tcpStageCharge
)

var (
	errTCPAdmission = errors.New("HY2 TCP operation capacity exceeded")
	errTCPStaging   = errors.New("HY2 TCP initial-write staging capacity exceeded")
	errTCPWriteSize = errors.New("HY2 TCP stream returned invalid write size")
	errTCPAddress   = net.InvalidAddrError("invalid or oversized HY2 TCP destination")
)

// Caller holds lifeAccess. A poisoned owner settles negatively without deleting
// its original references or refunding its uncertain staging/stream capacity.
func (c *clientQUICConnection) settleTCPLocked() {
	if c.closing && !c.tcpReported && (c.tcpFailure != nil || len(c.tcpStreams) == 0) {
		c.tcpReported = true
		close(c.tcpDone)
	}
}

func (c *clientConn) settleIdleLocked() {
	if c.closing && !c.opening && c.calls == 0 && c.controls == 0 && !c.idleReported {
		c.idleReported = true
		close(c.idle)
	}
}

func (c *clientConn) sealLocked() {
	if !c.closing {
		c.closing = true
		close(c.done)
	}
	c.settleIdleLocked()
}

func (c *clientQUICConnection) sealTCPLocked() [maxTCPConnections]*clientConn {
	var originals [maxTCPConnections]*clientConn
	i := 0
	for original := range c.tcpStreams {
		original.sealLocked()
		originals[i] = original
		i++
	}
	c.settleTCPLocked()
	return originals
}

func (c *clientConn) requestClose() {
	c.owner.lifeAccess.Lock()
	c.sealLocked()
	c.owner.lifeAccess.Unlock()
	c.startClose()
}

func (c *clientConn) startClose() {
	c.closeOnce.Do(func() { go c.finishClose() })
}

func (c *clientConn) finishClose() {
	var closeErr error
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			closeErr = errors.Join(closeErr, errClientUncertain)
		}
		c.owner.lifeAccess.Lock()
		if !returned || c.poisoned {
			c.poisoned = true
			c.owner.tcpFailure = errClientUncertain
			closeErr = errors.Join(closeErr, errClientUncertain)
		} else {
			delete(c.owner.tcpStreams, c)
		}
		c.closeErr = closeErr
		close(c.closed)
		c.owner.settleTCPLocked()
		uncertain := c.poisoned
		c.owner.lifeAccess.Unlock()
		if uncertain {
			c.owner.markUncertain()
		}
	}()
	<-c.opened
	if c.Stream != nil {
		// Control -> lifeAccess order. Previously admitted setters recheck seal
		// after taking control, so a later deadline cannot undo this wakeup.
		func() {
			c.control.Lock()
			defer c.control.Unlock()
			c.Stream.CancelRead(0)
			closeErr = c.Stream.SetWriteDeadline(time.Now())
		}()
	}
	<-c.idle
	c.owner.lifeAccess.Lock()
	uncertain := c.poisoned
	c.owner.lifeAccess.Unlock()
	if uncertain {
		returned = true
		return
	}
	// No admitted core Read/Write/Peek/control call still borrows these buffers.
	if c.header != nil {
		c.header.Release()
		c.header = nil
	}
	c.response = protocol.TCPResponseReader{}
	if c.Stream != nil {
		select {
		case <-c.Stream.Context().Done():
			// Already closed/reset write-half: Close-after-CancelWrite is invalid.
			// A known protocol cancellation is not a native cleanup uncertainty.
		default:
			closeErr = errors.Join(closeErr, c.Stream.Close())
		}
	}
	returned = true
}

func (c *clientConn) beginCall(control bool) error {
	c.owner.lifeAccess.Lock()
	defer c.owner.lifeAccess.Unlock()
	if c.poisoned || c.owner.tcpFailure != nil || c.owner.failure != nil {
		return errClientUncertain
	}
	if c.owner.closing || c.closing {
		return net.ErrClosed
	}
	if control {
		if c.owner.tcpControls >= maxTCPControls || c.controls >= 2 {
			return errTCPAdmission
		}
		c.controls++
		c.owner.tcpControls++
	} else {
		if c.owner.tcpCalls >= maxTCPCalls || c.calls >= maxTCPStreamCalls {
			return errTCPAdmission
		}
		c.calls++
		c.owner.tcpCalls++
	}
	return nil
}

// A serialized operation must seal uncertain state before unlocking its
// direction mutex. Otherwise an already admitted waiter could reuse its data
// between mutex release and the deferred call-accounting update.
func (c *clientConn) poisonBeforeUnlock() {
	c.owner.lifeAccess.Lock()
	c.poisoned = true
	c.owner.tcpFailure = errClientUncertain
	c.sealLocked()
	c.owner.settleTCPLocked()
	c.owner.lifeAccess.Unlock()
}

func (c *clientConn) endCall(control, returned bool) {
	c.owner.lifeAccess.Lock()
	if !returned {
		c.poisoned = true
		c.owner.tcpFailure = errClientUncertain
		c.sealLocked()
	}
	if control {
		c.controls--
		c.owner.tcpControls--
	} else {
		c.calls--
		c.owner.tcpCalls--
	}
	c.settleIdleLocked()
	c.owner.settleTCPLocked()
	c.owner.lifeAccess.Unlock()
	if !returned {
		c.owner.markUncertain()
	}
}

func (c *clientConn) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *clientConn) reserveStaging() error {
	c.owner.lifeAccess.Lock()
	defer c.owner.lifeAccess.Unlock()
	if c.poisoned || c.owner.tcpFailure != nil || c.owner.failure != nil {
		return errClientUncertain
	}
	if c.owner.closing || c.closing {
		return net.ErrClosed
	}
	if c.owner.tcpStaging >= maxTCPStaging || tcpStageCharge > maxTCPStageBytes-c.owner.tcpStageBytes {
		return errTCPStaging
	}
	c.owner.tcpStaging++
	c.owner.tcpStageBytes += tcpStageCharge
	return nil
}

func (c *clientConn) releaseStaging() {
	c.owner.lifeAccess.Lock()
	c.owner.tcpStaging--
	c.owner.tcpStageBytes -= tcpStageCharge
	c.owner.lifeAccess.Unlock()
}

func (c *clientQUICConnection) dialTCP(ctx context.Context, destination M.Socksaddr) (result net.Conn, err error) {
	ctxDone := ctx.Done()
	c.lifeAccess.Lock()
	if c.tcpFailure != nil || c.failure != nil {
		c.lifeAccess.Unlock()
		return nil, errClientUncertain
	}
	if c.closing {
		c.lifeAccess.Unlock()
		return nil, net.ErrClosed
	}
	if len(c.tcpStreams) >= maxTCPConnections || c.tcpCalls >= maxTCPCalls {
		c.lifeAccess.Unlock()
		return nil, errTCPAdmission
	}
	original := &clientConn{owner: c, opening: true, calls: 1,
		opened: make(chan struct{}), idle: make(chan struct{}), done: make(chan struct{}), closed: make(chan struct{})}
	c.tcpStreams[original] = struct{}{}
	c.tcpCalls++
	c.lifeAccess.Unlock()
	returned := false
	defer func() {
		c.lifeAccess.Lock()
		original.opening = false
		close(original.opened)
		c.lifeAccess.Unlock()
		original.endCall(false, returned)
		if result == nil {
			original.requestClose()
		}
	}()
	// Typed fields are bounded before String/Clone, and the stream reservation
	// precedes all destination, metadata and OpenStream work.
	if !destination.IsValid() || destination.Port == 0 || destination.IsIP() && destination.Fqdn != "" ||
		len(destination.Fqdn) > protocol.MaxAddressLength || len(destination.Addr.Zone()) > 64 ||
		strings.IndexFunc(destination.Fqdn, unicode.IsControl) >= 0 || strings.IndexFunc(destination.Addr.Zone(), unicode.IsControl) >= 0 {
		returned = true
		return nil, errTCPAddress
	}
	destination = destination.Unwrap()
	address := destination.String()
	if len(address) > protocol.MaxAddressLength {
		returned = true
		return nil, errTCPAddress
	}
	destination.Fqdn = strings.Clone(destination.Fqdn)
	original.destination, original.address = destination, address
	local, metadataErr := packetaddr.Capture(c.quicConn.LocalAddr())
	if metadataErr != nil {
		returned = true
		return nil, metadataErr
	}
	original.local = local
	select {
	case <-ctxDone:
		returned = true
		err = ctx.Err()
		if err == nil {
			err = context.Canceled
		}
		return nil, err
	default:
	}
	stream, openErr := c.quicConn.OpenStream()
	c.lifeAccess.Lock()
	original.Stream = stream // partial/late result remains in the original owner
	canPublish := !c.closing && !original.closing
	select {
	case <-ctxDone:
		canPublish = false
	default:
	}
	c.lifeAccess.Unlock()
	returned = true
	if openErr != nil {
		return nil, openErr
	}
	select {
	case <-ctxDone:
		err = ctx.Err()
		if err == nil {
			err = context.Canceled
		}
		return nil, err
	default:
	}
	if stream == nil || !canPublish {
		return nil, net.ErrClosed
	}
	return original, nil
}
