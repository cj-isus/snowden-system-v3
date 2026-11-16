package metadata

import (
	"errors"
	"time"
)

var (
	ErrProfileExpired   = errors.New("signed profile validity expired")
	ErrIdentityExpired  = errors.New("selected certificate validity expired")
	ErrTrustNotYetValid = errors.New("trust validity has not started")
)

// SessionValidity is an immutable policy snapshot. Zero expiry means unknown/
// unspecified, not infinity proved by measurements. Native TLS validation stays
// enabled. Known signed leaf deadlines are conservative when peer selection is
// unavailable: the earliest currently usable certificate expires the session.
type SessionValidity struct {
	ClockFloor      time.Time
	NotBefore       time.Time
	ProfileExpires  time.Time
	IdentityExpires time.Time
}

// Constrain retains deadlines of streams that may survive a selector change.
// Only a fully retired/replaced worker can drop the prior engine's constraints.
func (v SessionValidity) Constrain(previous SessionValidity) SessionValidity {
	v.ClockFloor = laterTime(v.ClockFloor, previous.ClockFloor)
	v.NotBefore = laterTime(v.NotBefore, previous.NotBefore)
	v.ProfileExpires = earlierDeadline(v.ProfileExpires, previous.ProfileExpires)
	v.IdentityExpires = earlierDeadline(v.IdentityExpires, previous.IdentityExpires)
	return v
}

func earlierDeadline(a, b time.Time) time.Time {
	if a.IsZero() || !b.IsZero() && b.Before(a) {
		return b
	}
	return a
}

func (v SessionValidity) Remaining(now time.Time) (time.Duration, error) {
	if err := CheckClockFloor(now, v.ClockFloor); err != nil {
		return 0, err
	}
	if !v.NotBefore.IsZero() && now.Before(v.NotBefore) {
		return 0, ErrTrustNotYetValid
	}
	if !v.ProfileExpires.IsZero() && !now.Before(v.ProfileExpires) {
		return 0, ErrProfileExpired
	}
	if !v.IdentityExpires.IsZero() && !now.Before(v.IdentityExpires) {
		return 0, ErrIdentityExpired
	}
	remaining := time.Second // clock discontinuity observation ceiling, not an OS SLO
	for _, deadline := range []time.Time{v.ProfileExpires, v.IdentityExpires} {
		if !deadline.IsZero() && deadline.Sub(now) < remaining {
			remaining = deadline.Sub(now)
		}
	}
	return remaining, nil
}
