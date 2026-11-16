package core

import (
	"fmt"
	"net"
	"strings"
)

const (
	EgressMatch    = "match"
	EgressMismatch = "mismatch"
	EgressUnknown  = "unknown"

	comparisonExpected    = "profile_expected"
	comparisonConsistency = "protected_responses"
	comparisonControl     = "control_vs_protected"
)

// ProbeComparison describes reported addresses, never local packet capture,
// actual route attestation or absence of forbidden sends. Value fields preserve
// existing owned report copies. No nonce, raw body or extra addresses are kept.
type ProbeComparison struct {
	Schema           int    `json:"schema"`
	Reference        string `json:"reference"`
	Outcome          string `json:"outcome"`
	ObservationPoint string `json:"observation_point"`
	Topology         string `json:"topology"`
	PathScope        string `json:"path_scope"`
	LocalSafety      string `json:"local_safety"`
	ViolationBasis   string `json:"violation_basis"`
}

func egressComparison(reference, outcome string) ProbeComparison {
	return ProbeComparison{Schema: 1, Reference: reference, Outcome: outcome,
		ObservationPoint: "remote_https_response", Topology: "unknown", PathScope: "unattested", LocalSafety: "unknown", ViolationBasis: "none"}
}

func (comparison ProbeComparison) Valid() bool {
	return comparison.Schema == 1 && (comparison.Reference == comparisonExpected || comparison.Reference == comparisonConsistency || comparison.Reference == comparisonControl) &&
		(comparison.Outcome == EgressMatch || comparison.Outcome == EgressMismatch || comparison.Outcome == EgressUnknown) &&
		comparison.ObservationPoint == "remote_https_response" && comparison.Topology == "unknown" && comparison.PathScope == "unattested" && comparison.LocalSafety == "unknown" && comparison.ViolationBasis == "none"
}

// NormalizeComparison may only restrict a result. A valid metadata shape alone
// cannot turn unknown addresses, a different reference or an IP relation into
// a successful expected-egress check or local violation evidence. Report copies
// and the private worker admission gate use this same value-only boundary.
func (result ProbeResult) NormalizeComparison() ProbeResult {
	reference := ""
	switch result.Name {
	case "Expected egress":
		reference = comparisonExpected
	case "Egress consistency":
		reference = comparisonConsistency
	case "Direct leak check":
		reference = comparisonControl
	}
	if reference == "" && result.Comparison == (ProbeComparison{}) {
		return result
	}
	if reference == "" || !result.Comparison.Valid() || result.Comparison.Reference != reference {
		result.Comparison = ProbeComparison{}
		result.Passed, result.Status = false, "unavailable"
		result.Detail = "Сравнение egress не соответствует шагу или поддерживаемой схеме; локальная безопасность не измерена"
		return result
	}
	if result.Status == "leak" {
		result.Comparison = ProbeComparison{}
		result.Passed, result.Status = false, "unavailable"
		result.Detail = "IP-сопоставление не подтверждает локальную утечку; требуется отдельное наблюдение"
		return result
	}
	if result.Comparison.Outcome == EgressUnknown {
		if result.Passed || result.Status == "pass" {
			result.Detail = "Недостаточно данных для сравнения IP; успешный результат не подтверждён"
		}
		result.Passed, result.Status = false, "unavailable"
	} else if reference != comparisonControl && result.Comparison.Outcome == EgressMismatch {
		if result.Passed || result.Status == "pass" {
			result.Detail = "Сообщённые IP расходятся с ожидаемым сравнением; успешный результат не подтверждён"
		}
		result.Passed, result.Status = false, "fail"
	}
	return result
}

func compareReportedEgress(left, right string) string {
	a, b := net.ParseIP(strings.TrimSpace(left)), net.ParseIP(strings.TrimSpace(right))
	if a == nil || b == nil {
		return EgressUnknown
	}
	if a.Equal(b) {
		return EgressMatch
	}
	return EgressMismatch
}

func expectedEgressResult(addresses []string, expected string) ProbeResult {
	outcome := compareReportedEgress(firstOr(addresses, ""), expected)
	result := ProbeResult{Name: "Expected egress", Comparison: egressComparison(comparisonExpected, outcome)}
	switch outcome {
	case EgressMatch:
		result.Passed, result.Status = true, "pass"
		result.Detail = fmt.Sprintf("egress %s, expected %s — адрес соответствует профилю; локальный маршрут этим не измерен", addresses[0], expected)
	case EgressMismatch:
		result.Status = "fail"
		result.Detail = fmt.Sprintf("egress %s, expected %s — адрес не соответствует профилю; причина расхождения не установлена", addresses[0], expected)
	default:
		result.Status, result.Detail = "unavailable", "Недостаточно данных для сравнения выходного адреса с профилем"
	}
	return result
}

func consistentEgressResult(addresses []string) ProbeResult {
	outcome := EgressUnknown
	if len(addresses) >= 2 {
		outcome = EgressMatch
		for _, address := range addresses {
			compared := compareReportedEgress(addresses[0], address)
			if compared == EgressUnknown {
				outcome = EgressUnknown
				break
			}
			if compared == EgressMismatch {
				outcome = EgressMismatch
			}
		}
	}
	result := ProbeResult{Name: "Egress consistency", Comparison: egressComparison(comparisonConsistency, outcome)}
	switch outcome {
	case EgressMatch:
		result.Passed, result.Status = true, "pass"
		result.Detail = "HTTPS-цели сообщили одинаковый выходной IP; это не доказательство локального маршрута"
	case EgressMismatch:
		result.Status, result.Detail = "fail", "HTTPS-цели сообщили разные выходные IP; топология и причина расхождения не установлены"
	default:
		result.Status, result.Detail = "unavailable", "Нет двух полных ответов с адресами для сравнения egress"
	}
	return result
}

// A same-image OS control can use an engine AppID exception or another VPN.
// Equal/different public addresses cannot distinguish these local topologies.
// In TUN mode this control cannot replace the original distinct native actor.
func directLeakResult(tunMode bool, result ProbeResult, controlIP string, protectedIPs []string) ProbeResult {
	result.Name = "Direct leak check" // stable machine ID; presentation names the actual control
	if tunMode {
		result.Requirement = ProbeInformational
	}
	outcome := EgressUnknown
	if result.Passed && result.Observation.Schema == 1 && result.Observation.Route == ProbeRouteOS && result.Observation.BodyComplete && result.Observation.HTTPStatus == 200 && result.Observation.ErrorKind == "" {
		outcome = compareReportedEgress(controlIP, firstOr(protectedIPs, ""))
	}
	result.Comparison = egressComparison(comparisonControl, outcome)
	if outcome == EgressUnknown {
		result.Passed, result.Status = false, "unavailable"
		result.Detail = "Контроль ОС не дал сопоставимых HTTPS/IP-данных; локальный маршрут и безопасность не измерены"
		if result.Observation.Schema == 1 {
			result.Detail = "Контроль ОС: " + result.Observation.Summary() + "; IP-сопоставление недоступно, локальная защита не измерена"
		}
		return result
	}
	comparison := "совпадает с"
	if outcome == EgressMismatch {
		comparison = "отличается от"
	}
	result.Detail = "Контрольный IP ОС " + comparison + " IP защищённого запроса; локальный путь и наличие утечек этим не установлены"
	if tunMode {
		result.Passed, result.Status = false, "unavailable"
		result.Detail += "; TUN требует отдельного original-helper/WFP gate"
	} else {
		result.Passed, result.Status = true, "pass" // only the stated address comparison succeeded
	}
	return result
}
