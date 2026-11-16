package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

func (pp *ProtectedProbe) resolverTargets() []string {
	if len(pp.DNSProbeTargets) != 0 {
		return slices.Clone(pp.DNSProbeTargets)
	}
	return metadata.DefaultDNSProbeTargets()
}

// This is one resolver-over-HTTPS step. Endpoint fallback is not native DNS,
// UDP, IPv6, DNSSEC validation, quorum or evidence of independent physical paths.
// Attempts share the caller's already admitted overall budget and SOCKS client.
func (pp *ProtectedProbe) checkDNS(ctx context.Context, client *http.Client) ProbeResult {
	result := ProbeResult{Name: "DNS через туннель (DoH)", Status: "unavailable"}
	if pp == nil || ctx == nil || client == nil || pp.Timeout <= 0 || pp.Timeout > 30*time.Second {
		result.Detail = "DoH observation configuration unavailable"
		return result
	}
	targets := pp.resolverTargets()
	if err := metadata.ValidateDNSProbeTargets(targets); err != nil {
		result.Detail = "Configured DoH observation sources are invalid; defaults not substituted"
		return result
	}
	query, err := dnsProbeQuery()
	if err != nil {
		result.Detail = "Bounded DNS question could not be prepared"
		return result
	}
	for index, target := range targets {
		if ctx.Err() != nil {
			break
		}
		remaining := pp.Timeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining = min(remaining, time.Until(deadline))
		}
		if remaining <= 0 {
			break
		}
		// Reserve time for a later source instead of letting the first endpoint
		// occupy the entire overall probe deadline. No new goroutine or queue.
		attemptContext, cancel := context.WithTimeout(ctx, remaining/time.Duration(len(targets)-index))
		fact, body, err := fetchProbeHTTPRequest(attemptContext, client, target, pp.tunnelRoute(), "resolver_doh_wire", dnsProbeMediaType, maxProbeBody,
			probeHTTPRequest{method: http.MethodPost, contentType: dnsProbeMediaType, expectedMediaType: dnsProbeMediaType, body: query})
		cancel()
		var addresses []string
		if err == nil {
			if fact.HTTPStatus != http.StatusOK {
				fact.ErrorKind = "http_status"
			} else if addresses, err = dnsProbeAnswers(body); err != nil {
				switch {
				case errors.Is(err, errDNSProbeAnswerMissing):
					fact.ErrorKind = "dns_answer_missing"
				case errors.Is(err, errDNSProbeTruncated):
					fact.ErrorKind = "dns_truncated"
				default:
					fact.ErrorKind = "invalid_dns_response"
				}
			}
		}
		result.Observation = fact
		result.ResolverAttempts[index] = fact
		result.ResolverAttemptCount = index + 1
		if err == nil && fact.HTTPStatus == http.StatusOK && len(addresses) != 0 && ctx.Err() == nil {
			result.Passed, result.Status = true, "pass"
			result.Detail = fmt.Sprintf("DoH wire A %s → %v; попытка %d/%d; native DNS/UDP/IPv6/DNSSEC не измерялись; %s", dnsCheckHost, addresses, index+1, len(targets), fact.Summary())
			return result
		}
	}
	summaries := make([]string, 0, result.ResolverAttemptCount)
	for _, attempt := range result.ResolverAttempts[:result.ResolverAttemptCount] {
		summaries = append(summaries, attempt.Summary())
	}
	result.Detail = "DoH NOERROR/A-ответ не подтверждён; " + strings.Join(summaries, "; ")
	return result
}
