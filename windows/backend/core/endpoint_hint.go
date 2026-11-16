package core

import (
	"context"
	"net"
	"net/netip"
	"reflect"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/netprobe"
)

// Set only by the original worker diagnostic or its original native flow.
// It is not serializable and is never attached to ordinary inbound traffic.
type initialBootstrapProbeKey struct{}

// Preserve the full normalized bytes for IPC, rollback and subject hashing.
// The SDK view lowers application metadata and its explicit bootstrap role,
// and disables unbounded raw SDK log output/observation. Both option validation
// and the original box constructor use this gate.
func sdkRuntimeConfiguration(raw []byte) ([]byte, *config.Config, error) {
	cfg, err := config.Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	sdk := *cfg
	sdk.BootstrapHints = nil
	// Raw SDK messages include connection destinations and opaque error text.
	// No bounded/redacted sink owns their file/observable lifetime here. Use
	// the pinned SDK's supported disabled factory; App lifecycle/probe events
	// remain separate. Never mutate the original normalized subject or share
	// its Log pointer while applying this private SDK policy.
	sdk.Log = &config.LogConfig{Disabled: true}
	sdk.Outbounds, _, err = lowerServiceDialers(cfg)
	if err != nil {
		return nil, nil, err
	}
	if plainBootstrapDirect(cfg) {
		// The pinned SDK rejects a detour to an empty direct outbound. Only
		// this exact no-options outbound permits the bootstrap-role lowering
		// to typed DNS socket defaults. Preserve custom settings/other detours.
		dns := *cfg.DNS // Parse required this contract; never mutate the normalized subject
		dns.Servers = append([]config.DNSServer(nil), cfg.DNS.Servers...)
		for index := range dns.Servers {
			server := &dns.Servers[index]
			if server.Tag == config.BootstrapDNSTag && server.Detour == "direct" {
				server.Detour = "" // typed literal UDP still uses the SDK default socket dialer
			}
		}
		sdk.DNS = &dns
	}
	encoded, err := jsonMarshalConfig(&sdk)
	if len(encoded) > maxWorkerConfigBytes {
		return nil, nil, ErrPreflightScope
	}
	return encoded, cfg, err
}

func plainBootstrapDirect(cfg *config.Config) bool {
	for _, outbound := range cfg.Outbounds {
		if outbound.IsPlainDirect() {
			return true
		}
	}
	return false
}

type endpointDNSRouter struct {
	adapter.DNSRouter
	worker         *bootstrapWorker
	transport      adapter.DNSTransport
	host, endpoint string
	mu             sync.Mutex
	seed           config.BootstrapHint
	seedPath       *bootstrapPath
	seedUntil      time.Time // process steady deadline, never serialized or renewed
	used           bool
}

// This borrowed view is not the owner of the original router's lifecycle.
func (router *endpointDNSRouter) Start(adapter.StartStage) error { return nil }
func (router *endpointDNSRouter) Close() error                   { return nil }

func (worker *bootstrapWorker) outboundContext(ctx context.Context, tag, kind string, raw any) (context.Context, error) {
	if kind != "vless" && kind != "hysteria2" {
		return ctx, nil
	}
	var host string
	var port uint16
	var resolver *option.DomainResolveOptions
	var optionsView option.DialerOptions
	switch options := raw.(type) {
	case *option.VLESSOutboundOptions:
		if kind != "vless" || options == nil {
			return nil, ErrPreflightScope
		}
		host, port, resolver, optionsView = options.Server, options.ServerPort, options.DomainResolver, options.DialerOptions
	case *option.Hysteria2OutboundOptions:
		if kind != "hysteria2" || options == nil {
			return nil, ErrPreflightScope
		}
		host, port, resolver, optionsView = options.Server, options.ServerPort, options.DomainResolver, options.DialerOptions
	default:
		return nil, ErrPreflightScope
	}
	spec := worker.services[serviceDialTag(tag)]
	if spec == nil || spec.peer.Type != kind || spec.peer.Server != host || spec.peer.ServerPort != port || resolver != nil ||
		!reflect.DeepEqual(optionsView, option.DialerOptions{Detour: spec.dial.Tag}) {
		return nil, ErrPreflightScope
	}
	return ctx, nil // the original protocol client uses the private socket detour
}

