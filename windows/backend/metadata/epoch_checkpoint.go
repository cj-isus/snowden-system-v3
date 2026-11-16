package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// EpochBoundary records the exact accepted source before an authorized epoch
// transition. Global time/pin/channel/key retention remains in the live state.
type EpochBoundary struct {
	Epoch          uint64 `json:"profile_epoch"`
	Counter        uint64 `json:"counter"`
	EnvelopeSHA256 string `json:"envelope_sha256"`
	TrustSHA256    string `json:"trust_sha256"`
	GrantSHA256    string `json:"grant_sha256"`
	AnchorSHA256   string `json:"anchor_sha256"`
}

func (b *EpochBoundary) UnmarshalJSON(data []byte) error {
	if err := checkCheckpointJSON(data); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != 6 {
		return errors.New("metadata: invalid epoch boundary object")
	}
	for _, name := range []string{"profile_epoch", "counter", "envelope_sha256", "trust_sha256", "grant_sha256", "anchor_sha256"} {
		if raw, exists := fields[name]; !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("metadata: missing/null epoch boundary field")
		}
	}
	type wireBoundary EpochBoundary
	var out wireBoundary
	if json.Unmarshal(data, &out) != nil {
		return errors.New("metadata: malformed epoch boundary")
	}
	*b = EpochBoundary(out)
	return nil
}

func (s *AcceptedProfileState) HasCheckpoint() bool {
	return s != nil && s.Version > 0 && (s.Schema == 1 && s.ProfileEpoch == 0 || s.Schema == 2 && s.ProfileEpoch > 0 && s.EpochOrigin == nil || s.Schema == 3 && s.ProfileEpoch > 0 && s.EpochOrigin != nil)
}

func (s *AcceptedProfileState) validateEpochHistory() error {
	if s.ProfileEpoch == 0 {
		if s.Schema != 1 || s.EpochOrigin != nil || len(s.EpochHistory) != 0 || s.EpochNamespace != "" || s.LastEpochGrantSHA256 != "" {
			return fmt.Errorf("%w: legacy checkpoint contains epoch authority", ErrProfileEpoch)
		}
		return nil
	}
	base := uint64(0)
	lastGrant := ""
	if s.EpochOrigin != nil {
		if s.Schema != 3 || s.EpochOrigin.validate() != nil || s.EpochOrigin.AnchorSHA256 != s.DeliveryAnchorSHA256 {
			return fmt.Errorf("%w: invalid retained origin binding", ErrProfileEpoch)
		}
		base, lastGrant = s.EpochOrigin.Epoch, s.EpochOrigin.GrantSHA256
	} else if s.Schema != 2 {
		return fmt.Errorf("%w: missing retained epoch origin", ErrProfileEpoch)
	}
	if s.ProfileEpoch < base || len(s.EpochHistory) > MaxRetainedProfileEpochs || uint64(len(s.EpochHistory)) != s.ProfileEpoch-base || !validDigest(s.LastEpochGrantSHA256) || !validDigest(s.DeliveryAnchorSHA256) || s.EpochNamespace != ProfileEpochNamespace(s.ProfileEpoch) {
		return fmt.Errorf("%w: missing/invalid retained epoch authority", ErrProfileEpoch)
	}
	for i, boundary := range s.EpochHistory {
		if boundary.Epoch != base+uint64(i) || boundary.Counter == 0 || !validDigest(boundary.EnvelopeSHA256) || !validDigest(boundary.TrustSHA256) || !validDigest(boundary.GrantSHA256) || boundary.AnchorSHA256 != s.DeliveryAnchorSHA256 {
			return fmt.Errorf("%w: invalid retained epoch source boundary", ErrProfileEpoch)
		}
		lastGrant = boundary.GrantSHA256
	}
	if lastGrant != s.LastEpochGrantSHA256 {
		return fmt.Errorf("%w: retained epoch grant continuity mismatch", ErrProfileEpoch)
	}
	if s.EpochOrigin != nil {
		first := ProfileVersion{Epoch: s.ProfileEpoch, Counter: s.Version}
		firstHash := s.EnvelopeSHA256
		if len(s.EpochHistory) != 0 {
			first = ProfileVersion{Epoch: s.EpochHistory[0].Epoch, Counter: s.EpochHistory[0].Counter}
			firstHash = s.EpochHistory[0].EnvelopeSHA256
		}
		if first.Epoch != base || first.Counter < s.EpochOrigin.Counter || first.Counter == s.EpochOrigin.Counter && firstHash != s.EpochOrigin.EnvelopeSHA256 {
			return fmt.Errorf("%w: retained source predates/conflicts with origin", ErrProfileEpoch)
		}
	}
	return nil
}
