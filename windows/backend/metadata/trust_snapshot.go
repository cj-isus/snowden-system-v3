package metadata

import (
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
)

// TrustSnapshot owns only the local authority and its bound checkpoint. Update
// and recovery can inspect this source without admitting an absent, corrupt or
// policy-restricted data-plane envelope. It grants no delivery/import authority.
type TrustSnapshot struct {
	policy   *TrustPolicy
	accepted *AcceptedProfileState
	present  bool
}

func (s *TrustSnapshot) TrustPolicy() *TrustPolicy {
	if s == nil {
		return nil
	}
	return s.policy.Clone()
}

func (s *TrustSnapshot) AcceptedProfile() *AcceptedProfileState {
	if s == nil {
		return nil
	}
	return s.accepted.Clone()
}

func (s *TrustSnapshot) PolicyPresent() bool { return s != nil && s.present }

// WithTrustSnapshot holds the existing-only shared native source scope until
// inspect returns. The callback may guard a mutation in a separate installation
// domain, but cannot mutate these profile files, enter SDK/vault/App callbacks,
// or acquire a nested profile read/writer scope. A retirement error is returned
// even if that separate mutation already happened; its owner must retain outcome.
func (s *Store) WithTrustSnapshot(inspect func(*TrustSnapshot) error) error {
	if s == nil || inspect == nil {
		return ErrTrustRole
	}
	return filetxn.WithRead(s.ImportJournal(), func() error {
		policy, present, accepted, err := s.loadTrustSource()
		if err != nil {
			return err
		}
		return inspect(&TrustSnapshot{policy: policy, accepted: accepted, present: present})
	})
}

// InspectTrustSnapshot releases the native scope before handing out owned data.
// It never creates a lock, enrolls authority or returns a partial captured value.
func (s *Store) InspectTrustSnapshot() (*TrustSnapshot, error) {
	var snapshot *TrustSnapshot
	err := s.WithTrustSnapshot(func(source *TrustSnapshot) error { snapshot = source; return nil })
	if err != nil {
		return nil, errors.Join(ErrTrustRole, err)
	}
	return snapshot, nil
}
