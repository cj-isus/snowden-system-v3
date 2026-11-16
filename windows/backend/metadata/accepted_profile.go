package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

const maxAcceptedProfileSize = 1 << 20
const maxRetainedChannels = 1024
const maxRetainedRevocations = 8192

// AcceptedProfileState is a local trust checkpoint, committed with the signed
// envelope, keys and vault. It is not a new signing authority. Local filesystem
// tampering remains an OS/ACL boundary; untrusted incoming profiles cannot reset
// it. A legacy decimal version.txt is upgraded on the next accepted profile.
type AcceptedProfileState struct {
	Schema               int                `json:"schema"`
	Version              uint64             `json:"version"`
	EnvelopeSHA256       string             `json:"envelope_sha256"`
	Pins                 []PinContinuity    `json:"pins,omitempty"`
	Retired              []string           `json:"retired,omitempty"`
	TimeFloor            time.Time          `json:"time_floor,omitzero"`
	IssuedAtFloor        time.Time          `json:"issued_at_floor,omitzero"`
	LastRecoverySHA256   string             `json:"last_recovery_sha256,omitempty"`
	DeliveryAnchorSHA256 string             `json:"delivery_anchor_sha256,omitempty"`
	ProfileEpoch         uint64             `json:"profile_epoch,omitempty"`
	EpochNamespace       string             `json:"epoch_namespace,omitempty"`
	LastEpochGrantSHA256 string             `json:"last_epoch_grant_sha256,omitempty"`
	EpochHistory         []EpochBoundary    `json:"epoch_history,omitempty"`
	EpochOrigin          *EpochOrigin       `json:"epoch_origin,omitempty"`
	PinPolicy            *RetainedPinPolicy `json:"pin_policy,omitempty"`
}

type PinContinuity struct {
	ChannelID string   `json:"channel_id"`
	Revoked   []string `json:"revoked,omitempty"`
	Purpose   string   `json:"purpose,omitempty"` // absent is legacy HY2; WS cannot reuse that authority
}

func (pin *PinContinuity) UnmarshalJSON(data []byte) error {
	fields, err := decodeMetadataObject(data, maxAcceptedProfileSize, "channel_id", "revoked", "purpose")
	if err != nil {
		return errors.New("metadata: invalid retained pin object")
	}
	if raw, present := fields["purpose"]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("metadata: retained pin purpose cannot be null")
	}
	type plain PinContinuity
	var out plain
	if json.Unmarshal(data, &out) != nil || !ValidChannelID(out.ChannelID) || out.Purpose != "" && out.Purpose != WebSocketServerAuthPurpose {
		return errors.New("metadata: invalid retained pin purpose/channel")
	}
	*pin = PinContinuity(out)
	return nil
}

// Clone keeps retained trust history owned across preparation and clock writes.
func (s *AcceptedProfileState) Clone() *AcceptedProfileState {
	if s == nil {
		return nil
	}
	out := *s
	out.Pins = slices.Clone(s.Pins)
	for i := range out.Pins {
		out.Pins[i].Revoked = slices.Clone(s.Pins[i].Revoked)
	}
	out.Retired = slices.Clone(s.Retired)
	out.EpochHistory = slices.Clone(s.EpochHistory)
	out.EpochOrigin = s.EpochOrigin.Clone()
	out.PinPolicy = s.PinPolicy.Clone()
	return &out
}

