package render

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/snowden-system/windows/backend/metadata"
)

type DependencyNode struct {
	ID, Namespace, Identity string
}

type DependencyBinding struct {
	ChannelID, Role, NodeID, Provenance, Relation string
}

type DependencyGraph struct {
	Nodes     []DependencyNode
	Bindings  []DependencyBinding
	byChannel map[string][]DependencyBinding
	nodes     map[string]DependencyNode
}

type DependencyComparison struct {
	Shared                 []string
	Different              []string
	Unknown                []string
	KnownRoles, TotalRoles int
	Coverage, RankHint     float64
}

// Integer weights are declared ranking policy, never physical independence
// probabilities. Unknown roles remain in the denominator and add zero benefit.
var dependencyRoles = []struct {
	name, label, namespace string
	weight                 int
}{
	{"origin", "origin", "endpoint", 22},
	{"origin_asn", "ASN origin", "asn", 12},
	{"carrier", "carrier", "provider", 10},
	{"exit", "exit", "endpoint", 18},
	{"exit_asn", "ASN exit", "asn", 10},
	{"resolver", "resolver", "endpoint", 8},
	{"hosting_account", "аккаунт хостинга", "account", 8},
	{"publisher", "издатель профиля", "provider", 4},
	{"profile_key", "ключ профиля", "profile_key", 4},
	{"distribution_account", "аккаунт доставки", "account", 4},
	{"donor", "TLS-донор", "endpoint", 5},
}

func dependencyIdentity(namespace, raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if namespace == "endpoint" {
		value = strings.TrimSuffix(value, ".")
	}
	if value == "" || value == "unknown" || !metadata.ValidDependencyLabel(value) {
		return ""
	}
	if namespace == "account" && !metadata.ValidDependencyAccount(value) {
		return ""
	}
	if namespace == "endpoint" {
		if ip := net.ParseIP(value); ip != nil {
			return ip.String()
		}
	}
	if namespace == "asn" && strings.HasPrefix(value, "as") {
		if number, err := strconv.ParseUint(value[2:], 10, 64); err == nil {
			return "as" + strconv.FormatUint(number, 10)
		}
	}
	return value
}

