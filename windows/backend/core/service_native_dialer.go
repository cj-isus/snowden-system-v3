package core

// Owned inner seam below the unchanged SDK resolver/Happy Eyeballs algorithm.
// It has no Upstream method: native calls cannot unwrap past this owner.

import (
	"context"
	"errors"
	"net"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const maxServiceNativeAttempts = 2 // pinned SDK A/AAAA racers; serial retries reuse retired slots

type serviceNativeDialer struct {
	outbound *serviceDialOutbound
	base     N.Dialer
}

type serviceNativeAttempt struct {
	task           *serviceDialTask
	owner          *preflightNetwork
	conn           *preflightConn
	active         bool // native call/defers/callback still pending; guarded by worker.mu
	callbackJoined bool
	closes         int
	released       bool
	fault          error
}

type serviceNativeConn struct {
	*preflightConn
	attempt *serviceNativeAttempt
}

func (conn *serviceNativeConn) Close() error { return conn.attempt.close() }

func (native *serviceNativeDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	conn, _, err := native.acquire(ctx, network, destination, false)
	return conn, err
}

func (native *serviceNativeDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	_, packet, err := native.acquire(ctx, N.NetworkUDP, destination, true)
	return packet, err
}

func (native *serviceNativeDialer) acquire(parent context.Context, network string, destination M.Socksaddr, packetMode bool) (result net.Conn, packet net.PacketConn, resultErr error) {
	if parent == nil || contextError(parent) != nil {
		return nil, nil, ErrPreflightScope
	}
	task, _ := parent.Value(serviceDialTaskKey{}).(*serviceDialTask)
	if task == nil || task.outbound != native.outbound || task.packetMode != packetMode {
		return nil, nil, ErrPreflightScope
	}
	worker := native.outbound.worker
	worker.mu.Lock()
	task.progressLocked()
	if task.sealed || !task.currentLocked() || !task.nativeDestinationLocked(network, destination) {
		sealed := task.sealed
		worker.mu.Unlock()
		if !sealed {
			worker.noteLocal(task.path, ErrPreflightScope)
		}
		return nil, nil, ErrPreflightScope
	}
	if len(task.attempts) >= maxServiceNativeAttempts {
		worker.mu.Unlock()
		worker.noteLocal(task.path, ErrPreflightBusy)
		return nil, nil, ErrPreflightBusy
	}
	attempt := &serviceNativeAttempt{task: task, owner: &preflightNetwork{}, active: true}
	task.attempts[attempt] = struct{}{} // before SDK base call, including conn+error/late acquisition
	worker.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(callbackDone); _ = attempt.owner.close() })
	completed := false
	var wrapped net.Conn
	var rawPacket net.PacketConn
	defer func() {
		if !completed {
			_ = recover()
			resultErr = ErrPreflightCleanup
			worker.mu.Lock()
			attempt.fault = ErrPreflightCleanup
			task.fenceLocked(attempt.fault)
			worker.mu.Unlock()
			worker.poison(ErrPreflightCleanup)
		}
		if !stop() {
			<-callbackDone
		}
		current := contextError(ctx) == nil && contextError(parent) == nil
		cancel() // callback stopped/joined; never closes a successful result on our own cancel
		worker.mu.Lock()
		current = current && !task.sealed && task.currentLocked()
		if resultErr == nil && current {
			var ok bool
			attempt.conn, ok = wrapped.(*preflightConn)
			if !ok || attempt.conn == nil || packetMode && task.packetCandidate != nil {
				attempt.fault, resultErr = ErrPreflightCleanup, ErrPreflightCleanup
			} else if packetMode {
				task.packetCandidate = attempt
			}
		} else if resultErr == nil {
			resultErr = ErrPreflightScope
		}
		worker.mu.Unlock()
		if resultErr != nil {
			_ = attempt.owner.close() // original native caller owns this cleanup, not a timeout wrapper
		}
		worker.mu.Lock()
		attempt.active, attempt.callbackJoined = false, true
		task.progressLocked()
		fault := task.cleanupErr
		worker.mu.Unlock()
		if fault != nil {
			worker.poison(fault)
		}
		if resultErr == nil {
			if packetMode {
				packet = rawPacket // SDK ListenSerial/NAT uses the original PacketConn
			} else {
				result = &serviceNativeConn{preflightConn: attempt.conn, attempt: attempt}
			}
		} else {
			result, packet = nil, nil
		}
	}()
	wrapped, resultErr = attempt.owner.dialWith(ctx, network, destination.String(), func(ctx context.Context, _, _ string) (net.Conn, error) {
		if !packetMode {
			return native.base.DialContext(ctx, network, destination)
		}
		var err error
		rawPacket, err = native.base.ListenPacket(ctx, destination)
		if rawPacket == nil {
			return nil, err
		}
		return bufio.NewBindPacketConn(rawPacket, destination), err
	})
	if errors.Is(resultErr, ErrPreflightCleanup) {
		worker.mu.Lock()
		attempt.fault = ErrPreflightCleanup
		worker.mu.Unlock()
	}
	completed = true
	return nil, nil, resultErr
}

var _ N.Dialer = (*serviceNativeDialer)(nil)
