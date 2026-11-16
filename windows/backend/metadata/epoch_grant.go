package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

const profileEpochPurpose = "snowden-system/profile-epoch/v1"

// ProfileEpochGrant has no standalone signing key. Authority comes only from
// the exact offline top-level TUF target proof of this byte representation.
type ProfileEpochGrant struct {
	Schema          int       `json:"schema"`
	Purpose         string    `json:"purpose"`
	FromEpoch       uint64    `json:"from_epoch"`
	ToEpoch         uint64    `json:"to_epoch"`
	ProfilePublic   string    `json:"profile_public_hex"`
	Namespace       string    `json:"profile_namespace"`
	MinimumProtocol uint64    `json:"minimum_protocol"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	GrantID         string    `json:"grant_id"`
}

func DecodeEpochGrant(raw []byte) (*ProfileEpochGrant, error) {
	if len(raw) == 0 || len(raw) > delivery.MaxEpochGrantBytes {
		return nil, errors.New("metadata: epoch grant outside bound")
	}
	if err := checkCheckpointJSON(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 10 {
		return nil, errors.New("metadata: exact epoch grant object required")
	}
	for _, name := range []string{"schema", "purpose", "from_epoch", "to_epoch", "profile_public_hex", "profile_namespace", "minimum_protocol", "issued_at", "expires_at", "grant_id"} {
		if value, exists := fields[name]; !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("metadata: unknown/missing/null epoch grant field")
		}
	}
	var grant ProfileEpochGrant
	if json.Unmarshal(raw, &grant) != nil {
		return nil, errors.New("metadata: malformed epoch grant")
	}
	return &grant, nil
}

func (g *ProfileEpochGrant) Validate(now time.Time) error {
	if g == nil || g.Schema != 1 || g.Purpose != profileEpochPurpose || g.FromEpoch == ^uint64(0) || g.ToEpoch != g.FromEpoch+1 || g.MinimumProtocol != ProfileEpochProtocol || g.Namespace != ProfileEpochNamespace(g.ToEpoch) || !validDigest(g.GrantID) {
		return errors.New("metadata: unsupported epoch transition/grant scope")
	}
	pub, err := hex.DecodeString(g.ProfilePublic)
	if err != nil || len(pub) != ed25519.PublicKeySize || g.ProfilePublic != hex.EncodeToString(pub) {
		return errors.New("metadata: invalid epoch profile public key")
	}
	if g.IssuedAt.IsZero() || !g.ExpiresAt.After(g.IssuedAt) {
		return errors.New("metadata: invalid epoch grant interval")
	}
	if g.IssuedAt.After(now.Add(MaxClockSkew)) {
		return ErrTrustNotYetValid
	}
	if !g.ExpiresAt.After(now) {
		return ErrProfileExpired
	}
	return nil
}

type epochAdvance struct {
	grantSHA256 string
	trustSHA256 string
	namespace   string
}

type verifiedEpochTarget struct {
	grant         *ProfileEpochGrant
	public        ed25519.PublicKey
	keyID         string
	contentSHA256 string
	grantSHA256   string
}

func verifyEpochTarget(policy *TrustPolicy, next *Envelope, profileProof *delivery.Prepared, targetPath string, profileRaw []byte, grantProof *delivery.Prepared, grantRaw []byte, now time.Time) (*verifiedEpochTarget, error) {
	grant, err := CheckEpochProofs(policy, profileProof, targetPath, profileRaw, grantProof, grantRaw, now)
	if err != nil {
		return nil, err
	}
	if err := checkDeliveredEnvelope(policy, next, profileProof, targetPath, profileRaw, now); err != nil {
		return nil, err
	}
	if next.ProfileEpoch != grant.ToEpoch || next.SchemaVersion != EpochEnvelopeSchema {
		return nil, ErrProfileEpoch
	}
	pub, _ := hex.DecodeString(grant.ProfilePublic)
	id := KeyIDFor(ed25519.PublicKey(pub))
	if next.KeyID != id {
		return nil, errors.New("metadata: epoch target signer mismatch")
	}
	if err := next.Verify(map[string]ed25519.PublicKey{id: ed25519.PublicKey(pub)}); err != nil {
		return nil, err
	}
	if err := next.ValidateCandidate(now); err != nil {
		return nil, err
	}
	digest, err := profileDigest(next)
	if err != nil {
		return nil, err
	}
	grantSum := sha256.Sum256(grantRaw)
	return &verifiedEpochTarget{grant: grant, public: ed25519.PublicKey(pub), keyID: id, contentSHA256: digest, grantSHA256: hex.EncodeToString(grantSum[:])}, nil
}

// Both private proofs must come from one retained repository state. A caller
// cannot turn a parsed grant or local recovery signature into epoch authority.
func CheckEpochProofs(policy *TrustPolicy, profile *delivery.Prepared, targetPath string, profileRaw []byte, grantProof *delivery.Prepared, grantRaw []byte, now time.Time) (*ProfileEpochGrant, error) {
	if err := CheckDeliveryProof(policy, profile, delivery.Profile, targetPath, profileRaw, now); err != nil {
		return nil, err
	}
	if err := CheckDeliveryProof(policy, grantProof, delivery.ProfileEpoch, delivery.EpochPolicyPath, grantRaw, now); err != nil {
		return nil, err
	}
	profileState, err := profile.NextState().Digest()
	if err != nil {
		return nil, err
	}
	grantState, err := grantProof.NextState().Digest()
	if err != nil || profile.SourceDigest() != grantProof.SourceDigest() || profileState != grantState {
		return nil, errors.New("metadata: mixed repository states for epoch/profile proof")
	}
	grant, err := DecodeEpochGrant(grantRaw)
	if err != nil {
		return nil, err
	}
	if err := grant.Validate(now); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(targetPath, grant.Namespace) {
		return nil, errors.New("metadata: epoch profile outside authorized namespace")
	}
	return grant, nil
}

// PrepareEpochRecovery need not load a lost old envelope. An intact checkpoint,
// enrolled lineage and two private proofs are mandatory. No counter/history or
// local role is reset by decoding an incoming grant. Caller commits all4 targets.
func (s *Store) PrepareEpochRecovery(next *Envelope, policy *TrustPolicy, profileProof *delivery.Prepared, targetPath string, profileRaw []byte, grantProof *delivery.Prepared, grantRaw []byte, now time.Time) (*TrustPolicy, *AcceptedProfileState, error) {
	state, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return nil, nil, err
	}
	if err := state.validate(); err != nil {
		return nil, nil, err
	}
	if policy == nil || policy.Delivery == nil {
		return nil, nil, ErrTrustRole
	}
	if err := CheckDeliveryBinding(policy, state); err != nil {
		return nil, nil, err
	}
	if err := CheckClockFloor(now, TrustClockFloor(policy, state)); err != nil {
		return nil, nil, err
	}
	now = laterTime(now, TrustClockFloor(policy, state))
	verified, err := verifyEpochTarget(policy, next, profileProof, targetPath, profileRaw, grantProof, grantRaw, now)
	if err != nil {
		return nil, nil, err
	}
	grant, pub, id := verified.grant, verified.public, verified.keyID
	digest, grantDigest := verified.contentSHA256, verified.grantSHA256
	if state.ProfileEpoch == grant.ToEpoch && state.ProfileVersion() == next.ProfileVersion() && state.EnvelopeSHA256 == digest && state.LastEpochGrantSHA256 == grantDigest && state.EpochNamespace == grant.Namespace {
		if !bytes.Equal(policy.VerificationKeys(RoleProfile)[id], pub) {
			return nil, nil, ErrTrustRole
		}
		return policy.Scoped(), state, nil // exact committed replay only
	}
	if state.ProfileEpoch != grant.FromEpoch {
		return nil, nil, errors.New("metadata: epoch source superseded or transition skipped")
	}
	trustDigest, err := policy.Digest()
	if err != nil {
		return nil, nil, err
	}
	updated := policy.Scoped()
	for i := range updated.Keys {
		if updated.Keys[i].Role == RoleProfile {
			updated.Keys[i].Retired = true
		}
	}
	updated, err = updated.WithKey(ed25519.PublicKey(pub), RoleProfile)
	if err != nil {
		return nil, nil, err // existing/retired/foreign role cannot be reused
	}
	accepted, err := advanceProfileWithEpoch(state, next, &epochAdvance{grantSHA256: grantDigest, trustSHA256: trustDigest, namespace: grant.Namespace})
	if err != nil {
		return nil, nil, err
	}
	return updated, accepted, nil
}
