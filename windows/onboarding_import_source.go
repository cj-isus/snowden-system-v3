package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/metadata"
)

// Expected public source for one import. Delivery-prefix persistence may replace
// policy before acceptance; accepted history is never adopted from a late read.
// Ordinary imports also bind the original installed-envelope bytes/absence.
// Explicit recovery and urgent pin-policy denials deliberately need no old
// envelope. Their separate authenticated contracts still require an intact
// checkpoint. This object supplies no import authority.
type profileImportSource struct {
	store           *metadata.Store
	policy          *metadata.TrustPolicy
	state           *metadata.AcceptedProfileState
	policyPresent   bool
	policyBytes     []byte
	checkpoint      []byte
	envelope        []byte
	envelopePresent bool
	envelopeBound   bool
	frozen          bool
}

func captureProfileImportSource(store *metadata.Store) (*profileImportSource, error) {
	snapshot, err := store.InspectTrustSnapshot()
	if err != nil {
		return nil, err
	}
	source := &profileImportSource{store: store, policy: snapshot.TrustPolicy(), state: snapshot.AcceptedProfile(), policyPresent: snapshot.PolicyPresent()}
	source.checkpoint, err = json.Marshal(source.state)
	if err != nil {
		return nil, err
	}
	if err := source.expectPolicy(source.policy); err != nil {
		return nil, err
	}
	return source, nil
}

// Only the caller's successful authenticated prefix result can advance this
// expected policy. Once envelope/history acceptance starts it is immutable.
func (s *profileImportSource) expectPolicy(policy *metadata.TrustPolicy) error {
	if s == nil || s.frozen || policy == nil {
		return metadata.ErrTrustRole
	}
	encoded, err := metadata.EncodeTrustPolicy(policy)
	if err != nil {
		return err
	}
	s.policy, s.policyBytes = policy.Clone(), encoded
	return nil
}

func (s *profileImportSource) match(policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState, present bool) error {
	encoded, err := metadata.EncodeTrustPolicy(policy)
	if err != nil {
		return err
	}
	checkpoint, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if present != s.policyPresent || policy.Schema != s.policy.Schema || !bytes.Equal(encoded, s.policyBytes) || !bytes.Equal(checkpoint, s.checkpoint) {
		return errors.New("onboarding: source authority/checkpoint changed after preparation; retry import")
	}
	return nil
}

// Caller holds a native scope. No decode error is swallowed as recovery.
func (s *profileImportSource) bindEnvelopeInScope() error {
	if s.envelopeBound {
		return metadata.ErrTrustRole
	}
	raw, present, err := s.store.ReadEnvelopeBytes()
	if err != nil {
		return err
	}
	s.envelope, s.envelopePresent, s.envelopeBound = raw, present, true
	return nil
}

func (s *profileImportSource) checkEnvelopeInScope() error {
	if !s.envelopeBound {
		return nil
	}
	raw, present, err := s.store.ReadEnvelopeBytes()
	if err != nil {
		return err
	}
	if present != s.envelopePresent || !bytes.Equal(raw, s.envelope) {
		return errors.New("onboarding: installed envelope changed after preparation; retry import")
	}
	return nil
}

// Caller already holds read/writer ownership, including Store.BeforeCommit.
func (s *profileImportSource) checkInScope() error {
	policy, present, err := s.store.LoadTrustPolicy()
	if err != nil {
		return err
	}
	state, err := metadata.LoadAcceptedProfile(s.store.Dir)
	if err != nil {
		return err
	}
	if err := s.match(policy, state, present); err != nil {
		return err
	}
	return s.checkEnvelopeInScope()
}

// Caller owns local Serialize. Read-only preparation cannot enter SDK/vault or
// mutate coordinated profile files while this native read scope is held.
func (s *profileImportSource) withCurrentSource(prepare func() error) error {
	if s == nil || s.frozen || prepare == nil {
		return metadata.ErrTrustRole
	}
	return s.store.WithTrustSnapshot(func(snapshot *metadata.TrustSnapshot) error {
		if err := s.match(snapshot.TrustPolicy(), snapshot.AcceptedProfile(), snapshot.PolicyPresent()); err != nil {
			return err
		}
		if err := s.checkEnvelopeInScope(); err != nil {
			return err
		}
		s.frozen = true // recovery also freezes policy without binding an old file
		return prepare()
	})
}
