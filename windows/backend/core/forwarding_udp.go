package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
)

// One original UDP bind belongs to the exact retained TCP input. This includes
// the time before SOCKS receives its first packet or enters admissionRouter.
// A published local association adds its own frame/watcher/buffer fences;
// unrelated opaque SDK/upstream tasks are not retired by a socket receipt.
type mixedPacket struct {
	input       *mixedIngress
	ctx         context.Context
	cancel      context.CancelFunc
	raw         net.PacketConn
	local       net.Addr          // original bind result, not a fresh raw call after Close
	association *mixedAssociation // original frame/watcher/first-buffer owner; never replaced
	peer        M.Socksaddr       // immutable after the first native datagram, before routing/parallel responses

	mu, controlMu               sync.Mutex
	bindReturned                bool
	bindCalled                  bool
	bindErr                     error
	closing, failed             bool
	ioPending                   int
	ioDoneClosed                bool
	closeStarted, closeCalled   bool
	rawCloseReturned            bool
	rawCloseErr                 error
	closeReturned, noResource   bool
	closeErr                    error
	bindDone, closeWait, ioDone chan struct{}
	done                        chan struct{}
}

func (inbound *ownedMixedInbound) ListenPacket(config net.ListenConfig, ctx context.Context, network, address string) (net.PacketConn, error) {
	_, conn, err := inbound.openOwnedPacket(config, ctx, network, address)
	return conn, err
}

func (inbound *ownedMixedInbound) openOwnedPacket(config net.ListenConfig, ctx context.Context, network, address string) (*mixedPacket, net.PacketConn, error) {
	if ctx == nil {
		return nil, nil, ErrForwardingNotAdmitted
	}
	input, ok := ctx.Value(mixedInputKey{}).(*mixedIngress)
	if !ok || input == nil || input.gate != inbound.gate {
		return nil, nil, ErrForwardingNotAdmitted
	}
	gate := inbound.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	if !owned || gate.closed || gate.routeFault || gate.rejectionsSealed {
		gate.mu.Unlock()
		return nil, nil, ErrForwardingNotAdmitted
	}
	input.mu.Lock()
	if input.closing || input.packet != nil || ctx.Err() != nil {
		input.mu.Unlock()
		gate.mu.Unlock()
		return nil, nil, ErrForwardingNotAdmitted // no retry/replacement of an earlier attempt
	}
	lifetime, cancel := context.WithCancel(ctx)
	packet := &mixedPacket{input: input, ctx: lifetime, cancel: cancel,
		bindDone: make(chan struct{}), closeWait: make(chan struct{}), ioDone: make(chan struct{}), done: make(chan struct{})}
	input.packet = packet // BEFORE original native acquisition, including partial failure
	input.mu.Unlock()
	gate.mu.Unlock()
	context.AfterFunc(lifetime, packet.closeTask)
	conn, err := packet.bind(func() (net.PacketConn, error) {
		return inbound.listen.ListenPacket(config, lifetime, network, address)
	})
	return packet, conn, err
}

func (packet *mixedPacket) bind(call func() (net.PacketConn, error)) (_ net.PacketConn, finalErr error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		packet.mu.Lock()
		packet.bindReturned, packet.bindErr = returned, finalErr
		packet.failed = packet.failed || fault
		close(packet.bindDone)
		packet.mu.Unlock()
		if fault {
			packet.input.gate.noteRouteFault()
		}
		if fault || finalErr != nil {
			packet.cancel() // partial raw handle still belongs to this exact attempt
		}
	}()
	if err := packet.ctx.Err(); err != nil {
		returned = true
		return nil, err // no native attempt; not a fabricated Close call
	}
	packet.mu.Lock()
	packet.bindCalled = true
	packet.mu.Unlock()
	raw, err := call()
	packet.mu.Lock()
	packet.raw = raw
	packet.mu.Unlock()
	if raw != nil {
		local := raw.LocalAddr() // still part of this original acquisition frame
		packet.mu.Lock()
		packet.local = local
		packet.mu.Unlock()
	}
	returned = true
	if err != nil {
		return nil, err
	}
	if raw == nil {
		packet.mu.Lock()
		packet.failed = true
		packet.mu.Unlock()
		return nil, ErrEngineCleanupFailed
	}
	if err := packet.ctx.Err(); err != nil {
		return nil, err // canceled after bind: retained original close task owns raw
	}
	return &mixedUDPSocket{packet}, nil
}

