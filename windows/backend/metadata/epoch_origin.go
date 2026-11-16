package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// EpochOrigin records an actually approved first profile, not fictional prior
// epoch transitions. It is immutable after initial four-target enrollment.
type EpochOrigin struct {
	Schema            int    `json:"schema"`
	Epoch             uint64 `json:"profile_epoch"`
	Counter           uint64 `json:"counter"`
	EnvelopeSHA256    string `json:"envelope_sha256"`
	GrantSHA256       string `json:"grant_sha256"`
	AnchorSHA256      string `json:"anchor_sha256"`
	SourceTrustSHA256 string `json:"source_trust_sha256"`
}

func (o *EpochOrigin) Clone() *EpochOrigin {
	if o == nil {
		return nil
	}
	out := *o
	return &out
}

func sameEpochOrigin(a, b *EpochOrigin) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (o *EpochOrigin) validate() error {
	if o == nil || o.Schema != 1 || o.Epoch == 0 || o.Counter == 0 || !validDigest(o.EnvelopeSHA256) || !validDigest(o.GrantSHA256) || !validDigest(o.AnchorSHA256) || !validDigest(o.SourceTrustSHA256) {
		return fmt.Errorf("%w: invalid initial epoch origin", ErrProfileEpoch)
	}
	return nil
}

func (o *EpochOrigin) UnmarshalJSON(data []byte) error {
	if err := checkCheckpointJSON(data); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != 7 {
		return fmt.Errorf("%w: exact epoch origin object required", ErrProfileEpoch)
	}
	for _, name := range []string{"schema", "profile_epoch", "counter", "envelope_sha256", "grant_sha256", "anchor_sha256", "source_trust_sha256"} {
		if raw, exists := fields[name]; !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%w: missing/null epoch origin field", ErrProfileEpoch)
		}
	}
	type wireOrigin EpochOrigin
	var out wireOrigin
	if json.Unmarshal(data, &out) != nil {
		return fmt.Errorf("%w: malformed epoch origin", ErrProfileEpoch)
	}
	next := EpochOrigin(out)
	if err := next.validate(); err != nil {
		return err
	}
	*o = next
	return nil
}
