package metadata

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
)

// ProfileSnapshot owns one observed public source. It is not installed-engine
// state, a delivery proof or authority to commit these bytes later. Private
// values do not escape; projection uses them without reopening the filesystem.
type ProfileSnapshot struct {
	policy      *TrustPolicy
	accepted    *AcceptedProfileState
	envelope    *Envelope
	restriction error
	source      profileSnapshotSource
}

type profileSnapshotSource struct {
	policy, accepted, envelope           [32]byte
	policyPresent, envelopeRead, present bool
}

func (p *ProfileSnapshot) TrustPolicy() *TrustPolicy {
	if p == nil {
		return nil
	}
	return p.policy.Clone()
}

func (p *ProfileSnapshot) AcceptedProfile() *AcceptedProfileState {
	if p == nil {
		return nil
	}
	return p.accepted.Clone()
}

// SameSource compares decoded public values plus policy/envelope presence.
// It includes the envelope signature but not raw whitespace/encoding. A match
// is not an authorization to mutate or resume a session.
func (p *ProfileSnapshot) SameSource(other *ProfileSnapshot) bool {
	return p != nil && other != nil && p.policy != nil && other.policy != nil && p.source == other.source
}

// InspectProfileSnapshot is read-only even on the first invocation: Windows
// requires an existing coordination lock created by normal startup/writes.
// Failure/uncertain scope retirement never returns a partial captured value.
func (s *Store) InspectProfileSnapshot() (*ProfileSnapshot, error) {
	var snapshot *ProfileSnapshot
	err := filetxn.WithRead(s.ImportJournal(), func() error {
		var err error
		snapshot, err = s.readProfileSnapshot()
		return err
	})
	if err != nil {
		return nil, errors.Join(ErrTrustRole, err)
	}
	return snapshot, nil
}

// Caller already owns a read or writer scope. Capture does no bootstrap/save.
func (s *Store) readProfileSnapshot() (*ProfileSnapshot, error) {
	policy, policyPresent, accepted, err := s.loadTrustSource()
	if err != nil {
		return nil, err
	}
	snapshot := &ProfileSnapshot{policy: policy, accepted: accepted}
	snapshot.source.policyPresent = policyPresent
	snapshot.restriction = CheckAppliedPinPolicy(policy.Delivery, accepted.PinPolicy)
	if snapshot.restriction == nil {
		snapshot.envelope, snapshot.source.present, err = s.LoadEnvelope()
		if err != nil {
			return nil, err
		}
		snapshot.source.envelopeRead = true
	} // Pending policy facts remain visible; no envelope is admitted/projected.
	// The persistence encoder promotes legacy policy schemas. Source comparison
	// must retain the observed schema and every field instead of normalizing it.
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	acceptedBytes, err := json.Marshal(accepted) // also retains an unanchored numeric floor
	if err != nil {
		return nil, err
	}
	envelopeBytes, err := json.Marshal(snapshot.envelope) // includes signature, unlike content SHA
	if err != nil {
		return nil, err
	}
	snapshot.source.policy, snapshot.source.accepted, snapshot.source.envelope = sha256.Sum256(policyBytes), sha256.Sum256(acceptedBytes), sha256.Sum256(envelopeBytes)
	return snapshot, nil
}
