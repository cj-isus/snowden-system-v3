package main

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
)

// Public source bindings for an offline issuer. No vault, private key, owner
// certificate, runtime configuration or old damaged envelope is read here.
type ProfileRecoveryContext struct {
	FromVersion        uint64                 `json:"from_version"`
	FromProfileEpoch   uint64                 `json:"from_profile_epoch"`
	ProfileIdentity    string                 `json:"profile_identity"`
	EpochOrigin        *metadata.EpochOrigin  `json:"epoch_origin,omitempty"`
	EpochTransitions   int                    `json:"retained_epoch_transitions"`
	FromEnvelopeSHA256 string                 `json:"from_envelope_sha256"`
	FromTrustSHA256    string                 `json:"from_trust_sha256"`
	ClockFloor         string                 `json:"clock_floor"`
	RecoveryKeys       []OnboardingKeyPreview `json:"recovery_keys"`
}

func (a *App) GetProfileRecoveryContext() (*ProfileRecoveryContext, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	policy, _, err := store.LoadTrustPolicy()
	if err != nil {
		return nil, err
	}
	state, err := metadata.LoadAcceptedProfile(dir)
	if err != nil {
		return nil, err
	}
	if !state.HasCheckpoint() || state.EnvelopeSHA256 == "" {
		return nil, errors.New("onboarding: для восстановления нужен неповреждённый принятый checkpoint")
	}
	digest, err := policy.Digest()
	if err != nil {
		return nil, err
	}
	out := &ProfileRecoveryContext{FromVersion: state.Version, FromEnvelopeSHA256: state.EnvelopeSHA256, FromTrustSHA256: digest, RecoveryKeys: []OnboardingKeyPreview{}}
	out.FromProfileEpoch = state.ProfileEpoch
	out.ProfileIdentity = state.ProfileVersion().String()
	out.EpochOrigin, out.EpochTransitions = state.EpochOrigin.Clone(), len(state.EpochHistory)
	if floor := metadata.TrustClockFloor(policy, state); !floor.IsZero() {
		out.ClockFloor = floor.UTC().Format(time.RFC3339Nano)
	}
	for _, key := range policy.Keys {
		if key.Role == metadata.RoleRecovery && !key.Retired {
			out.RecoveryKeys = append(out.RecoveryKeys, OnboardingKeyPreview{KeyID: key.KeyID, Fingerprint: onboarding.KeyFingerprint(key.Public), Comment: key.Comment})
		}
	}
	return out, nil // an empty list honestly means recovery was never enrolled
}

// Preserve exact uint64 JSON literals when exporting context through JavaScript.
// UI must copy this string, not parse/stringify the numeric compatibility DTO.
func (a *App) GetProfileRecoveryContextJSON() (string, error) {
	context, err := a.GetProfileRecoveryContext()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
