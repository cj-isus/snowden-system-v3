// DNS packet orchestration adapted from sing-box v1.13.19/protocol/dns/handle.go
// and dns/client_truncate.go. Copyright (C) 2022 by nekohasekai
// <contact-sagernet@sekai.icu>. SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"context"
	"io"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const forwardingDNSPacketHeadroom = 1024 // same room on each side as the pinned DNS packet handler

// Known private owners only. No SDK Upstream/cached/read-waiter unwrapping or
// reclassification of TUN/native packets as a locally owned association.
func dnsOriginalPacketConnection(raw io.Closer) *mixedIngressConnection {
	switch conn := raw.(type) {
	case *mixedAssociationConnection:
		if conn != nil && conn.association != nil && conn.association.packet != nil {
			original := dnsOriginalMixedConnection(conn.association.tcp)
			if original != nil && original.owner == conn.association.packet.input {
				return original
			}
		}
	case *ownedUotConnection:
		if conn != nil {
			original := dnsOriginalMixedConnection(conn.conn)
			if original != nil && original.owner != nil && original.owner.gate == conn.gate {
				return original
			}
		}
	}
	return nil
}

func (router admissionRouter) dispatchPacket(route *forwardingRoute, ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	original := dnsOriginalPacketConnection(route.raw)
	if !route.gate.ownedDNSPort53 || metadata.Destination.Port != 53 || metadata.InboundDetour != "" || original == nil {
		router.Router.RoutePacketConnectionEx(ctx, conn, metadata, callback)
		return
	}
	matched, err := router.firstDNSMetadata(metadata, N.NetworkUDP)
	if err != nil {
		_ = N.CloseOnHandshakeFailure(conn, callback, err)
		return
	}
	dns := service.FromContext[adapter.DNSRouter](ctx)
	scope, err := newForwardingDNSScope(route, original, nil, conn, dns, matched)
	if err == nil {
		err = scope.serve()
	}
	_ = N.CloseOnHandshakeFailure(conn, callback, err)
}

func (scope *forwardingDNSScope) responseOverhead() int {
	if scope.packet != nil {
		return 2 * forwardingDNSPacketHeadroom
	}
	return 2 // TCP length prefix
}

func (scope *forwardingDNSScope) servePackets() error {
	if err := scope.startIdle(); err != nil {
		return err
	}
	for {
		// The original activity watcher covers read/capacity idle time. A query's
		// own operation window starts after its datagram, not before the read.
		query, err := scope.reserve(maxForwardingDNSMessage)
		if err != nil {
			return err
		}
		if err := query.consume(time.Time{}, nil); err != nil {
			return err
		}
	}
}

func (query *forwardingDNSQuery) receivePacket() ([]byte, error) {
	scope := query.scope
	query.readPacket = buf.With(query.wire) // unmanaged; all raw bytes remain owned by this query
	destination, err := scope.packet.ReadPacket(query.readPacket)
	query.destination = destination
	if err != nil {
		return nil, err
	}
	if !destination.IsValid() {
		return nil, errForwardingDNSProtocol
	}
	if err := scope.idle.touch(); err != nil {
		return nil, err
	}
	query.budget, err = newMixedHandshakeBudget(scope.route.ctx, scope.original, time.Now().Add(C.DNSTimeout))
	if err != nil {
		return nil, err // original partial timer remains input-owned
	}
	return query.readPacket.Bytes(), nil
}

func (query *forwardingDNSQuery) packetLimit() int {
	limit := 512
	if option := query.request.IsEdns0(); option != nil && int(option.UDPSize()) > limit {
		limit = int(option.UDPSize())
	}
	return limit
}

func (scope *forwardingDNSScope) writePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	scope.writeMu.Lock()
	defer scope.writeMu.Unlock() // original raw Close never waits here before closing TCP/UDP
	return scope.packet.WritePacket(buffer, destination)
}
