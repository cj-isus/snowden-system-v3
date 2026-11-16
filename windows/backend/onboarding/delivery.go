package onboarding

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/delivery"
)

// ProfileBytes is base64 on the wire. RawMessage is compacted by outer JSON
// marshaling, so it cannot preserve TUF target length/hash across transport.
// Envelope remains for legacy readers; comparison permits the whitespace/HTML
// escaping applied by the outer JSON encoder, never replacing the target bytes.
type ProfileDelivery struct {
	TargetPath   string            `json:"target_path"`
	ProfileBytes []byte            `json:"profile_bytes"`
	Package      *delivery.Package `json:"package"`
}

func (p *BundlePayload) ExactProfileBytes() ([]byte, error) {
	if p == nil || len(p.Envelope) == 0 || len(p.Envelope) > MaxBundleSize {
		return nil, ErrBadFormat
	}
	if p.Delivery == nil {
		return bytes.Clone(p.Envelope), nil
	}
	d := p.Delivery
	if len(d.ProfileBytes) == 0 || len(d.ProfileBytes) > delivery.MaxProfileBytes || d.Package == nil {
		return nil, ErrBadFormat
	}
	original, originalErr := json.Marshal(json.RawMessage(d.ProfileBytes))
	carried, carriedErr := json.Marshal(p.Envelope)
	if originalErr != nil || carriedErr != nil || !bytes.Equal(original, carried) {
		return nil, errors.New("onboarding: exact delivered bytes differ from bundled envelope")
	}
	return bytes.Clone(d.ProfileBytes), nil
}