// Source labels distinguish configuration targets, owner assertions and a
// locally verified signing key. No label is promoted to live path evidence.
func BuildDependencyGraph(channels []ChannelDescriptor, profile ProfileProvenance) (*DependencyGraph, error) {
	if len(channels) > metadata.MaxProfileChannels {
		return nil, fmt.Errorf("dependencies: profile exceeds channel bound")
	}
	if err := profile.Dependencies.Validate(); err != nil {
		return nil, err
	}
	graph := &DependencyGraph{byChannel: map[string][]DependencyBinding{}, nodes: map[string]DependencyNode{}}
	add := func(id, role, namespace, identity, source, relation string) {
		value := dependencyIdentity(namespace, identity)
		nodeID := ""
		if value != "" {
			nodeID = namespace + ":" + value
			if _, exists := graph.nodes[nodeID]; !exists {
				node := DependencyNode{ID: nodeID, Namespace: namespace, Identity: value}
				graph.nodes[nodeID] = node
				graph.Nodes = append(graph.Nodes, node)
			}
		} else if relation != "not_applicable" {
			source = "unknown"
		}
		binding := DependencyBinding{ChannelID: id, Role: role, NodeID: nodeID, Provenance: source, Relation: relation}
		graph.Bindings = append(graph.Bindings, binding)
		graph.byChannel[id] = append(graph.byChannel[id], binding)
	}
	for _, channel := range channels {
		if !metadata.ValidChannelID(channel.ID) {
			return nil, fmt.Errorf("dependencies: invalid channel identity")
		}
		if _, duplicate := graph.byChannel[channel.ID]; duplicate {
			return nil, fmt.Errorf("dependencies: duplicate channel identity")
		}
		if err := metadata.ValidateFailureDomain(channel.FD); err != nil {
			return nil, err
		}
		fd := channel.effectiveFD()
		values := map[string]string{"origin": fd.Origin, "origin_asn": fd.ASN, "carrier": fd.Carrier, "exit": fd.Exit, "exit_asn": fd.ExitASN, "resolver": fd.Resolver, "hosting_account": fd.Account, "donor": fd.DonorSNI, "publisher": profile.Dependencies.Publisher, "distribution_account": profile.Dependencies.Account}
		if profile.Source == "verified_metadata_signature" {
			if raw, err := hex.DecodeString(profile.SignerFingerprint); err == nil && len(raw) == 32 && profile.Version != 0 {
				values["profile_key"] = profile.SignerFingerprint
			}
		}
		for _, role := range dependencyRoles {
			source := "owner_assertion"
			relation := "required"
			if role.name == "donor" && (channel.Transport == "ws" || channel.Transport == "quic") && channel.FD.DonorSNI == "" {
				source, relation = "transport_configuration", "not_applicable"
			}
			if role.name == "origin" && channel.FD.Origin == "" {
				source = "configured_origin"
			}
			if role.name == "donor" && channel.FD.DonorSNI == "" && relation != "not_applicable" {
				source = "configured_donor"
			}
			if role.name == "profile_key" && values[role.name] != "" {
				source = "verified_profile_signature"
			}
			add(channel.ID, role.name, role.namespace, values[role.name], source, relation)
		}
		access := channel.OriginServer
		if channel.Transport == "ws" {
			access = channel.Hostname
		}
		add(channel.ID, "access_target", "endpoint", access, "configured_target", "required")
		artifact := profile.EnvelopeSHA256
		if profile.Source == "embedded_configuration" {
			artifact = "embedded_configuration"
		}
		add(channel.ID, "local_profile", "artifact", artifact, "local_configuration", "required")
		add(channel.ID, "recovery_publisher", "provider", profile.Dependencies.RecoveryPublisher, "owner_assertion", "alternative_unverified")
		if target, err := url.Parse(ExternalTUNProbeURL); err == nil {
			add(channel.ID, "tun_measurement_endpoint", "endpoint", target.Hostname(), "compiled_tun_configuration", "conditional_measurement")
		}
		targets := channel.ProbeTargets
		if len(targets) == 0 {
			targets = DefaultProbeTargets
		}
		if len(targets) > 3 {
			return nil, fmt.Errorf("dependencies: measurement endpoint count exceeds bound")
		}
		for _, target := range targets {
			if parsed, err := url.Parse(target); err == nil {
				add(channel.ID, "measurement_endpoint", "endpoint", parsed.Hostname(), "configured_measurement", "measurement")
			}
		}
		resolvers, provenance := channel.DNSProbeTargets, "configured_measurement"
		if len(resolvers) == 0 {
			resolvers, provenance = metadata.DefaultDNSProbeTargets(), "compiled_measurement_configuration"
		}
		if err := metadata.ValidateDNSProbeTargets(resolvers); err != nil {
			return nil, err
		}
		for _, target := range resolvers {
			parsed, err := url.Parse(target)
			if err != nil {
				return nil, err
			}
			add(channel.ID, "resolver_measurement_endpoint", "endpoint", parsed.Hostname(), provenance, "measurement")
		}
	}
	return graph, nil
}