func (worker *bootstrapWorker) endpointContext(ctx context.Context, tag, kind, host string, resolver *option.DomainResolveOptions) (context.Context, error) {
	if net.ParseIP(host) != nil {
		return ctx, nil
	}
	canonical, err := netprobe.CanonicalDNSHost(host)
	if err != nil || resolver == nil || resolver.Server != config.BootstrapDNSTag ||
		C.DomainStrategy(resolver.Strategy) != C.DomainStrategyPreferIPv4 || !resolver.DisableCache ||
		resolver.RewriteTTL != nil || resolver.ClientSubnet != nil {
		return nil, ErrPreflightScope
	}
	original := service.FromContext[adapter.DNSRouter](ctx)
	manager := service.FromContext[adapter.DNSTransportManager](ctx)
	if original == nil || manager == nil {
		return nil, ErrPreflightScope
	}
	transport, exists := manager.Transport(config.BootstrapDNSTag)
	if !exists || transport == nil || transport.Type() != "udp" {
		return nil, ErrPreflightScope
	}
	owned, ok := transport.(*bootstrapDNSTransport)
	worker.mu.Lock()
	ownedHere := ok && owned.worker == worker && worker.transport == owned
	worker.mu.Unlock()
	if !ownedHere {
		return nil, ErrPreflightScope
	}
	view := &endpointDNSRouter{DNSRouter: original, worker: worker, transport: transport, host: canonical,
		endpoint: bootstrapEndpointDigest(tag, kind, host)}
	if hint, exists := worker.hints[tag]; exists {
		if !hint.ValidFor(host) {
			return nil, ErrPreflightScope
		}
		now, err := worker.now()
		if err != nil {
			return nil, err
		}
		if hint.FreshAt(now) {
			view.seed, view.seedUntil, view.seedPath = hint, time.Now().Add(hint.Until.Sub(now)), worker.pathSource()
		}
	}
	// ContextWith mutates the registry. Clone for this constructor only; user
	// DNS and sibling outbounds keep the original router and transport owner.
	return service.ContextWith[adapter.DNSRouter](service.ExtendContext(ctx), view), nil
}

func (router *endpointDNSRouter) initialHint(ctx context.Context) (netip.Addr, bool, error) {
	if ctx.Value(initialBootstrapProbeKey{}) != router.worker.admission {
		return netip.Addr{}, false, nil
	}
	admission := router.worker.admission
	admission.mu.Lock()
	eligible := !admission.closed && !admission.open.Load()
	admission.mu.Unlock()
	if !eligible || !router.worker.current(ctx) {
		return netip.Addr{}, false, nil
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	if router.used {
		return netip.Addr{}, false, nil
	}
	router.used = true // burn on first actual original diagnostic, including expiry/failure
	now, err := router.worker.now()
	if err != nil {
		return netip.Addr{}, false, err
	}
	if !router.seed.ValidFor(router.host) || !router.seed.FreshAt(now) || !time.Now().Before(router.seedUntil) || !router.worker.currentPath(ctx, router.seedPath) {
		return netip.Addr{}, false, nil
	}
	address, err := netip.ParseAddr(router.seed.Address)
	return address, err == nil, err
}

func (router *endpointDNSRouter) Lookup(ctx context.Context, domain string, options adapter.DNSQueryOptions) (addresses []netip.Addr, err error) {
	canonical, hostErr := netprobe.CanonicalDNSHost(domain)
	if hostErr != nil || canonical != router.host || options.Transport != router.transport || options.Strategy != C.DomainStrategyPreferIPv4 ||
		!options.DisableCache || options.RewriteTTL != nil || options.ClientSubnet.IsValid() || options.LookupStrategy != C.DomainStrategyAsIS {
		return router.DNSRouter.Lookup(ctx, domain, options)
	}
	path := router.worker.pathSource()
	lookupOwned := false
	defer func() {
		if err == nil {
			task, _ := ctx.Value(serviceDialTaskKey{}).(*serviceDialTask)
			if task == nil || task.outbound.worker != router.worker || !task.recordResolved(router.host, router.endpoint, path, addresses) {
				addresses, err = nil, ErrPreflightScope
				router.worker.noteLocal(path, err) // rejection happened after the original lookup's own local-fault receipt
			}
		}
		if !lookupOwned {
			router.worker.noteLocal(path, err)
		}
	}()
	if !router.worker.current(ctx) {
		return nil, ErrPreflightScope
	}
	if address, exists, err := router.initialHint(ctx); err != nil {
		return nil, err
	} else if exists {
		return []netip.Addr{address}, nil
	}
	lookupOwned = true
	return router.worker.lookup(ctx, router.host, router.endpoint, func(ctx context.Context) ([]netip.Addr, error) {
		use := &bootstrapDNSUse{worker: router.worker, path: path, host: router.host}
		return router.DNSRouter.Lookup(context.WithValue(ctx, bootstrapDNSUseKey{}, use), domain, options)
	})
}
