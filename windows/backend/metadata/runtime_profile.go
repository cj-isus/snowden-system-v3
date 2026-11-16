package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

// ChannelRestriction describes a local reduction of an already signed grant.
// It is never serialized into Envelope or promoted to a validation measurement.
type ChannelRestriction struct {
	ChannelID string
	Reason    string
	Disabled  bool
}

// RuntimePinActivation is an owned scheduling hint, not import authority or
// network-health evidence. Only a fresh ordinary start can use the channel.
type RuntimePinActivation struct {
	At                time.Time
	ChannelID         string
	CertificateSHA256 string
}

// RuntimeProfile contains owned presentation/runtime data, not signing or
// import authority. Signed bytes and accepted epoch/history stay unchanged.
type RuntimeProfile struct {
	Channels          []Channel
	Restrictions      []ChannelRestriction
	KeyID             string
	Checkpointed      bool
	DeclaredChannels  int
	SplitDirect       []string
	Version           ProfileVersion
	EnvelopeSHA256    string
	SignerFingerprint string
	IssuedAt          time.Time
	ExpiresAt         time.Time
	ClockFloor        time.Time
	ObservedAt        time.Time
	Dependencies      DeliveryDependencies
	PinPolicySHA256   string
	PinPolicyVersion  string
	NextPinActivation *RuntimePinActivation
}

// LoadRuntimeProfile requires the caller's existing local journal ownership.
// The shared native writer scope covers the entire read/derive/legacy-save path;
// callers must not already hold WithWriter or a Store transaction callback.
// The global-policy branch accepts only the exact previously committed profile
// identity under an active signer; it cannot import a replacement/reset a floor.
// PrepareAcceptance/ApplyVersion retain their stricter incoming-profile contract.
func (s *Store) LoadRuntimeProfile(now time.Time, minimum ProfileVersion) (*RuntimeProfile, error) {
	var profile *RuntimeProfile
	err := filetxn.WithWriter(s.ImportJournal(), func() error {
		var err error
		profile, err = s.loadRuntimeProfile(now, minimum, true)
		return err
	})
	if err != nil {
		return nil, err // lock retirement uncertainty cannot publish a runtime view
	}
	return profile, nil
}

// InspectRuntimeProfile has the same signer/version/pin projection gate but
// never commits a legacy content anchor or updates a clock/history. Captures
// one read scope; do not call from another read/writer scope. A legacy view is
// not installed acceptance and must not be reported as an applied/eligible profile.
func (s *Store) InspectRuntimeProfile(now time.Time, minimum ProfileVersion) (*RuntimeProfile, error) {
	snapshot, err := s.InspectProfileSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.InspectRuntimeProfile(now, minimum)
}

func (s *Store) loadRuntimeProfile(now time.Time, minimum ProfileVersion, commitLegacy bool) (*RuntimeProfile, error) {
	snapshot, err := s.readProfileSnapshot()
	if err != nil {
		return nil, err
	}
	view, accepted, err := snapshot.projectRuntimeProfile(now, minimum)
	if err != nil {
		return nil, err
	}
	if commitLegacy && accepted != nil && snapshot.accepted.PinPolicy == nil {
		if err := s.saveAcceptedProfile(accepted, view.ObservedAt); err != nil {
			return nil, err
		}
		view.Checkpointed = accepted.HasCheckpoint()
	}
	return view, nil
}

// InspectRuntimeProfile projects only the owned captured values. It performs
// no I/O, enrollment or clock/history save and may safely be called after capture.
func (snapshot *ProfileSnapshot) InspectRuntimeProfile(now time.Time, minimum ProfileVersion) (*RuntimeProfile, error) {
	view, _, err := snapshot.projectRuntimeProfile(now, minimum)
	return view, err
}

