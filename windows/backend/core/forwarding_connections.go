package core

import (
	"net"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio/deadline"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// Preserve protocol handshakes without exposing raw ingress through Upstream.
// Native/lazy controls serialize with Close; the value mutex stays unlocked.
type forwardingHandshake struct{ route *forwardingRoute }

func (handshake forwardingHandshake) HandshakeSuccess() error {
	return handshake.route.control(func() error { return N.ReportHandshakeSuccess(handshake.route.raw) })
}

func (handshake forwardingHandshake) ConnHandshakeSuccess(peer net.Conn) error {
	return handshake.route.control(func() error { return N.ReportConnHandshakeSuccess(handshake.route.raw, peer) })
}

func (handshake forwardingHandshake) PacketConnHandshakeSuccess(peer net.PacketConn) error {
	return handshake.route.control(func() error { return N.ReportPacketConnHandshakeSuccess(handshake.route.raw, peer) })
}

func (handshake forwardingHandshake) HandshakeFailure(cause error) error {
	return handshake.route.control(func() error {
		if reporter, ok := common.Cast[N.HandshakeFailure](handshake.route.raw); ok {
			return reporter.HandshakeFailure(cause)
		}
		return nil
	})
}

type forwardingTCPConnection struct {
	forwardingHandshake
	conn net.Conn
}

func (connection *forwardingTCPConnection) Read(buffer []byte) (n int, err error) {
	err = connection.route.perform(func() error {
		n, err = connection.conn.Read(buffer)
		return err
	})
	return
}

func (connection *forwardingTCPConnection) Write(buffer []byte) (n int, err error) {
	err = connection.route.perform(func() error {
		n, err = connection.conn.Write(buffer)
		return err
	})
	return
}

func (connection *forwardingTCPConnection) Close() error         { return connection.route.closeOriginal() }
func (connection *forwardingTCPConnection) LocalAddr() net.Addr  { return connection.conn.LocalAddr() }
func (connection *forwardingTCPConnection) RemoteAddr() net.Addr { return connection.conn.RemoteAddr() }

func (connection *forwardingTCPConnection) SetDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetDeadline(value) })
}

func (connection *forwardingTCPConnection) SetReadDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetReadDeadline(value) })
}

func (connection *forwardingTCPConnection) SetWriteDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetWriteDeadline(value) })
}

func (connection *forwardingTCPConnection) NeedHandshakeForRead() (needed bool) {
	_ = connection.route.control(func() error { needed = N.NeedHandshakeForRead(connection.conn); return nil })
	return
}

func (connection *forwardingTCPConnection) NeedHandshakeForWrite() (needed bool) {
	_ = connection.route.control(func() error { needed = N.NeedHandshakeForWrite(connection.conn); return nil })
	return
}

func (connection *forwardingTCPConnection) NeedAdditionalReadDeadline() (needed bool) {
	_ = connection.route.control(func() error { needed = deadline.NeedAdditionalReadDeadline(connection.conn); return nil })
	return
}

// SDK copy/cast optimizations must not bypass the per-entry I/O/Close receipt.
func (*forwardingTCPConnection) ReaderReplaceable() bool { return false }
func (*forwardingTCPConnection) WriterReplaceable() bool { return false }

type forwardingPacketConnection struct {
	forwardingHandshake
	conn N.PacketConn
}

func (connection *forwardingPacketConnection) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if buffer == nil {
		return destination, ErrForwardingNotAdmitted
	}
	err = connection.route.perform(func() error {
		destination, err = connection.conn.ReadPacket(buffer)
		return err
	})
	return
}

func (connection *forwardingPacketConnection) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) (finalErr error) {
	if buffer == nil {
		return ErrForwardingNotAdmitted
	}
	delegated := false
	defer func() {
		if !delegated {
			finalErr = connection.route.gate.rejectPacketBuffer(buffer, finalErr)
		}
	}()
	return connection.route.perform(func() error {
		delegated = true // do not guess/release again after the original writer
		return connection.conn.WritePacket(buffer, destination)
	})
}

func (connection *forwardingPacketConnection) Close() error { return connection.route.closeOriginal() }
func (connection *forwardingPacketConnection) LocalAddr() net.Addr {
	return connection.conn.LocalAddr()
}

func (connection *forwardingPacketConnection) SetDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetDeadline(value) })
}

func (connection *forwardingPacketConnection) SetReadDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetReadDeadline(value) })
}

func (connection *forwardingPacketConnection) SetWriteDeadline(value time.Time) error {
	return connection.route.control(func() error { return connection.conn.SetWriteDeadline(value) })
}

func (connection *forwardingPacketConnection) NeedAdditionalReadDeadline() (needed bool) {
	_ = connection.route.control(func() error { needed = deadline.NeedAdditionalReadDeadline(connection.conn); return nil })
	return
}

func (*forwardingPacketConnection) ReaderReplaceable() bool { return false }
func (*forwardingPacketConnection) WriterReplaceable() bool { return false }

var (
	_ net.Conn                     = (*forwardingTCPConnection)(nil)
	_ N.PacketConn                 = (*forwardingPacketConnection)(nil)
	_ N.ConnHandshakeSuccess       = (*forwardingTCPConnection)(nil)
	_ N.PacketConnHandshakeSuccess = (*forwardingPacketConnection)(nil)
)