func profileDigest(e *Envelope) (string, error) {
	if e == nil {
		return "", errors.New("metadata: missing accepted envelope")
	}
	copy := *e
	copy.Signature = ""
	data, err := json.Marshal(&copy)
	if err != nil {
		return "", errors.New("metadata: envelope cannot be canonicalized")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ContentSHA256 is the accepted canonical identity (signature excluded), not
// the raw TUF target hash. Issuer reports label both meanings explicitly.
func (e *Envelope) ContentSHA256() (string, error) { return profileDigest(e) }

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

// LoadAcceptedProfile reads only the public local checkpoint, never keys/vault.
// Missing is a clean initial state; malformed/inaccessible state is an error,
// never version zero. Symlinks and oversized/non-regular files are rejected.
func LoadAcceptedProfile(dir string) (result *AcceptedProfileState, finalErr error) {
	defer func() {
		if finalErr != nil && !errors.Is(finalErr, ErrTrustRole) {
			finalErr = errors.Join(ErrTrustRole, finalErr) // a local trust fault, not a network/channel failure
		}
	}()
	path := filepath.Join(dir, "version.txt")
	data, present, err := storedCheckpointFile.read(path)
	if err != nil {
		return nil, fmt.Errorf("metadata: accepted profile checkpoint: %w", err)
	}
	if !present {
		return &AcceptedProfileState{}, nil
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("metadata: empty accepted profile checkpoint")
	}
	if data[0] != '{' {
		version, err := strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return nil, errors.New("metadata: corrupt legacy profile version")
		}
		return &AcceptedProfileState{Version: version}, nil
	}
	if err := checkCheckpointJSON(data); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, errors.New("metadata: malformed accepted profile object")
	}
	for _, key := range []string{"time_floor", "issued_at_floor", "last_recovery_sha256", "delivery_anchor_sha256", "profile_epoch", "epoch_namespace", "last_epoch_grant_sha256", "epoch_history", "epoch_origin", "pin_policy"} {
		if value, exists := fields[key]; exists && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("metadata: retained trust property cannot be null")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var state AcceptedProfileState
	if err := dec.Decode(&state); err != nil {
		return nil, errors.New("metadata: malformed accepted profile checkpoint")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("metadata: trailing accepted profile data")
	}
	if err := state.validate(); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *AcceptedProfileState) validate() error {
	if !s.HasCheckpoint() || !validDigest(s.EnvelopeSHA256) || len(s.Pins) > maxRetainedChannels || len(s.Retired) > maxRetainedChannels {
		return errors.New("metadata: invalid accepted profile identity or retention bounds")
	}
	if err := s.validateEpochHistory(); err != nil {
		return err
	}
	if !s.TimeFloor.IsZero() && s.TimeFloor.Year() < 1 || !s.IssuedAtFloor.IsZero() && s.IssuedAtFloor.Year() < 1 {
		return errors.New("metadata: invalid retained clock floor")
	}
	if s.LastRecoverySHA256 != "" && !validDigest(s.LastRecoverySHA256) {
		return errors.New("metadata: invalid retained recovery identity")
	}
	if s.DeliveryAnchorSHA256 != "" && !validDigest(s.DeliveryAnchorSHA256) {
		return errors.New("metadata: invalid retained delivery anchor")
	}
	seen := map[string]bool{}
	count := 0
	if s.PinPolicy != nil {
		if err := s.PinPolicy.validate(); err != nil {
			return err
		}
		if !validDigest(s.DeliveryAnchorSHA256) {
			return errors.New("metadata: retained global pin policy requires delivery lineage")
		}
		count = len(s.PinPolicy.RevokedCertificates)
	}
	for _, pin := range s.Pins {
		if !ValidChannelID(pin.ChannelID) || seen[pin.ChannelID] || pin.Purpose != "" && pin.Purpose != WebSocketServerAuthPurpose {
			return errors.New("metadata: invalid retained pin channel")
		}
		seen[pin.ChannelID] = true
		hashes := map[string]bool{}
		for _, digest := range pin.Revoked {
			if !validDigest(digest) || hashes[digest] {
				return errors.New("metadata: invalid retained pin revocation")
			}
			hashes[digest] = true
			count++
		}
	}
	seen = map[string]bool{}
	for _, id := range s.Retired {
		if !ValidChannelID(id) || seen[id] {
			return errors.New("metadata: invalid retired channel")
		}
		seen[id] = true
	}
	if s.PinPolicy != nil {
		for _, id := range s.PinPolicy.RetiredChannels {
			if !seen[id] {
				return errors.New("metadata: globally retired channel missing from retained union")
			}
		}
	}
	if count > maxRetainedRevocations {
		return errors.New("metadata: retained revocation budget exhausted")
	}
	return nil
}

func EncodeAcceptedProfile(state *AcceptedProfileState) ([]byte, error) {
	if state == nil {
		return nil, errors.New("metadata: missing accepted profile state")
	}
	if err := state.validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil || len(data) > maxAcceptedProfileSize {
		return nil, errors.New("metadata: accepted profile checkpoint exceeds bound")
	}
	return data, nil
}

// PrepareAcceptance authenticates both the installed and incoming envelope.
// Historical expiry is intentionally not revalidated: an expired signed
// profile still anchors identity/revocation history during renewal. Callers
// serialize profile readers/writers and commit the returned bytes atomically.
func (s *Store) PrepareAcceptance(next *Envelope, keys map[string]ed25519.PublicKey, now time.Time) (*AcceptedProfileState, error) {
	return s.prepareAcceptance(next, keys, now, false)
}

func (s *Store) PrepareDeliveredAcceptance(next *Envelope, keys map[string]ed25519.PublicKey, proof *delivery.Prepared, path string, raw []byte, now time.Time) (*AcceptedProfileState, error) {
	policy, _, err := s.LoadTrustPolicy()
	if err != nil {
		return nil, err
	}
	if err := checkDeliveredEnvelope(policy, next, proof, path, raw, now); err != nil {
		return nil, err
	}
	state, err := LoadAcceptedProfile(s.Dir)
	if err != nil {
		return nil, err
	}
	if err := checkProfileTarget(state, next, path); err != nil {
		return nil, err
	}
	return s.prepareAcceptance(next, keys, now, true)
}

func (s *Store) prepareAcceptance(next *Envelope, keys map[string]ed25519.PublicKey, now time.Time, delivered bool) (*AcceptedProfileState, error) {
	installedPolicy, _, state, err := s.loadTrustSource()
	if err != nil {
		return nil, err
	}
	previous, present, err := s.LoadEnvelope()
	if err != nil {
		return nil, err
	}
	return prepareAcceptedProfile(state, installedPolicy, previous, present, next, keys, now, delivered)
}

// Value-only acceptance over one captured source. Import supplies its prepared
// inputs; runtime snapshots use the same algorithm without reopening files.
func prepareAcceptedProfile(state *AcceptedProfileState, installedPolicy *TrustPolicy, previous *Envelope, present bool, next *Envelope, keys map[string]ed25519.PublicKey, now time.Time, delivered bool) (*AcceptedProfileState, error) {
	if state == nil || installedPolicy == nil {
		return nil, ErrTrustRole
	}
	if installedPolicy.Delivery != nil && !delivered {
		digest, err := profileDigest(next)
		if err != nil || !state.HasCheckpoint() || state.ProfileVersion() != next.ProfileVersion() || state.EnvelopeSHA256 != digest {
			return nil, fmt.Errorf("%w: new profile requires exact delivery proof", ErrTrustRole)
		}
	}
	if !present && state.Version != 0 {
		return nil, fmt.Errorf("%w: accepted envelope missing; explicit recovery required", ErrTrustRole)
	}
	if present {
		if previous == nil {
			return nil, ErrTrustRole
		}
		if err := previous.Verify(installedPolicy.historicalProfileKeys()); err != nil {
			return nil, fmt.Errorf("metadata: installed trust anchor: %w", err)
		}
		digest, err := profileDigest(previous)
		if err != nil {
			return nil, err
		}
		if state.EnvelopeSHA256 != "" {
			if state.ProfileVersion() != previous.ProfileVersion() || state.EnvelopeSHA256 != digest {
				return nil, fmt.Errorf("%w: installed envelope differs from accepted checkpoint", ErrTrustRole)
			}
		} else {
			if previous.ProfileEpoch != 0 || previous.ProfileVersion().Compare(state.ProfileVersion()) < 0 {
				return nil, errors.New("metadata: installed envelope below retained version")
			}
			// One-time bootstrap for legacy numeric floors. The currently
			// authenticated installed envelope is the first content anchor.
			state, err = advanceProfile(&AcceptedProfileState{}, previous)
			if err != nil {
				return nil, err
			}
		}
	}
	if next == nil {
		return nil, errors.New("metadata: missing incoming profile")
	}
	if err := next.Verify(keys); err != nil {
		return nil, err
	}
	if err := CheckClockFloor(now, TrustClockFloor(installedPolicy, state)); err != nil {
		return nil, err
	}
	now = laterTime(now, TrustClockFloor(installedPolicy, state))
	if err := next.ValidateVersion(now, state.ProfileVersion()); err != nil {
		return nil, err
	}
	return advanceProfile(state, next)
}

func advanceProfile(state *AcceptedProfileState, next *Envelope) (*AcceptedProfileState, error) {
	return advanceProfileWithEpoch(state, next, nil)
}

func advanceProfileWithEpoch(state *AcceptedProfileState, next *Envelope, epoch *epochAdvance) (*AcceptedProfileState, error) {
	return advanceProfileValue(state, next, epoch, nil)
}

func advanceProfileValue(state *AcceptedProfileState, next *Envelope, epoch *epochAdvance, origin *EpochOrigin) (*AcceptedProfileState, error) {
	if state == nil {
		return nil, errors.New("metadata: missing source checkpoint")
	}
	if err := state.PinPolicy.CheckEnvelope(next); err != nil {
		return nil, err
	}
	digest, err := profileDigest(next)
	if err != nil {
		return nil, err
	}
	if next.MetadataVersion == 0 {
		return nil, errors.New("metadata: profile version downgrade")
	}
	if epoch == nil && origin == nil && (next.ProfileEpoch != state.ProfileEpoch || next.ProfileVersion().Compare(state.ProfileVersion()) < 0) {
		return nil, ErrProfileEpoch
	}
	if epoch != nil && (origin != nil || !state.HasCheckpoint() || state.ProfileEpoch == ^uint64(0) || len(state.EpochHistory) >= MaxRetainedProfileEpochs || next.ProfileEpoch != state.ProfileEpoch+1 || next.SchemaVersion != EpochEnvelopeSchema || epoch.namespace != ProfileEpochNamespace(next.ProfileEpoch) || !validDigest(epoch.grantSHA256) || !validDigest(epoch.trustSHA256)) {
		return nil, ErrProfileEpoch
	}
	if origin != nil && (epoch != nil || !state.emptyEpochSource() || origin.validate() != nil || next.SchemaVersion != EpochEnvelopeSchema || next.ProfileEpoch != origin.Epoch || next.MetadataVersion != origin.Counter || digest != origin.EnvelopeSHA256) {
		return nil, ErrProfileEpoch
	}
	if epoch == nil && origin == nil && next.ProfileVersion() == state.ProfileVersion() && state.EnvelopeSHA256 != "" && digest != state.EnvelopeSHA256 {
		return nil, errors.New("metadata: conflicting content under the same profile version")
	}
	out := &AcceptedProfileState{Schema: 1, Version: next.MetadataVersion, EnvelopeSHA256: digest, Retired: slices.Clone(state.Retired), TimeFloor: state.TimeFloor, IssuedAtFloor: laterTime(state.IssuedAtFloor, next.IssuedAt), LastRecoverySHA256: state.LastRecoverySHA256, DeliveryAnchorSHA256: state.DeliveryAnchorSHA256}
	out.ProfileEpoch, out.EpochNamespace, out.LastEpochGrantSHA256 = state.ProfileEpoch, state.EpochNamespace, state.LastEpochGrantSHA256
	out.EpochHistory = slices.Clone(state.EpochHistory)
	out.EpochOrigin = state.EpochOrigin.Clone()
	out.PinPolicy = state.PinPolicy.Clone()
	if origin != nil {
		out.EpochOrigin = origin.Clone()
		out.ProfileEpoch, out.EpochNamespace, out.LastEpochGrantSHA256 = origin.Epoch, ProfileEpochNamespace(origin.Epoch), origin.GrantSHA256
		out.DeliveryAnchorSHA256 = origin.AnchorSHA256
	}
	if epoch != nil {
		out.ProfileEpoch, out.EpochNamespace, out.LastEpochGrantSHA256 = next.ProfileEpoch, epoch.namespace, epoch.grantSHA256
		out.EpochHistory = append(out.EpochHistory, EpochBoundary{Epoch: state.ProfileEpoch, Counter: state.Version, EnvelopeSHA256: state.EnvelopeSHA256, TrustSHA256: epoch.trustSHA256, GrantSHA256: epoch.grantSHA256, AnchorSHA256: state.DeliveryAnchorSHA256})
	}
	if out.ProfileEpoch != 0 {
		out.Schema = 2
		if out.EpochOrigin != nil {
			out.Schema = 3
		}
	}
	retained := map[string][]string{}
	purposes := map[string]string{}
	globalRevoked := map[string]bool{}
	policyCertificates := map[string]bool{}
	if state.PinPolicy != nil {
		for _, fingerprint := range state.PinPolicy.RevokedCertificates {
			globalRevoked[fingerprint], policyCertificates[fingerprint] = true, true
		}
	}
	for _, policy := range state.Pins {
		retained[policy.ChannelID] = slices.Clone(policy.Revoked)
		purposes[policy.ChannelID] = policy.Purpose
		for _, digest := range policy.Revoked {
			globalRevoked[digest] = true
		}
	}
	channels := map[string]Channel{}
	for _, ch := range next.Channels {
		channels[ch.ID] = ch
		if pins, _ := ch.SignedLeafPins(); pins != nil && !slices.Contains(next.Revocations, ch.ID) {
			for _, digest := range pins.Revoked {
				globalRevoked[digest] = true
			}
		}
	}
	for _, id := range next.Revocations {
		if !slices.Contains(out.Retired, id) {
			out.Retired = append(out.Retired, id)
		}
	}
	for id := range retained {
		ch, present := channels[id]
		if !present && !slices.Contains(out.Retired, id) {
			out.Retired = append(out.Retired, id)
		}
		pins, purpose := ch.SignedLeafPins()
		if present && !slices.Contains(out.Retired, id) && (pins == nil || purpose != purposes[id]) {
			return nil, errors.New("metadata: signed leaf policy cannot lose trust or change transport/purpose; retire the ID and provision a new one")
		}
	}
	for _, ch := range next.Channels {
		if slices.Contains(out.Retired, ch.ID) {
			if !slices.Contains(next.Revocations, ch.ID) {
				return nil, errors.New("metadata: retired channel cannot be resurrected; use a new channel ID")
			}
			continue
		}
		pins, purpose := ch.SignedLeafPins()
		if pins == nil {
			continue
		}
		old := retained[ch.ID]
		entries := []CertificatePin{pins.Current}
		if pins.Next != nil {
			entries = append(entries, *pins.Next)
		}
		for _, pin := range entries {
			if globalRevoked[pin.SHA256] && !slices.Contains(pins.Revoked, pin.SHA256) {
				return nil, errors.New("metadata: revoked certificate cannot be reauthorized")
			}
		}
		for _, revoked := range pins.Revoked {
			if !policyCertificates[revoked] && !slices.Contains(old, revoked) {
				old = append(old, revoked)
			}
		}
		retained[ch.ID] = old
		purposes[ch.ID] = purpose
	}
	for id, revoked := range retained {
		sort.Strings(revoked)
		out.Pins = append(out.Pins, PinContinuity{ChannelID: id, Revoked: revoked, Purpose: purposes[id]})
	}
	sort.Slice(out.Pins, func(i, j int) bool { return out.Pins[i].ChannelID < out.Pins[j].ChannelID })
	sort.Strings(out.Retired)
	if _, err := EncodeAcceptedProfile(out); err != nil {
		return nil, err
	}
	return out, nil
}

// Reject duplicate/aliased checkpoint keys before encoding/json field folding.
func checkCheckpointJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 8 {
			return errors.New("metadata: checkpoint nesting too deep")
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, container := token.(json.Delim)
		if !container {
			return nil
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				keyToken, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] || key == "" {
					return errors.New("metadata: duplicate checkpoint property")
				}
				for _, c := range []byte(key) {
					if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
						return errors.New("metadata: noncanonical checkpoint property")
					}
				}
				seen[key] = true
			}
			if err := walk(depth + 1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(0); err != nil {
		return fmt.Errorf("metadata: checkpoint JSON: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("metadata: trailing checkpoint JSON")
	}
	return nil
}
