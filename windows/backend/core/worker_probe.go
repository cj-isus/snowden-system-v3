package core

import (
	"context"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

// Private command values, not a public report-to-proof conversion. The original
// worker selects its real outbound; the request cannot name a local dialer,
// disable OS control, install a TLS override or provide authentication callbacks.
type workerProbeRequest struct {
	Targets         []string `json:"targets"`
	DNSProbeTargets []string `json:"dns_probe_targets"`
	ExpectedEgress  string   `json:"expected_egress"`
	TimeoutMillis   int64    `json:"timeout_millis"`
	TUN             bool     `json:"tun"`
	NativeNonce     string   `json:"native_nonce,omitempty"`
}

func workerSubject(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}

func (request *workerProbeRequest) settings() (*ProtectedProbe, error) {
	if request == nil || request.TimeoutMillis < 1 || request.TimeoutMillis > 30000 || len(request.Targets) < 2 || len(request.Targets) > 8 || len(request.DNSProbeTargets) < 2 || len(request.ExpectedEgress) > 64 || strings.TrimSpace(request.ExpectedEgress) != request.ExpectedEgress {
		return nil, errWorkerProtocol
	}
	if request.NativeNonce != "" && (!request.TUN || !workerSubject(request.NativeNonce)) {
		return nil, errWorkerProtocol
	}
	for _, target := range request.Targets {
		if len(target) > 2048 || strings.TrimSpace(target) != target {
			return nil, errWorkerProtocol
		}
		endpoint, err := url.Parse(target)
		if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" || endpoint.Opaque != "" {
			return nil, errWorkerProtocol
		}
	}
	if err := metadata.ValidateDNSProbeTargets(request.DNSProbeTargets); err != nil {
		return nil, errWorkerProtocol
	}
	settings := &ProtectedProbe{
		Targets: slices.Clone(request.Targets), DNSProbeTargets: slices.Clone(request.DNSProbeTargets),
		ExpectedEgress: request.ExpectedEgress, Timeout: time.Duration(request.TimeoutMillis) * time.Millisecond,
		DirectLeakThroughTunnel: request.TUN,
	}
	if err := settings.validatePlan(); err != nil {
		return nil, errWorkerProtocol
	}
	return settings, nil
}

func newWorkerProbeRequest(settings ProtectedProbe) (*workerProbeRequest, error) {
	if settings.AllowDirectEgress || settings.LeakCheckTarget != "" || settings.Timeout <= 0 || settings.Timeout%time.Millisecond != 0 {
		return nil, errors.New("protected worker probe requires complete standard OS control")
	}
	request := &workerProbeRequest{Targets: slices.Clone(settings.Targets), DNSProbeTargets: settings.resolverTargets(), ExpectedEgress: settings.ExpectedEgress, TimeoutMillis: settings.Timeout.Milliseconds(), TUN: settings.DirectLeakThroughTunnel}
	_, err := request.settings()
	return request, err
}

// Reserve in the GUI process too: moving I/O into its engine must not create
// another application quota. The worker independently bounds its actual work.
func (request *workerProbeRequest) reserve(ctx context.Context) (context.Context, func(), error) {
	settings, err := request.settings()
	if err != nil {
		return nil, nil, err
	}
	dnsRequests := len(settings.resolverTargets())
	requests := len(settings.Targets) + dnsRequests + 1
	return AdmitDiagnostic(ctx, DiagnosticProtected, requests, int64(requests*(maxProbeBody+1)+dnsRequests*maxDNSProbeQuery), settings.Timeout)
}

func workerProbeFailure(causes ...error) *ProbeReport {
	report := &ProbeReport{ProtectionUnavailable: true, Results: []ProbeResult{{Name: "original worker diagnostic", Status: "unavailable", Detail: "protected worker subject or bounded diagnostic receipt unavailable"}}}
	for _, cause := range causes {
		if errors.Is(cause, ErrGUIAncestorUnavailable) || errors.Is(cause, ErrEngineCleanupFailed) || errors.Is(cause, ErrHelperCleanup) || errors.Is(cause, ErrHelperBusy) || errors.Is(cause, ErrPreflightCleanup) {
			report.CleanupUnconfirmed = true
		}
		var trust *TrustFaultError
		if errors.As(cause, &trust) && trust != nil {
			report.TrustUnavailable, report.TrustReason = true, trustFaultReason(trust.Reason)
		}
	}
	return report
}

// Pure shape/correlation checks after the original exchange checked wire
// version/ID. This does not validate current context/engine/native authority.
func validWorkerProbeReport(request *workerRequest, reply workerReply) bool {
	return request != nil && request.Action == "probe" && request.Probe != nil && request.Selector == "proxy" &&
		workerSubject(request.Subject) && validWorkerTag(request.Tag) && reply.Subject == request.Subject && reply.Previous == request.Tag &&
		reply.AdmissionEpoch != 0 && reply.Report != nil && len(reply.Report.Results) > 0 && len(reply.Report.Results) <= 16 && reply.ScopeNonce == ""
}

// Only the original private pipe exchange calls this after wire correlation.
// Retain finite local restrictions before done even if the caller is canceled.
// No reply/report is promoted to health, a permit or a Job-retirement receipt.
func workerProbeReplyFailure(request *workerRequest, reply workerReply) error {
	if reply.Code == "probe_failed" && reply.Report == nil {
		if !emptyWorkerFailureReply(reply) {
			return errWorkerProtocol
		}
		return ErrEngineWorkerUnavailable // complete ordinary failure, without an invented local fault
	}
	if reply.Code != "ok" && reply.Code != "probe_failed" || !validWorkerProbeReport(request, reply) {
		return errWorkerProtocol
	}
	cause := reportLifecycleError(reply.Report)
	if cause != nil {
		if reply.Report.AllPassed || reply.AdmissionToken != "" || reply.NativeAdmissionToken != "" || reply.Code == "probe_failed" && reply.Admitted {
			return errWorkerProtocol // contradictory/foreign reports cannot mint a typed local fact
		}
		return cause
	}
	if reply.Code != "ok" {
		return errWorkerProtocol // a diagnostic failure envelope must contain a finite restriction
	}
	return nil
}

// Bound text before serialization, retaining machine outcomes. No network body,
// local authentication value or raw configuration is carried as a report.
func boundWorkerProbeReport(report *ProbeReport) *ProbeReport {
	if report == nil || len(report.Results) > 16 {
		return workerProbeFailure()
	}
	out := report.Clone()
	cut := func(text string, limit int) string {
		if len(text) > limit {
			return strings.ToValidUTF8(text[:limit], "")
		}
		return text
	}
	boundObservation := func(value *ProbeObservation) {
		value.Kind = cut(value.Kind, 64)
		value.Route = cut(value.Route, 64)
		value.Endpoint = cut(value.Endpoint, 512)
		value.Stage = cut(value.Stage, 64)
		value.ErrorKind = cut(value.ErrorKind, 64)
	}
	out.TrustReason = cut(out.TrustReason, 64)
	required := 0
	for index := range out.Results {
		result := &out.Results[index]
		*result = result.NormalizeDiagnostic()
		if !result.Informational() {
			required++
		}
		if !result.Passed && !result.Informational() {
			out.AllPassed = false
		}
		result.Name = cut(result.Name, 128)
		result.Status = cut(result.Status, 32)
		result.Detail = cut(result.Detail, 512)
		boundObservation(&result.Observation)
		for attempt := range result.ResolverAttempts {
			boundObservation(&result.ResolverAttempts[attempt])
		}
	}
	if required == 0 {
		out.AllPassed = false
	}
	return out
}
