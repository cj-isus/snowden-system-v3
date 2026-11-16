package quic

import (
	"errors"
	"io"
	"net"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/packetaddr"
)

// A sendConn allows sending using a simple Write() on a non-connected packet conn.
type sendConn interface {
	Write(b []byte, gsoSize uint16, ecn protocol.ECN) error
	WriteTo([]byte, net.Addr) error
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetRemoteAddr(addr net.Addr)
	ChangeRemoteAddr(addr net.Addr, info packetInfo)

	capabilities() connCapabilities
}

type remoteAddrInfo struct {
	addr net.Addr
	oob  []byte
}

// Embedded in the admitted client/server/path opening. That original caller
// retains partial metadata and the sender through every provider callback.
type sendConnOpening struct {
	raw             rawConn
	remote          net.Addr
	local, adjusted net.Addr
	oob             []byte
	conn            *sconn
}

type sconn struct {
	rawConn

	localAddr packetaddr.Snapshot

	remoteAddrInfo atomic.Pointer[remoteAddrInfo]

	logger utils.Logger

	// If GSO enabled, and we receive a GSO error for this remote address, GSO is disabled.
	gotGSOError atomic.Bool
	// Used to catch the error sometimes returned by the first sendmsg call on Linux,
	// see https://github.com/golang/go/issues/63322.
	wroteFirstPacket bool
}

var _ sendConn = &sconn{}

func newSendConn(opening *sendConnOpening, c rawConn, remote net.Addr, info packetInfo, logger utils.Logger) (*sconn, error) {
	*opening = sendConnOpening{raw: c, remote: remote}
	if c == nil {
		return nil, packetaddr.ErrInvalid
	}
	sc := &sconn{rawConn: c, logger: logger}
	opening.conn = sc // before the first LocalAddr / Capture provider callback
	opening.local = c.LocalAddr()
	opening.adjusted = opening.local
	if info.addr.IsValid() {
		if udpAddr, ok := opening.local.(*net.UDPAddr); ok && udpAddr != nil {
			addrCopy := *udpAddr
			addrCopy.IP = info.addr.AsSlice()
			opening.adjusted = &addrCopy
		}
	}
	local, err := packetaddr.Capture(opening.adjusted)
	if err != nil {
		return nil, err
	}
	sc.localAddr = local

	opening.oob = info.OOB()
	// increase oob slice capacity, so we can add the UDP_SEGMENT and ECN control messages without allocating
	l := len(opening.oob)
	opening.oob = append(opening.oob, make([]byte, 64)...)[:l]
	sc.remoteAddrInfo.Store(&remoteAddrInfo{
		addr: remote,
		oob:  opening.oob,
	})
	return sc, nil
}

func (c *sconn) Write(p []byte, gsoSize uint16, ecn protocol.ECN) error {
	ai := c.remoteAddrInfo.Load()
	err := c.writePacket(p, ai.addr, ai.oob, gsoSize, ecn)
	if err != nil && gsoSize > 0 && !errors.Is(err, io.ErrShortWrite) && isGSOError(err) {
		// disable GSO for future calls
		c.gotGSOError.Store(true)
		if c.logger.Debug() {
			c.logger.Debugf("GSO failed when sending to %s", ai.addr)
		}
		// send out the packets one by one
		for len(p) > 0 {
			l := len(p)
			if l > int(gsoSize) {
				l = int(gsoSize)
			}
			if err := c.writePacket(p[:l], ai.addr, ai.oob, 0, ecn); err != nil {
				return err
			}
			p = p[l:]
		}
		return nil
	}
	return err
}

func (c *sconn) writePacket(p []byte, addr net.Addr, oob []byte, gsoSize uint16, ecn protocol.ECN) error {
	n, err := c.WritePacket(p, addr, oob, gsoSize, ecn)
	if n == 0 && err != nil && !c.wroteFirstPacket && isPermissionError(err) {
		n, err = c.WritePacket(p, addr, oob, gsoSize, ecn)
	}
	c.wroteFirstPacket = true
	return packetWriteResult(n, len(p), err)
}

func (c *sconn) WriteTo(b []byte, addr net.Addr) error {
	n, err := c.WritePacket(b, addr, nil, 0, protocol.ECNUnsupported)
	return packetWriteResult(n, len(b), err)
}

func packetWriteResult(n, wanted int, err error) error {
	if n < 0 || n > wanted {
		return errPacketOutputUncertain
	}
	if n > 0 && err != nil {
		// A provider reporting bytes plus an error must not trigger replay of
		// that prefix through GSO fallback. Preserve the original failure too.
		return errors.Join(io.ErrShortWrite, err)
	}
	if err == nil && n != wanted {
		return io.ErrShortWrite
	}
	return err
}

func (c *sconn) capabilities() connCapabilities {
	capabilities := c.rawConn.capabilities()
	if capabilities.GSO {
		capabilities.GSO = !c.gotGSOError.Load()
	}
	return capabilities
}

func (c *sconn) ChangeRemoteAddr(addr net.Addr, info packetInfo) {
	c.remoteAddrInfo.Store(&remoteAddrInfo{
		addr: addr,
		oob:  info.OOB(),
	})
}

func (c *sconn) SetRemoteAddr(addr net.Addr) {
	for {
		ai := c.remoteAddrInfo.Load()                    // load the current value of the pointer, so we can swap it
		newAi := *ai                                     // make a copy, so we can swap the pointer
		newAi.addr = addr                                // update the address
		if c.remoteAddrInfo.CompareAndSwap(ai, &newAi) { // if the pointer was swapped, we're done
			break
		}
	}
}

func (c *sconn) RemoteAddr() net.Addr { return c.remoteAddrInfo.Load().addr }
func (c *sconn) LocalAddr() net.Addr  { return c.localAddr.Addr() }