func (packet *mixedPacket) closeOriginal() (finalErr error) {
	packet.mu.Lock()
	packet.closing = true
	if packet.ioPending == 0 && !packet.ioDoneClosed {
		packet.ioDoneClosed = true
		close(packet.ioDone)
	}
	if packet.closeStarted {
		err := packet.closeErr
		if !packet.closeReturned {
			err = ErrEngineCleanupFailed
		}
		packet.mu.Unlock()
		return err
	}
	packet.closeStarted, packet.closeErr = true, ErrEngineCleanupFailed
	packet.mu.Unlock()
	packet.cancel()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		packet.mu.Lock()
		packet.closeReturned, packet.closeErr = returned, finalErr
		packet.failed = packet.failed || fault || finalErr != nil
		close(packet.closeWait)
		packet.mu.Unlock()
		if fault || finalErr != nil {
			packet.input.gate.noteRouteFault()
		}
	}()
	<-packet.bindDone // original acquisition cannot be declared absent while pending
	packet.mu.Lock()
	bound, raw := packet.bindReturned, packet.raw
	if bound && raw == nil {
		packet.noResource = true
	}
	packet.mu.Unlock()
	if raw == nil {
		returned = true
		if !bound {
			return ErrEngineCleanupFailed
		}
		return nil
	}
	packet.controlMu.Lock()
	defer packet.controlMu.Unlock()
	packet.mu.Lock()
	packet.closeCalled = true
	packet.mu.Unlock()
	finalErr = raw.Close() // exactly one actual native Close attempt
	packet.mu.Lock()
	packet.rawCloseReturned, packet.rawCloseErr = true, finalErr
	packet.mu.Unlock()
	if !bound {
		finalErr = errors.Join(ErrEngineCleanupFailed, finalErr) // close known raw even after failed bind frame
	}
	returned = true
	return finalErr
}

func (packet *mixedPacket) joinClose() error {
	packet.mu.Lock()
	started := packet.closeStarted
	packet.mu.Unlock()
	if !started {
		_ = packet.closeOriginal()
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	if !mixedOriginalDone(ctx, packet.closeWait) {
		return ErrEngineCleanupFailed
	}
	packet.mu.Lock()
	defer packet.mu.Unlock()
	if !packet.closeReturned || packet.closeErr != nil {
		return errors.Join(ErrEngineCleanupFailed, packet.closeErr)
	}
	return nil
}

func (packet *mixedPacket) closeTask() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		packet.mu.Lock()
		packet.failed = packet.failed || fault || !packet.bindReturned || !packet.closeReturned || packet.closeErr != nil || packet.ioPending != 0 || !(packet.noResource || packet.closeCalled && packet.rawCloseReturned && packet.rawCloseErr == nil)
		failed := packet.failed
		packet.mu.Unlock()
		if failed {
			packet.input.gate.noteRouteFault()
		}
		close(packet.done) // original socket plus published association/frame/watcher/buffer
	}()
	_ = packet.closeOriginal()
	<-packet.closeWait
	packet.mu.Lock()
	confirmed := packet.closeReturned && packet.closeErr == nil
	association := packet.association // closing forbids any later association publication
	packet.mu.Unlock()
	if confirmed {
		<-packet.ioDone
	}
	if association != nil {
		if err := association.retire(); err != nil {
			packet.mu.Lock()
			packet.failed = true
			packet.mu.Unlock()
		}
	}
	returned = true
}

func (packet *mixedPacket) perform(call func(net.PacketConn) error) (finalErr error) {
	packet.mu.Lock()
	if packet.closing || packet.ctx.Err() != nil || !packet.bindReturned || packet.raw == nil {
		packet.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	raw := packet.raw
	packet.ioPending++
	packet.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		packet.mu.Lock()
		packet.ioPending--
		packet.failed = packet.failed || fault
		if packet.closing && packet.ioPending == 0 && !packet.ioDoneClosed {
			packet.ioDoneClosed = true
			close(packet.ioDone)
		}
		packet.mu.Unlock()
		if fault {
			packet.input.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // no invented partial packet result
		}
	}()
	finalErr = call(raw)
	returned = true
	return finalErr
}

