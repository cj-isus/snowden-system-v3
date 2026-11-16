package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

const MaxRecoveryPermitSize = 16 << 10
const recoveryDomain = "snowden-system/profile-recovery/v1\x00"

// ProfileRecoveryPermit grants exactly one profile-key replacement from one
// accepted checkpoint. It cannot enroll root/update/recovery keys, reset time,
// lower versions, revive retired channels/pins, or authorize arbitrary targets.
// A separately enrolled offline recovery key is mandatory before a failure.
type ProfileRecoveryPermit struct {
	Schema             int       `json:"schema"`
	FromVersion        uint64    `json:"from_version"`
	ProfileEpoch       uint64    `json:"profile_epoch,omitempty"`
	FromEnvelopeSHA256 string    `json:"from_envelope_sha256"`
	FromTrustSHA256    string    `json:"from_trust_sha256"`
	TargetVersion      uint64    `json:"target_version"`
	TargetSHA256       string    `json:"target_sha256"`
	ProfilePublic      string    `json:"profile_public_hex"`
	IssuedAt           time.Time `json:"issued_at"`
	ExpiresAt          time.Time `json:"expires_at"`
	KeyID              string    `json:"key_id"`
	Signature          string    `json:"signature,omitempty"`
}

func (p *ProfileRecoveryPermit) Validate(now time.Time) error {
	if p == nil || p.Schema != 1 && p.Schema != 2 || p.Schema == 1 && p.ProfileEpoch != 0 || p.Schema == 2 && p.ProfileEpoch == 0 || p.FromVersion == 0 || p.TargetVersion <= p.FromVersion || !validDigest(p.FromEnvelopeSHA256) || !validDigest(p.FromTrustSHA256) || !validDigest(p.TargetSHA256) {
		return errors.New("metadata: invalid recovery checkpoint/target binding")
	}
	pub, err := hex.DecodeString(p.ProfilePublic)
	if err != nil || len(pub) != ed25519.PublicKeySize || p.ProfilePublic != hex.EncodeToString(pub) {
		return errors.New("metadata: invalid recovery profile public key")
	}
	if p.IssuedAt.IsZero() || p.ExpiresAt.IsZero() || !p.ExpiresAt.After(p.IssuedAt) {
		return errors.New("metadata: invalid recovery validity interval")
	}
	if p.IssuedAt.After(now.Add(MaxClockSkew)) {
		return ErrTrustNotYetValid
	}
	if !now.Before(p.ExpiresAt) {
		return ErrProfileExpired
	}
	return nil
}

func (p *ProfileRecoveryPermit) canonical() ([]byte, error) {
	if p == nil {
		return nil, errors.New("metadata: missing recovery permit")
	}
	copy := *p
	copy.Signature = ""
	data, err := json.Marshal(copy)
	if err != nil || len(data) > MaxRecoveryPermitSize {
		return nil, errors.New("metadata: recovery encoding exceeds bound")
	}
	return append([]byte(recoveryDomain), data...), nil
}

func (p *ProfileRecoveryPermit) Digest() (string, error) {
	data, err := p.canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (p *ProfileRecoveryPermit) Sign(priv ed25519.PrivateKey) error {
	if p == nil || len(priv) != ed25519.PrivateKeySize {
		return errors.New("metadata: invalid recovery signing key")
	}
	p.KeyID = KeyIDFor(priv.Public().(ed25519.PublicKey))
	data, err := p.canonical()
	if err != nil {
		return err
	}
	p.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, data))
	return nil
}

func (p *ProfileRecoveryPermit) Verify(policy *TrustPolicy) error {
	if p == nil || policy.validate() != nil {
		return ErrTrustRole
	}
	pub := policy.VerificationKeys(RoleRecovery)[p.KeyID]
	if len(pub) != ed25519.PublicKeySize {
		return ErrTrustRole
	}
	sig, err := base64.RawStdEncoding.DecodeString(p.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("metadata: invalid recovery signature encoding")
	}
	data, err := p.canonical()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, data, sig) {
		return errors.New("metadata: recovery signature mismatch")
	}
	return nil
}

func DecodeRecoveryPermit(data []byte) (*ProfileRecoveryPermit, error) {
	if len(data) == 0 || len(data) > MaxRecoveryPermitSize {
		return nil, errors.New("metadata: recovery permit size outside bound")
	}
	if err := checkCheckpointJSON(data); err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return nil, errors.New("metadata: recovery object required")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p ProfileRecoveryPermit
	if err := dec.Decode(&p); err != nil {
		return nil, errors.New("metadata: malformed recovery permit")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("metadata: trailing recovery data")
	}
	return &p, nil
}

