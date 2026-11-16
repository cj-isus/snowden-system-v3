package config

import (
	"errors"
	"fmt"
	"net"

	"github.com/snowden-system/windows/backend/netprobe"
)

const (
	ProtectedDNSTag                  = "tunnel-dns"
	BootstrapDNSTag                  = "bootstrap-dns"
	ProtectedDNSCacheCapacity uint32 = 1024 // pinned engine floors smaller values to 1024
)

var ErrDNSContract = errors.New("protected runtime DNS contract unsupported")

func dnsContractError(reason string) error { return fmt.Errorf("%w: %s", ErrDNSContract, reason) }

// No bootstrap route is selected by a user's question, including an endpoint
// or donor name. Private service dialers opt in to their resolver by reference.
// The same gate runs before the original worker consumes normalized config.
func (c *Config) validateDNS() error {
	dns := c.DNS
	if dns == nil || dns.Final != ProtectedDNSTag || dns.Strategy != "prefer_ipv4" ||
		!dns.IndependentCache || dns.CacheCapacity != ProtectedDNSCacheCapacity {
		return dnsContractError("explicit protected final and independent bounded cache are required; regenerate legacy runtime config from its profile")
	}
	if len(dns.Rules) != 0 {
		return dnsContractError("question-based DNS routing is outside this supported contract")
	}
	if len(dns.Servers) < 1 || len(dns.Servers) > 2 {
		return dnsContractError("expected one protected resolver and optional service bootstrap resolver")
	}
	protected, bootstrap := false, false
	for _, server := range dns.Servers {
		if server.Address != "" {
			return dnsContractError("legacy address-format DNS is not supported; regenerate runtime config from its profile")
		}
		if !netprobe.PublicIPv4(net.ParseIP(server.Server)) {
			return dnsContractError("DNS transport requires a public literal IPv4, not recursive local resolution")
		}
		switch server.Tag {
		case ProtectedDNSTag:
			if protected || server.Type != "tls" || server.Detour != "proxy" || server.ServerPort != 853 || server.TLS == nil || !server.TLS.Enabled {
				return dnsContractError("user DNS requires a unique authenticated TLS resolver through the protected selector")
			}
			if _, err := netprobe.CanonicalDNSHost(server.TLS.ServerName); err != nil {
				return dnsContractError("user DNS TLS identity is missing or invalid")
			}
			protected = true
		case BootstrapDNSTag:
			if bootstrap || server.Type != "udp" || server.Detour != "direct" || server.ServerPort != 53 || server.TLS != nil {
				return dnsContractError("service bootstrap requires a unique literal UDP resolver")
			}
			bootstrap = true
		default:
			return dnsContractError("unsupported DNS resolver role")
		}
	}
	if !protected {
		return dnsContractError("protected final resolver is absent")
	}
	needed := false
	for _, outbound := range c.Outbounds {
		if !isProtectedOutboundType(outbound.Type) {
			if outbound.DomainResolver != nil {
				return dnsContractError("non-service outbound cannot select bootstrap DNS")
			}
			continue
		}
		if net.ParseIP(outbound.Server) != nil {
			if outbound.DomainResolver != nil {
				return dnsContractError("literal service address must not retain a domain resolver")
			}
			continue
		}
		if _, err := netprobe.CanonicalDNSHost(outbound.Server); err != nil {
			return dnsContractError("invalid service address for bootstrap DNS")
		}
		resolver := outbound.DomainResolver
		if resolver == nil || resolver.Server != BootstrapDNSTag || resolver.Strategy != "prefer_ipv4" || !resolver.DisableCache {
			return dnsContractError("domain service address requires explicit non-caching bootstrap resolver")
		}
		needed = true
	}
	if needed != bootstrap {
		return dnsContractError("bootstrap presence must match actual service-domain dependencies")
	}
	if bootstrap {
		direct := false
		for _, outbound := range c.Outbounds {
			if outbound.IsPlainDirect() {
				direct = true
			}
		}
		if !direct {
			return dnsContractError("bootstrap direct detour is not the direct outbound")
		}
	}
	if len(c.Route.Rules) == 0 {
		return dnsContractError("port53 DNS capture must precede split and LAN routing")
	}
	first := c.Route.Rules[0]
	if first.Action != "hijack-dns" || len(first.Port) != 1 || first.Port[0] != 53 || first.Protocol != "" ||
		first.Outbound != "" || len(first.IPCidr) != 0 || len(first.DomainSuffix) != 0 || len(first.ProcessName) != 0 {
		return dnsContractError("first route must capture port53 without a sniff or split prerequisite")
	}
	for _, inbound := range c.Inbounds {
		if inbound.Type == "tun" && (!inbound.AutoRoute || !inbound.StrictRoute || len(inbound.RouteExcludeAddress) != 0) {
			return dnsContractError("TUN must use strict automatic routing without exclusions bypassing DNS capture")
		}
	}
	return nil
}