func (packet *mixedPacket) control(call func(net.PacketConn) error) error {
	packet.controlMu.Lock()
	defer packet.controlMu.Unlock()
	return packet.perform(call)
}

type mixedUDPSocket struct{ packet *mixedPacket }

func (packet *mixedPacket) pinPeer(peer M.Socksaddr) error {
	// The original TCP peer was captured before entering its SDK handler.
	// Pinning local UDP is demultiplexing custody, not authenticated peer proof.
	packet.input.mu.Lock()
	originalRemote := packet.input.remote
	packet.input.mu.Unlock()
	remote := M.SocksaddrFromNet(originalRemote).Unwrap()
	if !peer.IsIP() || peer.IsDomain() || !remote.IsIP() || peer.Addr != remote.Addr {
		return ErrForwardingNotAdmitted
	}
	packet.mu.Lock()
	defer packet.mu.Unlock()
	if packet.closing || packet.ctx.Err() != nil || packet.peer.IsValid() {
		return ErrForwardingNotAdmitted
	}
	packet.peer = peer
	return nil
}

func (socket *mixedUDPSocket) ReadFrom(buffer []byte) (n int, address net.Addr, err error) {
	for {
		err = socket.packet.perform(func(raw net.PacketConn) error { n, address, err = raw.ReadFrom(buffer); return err })
		if err != nil {
			return
		}
		if n < 0 || n > len(buffer) {
			socket.packet.input.gate.noteRouteFault()
			return 0, nil, ErrEngineCleanupFailed
		}
		socket.packet.mu.Lock()
		peer := socket.packet.peer
		socket.packet.mu.Unlock()
		if !peer.IsValid() || M.SocksaddrFromNet(address).Unwrap() == peer {
			return
		}
		clear(buffer[:n]) // discard a foreign sender in the same original read/deadline; no refresh or new socket
	}
}

func (socket *mixedUDPSocket) WriteTo(buffer []byte, address net.Addr) (n int, err error) {
	socket.packet.mu.Lock()
	peer := socket.packet.peer
	socket.packet.mu.Unlock()
	if peer.IsValid() && M.SocksaddrFromNet(address).Unwrap() != peer {
		return 0, ErrForwardingNotAdmitted
	}
	err = socket.packet.perform(func(raw net.PacketConn) error { n, err = raw.WriteTo(buffer, address); return err })
	if n < 0 || n > len(buffer) {
		socket.packet.input.gate.noteRouteFault()
		err = errors.Join(ErrEngineCleanupFailed, err)
	} else if n != len(buffer) && err == nil {
		err = io.ErrShortWrite // packet writers above this layer discard n; never turn a short native datagram into success
	}
	return
}

func (socket *mixedUDPSocket) Close() error { return socket.packet.joinClose() }
func (socket *mixedUDPSocket) LocalAddr() (address net.Addr) {
	socket.packet.mu.Lock()
	defer socket.packet.mu.Unlock()
	return socket.packet.local
}
func (socket *mixedUDPSocket) SetDeadline(value time.Time) error {
	return socket.packet.control(func(raw net.PacketConn) error { return raw.SetDeadline(value) })
}
func (socket *mixedUDPSocket) SetReadDeadline(value time.Time) error {
	return socket.packet.control(func(raw net.PacketConn) error { return raw.SetReadDeadline(value) })
}
func (socket *mixedUDPSocket) SetWriteDeadline(value time.Time) error {
	return socket.packet.control(func(raw net.PacketConn) error { return raw.SetWriteDeadline(value) })
}
func (*mixedUDPSocket) ReaderReplaceable() bool { return false }
func (*mixedUDPSocket) WriterReplaceable() bool { return false }

var (
	_ socks.PacketListener = (*ownedMixedInbound)(nil)
	_ net.PacketConn       = (*mixedUDPSocket)(nil)
)
