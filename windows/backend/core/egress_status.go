package core

import (
	"context"
	"errors"
	"time"
)

// Optional observation outcomes. None of these grants admission or diagnoses
// network censorship. A local budget/busy/skipped outcome is not a failed probe.
type EgressOutcome string

const (
	EgressSkipped            EgressOutcome = "skipped"
	EgressBusy               EgressOutcome = "busy"
	EgressBudget             EgressOutcome = "budget"
	EgressCanceled           EgressOutcome = "canceled"
	EgressTimeout            EgressOutcome = "timeout"
	EgressUnavailable        EgressOutcome = "unavailable"
	EgressInvalidResponse    EgressOutcome = "invalid_response"
	EgressHTTPStatus         EgressOutcome = "http_status"
	EgressCleanupUnconfirmed EgressOutcome = "cleanup_unconfirmed"
	EgressResponse           EgressOutcome = "response" // internal: bounded HTTP bytes, not a validated endpoint IP
	EgressOK                 EgressOutcome = "ok"       // assigned by the endpoint payload parser only
)

// Attempted means the synchronous target attempt was entered, not that a
// packet reached the server. Times describe that frame, not resource retirement.
// Duration/age use local monotonic time; wall timestamps are display metadata.
type EgressRouteObservation struct {
	Route         string        `json:"route"`
	Outcome       EgressOutcome `json:"outcome"`
	Reason        string        `json:"reason,omitempty"`
	Attempted     bool          `json:"attempted"`
	Stage         string        `json:"stage"`
	StartedAt     string        `json:"startedAt"`
	CompletedAt   string        `json:"completedAt"`
	ElapsedMicros *int64        `json:"elapsedMicros"`
	AgeMicros     *int64        `json:"ageMicros"`
	HTTPStatus    int           `json:"httpStatus,omitempty"`
	completed     time.Time
}

// Return a value copy. Reading a cached response never refreshes its original
// measurement or mutates shared pointer fields in that cache.
func (fact EgressRouteObservation) AtReply(now time.Time) EgressRouteObservation {
	fact.AgeMicros = nil
	if !fact.completed.IsZero() && !now.Before(fact.completed) {
		age := now.Sub(fact.completed).Microseconds()
		fact.AgeMicros = &age
	}
	return fact
}

func newEgressObservation(osControl, socks bool) EgressHTTPSObservation {
	makeFact := func(route string, requested bool) EgressRouteObservation {
		fact := EgressRouteObservation{Route: route, Outcome: EgressSkipped, Reason: "not_requested"}
		if requested {
			fact.Outcome, fact.Reason = EgressUnavailable, ""
		}
		return fact
	}
	return EgressHTTPSObservation{OSObservation: makeFact(ProbeRouteOS, osControl), SOCKSObservation: makeFact(ProbeRouteSOCKS, socks)}
}

func rejectEgressObservation(facts *EgressHTTPSObservation, osControl, socks bool, outcome EgressOutcome) {
	facts.OSControl, facts.SOCKS = nil, nil
	if osControl {
		facts.OSObservation.Outcome, facts.OSObservation.Reason = outcome, ""
		if !facts.OSObservation.Attempted && outcome == EgressInvalidResponse {
			facts.OSObservation.Outcome = EgressUnavailable
		}
	}
	if socks {
		facts.SOCKSObservation.Outcome, facts.SOCKSObservation.Reason = outcome, ""
		if !facts.SOCKSObservation.Attempted && outcome == EgressInvalidResponse {
			facts.SOCKSObservation.Outcome = EgressUnavailable
		}
	}
}

func egressContextOutcome(ctx context.Context) EgressOutcome {
	err := contextError(ctx)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return EgressTimeout
	case errors.Is(err, context.Canceled):
		return EgressCanceled
	default:
		return EgressUnavailable
	}
}
