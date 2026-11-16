package metadata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

// RetainedPinPolicy is local continuity, not reusable signing authority. Bans
// persist after snapshot expiry; a new snapshot still requires a fresh SDK proof.
type RetainedPinPolicy struct {
	Version             uint64    `json:"version"`
	SHA256              string    `json:"sha256"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	RevokedCertificates []string  `json:"revoked_certificates"`
	RetiredChannels     []string  `json:"retired_channels"`
}

func digestPinPolicy(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (p *RetainedPinPolicy) Clone() *RetainedPinPolicy {
	if p == nil {
		return nil
	}
	out := *p
	out.RevokedCertificates = slices.Clone(p.RevokedCertificates)
	out.RetiredChannels = slices.Clone(p.RetiredChannels)
	return &out
}

func (p *RetainedPinPolicy) validate() error {
	if p == nil || p.Version == 0 || !validDigest(p.SHA256) || p.IssuedAt.IsZero() || p.IssuedAt.Year() < 1 || !p.ExpiresAt.After(p.IssuedAt) {
		return errors.New("metadata: invalid retained pin-policy identity")
	}
	return pinPolicyLists(p.RevokedCertificates, p.RetiredChannels)
}

func (p *RetainedPinPolicy) UnmarshalJSON(raw []byte) error {
	if err := exactPinPolicyObject(raw, "version", "sha256", "issued_at", "expires_at", "revoked_certificates", "retired_channels"); err != nil {
		return err
	}
	type wire RetainedPinPolicy
	var value wire
	if json.Unmarshal(raw, &value) != nil {
		return errors.New("metadata: malformed retained pin policy")
	}
	next := RetainedPinPolicy(value)
	if err := next.validate(); err != nil {
		return err
	}
	*p = next
	return nil
}

func samePinPolicy(a, b *RetainedPinPolicy) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && bytes.Equal(x, y)
}

func pinPolicySuperset(next, previous []string) bool {
	i := 0
	for _, value := range next {
		if i < len(previous) && previous[i] == value {
			i++
		}
	}
	return i == len(previous)
}

// Apply prepares retained public state; it never writes files, retires a
// Manager, accepts a credential, or authorizes any new certificate. App wiring
// must retire/invalidate an unsafe active session before committing all4 files.
func (v *VerifiedPinPolicy) Apply(policy *TrustPolicy, state *AcceptedProfileState, now time.Time) (*AcceptedProfileState, error) {
	if err := v.Check(policy, now); err != nil {
		return nil, err
	}
	if state == nil || state.validate() != nil || state.DeliveryAnchorSHA256 != v.proof.NextState().AnchorSHA256 {
		return nil, errors.New("metadata: pin-policy checkpoint lineage missing or changed")
	}
	if err := CheckClockFloor(now, TrustClockFloor(policy, state)); err != nil {
		return nil, err
	}
	next, err := v.retain(state.PinPolicy)
	if err != nil {
		return nil, err
	}
	out := state.Clone()
	out.PinPolicy = next
	retired := make(map[string]bool, len(out.Retired))
	for _, id := range out.Retired {
		retired[id] = true
	}
	for _, id := range next.RetiredChannels {
		if !retired[id] {
			out.Retired = append(out.Retired, id)
			retired[id] = true
		}
	}
	sort.Strings(out.Retired)
	// Certificate bans were already global across ChannelIDs. Remove only
	// duplicate local entries now represented by the non-removable global set;
	// retain every ChannelID continuity marker, even when its local list is empty.
	for i := range out.Pins {
		old := out.Pins[i].Revoked
		kept := make([]string, 0, len(old))
		for _, fingerprint := range old {
			if _, exists := slices.BinarySearch(next.RevokedCertificates, fingerprint); !exists {
				kept = append(kept, fingerprint)
			}
		}
		out.Pins[i].Revoked = kept
	}
	out, err = WithClockObservation(out, now)
	if err != nil {
		return nil, err
	}
	if _, err := EncodeAcceptedProfile(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyProfile overlays a privately verified global policy onto an already
// authorized profile acceptance, then repeats all retained pin/ID rules against
// those exact delivered bytes. It grants no signer, epoch or enrollment itself.
func (v *VerifiedPinPolicy) ApplyProfile(policy *TrustPolicy, accepted *AcceptedProfileState, profile *delivery.Prepared, target string, raw []byte, envelope *Envelope, now time.Time) (*AcceptedProfileState, error) {
	if err := v.CheckProfile(policy, profile, target, raw, now); err != nil {
		return nil, err
	}
	if err := checkDeliveredEnvelope(policy, envelope, profile, target, raw, now); err != nil {
		return nil, err
	}
	digest, err := profileDigest(envelope)
	if err != nil || accepted == nil || accepted.ProfileVersion() != envelope.ProfileVersion() || accepted.EnvelopeSHA256 != digest {
		return nil, errors.New("metadata: pin-policy overlay requires exact prepared profile identity")
	}
	next, err := v.Apply(policy, accepted, now)
	if err != nil {
		return nil, err
	}
	return advanceProfile(next, envelope)
}