func (snapshot *ProfileSnapshot) projectRuntimeProfile(now time.Time, minimum ProfileVersion) (*RuntimeProfile, *AcceptedProfileState, error) {
	if snapshot == nil || snapshot.policy == nil || snapshot.accepted == nil {
		return nil, nil, ErrTrustRole
	}
	if snapshot.restriction != nil {
		return nil, nil, snapshot.restriction
	}
	policy, state := snapshot.policy, snapshot.accepted
	envelope, present := snapshot.envelope, snapshot.source.present
	floor := TrustClockFloor(policy, state)
	if err := CheckClockFloor(now, floor); err != nil {
		return nil, nil, err
	}
	now = laterTime(now, floor)
	if !present {
		if state.ProfileVersion() != (ProfileVersion{}) || minimum != (ProfileVersion{}) {
			return nil, nil, fmt.Errorf("%w: accepted runtime envelope missing", ErrTrustRole)
		}
		return nil, nil, nil
	}
	if envelope == nil {
		return nil, nil, ErrTrustRole
	}
	keys := policy.VerificationKeys(RoleProfile)
	if err := envelope.Verify(keys); err != nil {
		return nil, nil, err
	}
	if state.ProfileVersion().Compare(minimum) > 0 {
		minimum = state.ProfileVersion()
	}
	if err := envelope.ValidateVersion(now, minimum); err != nil {
		return nil, nil, err
	}
	checkpointed := state.HasCheckpoint()
	if state.PinPolicy == nil {
		var err error
		state, err = prepareAcceptedProfile(state, policy, envelope, true, envelope, keys, now, false)
		if err != nil {
			return nil, nil, err
		}
	} else {
		digest, digestErr := profileDigest(envelope)
		if digestErr != nil || !state.HasCheckpoint() || state.ProfileVersion() != envelope.ProfileVersion() || state.EnvelopeSHA256 != digest {
			return nil, nil, fmt.Errorf("%w: runtime projection requires exact installed profile identity", ErrTrustRole)
		}
	}
	if checkpointed {
		_, unchanged, err := runtimeCheckpointUpdate(snapshot.accepted, state, now)
		if err != nil {
			return nil, nil, err
		}
		if !unchanged {
			return nil, nil, fmt.Errorf("%w: runtime projection cannot claim unretained history", ErrTrustRole)
		}
	}
	channels, restrictions, err := projectRuntimeChannels(envelope, state, now)
	if err != nil {
		return nil, nil, err
	}
	fingerprint := sha256.Sum256(keys[envelope.KeyID])
	view := &RuntimeProfile{Channels: channels, Restrictions: restrictions, KeyID: envelope.KeyID, Checkpointed: checkpointed, DeclaredChannels: len(envelope.Channels), SplitDirect: slices.Clone(envelope.SplitDirect),
		Version: envelope.ProfileVersion(), EnvelopeSHA256: state.EnvelopeSHA256, SignerFingerprint: hex.EncodeToString(fingerprint[:]),
		IssuedAt: envelope.IssuedAt, ExpiresAt: envelope.ExpiresAt, ClockFloor: TrustClockFloor(policy, state), ObservedAt: now, Dependencies: envelope.Dependencies}
	if state.PinPolicy != nil {
		view.PinPolicySHA256, view.PinPolicyVersion = state.PinPolicy.SHA256, fmt.Sprint(state.PinPolicy.Version)
	}
	view.NextPinActivation, err = nextRuntimePinActivation(envelope, channels, restrictions, now)
	if err != nil {
		return nil, nil, err
	}
	return view, state, nil
}

func nextRuntimePinActivation(envelope *Envelope, channels []Channel, restrictions []ChannelRestriction, now time.Time) (*RuntimePinActivation, error) {
	certificateOnly := map[string]bool{}
	for _, restriction := range restrictions {
		if restriction.Disabled && (restriction.Reason == "certificate_unavailable" || restriction.Reason == "revoked_certificate_unavailable") {
			certificateOnly[restriction.ChannelID] = true
		}
	}
	signedEnabled := map[string]bool{}
	for _, channel := range envelope.Channels {
		signedEnabled[channel.ID] = channel.Enabled
	}
	var next *RuntimePinActivation
	for _, channel := range channels {
		pins, _ := channel.SignedLeafPins()
		if !signedEnabled[channel.ID] || !certificateOnly[channel.ID] || pins == nil || pins.Next == nil || channel.ValidationStatus != "configured" && channel.ValidationStatus != "live-verified" {
			continue
		}
		at, err := pins.NextActivation(channel.Hostname, now)
		if err != nil {
			return nil, err
		}
		if at.IsZero() || !at.Before(envelope.ExpiresAt) {
			continue
		}
		if next == nil || at.Before(next.At) {
			next = &RuntimePinActivation{At: at, ChannelID: channel.ID, CertificateSHA256: pins.Next.SHA256}
		}
	}
	return next, nil
}