// PrepareProfileRecovery intentionally need not read the old envelope: a valid
// retained checkpoint + pre-enrolled recovery authority may restore a missing/
// damaged profile file. A missing/corrupt checkpoint is never treated as zero.
// The caller serializes and commits keys/envelope/checkpoint/vault together.
func (s *Store) PrepareProfileRecovery(next *Envelope, permit *ProfileRecoveryPermit, policy *TrustPolicy, now time.Time) (*TrustPolicy, *AcceptedProfileState, error) {
	if policy != nil && policy.Delivery != nil {
		return nil, nil, errors.New("metadata: enrolled recovery requires exact delivery proof")
	}
	return s.prepareProfileRecovery(next, permit, policy, now)
}

func (s *Store) PrepareDeliveredRecovery(next *Envelope, permit *ProfileRecoveryPermit, policy *TrustPolicy, proof *delivery.Prepared, path string, raw []byte, now time.Time) (*TrustPolicy, *AcceptedProfileState, error) {
	if err := checkDeliveredEnvelope(policy, next, proof, path, raw, now); err != nil {
		return nil, nil, err
	}
	state, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return nil, nil, err
	}
	if err := checkProfileTarget(state, next, path); err != nil {
		return nil, nil, err
	}
	return s.prepareProfileRecovery(next, permit, policy, now)
}

func (s *Store) prepareProfileRecovery(next *Envelope, permit *ProfileRecoveryPermit, policy *TrustPolicy, now time.Time) (*TrustPolicy, *AcceptedProfileState, error) {
	state, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return nil, nil, err
	}
	if err := state.validate(); err != nil {
		return nil, nil, errors.New("metadata: recovery requires an intact accepted checkpoint")
	}
	if err := CheckDeliveryBinding(policy, state); err != nil {
		return nil, nil, err
	}
	if err := permit.Verify(policy); err != nil {
		return nil, nil, err
	}
	if err := CheckClockFloor(now, TrustClockFloor(policy, state)); err != nil {
		return nil, nil, err
	}
	now = laterTime(now, TrustClockFloor(policy, state))
	if err := permit.Validate(now); err != nil {
		return nil, nil, err
	}
	if next == nil || next.ProfileEpoch != state.ProfileEpoch || permit.ProfileEpoch != state.ProfileEpoch || next.MetadataVersion != permit.TargetVersion {
		return nil, nil, errors.New("metadata: recovery profile version mismatch")
	}
	pub, _ := hex.DecodeString(permit.ProfilePublic) // Validate checked bytes
	id := KeyIDFor(ed25519.PublicKey(pub))
	if next.KeyID != id {
		return nil, nil, errors.New("metadata: recovery profile signer mismatch")
	}
	if err := next.Verify(map[string]ed25519.PublicKey{id: ed25519.PublicKey(pub)}); err != nil {
		return nil, nil, err
	}
	if err := next.ValidateVersion(now, state.ProfileVersion()); err != nil {
		return nil, nil, err
	}
	digest, err := profileDigest(next)
	if err != nil || digest != permit.TargetSHA256 {
		return nil, nil, errors.New("metadata: recovery target content mismatch")
	}
	permitDigest, err := permit.Digest()
	if err != nil {
		return nil, nil, err
	}
	if state.Version == permit.TargetVersion && state.EnvelopeSHA256 == digest && state.LastRecoverySHA256 == permitDigest {
		if !bytes.Equal(policy.VerificationKeys(RoleProfile)[id], pub) {
			return nil, nil, ErrTrustRole
		}
		return policy.Scoped(), state, nil // exact committed replay, no new authority
	}
	trustDigest, err := policy.Digest()
	if err != nil || trustDigest != permit.FromTrustSHA256 || state.Version != permit.FromVersion || state.EnvelopeSHA256 != permit.FromEnvelopeSHA256 {
		return nil, nil, errors.New("metadata: recovery source was replaced; new permit required")
	}
	updated := policy.Scoped()
	for i := range updated.Keys {
		if updated.Keys[i].Role == RoleProfile {
			updated.Keys[i].Retired = true // keep public history, remove live authority
		}
	}
	updated, err = updated.WithKey(ed25519.PublicKey(pub), RoleProfile)
	if err != nil {
		return nil, nil, err // retired/other-role key cannot be reused or promoted
	}
	accepted, err := advanceProfile(state, next)
	if err != nil {
		return nil, nil, err
	}
	accepted.LastRecoverySHA256 = permitDigest
	return updated, accepted, nil
}
