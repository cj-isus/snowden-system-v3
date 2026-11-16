package metadata

import (
	"errors"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

var ErrClockUncertain = errors.New("local clock continuity is uncertain")

const ClockCheckpointInterval = time.Minute // write ceiling, not authenticated-time precision

// SampleClock combines local wall time and process steady elapsed time. Neither
// is authenticated UTC or evidence about offline time after a reboot.
type ClockSample struct {
	Wall   time.Time
	Steady time.Duration
}

var clockProcessStart = time.Now()

func SampleClock() ClockSample {
	return ClockSample{Wall: time.Now().UTC(), Steady: time.Since(clockProcessStart)}
}

type ObservedClock struct {
	mu          sync.Mutex
	initialized bool
	steady      time.Duration
	logical     time.Time
}

// Observe is value-only. A large wall/steady discontinuity is rejected, never
// silently used to extend validity or poison a persistent floor. Restoring wall
// time to the prior projection may recover; a process restart cannot lower floor.
func (c *ObservedClock) Observe(sample ClockSample, floor time.Time) (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.observeLocked(sample, floor)
}

// Now samples while holding the observer lock so concurrent callers cannot
// reorder samples and manufacture a backwards steady-clock observation.
func (c *ObservedClock) Now(floor time.Time) (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.observeLocked(SampleClock(), floor)
}

func (c *ObservedClock) observeLocked(sample ClockSample, floor time.Time) (time.Time, error) {
	if sample.Wall.IsZero() || sample.Steady < 0 {
		return time.Time{}, ErrClockUncertain
	}
	wall := sample.Wall.UTC()
	if err := CheckClockFloor(wall, floor); err != nil {
		return time.Time{}, err
	}
	if !c.initialized {
		c.logical, c.steady, c.initialized = laterTime(wall, floor), sample.Steady, true
		return c.logical, nil
	}
	if sample.Steady < c.steady {
		return time.Time{}, ErrClockUncertain
	}
	projected := c.logical.Add(sample.Steady - c.steady)
	if wall.Before(projected.Add(-MaxClockSkew)) || wall.After(projected.Add(MaxClockSkew)) {
		return time.Time{}, ErrClockUncertain
	}
	c.logical = laterTime(laterTime(wall, projected), floor)
	c.steady = sample.Steady
	return c.logical, nil
}

func CheckClockFloor(now, floor time.Time) error {
	if now.IsZero() || !floor.IsZero() && now.Before(floor.Add(-MaxClockSkew)) {
		return ErrClockUncertain
	}
	return nil
}

func laterTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// AcceptanceClockFloor retains observations, not a trusted-time attestation.
func AcceptanceClockFloor(state *AcceptedProfileState) time.Time {
	if state == nil {
		return time.Time{}
	}
	floor := laterTime(state.TimeFloor, state.IssuedAtFloor)
	if state.PinPolicy != nil {
		floor = laterTime(floor, state.PinPolicy.IssuedAt)
	}
	return floor
}

// TrustClockFloor includes a retained delivery observation without applying
// repository expiry to the already accepted data plane. It is still local time.
func TrustClockFloor(policy *TrustPolicy, state *AcceptedProfileState) time.Time {
	floor := AcceptanceClockFloor(state)
	if policy != nil && policy.Delivery != nil {
		floor = laterTime(floor, policy.Delivery.ClockFloor)
	}
	return floor
}

// WithClockObservation returns an owned state value. UI descriptor reads do not
// call it: an exact replay does not create a write on every poll.
func WithClockObservation(state *AcceptedProfileState, observed time.Time) (*AcceptedProfileState, error) {
	if state == nil || observed.IsZero() || CheckClockFloor(observed, AcceptanceClockFloor(state)) != nil {
		return nil, ErrClockUncertain
	}
	out := state.Clone()
	if state.TimeFloor.IsZero() || observed.Sub(state.TimeFloor) >= ClockCheckpointInterval {
		out.TimeFloor = laterTime(state.TimeFloor, observed.UTC())
	}
	return out, nil
}

// RecordClockObservation updates only the current accepted public checkpoint.
// It cannot lower version, history or time, or write for a replaced profile.
// Call outside App/runtime/epoch/Manager locks; the four import targets stay fixed.
func (s *Store) RecordClockObservation(version uint64, digest string, observed time.Time) error {
	return s.RecordProfileClockObservation(ProfileVersion{Counter: version}, digest, observed)
}

func (s *Store) RecordProfileClockObservation(version ProfileVersion, digest string, observed time.Time) error {
	return s.recordRuntimeClockObservation(version, digest, "", false, observed)
}

// RecordRuntimeClockObservation also fences the effective global policy of the
// running config. A changed/pending policy cannot refresh an old session's proof.
func (s *Store) RecordRuntimeClockObservation(version ProfileVersion, digest, pinPolicySHA string, observed time.Time) error {
	return s.recordRuntimeClockObservation(version, digest, pinPolicySHA, true, observed)
}

func (s *Store) recordRuntimeClockObservation(version ProfileVersion, digest, pinPolicySHA string, checkPin bool, observed time.Time) error {
	unlock := filetxn.Serialize(s.ImportJournal())
	defer unlock()
	return filetxn.WithWriter(s.ImportJournal(), func() error {
		return s.recordRuntimeClockObservationLocked(version, digest, pinPolicySHA, checkPin, observed)
	})
}

func (s *Store) recordRuntimeClockObservationLocked(version ProfileVersion, digest, pinPolicySHA string, checkPin bool, observed time.Time) error {
	state, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return err
	}
	if state.ProfileVersion() != version || state.EnvelopeSHA256 != digest || version.Counter == 0 || !validDigest(digest) {
		return errors.New("metadata: clock observation profile superseded")
	}
	policy, _, err := s.LoadTrustPolicy()
	if err != nil {
		return err
	}
	if err = CheckAppliedPinPolicy(policy.Delivery, state.PinPolicy); err != nil {
		return err
	}
	if checkPin {
		actual := ""
		if state.PinPolicy != nil {
			actual = state.PinPolicy.SHA256
		}
		if actual != pinPolicySHA {
			return ErrPinPolicyPending
		}
	}
	next, err := WithClockObservation(state, observed)
	if err != nil {
		return err
	}
	return s.saveAcceptedProfile(next, observed)
}
