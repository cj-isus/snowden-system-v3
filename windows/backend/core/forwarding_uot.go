// UoT orchestration/framing adapted from sing-box v1.13.19/common/uot/router.go
// and sing v0.8.13/common/uot/conn.go. Copyright (C) 2022 by nekohasekai
// <contact-sagernet@sekai.icu>. SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

type ownedUotRouter struct{ router admissionRouter }

func uotDestination(metadata adapter.InboundContext) bool {
	return metadata.Destination.Fqdn == uot.MagicAddress || metadata.Destination.Fqdn == uot.LegacyMagicAddress
}

func (router *ownedUotRouter) prepare(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) (*ownedUotConnection, adapter.InboundContext, error) {
	if ctx == nil || conn == nil || contextError(ctx) != nil {
		return nil, metadata, ErrForwardingNotAdmitted
	}
	input, ok := ctx.Value(mixedInputKey{}).(*mixedIngress)
	if !ok || input == nil || input.gate != router.router.admission || input.ctx.Err() != nil {
		return nil, metadata, ErrForwardingNotAdmitted
	}
	metadata.Network = N.NetworkUDP
	if err := router.router.allowedPacket(metadata); err != nil {
		return nil, metadata, err // early negative gate, not a permit; registration rechecks afterwards
	}
	magic := metadata.Destination.Fqdn
	request := uot.Request{}
	if magic == uot.MagicAddress {
		parsed, err := uot.ReadRequest(conn) // existing bounded wire codec, original input/handler owns this read
		if err != nil {
			return nil, metadata, err
		}
		request = *parsed
		metadata.Destination = request.Destination
	} else {
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
	}
	metadata.Domain = magic
	return &ownedUotConnection{conn: conn, gate: input.gate, request: request}, metadata, nil
}

func (router *ownedUotRouter) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if !uotDestination(metadata) {
		return router.router.RouteConnection(ctx, conn, metadata)
	}
	packet, metadata, err := router.prepare(ctx, conn, metadata)
	if err != nil {
		return router.router.rejectIngress(conn, nil, err)
	}
	return router.router.RoutePacketConnection(ctx, packet, metadata)
}
func (router *ownedUotRouter) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	if !uotDestination(metadata) {
		router.router.RouteConnectionEx(ctx, conn, metadata, callback)
		return
	}
	packet, metadata, err := router.prepare(ctx, conn, metadata)
	if err != nil {
		_ = router.router.rejectIngress(conn, callback, err)
		return
	}
	router.router.RoutePacketConnectionEx(ctx, packet, metadata, callback)
}
func (router *ownedUotRouter) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return router.router.RoutePacketConnection(ctx, conn, metadata)
}
func (router *ownedUotRouter) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	router.router.RoutePacketConnectionEx(ctx, conn, metadata, callback)
}

// This adapter creates no socket/helper goroutine. Original ingress plus the
// ordinary forwarding route retain its TCP resource; each original write retains
// its own input/frame until their actual retirement. No vector/raw escape.
type ownedUotConnection struct {
	conn                                       net.Conn
	gate                                       *forwardingAdmission
	request                                    uot.Request
	readMu, writeMu                            sync.Mutex // independent stream-direction serialization; never acquired by Close
	mu                                         sync.Mutex // values only, no native IO/callback work
	closing, protocolFailed                    atomic.Bool
	pendingWrite                               *uotWrite
	closeStarted, closeFinished, closeReturned bool
	ingressCloseReturned                       bool
	ingressCloseErr                            error
	closeErr                                   error
}

func (conn *ownedUotConnection) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, finalErr error) {
	if buffer == nil {
		return destination, ErrForwardingNotAdmitted
	}
	conn.readMu.Lock()
	defer conn.readMu.Unlock()
	if conn.closing.Load() || conn.protocolFailed.Load() {
		return destination, ErrForwardingNotAdmitted
	}
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault || finalErr != nil {
			conn.protocolFailed.Store(true) // never continue at an unknown/partial frame boundary
		}
		if fault {
			conn.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	var err error
	if conn.request.IsConnect {
		destination = conn.request.Destination
	} else {
		destination, err = uot.AddrParser.ReadAddrPort(conn.conn)
		if err != nil {
			returned = true
			return destination, err
		}
	}
	var size [2]byte
	if _, err = io.ReadFull(conn.conn, size[:]); err != nil {
		returned = true
		return destination, err
	}
	length := int(binary.BigEndian.Uint16(size[:]))
	if buffer.FreeLen() < length {
		returned = true
		return destination, io.ErrShortBuffer
	}
	_, err = buffer.ReadFullFrom(conn.conn, length)
	returned = true
	if err != nil {
		return M.Socksaddr{}, err
	}
	return destination, err
}

func (conn *ownedUotConnection) Close() (finalErr error) {
	conn.closing.Store(true)
	conn.mu.Lock()
	if conn.closeStarted {
		err := conn.closeErr
		if !conn.closeFinished || !conn.closeReturned {
			err = ErrEngineCleanupFailed
		}
		conn.mu.Unlock()
		return err // original outcome only, no native retry or reentrant self-wait
	}
	conn.closeStarted, conn.closeErr = true, ErrEngineCleanupFailed
	write := conn.pendingWrite
	conn.mu.Unlock()
	returned, ingressReturned := false, false
	var ingressErr error
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		conn.mu.Lock()
		conn.closeFinished, conn.closeReturned, conn.ingressCloseReturned, conn.ingressCloseErr, conn.closeErr = true, returned, ingressReturned, ingressErr, finalErr
		conn.mu.Unlock()
		if fault || finalErr != nil {
			conn.gate.noteRouteFault()
		}
	}()
	ingressErr = closeForwardingIngress(conn.gate, conn.conn, nil) // original wrapped ingress close BEFORE writer join
	ingressReturned = true
	var writerErr error
	if write != nil {
		ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
		defer cancel()
		if !mixedOriginalDone(ctx, write.done) {
			writerErr = ErrEngineCleanupFailed
		} else if write.failed {
			writerErr = ErrEngineCleanupFailed
		}
	}
	returned = true
	return errors.Join(ingressErr, writerErr)
}

func (conn *ownedUotConnection) HandshakeFailure(cause error) error {
	conn.protocolFailed.Store(true)
	// An error callback must not wait for a data writer that needs this very
	// callback's cancellation/Close to unblock. Preserve the original reporter.
	if reporter, ok := common.Cast[N.HandshakeFailure](conn.conn); ok {
		return reporter.HandshakeFailure(cause) // protocol reporting only, no exposed raw/vector writer
	}
	return nil
}

func (conn *ownedUotConnection) LocalAddr() net.Addr { return conn.conn.LocalAddr() }
func (conn *ownedUotConnection) SetDeadline(value time.Time) error {
	return conn.conn.SetDeadline(value)
}
func (conn *ownedUotConnection) SetReadDeadline(value time.Time) error {
	return conn.conn.SetReadDeadline(value)
}
func (conn *ownedUotConnection) SetWriteDeadline(value time.Time) error {
	return conn.conn.SetWriteDeadline(value)
}
func (*ownedUotConnection) NeedAdditionalReadDeadline() bool { return true }
func (*ownedUotConnection) ReaderReplaceable() bool          { return false }
func (*ownedUotConnection) WriterReplaceable() bool          { return false }

var (
	_ adapter.ConnectionRouterEx = (*ownedUotRouter)(nil)
	_ N.PacketConn               = (*ownedUotConnection)(nil)
)
