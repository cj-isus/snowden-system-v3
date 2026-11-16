package core

const (
	ProbeRequired      = "required"
	ProbeInformational = "informational"
)

// Requirement describes how a diagnostic is aggregated, never forwarding
// authority. Legacy/missing values remain required. Only the same-image OS
// comparison may be informational; it retains its unavailable path verdict.
func (result ProbeResult) Informational() bool {
	return result.Requirement == ProbeInformational && result.Name == "Direct leak check" &&
		result.Comparison.Valid() && result.Comparison.Reference == comparisonControl &&
		!result.Passed && result.Status == "unavailable"
}

// A role cannot downgrade another check, a malformed comparison or a positive
// safety claim. Normalize before aggregation at every report boundary.
func (result ProbeResult) NormalizeDiagnostic() ProbeResult {
	information := result.Informational()
	result = result.NormalizeComparison()
	if result.Requirement == "" || result.Requirement == ProbeRequired || information && result.Informational() {
		return result
	}
	result.Requirement = ProbeRequired
	result.Passed = false
	if result.Status == "leak" {
		// NormalizeComparison has already rejected IP-only leak labels. A bad
		// role cannot erase an independent safety fact or make it optional.
		return result
	}
	// Do not leave a valid mismatch behind a role failure: the next boundary
	// normalizes again and must not turn unavailable into observed divergence.
	result.Comparison = ProbeComparison{}
	result.Status = "unavailable"
	result.Detail = "Роль шага диагностики не подтверждена; обязательная проверка не пройдена"
	return result
}
