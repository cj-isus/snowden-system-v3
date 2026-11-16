// SOCKS4 orchestration adapted from sing v0.8.13/protocol/socks/handshake.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/auth"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks/socks4"
)

// Keep the pinned request codec (including its 255-byte string limits). Local
// orchestration retires the negotiation timer before deferred reply/route delegation.
func (processor *ownedMixedProcessor) socks4(ctx context.Context, conn net.Conn, reader *bufio.Reader, metadata adapter.InboundContext, callback N.CloseHandlerFunc) error {
	var request socks4.Request
	err := withMixedHandshakeBudget(ctx, conn, time.Time{}, func() error {
		var err error
		request, err = socks4.ReadRequest(reader)
		if err != nil {
			return err
		}
		if request.Command != socks4.CommandConnect || processor.authenticator != nil && !processor.authenticator.Verify(request.Username, "") {
			// The codec writes Addr.AsSlice verbatim. A zero/invalid address
			// would omit the required four bytes of this eight-byte reply.
			response := socks4.Response{ReplyCode: socks4.ReplyCodeRejectedOrFailed, Destination: M.Socksaddr{Addr: netip.IPv4Unspecified()}}
			if err := socks4.WriteResponse(mixedHandshakeWriter{conn}, response); err != nil {
				return err
			}
			return errors.New("SOCKS4 command or authentication rejected") // no supplied credentials
		}
		return nil
	})
	if err != nil {
		return err
	}
	ctx = auth.ContextWithUser(ctx, request.Username)
	buffered, err := retainMixedPrefetch(ctx, conn, reader)
	if err != nil {
		return err // any partial original prefix stays attached to the input
	}
	if request.Destination.IsValid() {
		metadata.Destination = request.Destination
	}
	deferred, err := newMixedSOCKSConnection(ctx, conn, buffered, socks4.Version)
	if err != nil {
		return err
	}
	processor.connection(ctx, deferred, metadata, callback)
	return nil
}
