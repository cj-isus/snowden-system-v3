package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

const PinPolicyProtocol = 1
const pinPolicyPurpose = "snowden-system/pin-history/v1"

// PinPolicySnapshot has no application signing key. Only the reserved offline
// top-level TUF target proof can authorize this cumulative repository policy.
type PinPolicySnapshot struct {
	Schema              int       `json:"schema"`
	Purpose             string    `json:"purpose"`
	Version             uint64    `json:"version"`
	ProfileNamespace    string    `json:"profile_namespace"`
	MinimumProtocol     uint64    `json:"minimum_protocol"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	RevokedCertificates []string  `json:"revoked_certificates"`
	RetiredChannels     []string  `json:"retired_channels"`
}

func pinPolicyLists(certificates, channels []string) error {
	if certificates == nil || channels == nil || len(certificates) > maxRetainedRevocations || len(channels) > maxRetainedChannels {
		return errors.New("metadata: bounded cumulative pin-policy lists required")
	}
	last := ""
	for _, fingerprint := range certificates {
		if !validDigest(fingerprint) || fingerprint <= last {
			return errors.New("metadata: pin-policy certificates must be canonical sorted unique leaf SHA256")
		}
		last = fingerprint
	}
	last = ""
	for _, id := range channels {
		if !ValidChannelID(id) || id <= last {
			return errors.New("metadata: pin-policy channel IDs must be portable sorted unique values")
		}
		last = id
	}
	return nil
}

func exactPinPolicyObject(raw []byte, names ...string) error {
	if err := checkCheckpointJSON(raw); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(names) {
		return errors.New("metadata: exact pin-policy object required")
	}
	for _, name := range names {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("metadata: missing/null/unknown pin-policy field")
		}
	}
	return nil
}

func DecodePinPolicy(raw []byte) (*PinPolicySnapshot, error) {
	if len(raw) == 0 || len(raw) > delivery.MaxPinPolicyBytes {
		return nil, delivery.ErrBudget
	}
	raw = bytes.Clone(raw)
	if err := exactPinPolicyObject(raw, "schema", "purpose", "version", "profile_namespace", "minimum_protocol", "issued_at", "expires_at", "revoked_certificates", "retired_channels"); err != nil {
		return nil, err
	}
	var value PinPolicySnapshot
	if json.Unmarshal(raw, &value) != nil {
		return nil, errors.New("metadata: malformed pin-policy snapshot")
	}
	if err := value.shape(); err != nil {
		return nil, err
	}
	return &value, nil
}

func (p *PinPolicySnapshot) shape() error {
	if p == nil || p.Schema != 1 || p.Purpose != pinPolicyPurpose || p.Version == 0 || p.ProfileNamespace != "profiles/" || p.MinimumProtocol != PinPolicyProtocol || p.IssuedAt.IsZero() || p.IssuedAt.Year() < 1 || !p.ExpiresAt.After(p.IssuedAt) {
		return errors.New("metadata: unsupported pin-policy purpose/scope/protocol/interval")
	}
	return pinPolicyLists(p.RevokedCertificates, p.RetiredChannels)
}

func (p *PinPolicySnapshot) Validate(now time.Time) error {
	if err := p.shape(); err != nil {
		return err
	}
	if now.IsZero() || p.IssuedAt.After(now.Add(MaxClockSkew)) {
		return ErrTrustNotYetValid
	}
	if !now.Before(p.ExpiresAt) {
		return ErrProfileExpired
	}
	return nil
}

func EncodePinPolicy(p *PinPolicySnapshot, now time.Time) ([]byte, error) {
	if err := p.Validate(now); err != nil {
		return nil, err
	}
	owned := *p
	owned.IssuedAt, owned.ExpiresAt = p.IssuedAt.UTC(), p.ExpiresAt.UTC()
	raw, err := json.Marshal(&owned)
	if err != nil || len(raw) > delivery.MaxPinPolicyBytes {
		return nil, delivery.ErrBudget
	}
	return raw, nil
}

// VerifiedPinPolicy is an owned private SDK proof, not a parsed JSON token.
// It prepares a value only; caller still owns lifecycle/source/fixed4 persistence.
type VerifiedPinPolicy struct {
	value *PinPolicySnapshot
	raw   []byte
	proof *delivery.Prepared
}

func PreparePinPolicy(policy *TrustPolicy, proof *delivery.Prepared, raw []byte, now time.Time) (*VerifiedPinPolicy, error) {
	if len(raw) == 0 || len(raw) > delivery.MaxPinPolicyBytes {
		return nil, delivery.ErrBudget
	}
	owned := bytes.Clone(raw) // proof check and parse share one immutable input
	if err := CheckDeliveryProof(policy, proof, delivery.PinPolicy, delivery.PinPolicyPath, owned, now); err != nil {
		return nil, err
	}
	value, err := DecodePinPolicy(owned)
	if err != nil {
		return nil, err
	}
	if err = value.Validate(now); err != nil {
		return nil, err
	}
	return &VerifiedPinPolicy{value: value, raw: owned, proof: proof}, nil
}

func (v *VerifiedPinPolicy) Check(policy *TrustPolicy, now time.Time) error {
	if v == nil || v.value == nil {
		return delivery.ErrTrust
	}
	if err := CheckDeliveryProof(policy, v.proof, delivery.PinPolicy, delivery.PinPolicyPath, v.raw, now); err != nil {
		return err
	}
	return v.value.Validate(now)
}

// A normal profile bundle must carry both proofs from one complete SDK batch.
// Separate policy-only urgent acceptance is a distinct lifecycle operation.
func (v *VerifiedPinPolicy) CheckProfile(policy *TrustPolicy, profile *delivery.Prepared, target string, raw []byte, now time.Time) error {
	return v.CheckArtifact(policy, profile, delivery.Profile, target, raw, now)
}

// CheckArtifact also supports a publication batch containing an update and
// the global policy. It does not replace the independent application-role gate.
func (v *VerifiedPinPolicy) CheckArtifact(policy *TrustPolicy, artifact *delivery.Prepared, kind delivery.Kind, target string, raw []byte, now time.Time) error {
	if kind != delivery.Profile && kind != delivery.UpdateManifest {
		return delivery.ErrTrust
	}
	if err := v.Check(policy, now); err != nil {
		return err
	}
	if err := CheckDeliveryProof(policy, artifact, kind, target, raw, now); err != nil {
		return err
	}
	a, err := artifact.NextState().Digest()
	if err != nil {
		return err
	}
	b, err := v.proof.NextState().Digest()
	if err != nil || artifact.SourceDigest() != v.proof.SourceDigest() || a != b {
		return errors.New("metadata: mixed pin-policy/artifact repository states")
	}
	return nil
}

func (v *VerifiedPinPolicy) ValidUntil() time.Time {
	if v == nil || v.value == nil || v.proof == nil {
		return time.Time{}
	}
	deadline := v.value.ExpiresAt
	if v.proof.ValidUntil().Before(deadline) {
		deadline = v.proof.ValidUntil()
	}
	return deadline
}

func (v *VerifiedPinPolicy) retained() *RetainedPinPolicy {
	return &RetainedPinPolicy{Version: v.value.Version, SHA256: digestPinPolicy(v.raw), IssuedAt: v.value.IssuedAt, ExpiresAt: v.value.ExpiresAt, RevokedCertificates: slices.Clone(v.value.RevokedCertificates), RetiredChannels: slices.Clone(v.value.RetiredChannels)}
}
