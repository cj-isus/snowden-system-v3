package core

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/snowden-system/windows/backend/netprobe"
)

// Pending calls only. Successfully handed-off native objects belong to the
// unchanged SDK protocol client, not this path's DNS/dial retirement domain.
type serviceDialTask struct {
	outbound        *serviceDialOutbound
	path            *bootstrapPath
	parent          context.Context
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	packetMode      bool
	attempts        map[*serviceNativeAttempt]struct{} // all metadata below guarded by worker.mu
	resolved        []netip.Addr
	packetCandidate *serviceNativeAttempt
	sealed          bool
	pathJoined      bool
	outerReturned   bool
	doneClosed      bool
	cleanupErr      error // immutable after done; uncertain originals remain registered
}

type serviceDialTaskKey struct{}

func (outbound *serviceDialOutbound) endpointAllowed(network string, destination M.Socksaddr) bool {
	peer := outbound.spec.peer
	if !peer.AllowsServicePort(destination.Port) || peer.Type == "vless" && N.NetworkName(network) != N.NetworkTCP ||
		peer.Type == "hysteria2" && N.NetworkName(network) != N.NetworkUDP {
		return false
	}
	destination = destination.Unwrap()
	if address := net.ParseIP(peer.Server); address != nil {
		return destination.IsIP() && address.Equal(net.IP(destination.Addr.AsSlice()))
	}
	if !destination.IsDomain() {
		return false
	}
	host, err := netprobe.CanonicalDNSHost(strings.TrimSuffix(destination.Fqdn, "."))
	expected, expectedErr := netprobe.CanonicalDNSHost(peer.Server)
	return err == nil && expectedErr == nil && host == expected
}

// worker.mu held. This is a negative continuity check, not socket/TLS/QUIC or
// forwarding attestation. The source is captured BEFORE the delegated Lookup.
func (task *serviceDialTask) currentLocked() bool {
	outbound := task.outbound
	worker := outbound.worker
	return !worker.closed && !worker.paused && worker.fault == nil && worker.services[outbound.Tag()] == outbound.spec && worker.serviceOutbounds[outbound.Tag()] == outbound &&
		outbound.started && !outbound.closed && worker.path == task.path && task.path.live &&
		contextError(worker.ctx) == nil && contextError(task.path.ctx) == nil && contextError(task.ctx) == nil && contextError(task.parent) == nil
}

func (outbound *serviceDialOutbound) serviceDial(parent context.Context, network string, destination M.Socksaddr, packetMode bool, dial func(context.Context) (net.Conn, error)) (result net.Conn, resultErr error) {
	if parent == nil || contextError(parent) != nil || !outbound.endpointAllowed(network, destination) {
		return nil, ErrPreflightScope
	}
	worker := outbound.worker
	worker.mu.Lock()
	worker.serviceDialProgressLocked()
	if worker.path == nil || len(worker.serviceDials) >= maxBootstrapCallers {
		path := worker.path
		worker.mu.Unlock()
		worker.noteLocal(path, ErrPreflightBusy)
		return nil, ErrPreflightBusy
	}
	ctx, cancel := context.WithCancel(parent)
	task := &serviceDialTask{outbound: outbound, path: worker.path, parent: parent, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), packetMode: packetMode, attempts: make(map[*serviceNativeAttempt]struct{})}
	if !task.currentLocked() {
		worker.mu.Unlock()
		cancel()
		return nil, ErrPreflightScope
	}
	worker.serviceDials[task] = struct{}{} // original domain before DNS or any inner native call
	worker.mu.Unlock()
	pathDone := make(chan struct{})
	pathStop := context.AfterFunc(task.path.ctx, func() { defer close(pathDone); task.cancel() })
	completed, handed := false, false
	var wrapped net.Conn
	var candidate *serviceNativeAttempt
	defer func() {
		if !completed {
			_ = recover()
			worker.mu.Lock()
			task.cleanupErr = ErrPreflightCleanup
			worker.mu.Unlock()
			resultErr = ErrPreflightCleanup
		}
		worker.mu.Lock()
		task.sealed = true // late SDK racers cannot begin another native acquisition
		worker.mu.Unlock()
		if !pathStop() {
			<-pathDone
		}
		worker.mu.Lock()
		task.pathJoined = true
		if resultErr == nil && candidate != nil && candidate.task == task && !candidate.active && candidate.closes == 0 &&
			candidate.callbackJoined && task.cleanupErr == nil && task.currentLocked() {
			raw, err := candidate.owner.handoff(candidate.conn)
			if err == nil {
				candidate.released = true
				delete(task.attempts, candidate)
				if task.packetCandidate == candidate {
					task.packetCandidate = nil
				}
				result, handed = raw, true
				if packetMode {
					result = wrapped // keep the SDK's NAT/packet view; its native PacketConn is transferred
				}
			} else {
				task.cleanupErr, resultErr = ErrPreflightCleanup, err
			}
		} else if resultErr == nil {
			resultErr = ErrPreflightScope
		}
		if task.cleanupErr != nil {
			task.fenceLocked(task.cleanupErr)
		}
		worker.mu.Unlock()
		task.cancel()
		if !handed {
			task.closeAttempts() // failed result: checked original cleanup, never a wrapper reaper
		}
		worker.mu.Lock()
		task.outerReturned = true
		task.progressLocked()
		cleanup := task.cleanupErr
		worker.mu.Unlock()
		if cleanup != nil {
			worker.poison(cleanup)
			if !handed {
				result, resultErr = nil, errors.Join(ErrPreflightCleanup, resultErr, cleanup)
			}
		}
		if resultErr != nil {
			worker.noteLocal(task.path, resultErr) // local scope/budget/cancellation must not become channel health evidence
		}
	}()
	wrapped, resultErr = dial(context.WithValue(ctx, serviceDialTaskKey{}, task))
	if resultErr == nil {
		worker.mu.Lock()
		if packetMode {
			candidate = task.packetCandidate // pinned SDK ListenSerial has exactly one successful packet acquisition
		} else if owned, ok := wrapped.(*serviceNativeConn); ok {
			candidate = owned.attempt
		}
		if candidate == nil || candidate.task != task {
			task.cleanupErr, resultErr = ErrPreflightCleanup, ErrPreflightCleanup
		}
		worker.mu.Unlock()
		if resultErr == nil {
			resultErr = wrapped.SetDeadline(time.Time{})
		}
	}
	completed = true
	return nil, resultErr // the original-source commit in the defer decides handoff
}

func (task *serviceDialTask) recordResolved(host, endpoint string, path *bootstrapPath, addresses []netip.Addr) bool {
	worker := task.outbound.worker
	worker.mu.Lock()
	defer worker.mu.Unlock()
	peer := task.outbound.spec.peer
	expected, err := netprobe.CanonicalDNSHost(peer.Server)
	if err != nil || expected != host || endpoint != bootstrapEndpointDigest(peer.Tag, peer.Type, peer.Server) || path != task.path ||
		task.sealed || task.resolved != nil || !task.currentLocked() || len(addresses) == 0 || len(addresses) > maxBootstrapAddresses {
		return false
	}
	for _, address := range addresses {
		if !address.IsValid() || address.Zone() != "" {
			return false
		}
	}
	task.resolved = append([]netip.Addr(nil), addresses...)
	return true // observed service addresses, not authenticated peer/route/health evidence
}