func projectRuntimeChannels(envelope *Envelope, state *AcceptedProfileState, now time.Time) ([]Channel, []ChannelRestriction, error) {
	banned := retainedCertificateBans(state)
	retired := retainedChannelBans(state)
	channels := make([]Channel, 0, len(envelope.Channels))
	restrictions := []ChannelRestriction{}
	for _, original := range envelope.Channels {
		if slices.Contains(envelope.Revocations, original.ID) {
			continue // keep the existing signed-revocation presentation contract
		}
		channel := original.Clone()
		if retired[channel.ID] {
			channel.Enabled = false
			restrictions = append(restrictions, ChannelRestriction{channel.ID, "channel_revoked", true})
		} else if state.PinPolicy != nil && channel.Protocol == "hysteria2" && channel.HY2Pins == nil {
			channel.Enabled = false
			restrictions = append(restrictions, ChannelRestriction{channel.ID, "signed_pin_required", true})
		} else if pins, _ := channel.SignedLeafPins(); pins != nil {
			entries := []CertificatePin{pins.Current}
			if pins.Next != nil {
				entries = append(entries, *pins.Next)
			}
			filtered := false
			for _, pin := range entries {
				if banned[pin.SHA256] && !slices.Contains(pins.Revoked, pin.SHA256) {
					pins.Revoked = append(pins.Revoked, pin.SHA256) // only deny signed entries, never add a certificate
					filtered = true
				}
			}
			if _, err := pins.Certificates(channel.Hostname, now); err != nil {
				if !errors.Is(err, ErrNoActivePin) {
					return nil, nil, err
				}
				channel.Enabled = false
				reason := "certificate_unavailable"
				if filtered {
					reason = "revoked_certificate_unavailable"
				}
				restrictions = append(restrictions, ChannelRestriction{channel.ID, reason, true})
			} else if filtered {
				restrictions = append(restrictions, ChannelRestriction{channel.ID, "certificate_filtered", false})
			}
		}
		if channel.ValidationStatus == "live-verified" && channel.ValidatedAt == nil {
			channel.ValidationStatus = "configured"
		}
		if channel.Enabled && channel.CheckProtectedPayload() != nil {
			channel.Enabled = false // local capability, never a signed revocation/history edit
			restrictions = append(restrictions, ChannelRestriction{channel.ID, RestrictionPayloadConfidentiality, true})
		}
		channels = append(channels, channel)
	}
	return channels, restrictions, nil
}

func retainedCertificateBans(state *AcceptedProfileState) map[string]bool {
	banned := map[string]bool{}
	if state == nil {
		return banned
	}
	if state.PinPolicy != nil {
		for _, id := range state.PinPolicy.RevokedCertificates {
			banned[id] = true
		}
	}
	for _, retained := range state.Pins {
		for _, id := range retained.Revoked {
			banned[id] = true // existing certificate bans were global across IDs
		}
	}
	return banned
}

// RevocationCounts describes retained local/global prohibitions, not network
// health or proof that unseen private history has been recovered.
func (s *AcceptedProfileState) RevocationCounts() (certificates, channels int) {
	return len(retainedCertificateBans(s)), len(retainedChannelBans(s))
}

func retainedChannelBans(s *AcceptedProfileState) map[string]bool {
	retired := map[string]bool{}
	if s != nil {
		for _, id := range s.Retired {
			retired[id] = true
		}
		if s.PinPolicy != nil {
			for _, id := range s.PinPolicy.RetiredChannels {
				retired[id] = true
			}
		}
	}
	return retired
}
