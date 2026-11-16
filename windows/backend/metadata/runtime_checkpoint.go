package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

// SaveAcceptedProfile preserves an installed checkpoint's entire trust identity
// and may only advance its observed TimeFloor. A legacy/missing checkpoint is
// anchored only to the freshly authenticated installed envelope. This is not an
// import API: imports commit EncodeAcceptedProfile in the four-file journal.
// Caller keeps local serialization; this entry acquires the native writer scope.
func (s *Store) SaveAcceptedProfile(state *AcceptedProfileState) error {
	return filetxn.WithWriter(s.ImportJournal(), func() error {
		return s.saveAcceptedProfile(state, time.Now().UTC())
	})
}

// Caller already holds the journal writer scope across read/derive/save.
// observed is the caller's local clock observation, not authenticated UTC.
func (s *Store) saveAcceptedProfile(state *AcceptedProfileState, observed time.Time) error {
	if err := filetxn.Check(s.ImportJournal()); err != nil {
		return err
	}
	if state == nil {
		return ErrTrustRole
	}
	if err := state.validate(); err != nil {
		return errors.Join(ErrTrustRole, err)
	}
	previous, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return err
	}
	expected := previous
	if !previous.HasCheckpoint() {
		// Do not use a caller-provided hash/pin set as a bootstrap capability.
		// PrepareAcceptance rechecks the retained numeric floor, installed
		// historical signature, active signer, validity and pin continuity.
		expected, err = s.runtimeBootstrapCheckpoint(observed)
		if err != nil {
			return err
		}
	}
	encoded, unchanged, err := runtimeCheckpointUpdate(expected, state, observed)
	if err != nil {
		return err
	}
	if previous.HasCheckpoint() && unchanged {
		return nil
	}
	return storedCheckpointFile.write(filepath.Join(s.Dir, "version.txt"), encoded)
}

// The read-only runtime projection uses this same whole-state guard to avoid
// presenting recomputed history as if it were already retained on disk.
func runtimeCheckpointUpdate(expected, state *AcceptedProfileState, observed time.Time) ([]byte, bool, error) {
	candidate, encoded, err := canonicalRuntimeCheckpoint(state)
	if err != nil {
		return nil, false, err
	}
	expected, old, err := canonicalRuntimeCheckpoint(expected)
	if err != nil {
		return nil, false, err
	}
	if candidate.TimeFloor.Before(expected.TimeFloor) ||
		candidate.TimeFloor.After(expected.TimeFloor) && (observed.IsZero() || candidate.TimeFloor.After(observed)) {
		return nil, false, errors.Join(ErrTrustRole, ErrClockUncertain)
	}
	// Seal every encoded field except the one permitted clock observation.
	// Version/content, issued-at/recovery, delivery/epoch/origin, per-channel
	// revocations/retired channels and global policy cannot be assigned here.
	// New serialized trust fields are immutable by default too.
	expected.TimeFloor = candidate.TimeFloor
	allowed, err := EncodeAcceptedProfile(expected)
	if err != nil {
		return nil, false, err
	}
	if !bytes.Equal(allowed, encoded) {
		return nil, false, fmt.Errorf("%w: runtime checkpoint writer cannot change accepted identity or history", ErrTrustRole)
	}
	return encoded, bytes.Equal(old, encoded), nil
}

func (s *Store) runtimeBootstrapCheckpoint(observed time.Time) (*AcceptedProfileState, error) {
	policy, _, err := s.LoadTrustPolicy()
	if err != nil {
		return nil, err
	}
	if policy.Delivery != nil {
		return nil, fmt.Errorf("%w: runtime bootstrap cannot enroll delivery authority", ErrTrustRole)
	}
	envelope, present, err := s.LoadEnvelope()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("%w: runtime bootstrap requires the installed signed envelope", ErrTrustRole)
	}
	return s.PrepareAcceptance(envelope, policy.VerificationKeys(RoleProfile), observed)
}

// Normalize only unordered local retention sets in an owned copy. Do not sort
// epoch history or change signed content/global-policy/authority fields.
func canonicalRuntimeCheckpoint(state *AcceptedProfileState) (*AcceptedProfileState, []byte, error) {
	if state == nil {
		return nil, nil, fmt.Errorf("%w: missing runtime checkpoint", ErrTrustRole)
	}
	if err := state.validate(); err != nil {
		return nil, nil, errors.Join(ErrTrustRole, err)
	}
	owned := state.Clone()
	for i := range owned.Pins {
		sort.Strings(owned.Pins[i].Revoked)
	}
	sort.Slice(owned.Pins, func(i, j int) bool { return owned.Pins[i].ChannelID < owned.Pins[j].ChannelID })
	sort.Strings(owned.Retired)
	raw, err := EncodeAcceptedProfile(owned)
	return owned, raw, err
}