func (g *DependencyGraph) Compare(a, b string) DependencyComparison {
	out := DependencyComparison{TotalRoles: len(dependencyRoles)}
	left, right := g.byChannel[a], g.byChannel[b]
	leftRoles, rightRoles := map[string]DependencyBinding{}, map[string]DependencyBinding{}
	leftNodes, rightNodes := map[string][]string{}, map[string][]string{}
	leftAlternatives, rightAlternatives := map[string][]string{}, map[string][]string{}
	for _, binding := range left {
		leftRoles[binding.Role] = binding
		if binding.NodeID != "" && binding.Relation == "alternative_unverified" {
			leftAlternatives[binding.NodeID] = append(leftAlternatives[binding.NodeID], binding.Role)
		}
		if binding.NodeID != "" && binding.Relation != "alternative_unverified" {
			leftNodes[binding.NodeID] = append(leftNodes[binding.NodeID], binding.Role)
		}
	}
	for _, binding := range right {
		rightRoles[binding.Role] = binding
		if binding.NodeID != "" && binding.Relation == "alternative_unverified" {
			rightAlternatives[binding.NodeID] = append(rightAlternatives[binding.NodeID], binding.Role)
		}
		if binding.NodeID != "" && binding.Relation != "alternative_unverified" {
			rightNodes[binding.NodeID] = append(rightNodes[binding.NodeID], binding.Role)
		}
	}
	criticalShared := false
	sharedNames := map[string]bool{}
	for node, roles := range leftAlternatives {
		if other := append(slices.Clone(rightAlternatives[node]), rightNodes[node]...); len(other) > 0 {
			sharedNames[dependencyRoleLabels(roles)+" ↔ "+dependencyRoleLabels(other)+" (резерв не проверен)"] = true
		}
	}
	for node, roles := range rightAlternatives {
		if other := leftNodes[node]; len(other) > 0 {
			sharedNames[dependencyRoleLabels(other)+" ↔ "+dependencyRoleLabels(roles)+" (резерв не проверен)"] = true
		}
	}
	for node, roles := range leftNodes {
		other, shared := rightNodes[node]
		if !shared {
			continue
		}
		name, otherName := dependencyRoleLabels(roles), dependencyRoleLabels(other)
		if name != otherName {
			name += " ↔ " + otherName
		}
		sharedNames[name] = true
		for _, role := range append(slices.Clone(roles), other...) {
			if role == "origin" || role == "access_target" {
				criticalShared = true
			}
		}
	}
	for name := range sharedNames {
		out.Shared = append(out.Shared, name)
	}
	slices.Sort(out.Shared)
	total, known, different := 0, 0, 0
	for _, role := range dependencyRoles {
		l, r := leftRoles[role.name], rightRoles[role.name]
		if l.Relation == "not_applicable" && r.Relation == "not_applicable" {
			out.TotalRoles--
			continue
		}
		total += role.weight
		if l.Relation == "not_applicable" && r.NodeID != "" || r.Relation == "not_applicable" && l.NodeID != "" {
			known += role.weight
			out.KnownRoles++
			_, sharedLeft := rightNodes[l.NodeID]
			_, sharedRight := leftNodes[r.NodeID]
			if !sharedLeft && !sharedRight && a != b {
				different += role.weight
				out.Different = append(out.Different, role.label)
			}
			continue
		}
		if l.NodeID == "" || r.NodeID == "" {
			out.Unknown = append(out.Unknown, role.label)
			continue
		}
		known += role.weight
		out.KnownRoles++
		_, leftIsShared := rightNodes[l.NodeID]
		_, rightIsShared := leftNodes[r.NodeID]
		if a != b && l.NodeID != r.NodeID && !leftIsShared && !rightIsShared {
			different += role.weight
			out.Different = append(out.Different, role.label)
		}
	}
	if total > 0 {
		out.Coverage = float64(known) / float64(total)
		out.RankHint = float64(different) / float64(total)
	}
	if criticalShared && out.RankHint > .25 {
		out.RankHint = .25
	}
	return out
}

func dependencyRoleLabels(roles []string) string {
	labels := make([]string, 0, len(roles))
	for _, role := range roles {
		label := role
		for _, known := range dependencyRoles {
			if known.name == role {
				label = known.label
				break
			}
		}
		switch role {
		case "access_target":
			label = "адрес входа"
		case "local_profile":
			label = "локальный профиль"
		case "measurement_endpoint":
			label = "HTTP-измеритель"
		case "tun_measurement_endpoint":
			label = "внешний IPv4 TUN-измеритель (условно)"
		case "recovery_publisher":
			label = "резервный издатель"
		}
		if !slices.Contains(labels, label) {
			labels = append(labels, label)
		}
	}
	slices.Sort(labels)
	return strings.Join(labels, "/")
}

func CompareDependencies(a, b ChannelDescriptor, profiles ...ProfileProvenance) DependencyComparison {
	var profile ProfileProvenance
	if len(profiles) == 1 {
		profile = profiles[0]
	}
	graph, err := BuildDependencyGraph([]ChannelDescriptor{a, b}, profile)
	if err != nil {
		graph, _ = BuildDependencyGraph(nil, ProfileProvenance{})
	}
	return graph.Compare(a.ID, b.ID)
}

func (c DependencyComparison) Summary() string {
	parts := []string{fmt.Sprintf("заполнено %d/%d ролей", c.KnownRoles, c.TotalRoles)}
	if len(c.Shared) > 0 {
		parts = append(parts, "общие зависимости: "+strings.Join(c.Shared, ", "))
	}
	if len(c.Unknown) > 0 {
		parts = append(parts, "неизвестно: "+strings.Join(c.Unknown, ", "))
	}
	return strings.Join(parts, "; ") + "; различие меток не доказывает физическую независимость"
}
