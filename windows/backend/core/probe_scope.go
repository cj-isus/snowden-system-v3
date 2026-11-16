package core

import (
	"strings"
	"time"
)

const (
	ProbeScopeObserved   = "observed"
	ProbeScopeIncomplete = "incomplete"
	ProbeScopeUnmeasured = "unmeasured"
)

// ProbeScope describes evidence in one report, not current reachability or a
// forwarding permit. Requested engine/SOCKS HTTP routes are application scope;
// neither their egress IP nor a DNS A answer identifies the native IP family.
// Value-only fields preserve existing report/view ownership without new slices.
type ProbeScope struct {
	Schema      int    `json:"schema"`
	HTTPS       string `json:"https"`
	DoH         string `json:"doh"`
	IPv4TCP     string `json:"ipv4_tcp"`
	IPv4UDP     string `json:"ipv4_udp"`
	IPv6TCP     string `json:"ipv6_tcp"`
	IPv6UDP     string `json:"ipv6_udp"`
	LocalSafety string `json:"local_safety"`
	Historical  bool   `json:"historical"`
}

type scopeApplicationGroup struct{ total, typed, complete int }

func (group scopeApplicationGroup) status(usable bool) string {
	if group.typed == 0 {
		return ProbeScopeUnmeasured
	}
	if !usable || group.complete != group.total {
		return ProbeScopeIncomplete
	}
	return ProbeScopeObserved
}

// DescribeProbeScope can only describe/restrict existing measurements. It does
// not parse Detail, infer a family from IP/port/protocol labels, or change a
// report's aggregate/required steps. Public native step names are not receipts.
func DescribeProbeScope(report *ProbeReport) ProbeScope {
	scope := ProbeScope{Schema: 1, HTTPS: ProbeScopeUnmeasured, DoH: ProbeScopeUnmeasured,
		IPv4TCP: ProbeScopeUnmeasured, IPv4UDP: ProbeScopeUnmeasured,
		IPv6TCP: ProbeScopeUnmeasured, IPv6UDP: ProbeScopeUnmeasured, LocalSafety: ProbeScopeUnmeasured}
	if report == nil {
		return scope
	}
	scope.Historical = report.Stale
	if len(report.Results) > 16 {
		return scope
	}
	usable := !report.Stale && !report.StartedAt.IsZero() && !report.CompletedAt.IsZero() && !report.CompletedAt.Before(report.StartedAt)
	var https, doh scopeApplicationGroup
	for _, input := range report.Results {
		result := input.NormalizeDiagnostic()
		fact := result.Observation
		var group *scopeApplicationGroup
		kind := ""
		switch {
		case strings.HasPrefix(result.Name, "HTTPS "):
			group, kind = &https, "egress_http"
		case result.Name == "DNS через туннель (DoH)":
			group, kind = &doh, "resolver_doh_wire"
		default:
			continue
		}
		group.total++
		if fact.Schema != 1 || fact.Kind != kind || fact.Route != ProbeRouteEngineOutbound && fact.Route != ProbeRouteSOCKS {
			continue
		}
		group.typed++
		complete := usable && result.Passed && result.Status == "pass" && !result.Informational() &&
			fact.Endpoint != "" && fact.Endpoint != "invalid-endpoint" && len(fact.Endpoint) <= 512 &&
			fact.Stage == "body" && fact.BodyComplete && fact.ErrorKind == "" && fact.HTTPStatus == 200 &&
			fact.PayloadLimit > 0 && fact.PayloadLimit <= maxProbeBody && fact.PayloadBytes > 0 && fact.PayloadBytes <= fact.PayloadLimit &&
			!fact.StartedAt.IsZero() && !fact.StartedAt.Before(report.StartedAt) && !fact.StartedAt.After(report.CompletedAt) &&
			fact.ElapsedMicros >= 0 && fact.ElapsedMicros <= (30*time.Second).Microseconds()
		if kind == "egress_http" {
			complete = complete && result.Name == "HTTPS "+fact.Endpoint
		} else {
			count := result.ResolverAttemptCount
			complete = complete && count > 0 && count <= len(result.ResolverAttempts) && result.ResolverAttempts[count-1] == fact
		}
		if complete {
			group.complete++
		}
	}
	scope.HTTPS, scope.DoH = https.status(usable), doh.status(usable)
	return scope
}
