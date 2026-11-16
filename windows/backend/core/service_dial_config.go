package core

// SDK-only socket detours keep the protocol clients and normalized subject
// unchanged. These tags are implementation bindings, never selector candidates.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/sagernet/sing-box/option"

	"github.com/snowden-system/windows/backend/config"
)

const serviceDialPrefix = "__snowden_service_"

func serviceDialReserved(tag string) bool { return strings.HasPrefix(tag, serviceDialPrefix) }

type serviceDialSpec struct {
	peer config.Outbound // endpoint/port metadata only, no credentials or TLS material
	dial config.Outbound // original socket options as a private SDK direct projection
}

func serviceDialTag(tag string) string {
	digest := sha256.Sum256([]byte(tag))
	return serviceDialPrefix + hex.EncodeToString(digest[:])
}

func lowerServiceDialers(cfg *config.Config) ([]config.Outbound, map[string]*serviceDialSpec, error) {
	for _, outbound := range cfg.Outbounds {
		if serviceDialReserved(outbound.Tag) {
			return nil, nil, ErrPreflightScope // no collision or caller-defined private constructor
		}
	}
	outbounds := append([]config.Outbound(nil), cfg.Outbounds...)
	private := make([]config.Outbound, 0)
	specs := make(map[string]*serviceDialSpec)
	for index, source := range cfg.Outbounds {
		if source.Type != "vless" && source.Type != "hysteria2" {
			continue
		}
		tag := serviceDialTag(source.Tag)
		if specs[tag] != nil {
			return nil, nil, ErrPreflightScope
		}
		projection := config.Outbound{Type: "direct", Tag: tag, Detour: source.Detour, DomainResolver: source.DomainResolver,
			TCPFastOpen: source.TCPFastOpen, TCPKeepAlive: source.TCPKeepAlive, TCPKeepAliveIt: source.TCPKeepAliveIt}
		specs[tag] = &serviceDialSpec{dial: projection, peer: config.Outbound{Type: source.Type, Tag: source.Tag, Server: source.Server,
			ServerPort: source.ServerPort, ServerPorts: append([]string(nil), source.ServerPorts...)}}
		outbounds[index].Detour, outbounds[index].DomainResolver = tag, nil
		outbounds[index].TCPFastOpen, outbounds[index].TCPKeepAlive, outbounds[index].TCPKeepAliveIt = false, "", ""
		private = append(private, projection)
	}
	// Constructors that inspect a detour must find its original object. The
	// normalized explicit route.final/selector remain unchanged by this order.
	return append(private, outbounds...), specs, nil
}

// Decode our bounded projection with the same official option decoder used by
// the SDK view. This does not construct a second client or confer authority.
func (spec *serviceDialSpec) options(ctx context.Context) (*option.DirectOutboundOptions, error) {
	raw, err := json.Marshal(spec.dial)
	if err != nil {
		return nil, ErrPreflightScope
	}
	var outbound option.Outbound
	if err := outbound.UnmarshalJSONContext(ctx, raw); err != nil {
		return nil, ErrPreflightScope
	}
	options, ok := outbound.Options.(*option.DirectOutboundOptions)
	if !ok || options == nil {
		return nil, ErrPreflightScope
	}
	return options, nil
}
