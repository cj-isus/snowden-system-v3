// Deferred SOCKS reply orchestration adapted from sing v0.8.13/protocol/socks/lazy.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

var errMixedSOCKSReplyRejected = errors.New("SOCKS reply did not admit payload")

// One original reply frame belongs to the exact input before routing. It may
// retain a peer while reading bind metadata, but the SDK still owns that peer's
// native Close. A reply/frame receipt is not whole upstream/worker retirement.
type mixedSOCKSConnection struct {
	input    *mixedIngress
	ctx      context.Context
	original *mixedIngressConnection
	stream   net.Conn // exact original connection or its retained prefix, never unwrapped
	version  byte
	mu       sync.Mutex // values only; no peer/socket/timer API, cancel or wait while held

	closing, started, returned, success, failed bool
	peer                                        net.Conn
	err                                         error
	done                                        chan struct{}
}

func newMixedSOCKSConnection(ctx context.Context, raw, stream net.Conn, version byte) (*mixedSOCKSConnection, error) {
	original, ok := raw.(*mixedIngressConnection)
	if !ok || original == nil || original.owner == nil || original.owner.gate == nil || ctx == nil || ctx.Err() != nil || ctx.Value(mixedInputKey{}) != original.owner || version != socks4.Version && version != socks5.Version {
		return nil, ErrForwardingNotAdmitted
	}
	switch underlying := stream.(type) {
	case *mixedIngressConnection:
		if underlying != original {
			return nil, ErrForwardingNotAdmitted
		}
	case *mixedPrefetchedConnection:
		if underlying == nil || underlying.mixedIngressConnection != original || underlying.prefix == nil || underlying.prefix.input != original.owner {
			return nil, ErrForwardingNotAdmitted
		}
	default:
		return nil, ErrForwardingNotAdmitted
	}
	input, gate := original.owner, original.owner.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	input.mu.Lock()
	if !owned || gate.closed || gate.routeFault || gate.rejectionsSealed || input.closing || input.socksReplySealed || input.socksReply != nil || input.ctx.Err() != nil || ctx.Err() != nil {
		input.mu.Unlock()
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	conn := &mixedSOCKSConnection{input: input, ctx: ctx, original: original, stream: stream, version: version, done: make(chan struct{})}
	input.socksReply = conn // BEFORE SDK routing, peer metadata, response allocation or IO
	input.mu.Unlock()
	gate.mu.Unlock()
	return conn, nil
}

func (conn *mixedSOCKSConnection) ConnHandshakeSuccess(peer net.Conn) error {
	return conn.respond(true, peer, nil)
}

func (conn *mixedSOCKSConnection) HandshakeFailure(cause error) error {
	return conn.respond(false, nil, cause)
}

func (conn *mixedSOCKSConnection) respond(success bool, peer net.Conn, cause error) (finalErr error) {
	conn.mu.Lock()
	if conn.closing || conn.ctx.Err() != nil {
		conn.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	if conn.started {
		conn.mu.Unlock()
		err := conn.joinReply()
		if !success {
			return errors.Join(os.ErrInvalid, err) // no second response, even after a failed first write
		}
		return err
	}
	conn.started, conn.peer = true, peer // one original attempt, before inspecting peer or constructing a timer
	conn.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		conn.mu.Lock()
		conn.failed = conn.failed || fault || errors.Is(finalErr, ErrEngineCleanupFailed)
		if conn.failed {
			finalErr = errors.Join(ErrEngineCleanupFailed, finalErr)
		}
		conn.returned, conn.err = returned, finalErr
		conn.success = success && returned && finalErr == nil
		failed := conn.failed
		conn.mu.Unlock()
		if failed {
			conn.input.gate.noteRouteFault()
		}
		if !success || finalErr != nil || failed {
			conn.input.cancel() // original raw Close/task owns cleanup, never join this reply from itself
		}
		close(conn.done) // original reply frame, not a fabricated peer/socket/worker receipt
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	finalErr = withMixedHandshakeBudget(conn.ctx, conn.original, time.Time{}, func() error {
		bind := M.Socksaddr{Addr: netip.IPv4Unspecified()}
		if success {
			if peer == nil {
				return ErrForwardingNotAdmitted
			}
			bind = M.SocksaddrFromNet(peer.LocalAddr()).Unwrap()
		}
		writer := mixedHandshakeWriter{conn.original}
		if conn.version == socks4.Version {
			if !bind.IsIPv4() {
				bind = M.Socksaddr{Addr: netip.IPv4Unspecified()} // SOCKS4 has exactly four address bytes
			}
			code := socks4.ReplyCodeRejectedOrFailed
			if success {
				code = socks4.ReplyCodeGranted
			}
			return socks4.WriteResponse(writer, socks4.Response{ReplyCode: code, Destination: bind})
		}
		code := socks5.ReplyCodeForError(cause)
		if success {
			code = socks5.ReplyCodeSuccess
		}
		return socks5.WriteResponse(writer, socks5.Response{ReplyCode: code, Bind: bind})
	})
	returned = true
	return finalErr
}

// Repeated/concurrent callers observe the original reply, never re-send it.
func (conn *mixedSOCKSConnection) joinReply() error {
	observed := false
	select {
	case <-conn.done:
		observed = true // ordinary payload path: no new timer/context per read/write
	default:
	}
	if !observed {
		ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
		observed = mixedOriginalDone(ctx, conn.done)
		cancel()
	}
	conn.mu.Lock()
	unknown := !observed || !conn.returned || conn.failed
	conn.failed = conn.failed || unknown
	success, err := conn.success, conn.err
	conn.mu.Unlock()
	if unknown {
		conn.input.gate.noteRouteFault()
		return errors.Join(ErrEngineCleanupFailed, err)
	}
	if !success {
		return errors.Join(errMixedSOCKSReplyRejected, err)
	}
	return err
}

func (conn *mixedSOCKSConnection) Read(buffer []byte) (int, error) {
	// Preserve the pinned lazy fallback used by local DNS and UoT. This is
	// protocol readiness, not evidence of an established external destination.
	if err := conn.respond(true, conn.original, nil); err != nil {
		return 0, err
	}
	return conn.stream.Read(buffer) // reply timer/callback retired before payload IO
}

func (conn *mixedSOCKSConnection) Write(buffer []byte) (int, error) {
	if err := conn.respond(true, conn.original, nil); err != nil {
		return 0, err
	}
	return conn.stream.Write(buffer)
}

func (conn *mixedSOCKSConnection) Close() error {
	conn.input.cancel()
	return conn.stream.Close() // existing TCP/prefix Close; never wait on the reply frame here
}

func (conn *mixedSOCKSConnection) LocalAddr() net.Addr  { return conn.stream.LocalAddr() }
func (conn *mixedSOCKSConnection) RemoteAddr() net.Addr { return conn.stream.RemoteAddr() }
func (conn *mixedSOCKSConnection) SetDeadline(value time.Time) error {
	return conn.stream.SetDeadline(value)
}
func (conn *mixedSOCKSConnection) SetReadDeadline(value time.Time) error {
	return conn.stream.SetReadDeadline(value)
}
func (conn *mixedSOCKSConnection) SetWriteDeadline(value time.Time) error {
	return conn.stream.SetWriteDeadline(value)
}
func (conn *mixedSOCKSConnection) NeedAdditionalReadDeadline() bool {
	return conn.original.NeedAdditionalReadDeadline()
}
func (*mixedSOCKSConnection) ReaderReplaceable() bool { return false }
func (*mixedSOCKSConnection) WriterReplaceable() bool { return false }

func (input *mixedIngress) sealSOCKSReply() *mixedSOCKSConnection {
	input.mu.Lock()
	input.socksReplySealed = true
	conn := input.socksReply
	input.mu.Unlock()
	if conn != nil {
		conn.mu.Lock()
		conn.closing = true // no late response can start after the root snapshot
		conn.mu.Unlock()
	}
	return conn
}

// Called only by the original input cleanup, after raw TCP Close. Sealing a
// never-started reply is distinct from observing a real response frame return.
func (conn *mixedSOCKSConnection) retire() error {
	conn.mu.Lock()
	sealed, started := conn.closing, conn.started
	conn.mu.Unlock()
	if !sealed {
		return ErrEngineCleanupFailed
	}
	if !started {
		return nil // no frame acquired, future acquisition was sealed; done is not closed
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	observed := mixedOriginalDone(ctx, conn.done)
	conn.mu.Lock()
	unknown := !observed || !conn.returned || conn.failed
	conn.failed = conn.failed || unknown
	conn.mu.Unlock()
	if unknown {
		conn.input.gate.noteRouteFault()
		return ErrEngineCleanupFailed
	}
	return nil // a known negative/transport error can retire; it never admits payload
}

var (
	_ net.Conn               = (*mixedSOCKSConnection)(nil)
	_ N.ConnHandshakeSuccess = (*mixedSOCKSConnection)(nil)
	_ N.HandshakeFailure     = (*mixedSOCKSConnection)(nil)
)
