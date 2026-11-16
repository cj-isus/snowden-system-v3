package main

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// A captured display cohort, not a new operation/session or admission receipt.
type factsEgressSource struct {
	registration           intentRegistration
	sourceCurrent, desired bool
	control                *core.OperationSource
	subject                appProbeSubject
}

// Optional display-only schema; original route ownership stays private.
type NetworkEgressFacts struct {
	Schema    int                         `json:"schema"`
	OSControl core.EgressRouteObservation `json:"osControl"`
	SOCKS     core.EgressRouteObservation `json:"socks"`
}

func (facts NetworkFacts) withEgressAge(now time.Time) NetworkFacts {
	if facts.Egress == nil {
		return facts
	}
	copy := *facts.Egress
	copy.OSControl, copy.SOCKS = copy.OSControl.AtReply(now), copy.SOCKS.AtReply(now)
	facts.Egress = &copy
	return facts
}

func (source *factsCollectSource) captureEgressSource() {
	a := source.app
	a.mu.Lock()
	source.egress.registration = a.intentRegistrationLocked()
	source.egress.sourceCurrent = source.egress.registration.source.Current()
	source.egress.desired = a.protectionDesired
	a.mu.Unlock()
	netEpoch.mu.Lock()
	source.egress.control = netEpoch.controlSource
	netEpoch.mu.Unlock()
	runtime_.mu.Lock()
	cc := runtime_.cc
	runtime_.mu.Unlock()
	if cc != nil {
		subject := a.appliedProbeSubject(cc)
		if subject.path == source.path && subject.epoch == source.epoch && subject.current(a, source.parent, cc) {
			source.egress.subject = subject
		}
	}
}

func (source *factsCollectSource) egressCurrent() bool {
	a := source.app
	a.mu.Lock()
	registration := source.egress.registration
	valid := a.operationID == registration.intent && a.intentSource == registration.source &&
		registration.source.Current() == source.egress.sourceCurrent && a.protectionDesired == source.egress.desired &&
		!a.profileTransition && !a.trustBlocked && !a.installedImageChanged && a.restartCommit == nil && a.releaseCommit == nil &&
		a.startupErr == nil && !a.watchdogBlocked.Load() && a.state.State != "starting" && a.state.State != "switching" && a.state.State != "stopping"
	a.mu.Unlock()
	if !valid {
		return false
	}
	netEpoch.mu.Lock()
	valid = source.egress.control == netEpoch.controlSource && !netEpoch.knownDown && !netEpoch.powerBlocked.Load()
	netEpoch.mu.Unlock()
	if !valid {
		return false
	}
	if source.egress.subject.applied != nil {
		return source.egress.subject.current(a, source.parent, source.egress.subject.applied)
	}
	return true
}

func (source *factsCollectSource) collectEgress(out *NetworkFacts) {
	skipped := func(route, reason string) core.EgressRouteObservation {
		return core.EgressRouteObservation{Route: route, Outcome: core.EgressSkipped, Reason: reason}
	}
	out.Egress = &NetworkEgressFacts{Schema: 1, OSControl: skipped(core.ProbeRouteOS, "no_intent"), SOCKS: skipped(core.ProbeRouteSOCKS, "no_intent")}
	if !source.current() || !source.egress.sourceCurrent {
		return
	}
	// OS control is an explicitly labelled observation, not a fallback from a
	// failed SOCKS request. TUN/strict protection never admits this control path.
	osControl := !source.tun && !strictProtectionRetained()
	out.Egress.OSControl = skipped(core.ProbeRouteOS, "policy")
	out.Egress.SOCKS = skipped(core.ProbeRouteSOCKS, "no_session")
	if source.tun {
		out.Egress.SOCKS = skipped(core.ProbeRouteSOCKS, "policy")
	}
	proxyAddress := ""
	if source.egress.subject.applied != nil && !source.tun {
		proxyAddress = net.JoinHostPort(render.MixedListenAddress, strconv.Itoa(render.MixedListenPort))
	}
	if !osControl && proxyAddress == "" {
		return
	}
	ctx := context.WithValue(source.parent, networkPathContextKey{}, source.path)
	ctx = core.WithOperationSources(ctx, core.OperationSources{Intent: source.egress.registration.source, Dependency: source.egress.control, Path: &source.path.current, Closing: &source.app.closing, Blocked: &source.app.watchdogBlocked})
	current := func() bool { return source.current() && (!osControl || !strictProtectionRetained()) }
	observed, err := core.ObserveEgressHTTPS(ctx, osControl, proxyAddress, current)
	if !source.current() {
		return
	}
	if osControl {
		out.Egress.OSControl = observed.OSObservation
	}
	if proxyAddress != "" {
		out.Egress.SOCKS = observed.SOCKSObservation
	}
	if err != nil {
		return
	} // local/budget/cleanup outcome is visible, never a positive IP
	var payload struct {
		IP           string `json:"ip"`
		Country      string `json:"country_name"`
		Organization string `json:"org"`
	}
	if out.Egress.OSControl.Outcome == core.EgressResponse {
		out.Egress.OSControl.Outcome = core.EgressInvalidResponse
		if len(observed.OSControl) != 0 && json.Unmarshal(observed.OSControl, &payload) == nil && safeFactsLabel(payload.Country, 160) && safeFactsLabel(payload.Organization, 256) {
			if ip := reportedEgressIP(payload.IP); ip != "" {
				out.PublicIP, out.Country, out.ISP = ip, payload.Country, payload.Organization
				out.CountryOrigin = "https://ipapi.co/json/" // actual requested URL, no invented fallback
				out.Egress.OSControl.Outcome = core.EgressOK
			}
		}
	}
	if out.Egress.SOCKS.Outcome == core.EgressResponse {
		out.Egress.SOCKS.Outcome = core.EgressInvalidResponse
		if ip := reportedEgressIP(strings.TrimSpace(string(observed.SOCKS))); ip != "" {
			out.TunnelIP = ip
			out.DNSViaTunnel = true // legacy field: SOCKS HTTPS reply only, never DNS/TUN proof
			out.Egress.SOCKS.Outcome = core.EgressOK
		}
	}
}

func reportedEgressIP(raw string) string {
	if len(raw) == 0 || len(raw) > 64 {
		return ""
	}
	ip, err := netip.ParseAddr(raw)
	if err != nil || ip.Zone() != "" {
		return ""
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
		return ""
	}
	return ip.String()
}

func safeFactsLabel(value string, limit int) bool {
	if len(value) > limit {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (a *App) retireFactsEgress() error {
	// Independent cleanup context only. No request/dial is created from it.
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	return core.RetireEgressHTTPS(ctx)
}
