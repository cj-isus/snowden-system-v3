package config

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"unicode"

	"github.com/snowden-system/windows/backend/netprobe"
)

var ErrRoutingContract = errors.New("protected runtime routing contract unsupported")

// IsPlainDirect describes the one public direct role. Private SDK socket
// projections are created only after validation and are not accepted here.
func (o Outbound) IsPlainDirect() bool {
	return reflect.DeepEqual(o, Outbound{Type: "direct", Tag: "direct"})
}

// The caller has already validated proxy and every one of its candidates.
// Extra executable nodes must not introduce another selector, bootstrap
// dialer or an unvalidated service outside that original admitted graph.
func (c *Config) validateRuntimeGraph(tags map[string]Outbound) error {
	selector := tags["proxy"]
	candidates := make(map[string]struct{}, len(selector.Outbounds))
	for _, tag := range selector.Outbounds {
		candidates[tag] = struct{}{}
	}
	for _, outbound := range c.Outbounds {
		switch outbound.Tag {
		case "proxy":
			declaration := Outbound{Type: "selector", Tag: "proxy", Outbounds: selector.Outbounds,
				Default: selector.Default, InterruptExistConnections: selector.InterruptExistConnections}
			if !reflect.DeepEqual(outbound, declaration) {
				return fmt.Errorf("%w: proxy has non-selector options", ErrRoutingContract)
			}
		case "direct":
			if !outbound.IsPlainDirect() {
				return fmt.Errorf("%w: direct must be the plain canonical outbound", ErrRoutingContract)
			}
		case "block":
			if !reflect.DeepEqual(outbound, Outbound{Type: "block", Tag: "block"}) {
				return fmt.Errorf("%w: block must be the plain canonical outbound", ErrRoutingContract)
			}
		default:
			if _, ok := candidates[outbound.Tag]; !ok {
				return fmt.Errorf("%w: outbound is outside the protected selector and canonical split roles", ErrRoutingContract)
			}
		}
	}
	if c.Route.Final != "proxy" {
		return fmt.Errorf("%w: route.final must use the protected proxy selector", ErrRoutingContract)
	}
	for index, rule := range c.Route.Rules {
		if err := c.validateRouteRule(rule, tags); err != nil {
			return fmt.Errorf("%w: rule %d: %s", ErrRoutingContract, index+1, err)
		}
	}
	return nil
}

func (c *Config) validateRouteRule(rule RouteRule, tags map[string]Outbound) error {
	if rule.Protocol != "" && (strings.TrimSpace(rule.Protocol) != rule.Protocol || strings.IndexFunc(rule.Protocol, unicode.IsControl) >= 0) {
		return errors.New("protocol condition is not a nonempty protocol name")
	}
	for _, prefix := range rule.IPCidr {
		parsed, err := netip.ParsePrefix(prefix)
		if err != nil || parsed.Addr().Is4In6() || (rule.Outbound == "direct" && parsed.Bits() == 0) {
			return errors.New("IP condition requires an unmapped prefix; direct cannot use a /0 prefix")
		}
	}
	for _, suffix := range rule.DomainSuffix {
		name := strings.TrimPrefix(suffix, ".")
		canonical, err := netprobe.CanonicalDNSHost(name)
		if err != nil || canonical != name {
			return errors.New("domain suffix must contain a canonical nonempty hostname")
		}
	}
	for _, process := range rule.ProcessName {
		if process == "" || strings.ContainsAny(process, `/\*?`) || strings.IndexFunc(process, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) >= 0 {
			return errors.New("process condition must contain explicit nonempty basenames")
		}
	}
	if len(rule.ProcessName) != 0 && !c.Route.FindProcess {
		return errors.New("process conditions require find_process")
	}
	for _, port := range rule.Port {
		if port == 0 {
			return errors.New("port condition cannot contain zero")
		}
	}
	switch rule.Action {
	case "", "route":
		// A named candidate would bypass the selected-channel admission even
		// if its protocol was validated. Split rules target only these roles.
		switch rule.Outbound {
		case "proxy", "direct", "block":
		default:
			return errors.New("route target must be proxy, direct or block")
		}
		if _, exists := tags[rule.Outbound]; !exists {
			return errors.New("route target is absent")
		}
		if rule.Outbound == "direct" && rule.Protocol == "" && len(rule.IPCidr) == 0 && len(rule.DomainSuffix) == 0 && len(rule.ProcessName) == 0 && len(rule.Port) == 0 {
			return errors.New("direct requires an explicit nonempty split condition")
		}
	case "hijack-dns":
		if rule.Outbound != "" {
			return errors.New("DNS action cannot carry an outbound")
		}
		port53 := rule.Protocol == "" && len(rule.Port) == 1 && rule.Port[0] == 53
		dnsProtocol := rule.Protocol == "dns" && len(rule.Port) == 0
		if (!port53 && !dnsProtocol) || len(rule.IPCidr) != 0 || len(rule.DomainSuffix) != 0 || len(rule.ProcessName) != 0 {
			return errors.New("DNS action requires the supported port53 or DNS-protocol capture rule")
		}
	default:
		// In particular, SDK direct/bypass/resolve/route-options actions do
		// not acquire authority merely by carrying a nonempty action string.
		return errors.New("action is outside the supported route and DNS-capture contract")
	}
	return nil
}
