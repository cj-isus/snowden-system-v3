package core

import (
	"context"
	"net"
	"reflect"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type serviceDialOutbound struct {
	outbound.Adapter
	worker  *bootstrapWorker
	spec    *serviceDialSpec
	dialer  N.Dialer // borrowed SDK dependencies retain their original box lifecycle
	base    N.Dialer // original SDK socket/detour initializer; no resolver outside our inner adapter
	started bool     // guarded by worker.mu
	closed  bool
}

func (worker *bootstrapWorker) newServiceDialOutbound(ctx context.Context, tag, kind string, raw any, spec *serviceDialSpec) (adapter.Outbound, error) {
	options, ok := raw.(*option.DirectOutboundOptions)
	expected, err := spec.options(ctx)
	if err != nil || !ok || options == nil || kind != "direct" || tag != spec.dial.Tag || !reflect.DeepEqual(options, expected) {
		return nil, ErrPreflightScope
	}
	ctx, err = worker.endpointContext(ctx, spec.peer.Tag, spec.peer.Type, spec.peer.Server, expected.DomainResolver)
	if err != nil {
		return nil, err
	}
	dialOptions := expected.DialerOptions
	if spec.peer.Type == "hysteria2" {
		dialOptions.UDPFragmentDefault = true // the unchanged HY2 constructor applies this same default
	}
	owned := &serviceDialOutbound{Adapter: outbound.NewAdapterWithDialerOptions(kind, tag, []string{N.NetworkTCP, N.NetworkUDP}, dialOptions), worker: worker, spec: spec}
	worker.mu.Lock()
	if worker.closed || worker.fault != nil || worker.serviceOutbounds[tag] != nil {
		worker.mu.Unlock()
		return nil, ErrPreflightScope
	}
	worker.serviceOutbounds[tag] = owned
	worker.mu.Unlock()
	instance, err := dialer.New(ctx, dialOptions, false)
	if err != nil || instance == nil {
		worker.poison(ErrPreflightCleanup)
		return nil, ErrPreflightScope
	}
	worker.mu.Lock()
	owned.base = instance
	worker.mu.Unlock()
	inner := &serviceNativeDialer{outbound: owned, base: instance}
	var resolved N.Dialer = inner
	if net.ParseIP(spec.peer.Server) == nil {
		resolver := dialOptions.DomainResolver // endpointContext already checked this exact service resolver
		query := adapter.DNSQueryOptions{Strategy: C.DomainStrategy(resolver.Strategy), DisableCache: resolver.DisableCache}
		resolved = dialer.NewResolveDialer(ctx, inner, dialOptions.Detour == "" && !dialOptions.TCPFastOpen,
			resolver.Server, query, time.Duration(dialOptions.FallbackDelay))
	}
	worker.mu.Lock()
	owned.dialer = resolved
	worker.mu.Unlock()
	return owned, nil
}

func (outbound *serviceDialOutbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	worker := outbound.worker
	worker.mu.Lock()
	valid := worker.serviceOutbounds[outbound.Tag()] == outbound && !worker.closed && worker.fault == nil && !outbound.closed && outbound.dialer != nil && outbound.base != nil
	worker.mu.Unlock()
	if !valid {
		return ErrPreflightScope
	}
	if err := dialer.InitializeDetour(outbound.base); err != nil {
		return err
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.closed || worker.fault != nil || outbound.closed {
		return ErrPreflightScope
	}
	outbound.started = true
	return nil
}

func (outbound *serviceDialOutbound) Close() error {
	outbound.worker.mu.Lock()
	outbound.closed = true
	outbound.worker.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), workerCloseTimeout)
	defer cancel()
	return outbound.worker.retireServiceDials(ctx, outbound)
}

func (outbound *serviceDialOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return outbound.serviceDial(ctx, network, destination, false, func(ctx context.Context) (net.Conn, error) {
		return outbound.dialer.DialContext(ctx, network, destination)
	})
}

func (outbound *serviceDialOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	var packet net.PacketConn
	_, err := outbound.serviceDial(ctx, N.NetworkUDP, destination, true, func(ctx context.Context) (net.Conn, error) {
		var err error
		packet, err = outbound.dialer.ListenPacket(ctx, destination)
		if packet == nil {
			return nil, err
		}
		// Only the pending acquisition's Close/deadline owner uses this view.
		// Successful handoff returns the original PacketConn below.
		return bufio.NewBindPacketConn(packet, destination), err
	})
	if err != nil {
		return nil, err
	}
	return packet, nil
}

// Start/Close are forwarded by the SDK lifecycle; original protocol Outbounds
// are never wrapped, preserving mux/interface-update and concrete capabilities.
var (
	_ adapter.Outbound  = (*serviceDialOutbound)(nil)
	_ adapter.Lifecycle = (*serviceDialOutbound)(nil)
)
