package main

import (
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/netprobe"
)

// These value-only views deliberately omit packets, transaction IDs, nonces,
// worker/config/source identities and credentials. Fixed slots bound the API;
// only slots below the explicit counts contain observations.
type NativeDNSAttemptView struct {
	Endpoint      string `json:"endpoint"`
	Dependency    string `json:"dependency"`
	State         string `json:"state"`
	Stage         string `json:"stage"`
	Reason        string `json:"reason"`
	ElapsedMicros int64  `json:"elapsed_micros"`
	ResponseBytes int    `json:"response_bytes"`
}

type NativeDNSFamilyView struct {
	Network      string                  `json:"network"`
	State        string                  `json:"state"`
	AttemptCount int                     `json:"attempt_count"`
	Attempts     [2]NativeDNSAttemptView `json:"attempts"`
}

// Observed means a complete DNS batch, including unsuccessful attempts. It is
// not an aggregate VPN verdict, a DNSSEC/quorum claim or route attestation.
type NativeProbeReportView struct {
	Schema      int                    `json:"schema"`
	State       string                 `json:"state"`
	Reason      string                 `json:"reason"`
	StartedAt   string                 `json:"started_at"`
	CompletedAt string                 `json:"completed_at"`
	ExpiresAt   string                 `json:"expires_at"`
	FamilyCount int                    `json:"family_count"`
	Families    [4]NativeDNSFamilyView `json:"families"`
}

func nativeProbeNetworks() []string { return []string{"udp4", "tcp4", "udp6", "tcp6"} }

func emptyNativeProbeView(state, reason string) NativeProbeReportView {
	return NativeProbeReportView{Schema: 1, State: state, Reason: reason}
}

func nativeProbeExpiry(now time.Time, scope *nativeProbeScope) time.Time {
	expires := now.Add(60 * time.Second)
	for _, deadline := range []time.Time{scope.config.Validity.ProfileExpires, scope.config.Validity.IdentityExpires} {
		if !deadline.IsZero() && deadline.Before(expires) {
			expires = deadline
		}
	}
	return expires
}

func nativeProbeView(reply netprobe.NativeReply, started, completed, expires time.Time) (NativeProbeReportView, error) {
	if reply.Schema != netprobe.NativeProtocol || !reply.Complete || len(reply.Results) < 4 || len(reply.Results) > netprobe.MaxNativeAttempts || completed.Before(started) || !expires.After(completed) {
		return NativeProbeReportView{}, core.ErrNativeProbeIncomplete
	}
	view := emptyNativeProbeView("observed", "dns_batch_completed")
	view.StartedAt, view.CompletedAt, view.ExpiresAt = started.UTC().Format(time.RFC3339Nano), completed.UTC().Format(time.RFC3339Nano), expires.UTC().Format(time.RFC3339Nano)
	position := 0
	for index, network := range nativeProbeNetworks() {
		family := NativeDNSFamilyView{Network: network, State: "unavailable"}
		endpoints := netprobe.NativeEndpoints(network)
		for position < len(reply.Results) && reply.Results[position].Network == network && family.AttemptCount < 2 && family.State != "answered" {
			result := reply.Results[position]
			attempt := family.AttemptCount
			if result.ServerIP != endpoints[attempt] || len(result.Packet) > netprobe.MaxDNSMessage || result.ElapsedMicros < 0 || result.ElapsedMicros > int64(nativeProbeLimit/time.Microsecond) {
				return NativeProbeReportView{}, netprobe.ErrNativeProtocol
			}
			// DecodeNativeReply already validated stage/error and DNS correlation.
			// Keep the finite vocabulary explicit at this last public boundary too.
			switch result.Stage {
			case "dial", "write", "read", "response":
			default:
				return NativeProbeReportView{}, netprobe.ErrNativeProtocol
			}
			switch result.ErrorKind {
			case "", "invalid_dns_response", "dns_answer_missing", "dns_truncated", "canceled", "timeout", "dial_unavailable", "write_incomplete", "read_unavailable", "message_limit":
			default:
				return NativeProbeReportView{}, netprobe.ErrNativeProtocol
			}
			dependency := "cloudflare_public_dns"
			if attempt == 1 {
				dependency = "google_public_dns"
			}
			fact := NativeDNSAttemptView{Endpoint: result.ServerIP, Dependency: dependency, State: "unavailable", Stage: result.Stage,
				Reason: result.ErrorKind, ElapsedMicros: result.ElapsedMicros, ResponseBytes: len(result.Packet)}
			if result.ErrorKind == "" {
				fact.State, family.State = "answered", "answered"
			}
			family.Attempts[attempt] = fact
			family.AttemptCount++
			position++
		}
		if family.AttemptCount == 0 || family.State != "answered" && family.AttemptCount != 2 {
			return NativeProbeReportView{}, core.ErrNativeProbeIncomplete
		}
		view.Families[index] = family
		view.FamilyCount++
	}
	if position != len(reply.Results) {
		return NativeProbeReportView{}, netprobe.ErrNativeProtocol
	}
	return view, nil
}
