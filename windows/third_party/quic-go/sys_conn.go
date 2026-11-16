package quic

import (
	"io"
	"net"
	"syscall"
	"time"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/packetaddr"
)

type connCapabilities struct {
	// This connection has the Don't Fragment (DF) bit set.
	// This means it makes to run DPLPMTUD.
	DF bool
	// GSO (Generic Segmentation Offload) supported
	GSO bool
	// ECN (Explicit Congestion Notifications) supported
	ECN bool
}

// rawConn is a connection that allow reading of a receivedPackeh.
type rawConn interface {
	ReadPacket() (receivedPacket, error)
	// WritePacket writes a packet on the wire.
	// gsoSize is the size of a single packet, or 0 to disable GSO.
	// It is invalid to set gsoSize if capabilities.GSO is not set.
	WritePacket(b []byte, addr net.Addr, packetInfoOOB []byte, gsoSize uint16, ecn protocol.ECN) (int, error)
	LocalAddr() net.Addr
	SetReadDeadline(time.Time) error
	io.Closer

	capabilities() connCapabilities
}

// OOBCapablePacketConn is a connection that allows the reading of ECN bits from the IP header.
// If the PacketConn passed to the [Transport] satisfies this interface, quic-go will use it.
// In this case, ReadMsgUDP() will be used instead of ReadFrom() to read packets.
type OOBCapablePacketConn interface {
	net.PacketConn
	SyscallConn() (syscall.RawConn, error)
	SetReadBuffer(int) error
	ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error)
	WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error)
}

var _ OOBCapablePacketConn = &net.UDPConn{}

// One embedded original-initializer prefix. The initOnce owner alone writes
// these fields; root cleanup joins initDone before observing its result. Known
// return clears the prefix; abnormal provider exit keeps every attached value.
type transportConnOpening struct {
	socket  net.PacketConn
	address net.Addr
	local   packetaddr.Snapshot
	control syscall.RawConn
	wrapped rawConn
}

func wrapConn(opening *transportConnOpening) (rawConn, error) {
	pc := opening.socket // the same original socket that the abort owner captured
	if pc == nil {
		return nil, packetaddr.ErrInvalid
	}
	opening.address = pc.LocalAddr()
	local, err := packetaddr.Capture(opening.address)
	if err != nil {
		return nil, err
	}
	opening.local = local
	_ = setReceiveBuffer(pc, opening)
	_ = setSendBuffer(pc, opening)

	conn, ok := pc.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	// An opaque provider or a non-UDP address doesn't prove native DF setup.
	// Only an observed successful setDF operation enables PMTU probing.
	var supportsDF bool
	if ok {
		opening.control, err = conn.SyscallConn()
		if err != nil {
			return nil, err
		}
		if opening.control == nil {
			return nil, packetaddr.ErrInvalid
		}

		// only set DF on UDP sockets
		if _, ok := local.Addr().(*net.UDPAddr); ok {
			var err error
			supportsDF, err = setDF(opening.control)
			if err != nil {
				return nil, err
			}
		}
	}
	c, ok := pc.(OOBCapablePacketConn)
	if !ok {
		opening.wrapped = &basicConn{PacketConn: pc, supportsDF: supportsDF}
		utils.DefaultLogger.Infof("PacketConn is not a net.UDPConn. Disabling optimizations possible on UDP connections.")
	} else {
		wrapped, err := newConn(c, supportsDF)
		opening.wrapped = wrapped // Windows constructor has no provider callbacks
		if err != nil {
			return nil, err
		}
	}
	opening.wrapped = &localAddrConn{rawConn: opening.wrapped, local: opening.local}
	return opening.wrapped, nil
}

// Transport-owned local metadata remains usable after native Close. The rawConn
// interface is preserved, including capabilities and the original close owner.
type localAddrConn struct {
	rawConn
	local packetaddr.Snapshot
}

func (c *localAddrConn) LocalAddr() net.Addr { return c.local.Addr() }

// The basicConn is the most trivial implementation of a rawConn.
// It reads a single packet from the underlying net.PacketConn.
// It is used when
// * the net.PacketConn is not a OOBCapablePacketConn, and
// * when the OS doesn't support OOB.
type basicConn struct {
	net.PacketConn
	supportsDF bool
	read       packetReadState
}

var _ rawConn = &basicConn{}

func (c *basicConn) WritePacket(b []byte, addr net.Addr, _ []byte, gsoSize uint16, ecn protocol.ECN) (n int, err error) {
	if gsoSize != 0 {
		panic("cannot use GSO with a basicConn")
	}
	if ecn != protocol.ECNUnsupported {
		panic("cannot use ECN with a basicConn")
	}
	return c.WriteTo(b, addr)
}

func (c *basicConn) capabilities() connCapabilities { return connCapabilities{DF: c.supportsDF} }
