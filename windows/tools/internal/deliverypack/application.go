package deliverypack

import (
	"errors"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
)

// ApplicationFacts are owned parsed values, not an acceptance token. Publisher
// authoring and persisted-candidate verification share the same local-role gate;
// installed-client continuity/credentials/lifecycle checks remain separate.
type ApplicationFacts struct {
	KeyID   string
	Profile *metadata.Envelope
	Update  *update.Manifest
}

func CheckApplicationArtifact(kind delivery.Kind, target string, raw []byte, policy *metadata.TrustPolicy, now time.Time) (*ApplicationFacts, error) {
	if err := (delivery.Request{Kind: kind, Path: target, Bytes: raw}).Validate(); err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, metadata.ErrTrustRole
	}
	switch kind {
	case delivery.Profile:
		envelope, err := metadata.DecodeStrict(raw)
		if err != nil {
			return nil, err
		}
		if err = envelope.Verify(policy.VerificationKeys(metadata.RoleProfile)); err != nil {
			return nil, err
		}
		if err = envelope.ValidateCandidate(now); err != nil {
			return nil, err
		}
		if envelope.ProfileEpoch != 0 && !strings.HasPrefix(target, metadata.ProfileEpochNamespace(envelope.ProfileEpoch)) {
			return nil, errors.New("delivery-pack: epoch profile target outside its namespace")
		}
		return &ApplicationFacts{KeyID: envelope.KeyID, Profile: envelope}, nil
	case delivery.UpdateManifest:
		manifest, err := update.DecodeStrict(raw)
		if err != nil {
			return nil, err
		}
		if err = manifest.Verify(policy.VerificationKeys(metadata.RoleUpdate)); err != nil {
			return nil, err
		}
		if manifest.ReleasedAt.After(now.Add(metadata.MaxClockSkew)) {
			return nil, errors.New("delivery-pack: release time is ahead of local time")
		}
		return &ApplicationFacts{KeyID: manifest.KeyID, Update: manifest}, nil
	default:
		return nil, delivery.ErrTrust
	}
}
