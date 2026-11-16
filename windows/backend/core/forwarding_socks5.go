// SOCKS5 orchestration adapted from sing v0.8.13/protocol/socks/handshake.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"bufio"
	"context"
	"errors"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	SB "github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

// The pinned codecs/authenticator remain authoritative for wire parsing. Local
// orchestration adds original association task/buffer custody, not a second
// outbound policy or a fallback dialer.
func (processor *ownedMixedProcessor) socks5(ctx context.Context, conn net.Conn, reader *bufio.Reader, metadata adapter.InboundContext, callback N.CloseHandlerFunc) error {
	until := time.Now().Add(mixedHandshakeTimeout) // one window, not renewed for each field/reply
	var command socks5.Request
	err := withMixedHandshakeBudget(ctx, conn, until, func() error {
		var err error
		ctx, command, err = processor.socks5Negotiate(ctx, conn, reader)
		return err
	})
	if err != nil {
		return err
	}
	// The original negotiation timer/callback has retired before either handoff.
	switch command.Command {
	case socks5.CommandConnect:
		buffered, err := retainMixedPrefetch(ctx, conn, reader)
		if err != nil {
			return err
		}
		if command.Destination.IsValid() {
			metadata.Destination = command.Destination
		}
		deferred, err := newMixedSOCKSConnection(ctx, conn, buffered, socks5.Version)
		if err != nil {
			return err
		}
		processor.connection(ctx, deferred, metadata, callback)
		return nil
	case socks5.CommandUDPAssociate:
		return processor.udpAssociation(ctx, conn, metadata, callback, until)
	default:
		return ErrForwardingNotAdmitted // negotiation admits only these two commands
	}
}

func (processor *ownedMixedProcessor) socks5Negotiate(ctx context.Context, conn net.Conn, reader *bufio.Reader) (context.Context, socks5.Request, error) {
	writer := mixedHandshakeWriter{conn} // pinned response codecs otherwise discard short nil-error counts
	version, err := reader.ReadByte()
	if err != nil {
		return ctx, socks5.Request{}, err
	}
	if version != socks5.Version {
		return ctx, socks5.Request{}, ErrForwardingNotAdmitted
	}
	request, err := socks5.ReadAuthRequest0(reader)
	if err != nil {
		return ctx, socks5.Request{}, err
	}
	method := socks5.AuthTypeNotRequired
	if processor.authenticator != nil {
		method = socks5.AuthTypeUsernamePassword
	}
	if !common.Contains(request.Methods, method) {
		if err := socks5.WriteAuthResponse(writer, socks5.AuthResponse{Method: socks5.AuthTypeNoAcceptedMethods}); err != nil {
			return ctx, socks5.Request{}, err
		}
		return ctx, socks5.Request{}, errors.New("SOCKS authentication method rejected") // one rejection, no second negotiation
	}
	if err := socks5.WriteAuthResponse(writer, socks5.AuthResponse{Method: method}); err != nil {
		return ctx, socks5.Request{}, err
	}
	if method == socks5.AuthTypeUsernamePassword {
		credentials, err := socks5.ReadUsernamePasswordAuthRequest(reader)
		if err != nil {
			return ctx, socks5.Request{}, err
		}
		status := socks5.UsernamePasswordStatusFailure
		if processor.authenticator.Verify(credentials.Username, credentials.Password) {
			status = socks5.UsernamePasswordStatusSuccess
		}
		if err := socks5.WriteUsernamePasswordAuthResponse(writer, socks5.UsernamePasswordAuthResponse{Status: status}); err != nil {
			return ctx, socks5.Request{}, err
		}
		if status != socks5.UsernamePasswordStatusSuccess {
			return ctx, socks5.Request{}, errors.New("SOCKS authentication rejected") // credentials never enter this error
		}
		ctx = auth.ContextWithUser(ctx, credentials.Username)
	}
	command, err := socks5.ReadRequest(reader)
	if err != nil {
		return ctx, socks5.Request{}, err
	}
	switch command.Command {
	case socks5.CommandConnect, socks5.CommandUDPAssociate:
		return ctx, command, nil
	default:
		if err := socks5.WriteResponse(writer, socks5.Response{ReplyCode: socks5.ReplyCodeUnsupported}); err != nil {
			return ctx, socks5.Request{}, err
		}
		return ctx, socks5.Request{}, errors.New("SOCKS command unsupported")
	}
}

func (processor *ownedMixedProcessor) udpAssociation(ctx context.Context, tcp net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc, until time.Time) error {
	var packet *mixedPacket
	var socket net.PacketConn
	err := withMixedHandshakeBudget(ctx, tcp, until, func() error {
		local := M.AddrFromNet(tcp.LocalAddr())
		var err error
		packet, socket, err = processor.packets.openOwnedPacket(net.ListenConfig{}, ctx, M.NetworkFromNetAddr("udp", local), M.SocksaddrFrom(local, 0).String())
		return err
	})
	if err != nil {
		return err // the original UDP reservation still owns any partial handle
	}
	association := &mixedAssociation{packet: packet, tcp: tcp, ctx: ctx, frameDone: make(chan struct{}), watcherDone: make(chan struct{})}
	packet.mu.Lock()
	if packet.closing || packet.association != nil || ctx.Err() != nil {
		packet.mu.Unlock()
		return ErrForwardingNotAdmitted // no task/buffer acquisition after original socket close
	}
	packet.association = association // BEFORE response, watcher, allocation or first read
	packet.mu.Unlock()
	return association.frame(func() error {
		if err := withMixedHandshakeBudget(ctx, tcp, until, func() error {
			return socks5.WriteResponse(mixedHandshakeWriter{tcp}, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess, Bind: M.SocksaddrFromNet(socket.LocalAddr()).Unwrap()})
		}); err != nil {
			return err
		}
		// Acquisition/reply timers are retired. First-packet/association lifetime
		// keeps its existing owned socket/watcher/UDPTimeout policy below.
		firstPeer := SB.NewServerPacketConn(socket)
		framed := socks.NewAssociatePacketConn(firstPeer, M.Socksaddr{}, tcp)
		if err := association.startWatcher(); err != nil {
			return err
		}
		if processor.udpTimeout > 0 {
			if err := socket.SetReadDeadline(time.Now().Add(processor.udpTimeout)); err != nil {
				return err
			}
		}
		destination, err := association.readFirst(framed)
		if err != nil {
			return err
		}
		// The SDK server adapter changes its last-sender address on every read.
		// Capture it before delegation/parallel responses; all later replies use
		// the existing immutable bind adapter and the same original UDP socket.
		peer := M.SocksaddrFromNet(firstPeer.RemoteAddr()).Unwrap()
		if err := packet.pinPeer(peer); err != nil {
			return err
		}
		framed = socks.NewAssociatePacketConn(SB.NewBindPacketConn(socket, peer.UDPAddr()), M.Socksaddr{}, tcp)
		var stream N.PacketConn = framed
		if processor.udpTimeout > 0 {
			if err := socket.SetReadDeadline(time.Time{}); err != nil {
				return err
			}
			// Deadline support was checked on this owned native UDP socket.
			// This existing SDK implementation creates no hidden timer goroutine.
			ctx, stream = canceler.NewTimeoutPacketConn(ctx, stream, processor.udpTimeout)
		}
		if destination.IsValid() {
			metadata.Destination = destination
		}
		processor.packet(ctx, &mixedAssociationConnection{association: association, inner: stream}, metadata, callback)
		return nil
	})
}
