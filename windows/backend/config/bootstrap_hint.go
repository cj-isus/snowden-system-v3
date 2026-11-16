package config

import (
	"fmt"
	"net"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

const (
	MaxBootstrapHints   = 8
	MaxBootstrapHintTTL = 5 * time.Minute
)

// BootstrapHint is an expiring address suggestion, never a server identity or
// forwarding permission. Retained rollback bytes may contain expired hints;
// temporal freshness is checked at use, without changing the original interval.
type BootstrapHint struct {
	Host       string    `json:"host"`
	Address    string    `json:"address"`
	ObservedAt time.Time `json:"observed_at"`
	Until      time.Time `json:"until"`
}

func (hint BootstrapHint) ValidFor(server string) bool {
	host, err := netprobe.CanonicalDNSHost(hint.Host)
	service, serviceErr := netprobe.CanonicalDNSHost(server)
	ip := net.ParseIP(hint.Address)
	return err == nil && serviceErr == nil && host == hint.Host && host == service &&
		netprobe.PublicIPv4(ip) && ip.String() == hint.Address &&
		!hint.ObservedAt.IsZero() && hint.ObservedAt.Year() >= 1 && hint.ObservedAt.Year() <= 9999 &&
		hint.Until.Year() >= 1 && hint.Until.Year() <= 9999 && hint.Until.After(hint.ObservedAt) &&
		hint.Until.Sub(hint.ObservedAt) <= MaxBootstrapHintTTL
}

func (hint BootstrapHint) FreshAt(now time.Time) bool {
	return !now.IsZero() && !now.Before(hint.ObservedAt) && now.Before(hint.Until)
}

func (c *Config) validateBootstrapHints() error {
	if len(c.BootstrapHints) > MaxBootstrapHints {
		return fmt.Errorf("bootstrap hints exceed bounded channel set")
	}
	for tag, hint := range c.BootstrapHints {
		found := false
		for _, outbound := range c.Outbounds {
			if outbound.Tag != tag {
				continue
			}
			found = (outbound.Type == "vless" || outbound.Type == "hysteria2") && hint.ValidFor(outbound.Server) &&
				outbound.DomainResolver != nil && outbound.DomainResolver.Server == BootstrapDNSTag
			break
		}
		if !found {
			return fmt.Errorf("bootstrap hint has no matching protected service: %s", tag)
		}
	}
	return nil
}
