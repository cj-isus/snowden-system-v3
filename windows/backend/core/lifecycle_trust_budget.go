package core

import "time"

// Immutable local observation budget, not a new trust decision or OS lease.
// LifetimeRemaining may cap the duration for clock reobservation; exhausting
// it means trust_unavailable, not proof of a particular certificate's expiry.
type lifecycleTrustBudget struct {
	until   time.Time
	bounded bool
}

// Call only outside Manager/source value locks. Charge callback time from the
// beginning of the sample, retaining time.Now's monotonic component. A new
// sample may replace this budget; a later check of the same budget cannot.
func observeLifecycleTrust(probe Probe) (*lifecycleTrustBudget, error) {
	started := time.Now()
	remaining, err := lifetimeRemaining(probe)
	if err != nil {
		return nil, err
	}
	_, bounded := probe.(LifetimeProbe)
	budget := &lifecycleTrustBudget{bounded: bounded}
	if bounded {
		budget.until = started.Add(remaining)
	}
	return budget, budget.current()
}

// Value-only negative check, including after source release and watch reserve.
// The legacy non-Lifetime default duration is deliberately not a measured TTL.
func (budget *lifecycleTrustBudget) current() error {
	if budget == nil || budget.bounded && !time.Now().Before(budget.until) {
		return &TrustFaultError{Reason: "trust_unavailable"}
	}
	return nil
}
