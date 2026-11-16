package render

import (
	"fmt"
	"net"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/netprobe"
)

// Bind bootstrap to the actual retained service address. TLS donor/SNI/Host
// names are service identities, not addresses
// to resolve locally. User queries for any of these names still use DNS final.
func configureDNS(cfg *config.Config) error {
	needsBootstrap := false
	for index := range cfg.Outbounds {
		outbound := &cfg.Outbounds[index]
		if outbound.Type != "vless" && outbound.Type != "hysteria2" {
			continue
		}
		if net.ParseIP(outbound.Server) != nil {
			continue
		}
		if _, err := netprobe.CanonicalDNSHost(outbound.Server); err != nil {
			return fmt.Errorf("channel %s: invalid bootstrap service name", outbound.Tag)
		}
		outbound.DomainResolver = &config.DomainResolveOptions{
			Server: config.BootstrapDNSTag, Strategy: "prefer_ipv4", DisableCache: true,
		}
		needsBootstrap = true
	}
	cfg.DNS = &config.DNSConfig{
		Final: config.ProtectedDNSTag, Strategy: "prefer_ipv4", IndependentCache: true,
		CacheCapacity: config.ProtectedDNSCacheCapacity,
		Servers: []config.DNSServer{{
			Tag: config.ProtectedDNSTag, Type: "tls", Server: "8.8.8.8", ServerPort: 853, Detour: "proxy",
			TLS: &config.DNSTLSConfig{Enabled: true, ServerName: "dns.google"},
		}},
	}
	if needsBootstrap {
		cfg.DNS.Servers = append(cfg.DNS.Servers, config.DNSServer{
			Tag: config.BootstrapDNSTag, Type: "udp", Server: "1.1.1.1", ServerPort: 53, Detour: "direct",
		})
	}
	return nil
}

func applyBootstrapHints(cfg *config.Config, hints map[string]config.BootstrapHint, now time.Time) error {
	if len(hints) > config.MaxBootstrapHints {
		return fmt.Errorf("bootstrap hints exceed bounded channel set")
	}
	for tag, hint := range hints {
		found := false
		for _, outbound := range cfg.Outbounds {
			if outbound.Tag == tag && (outbound.Type == "vless" || outbound.Type == "hysteria2") && hint.ValidFor(outbound.Server) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("bootstrap hint has no matching service: %s", tag)
		}
		if hint.FreshAt(now) {
			if cfg.BootstrapHints == nil {
				cfg.BootstrapHints = make(map[string]config.BootstrapHint)
			}
			cfg.BootstrapHints[tag] = hint
		}
	}
	return nil
}
