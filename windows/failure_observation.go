package main

import (
	"strings"

	"github.com/snowden-system/windows/backend/core"
)

// These codes describe observations. They never identify a censor, attribute a
// reset, infer an allowlist from a timeout, or select a transport by port alone.
func observedFailure(code string) FailureClass {
	return FailureClass{Code: code, Hint: failclassHints[code]}
}

func resultFailureCode(result core.ProbeResult) string {
	fact := result.Observation
	if fact.Schema != 1 {
		return fcObservationUnavailable
	}
	if fact.HTTPStatus != 0 && fact.HTTPStatus != 200 {
		return fcEndpointFailure
	}
	switch fact.ErrorKind {
	case "canceled", "body_limit", "invalid_request":
		return fcObservationUnavailable
	case "invalid_response", "http_status", "dns_answer_missing":
		return fcEndpointFailure
	case "timeout":
		return fcRequestTimeout
	case "tls_verification":
		return fcTLSVerification
	case "tls_protocol":
		return fcTLSHandshake
	case "short_response", "read_error":
		if fact.PayloadBytes > 0 || fact.HTTPStatus != 0 {
			return fcResponseIncomplete
		}
	}
	if fact.Stage == "tls" {
		return fcTLSHandshake
	}
	if fact.ErrorKind != "" {
		return fcConnectionFailed
	}
	return fcObservationUnavailable
}

// A missing comparison is not a divergent address. Normalize the step first:
// the machine name, reference, role and outcome must all describe this check.
// A real required mismatch survives unrelated missing measurement steps.
func observedEgressDivergence(report *core.ProbeReport) bool {
	if report == nil {
		return false
	}
	for _, step := range report.Results {
		step = step.NormalizeDiagnostic()
		if (step.Name == "Expected egress" || step.Name == "Egress consistency") &&
			!step.Informational() && !step.Passed && step.Status == "fail" &&
			step.Comparison.Valid() && step.Comparison.Outcome == core.EgressMismatch {
			return true
		}
	}
	return false
}

func classifyObservedFailure(report *core.ProbeReport) FailureClass {
	if report == nil {
		return observedFailure(fcObservationUnavailable)
	}
	// Safety facts outrank a missing control/configuration failure.
	if report.LeakConfirmed {
		return FailureClass{Code: fcLeakConfirmed, Confidence: 1, Hint: failclassHints[fcLeakConfirmed]}
	}
	for _, step := range report.Results {
		step = step.NormalizeDiagnostic()
		if step.Status == "leak" {
			return FailureClass{Code: fcLeakConfirmed, Confidence: 1, Hint: failclassHints[fcLeakConfirmed]}
		}
	}
	if report.Stale {
		return observedFailure(fcObservationUnavailable)
	}
	if report.TrustUnavailable {
		return FailureClass{Code: fcObservationUnavailable, Hint: "Срок доверия или непрерывность локальных часов не подтверждены; это не диагноз сети"}
	}
	if report.ProtectionUnavailable || report.CleanupUnconfirmed {
		return FailureClass{Code: fcObservationUnavailable, Hint: "Локальная защита или возврат ресурсов не подтверждены; это не отрицательный факт о канале"}
	}
	if observedEgressDivergence(report) {
		return observedFailure(fcEgressDivergence)
	}
	if len(report.Results) == 0 {
		return observedFailure(fcObservationUnavailable)
	}
	passed, dnsFailed, unavailable := 0, false, false
	var failed []core.ProbeResult
	for _, step := range report.Results {
		step = step.NormalizeDiagnostic()
		if step.Informational() {
			continue
		}
		ok := step.Passed && (step.Status == "" || step.Status == "pass")
		switch {
		case step.Name == "Expected egress" || step.Name == "Egress consistency":
			unavailable = unavailable || !ok
		case step.Name == "Direct leak check":
			unavailable = unavailable || !ok
		case step.Observation.Kind == "resolver_doh" || step.Observation.Kind == "resolver_doh_wire" || strings.HasPrefix(step.Name, "DNS"):
			dnsFailed = dnsFailed || !ok
		case step.Observation.Kind == "egress_http" || strings.HasPrefix(step.Name, "HTTPS "):
			if ok {
				passed++
			} else {
				failed = append(failed, step)
			}
		default:
			unavailable = unavailable || !ok
		}
	}
	if unavailable {
		return observedFailure(fcObservationUnavailable)
	}
	if len(failed) == 0 && dnsFailed {
		return observedFailure(fcDNSPathFailure)
	}
	if passed > 0 && len(failed) > 0 {
		return observedFailure(fcDegraded)
	}
	if len(failed) > 0 {
		code := resultFailureCode(failed[0])
		for _, step := range failed[1:] {
			if resultFailureCode(step) != code {
				return observedFailure(fcUnknown)
			}
		}
		return observedFailure(code)
	}
	if !report.AllPassed {
		return observedFailure(fcObservationUnavailable)
	}
	return FailureClass{Code: fcUnknown, Hint: "Наблюдение успешно; причина отказа не классифицируется"}
}

// Do not quarantine an otherwise usable channel because a measurement target,
// local budget, certificate/clock or optional resolver failed. A safety failure
// still blocks protection. Empty, unsupported and legacy causal codes also
// abstain: a caller-provided label is not a typed channel fact. Correlation of
// transport faults belongs to TH-03/04.
func measurementOnlyFailure(code string) bool {
	switch code {
	case fcLeakConfirmed, fcEgressDivergence:
		return false
	}
	return true
}
