// Mixed dispatch adapted from sing-box v1.13.19/protocol/mixed/inbound.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"bufio"
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

// Only orchestration lives here; authentication and SOCKS/HTTP framing retain
// their pinned SDK implementations. PacketListener is the acquisition boundary
// that stock mixed hides inside its own listener.
type ownedMixedProcessor struct {
	router        adapter.ConnectionRouterEx
	logger        log.ContextLogger
	tag           string
	authenticator *auth.Authenticator
	udpTimeout    time.Duration
	packets       *ownedMixedInbound
}

func newOwnedMixedProcessor(router admissionRouter, logger log.ContextLogger, tag string, options option.HTTPMixedInboundOptions, packets *ownedMixedInbound) *ownedMixedProcessor {
	udpTimeout := time.Duration(options.UDPTimeout)
	if udpTimeout == 0 {
		udpTimeout = C.UDPTimeout
	}
	return &ownedMixedProcessor{router: &ownedUotRouter{router: router}, logger: logger, tag: tag,
		authenticator: auth.NewAuthenticator(options.Users), udpTimeout: udpTimeout, packets: packets}
}

func (processor *ownedMixedProcessor) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	err := processor.handshake(ctx, conn, metadata, callback)
	_ = N.CloseOnHandshakeFailure(conn, callback, err)
	if err != nil {
		// The pinned auth parsers can include supplied credentials in errors.
		// Keep finite local categories; never log their raw error payload.
		if E.IsClosedOrCanceled(err) {
			processor.logger.DebugContext(ctx, "mixed handshake closed")
		} else {
			processor.logger.ErrorContext(ctx, "mixed handshake rejected")
		}
	}
}

func (processor *ownedMixedProcessor) handshake(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) error {
	source, err := newMixedHandshakeReader(ctx, conn)
	if err != nil {
		return err
	}
	reader := bufio.NewReaderSize(source, mixedHandshakeReadSize)
	var header []byte
	err = withMixedHandshakeBudget(ctx, conn, time.Time{}, func() error {
		var err error
		header, err = reader.Peek(1)
		return err
	})
	if err != nil {
		return err
	}
	if header[0] == socks5.Version {
		return processor.socks5(ctx, conn, reader, metadata, callback)
	}
	if header[0] == socks4.Version {
		return processor.socks4(ctx, conn, reader, metadata, callback)
	}
	return processor.http(ctx, conn, reader, source, metadata, callback)
}

func (processor *ownedMixedProcessor) metadata(ctx context.Context, metadata adapter.InboundContext) adapter.InboundContext {
	metadata.Inbound, metadata.InboundType = processor.tag, C.TypeMixed
	if user, loaded := auth.UserFromContext[string](ctx); loaded {
		metadata.User = user
	}
	return metadata
}

func (processor *ownedMixedProcessor) connection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	metadata = processor.metadata(ctx, metadata)
	processor.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	processor.router.RouteConnectionEx(ctx, conn, metadata, callback)
}

func (processor *ownedMixedProcessor) packet(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	metadata = processor.metadata(ctx, metadata)
	processor.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	processor.router.RoutePacketConnectionEx(ctx, conn, metadata, callback)
}

var _ adapter.ConnectionHandlerEx = (*ownedMixedProcessor)(nil)
