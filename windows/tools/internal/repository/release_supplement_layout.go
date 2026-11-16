package repository

import (
	"bytes"
	"errors"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/onboarding"
)

// Rebuild public transports and their release inventory. The signed targets,
// exact profile_bytes, epoch grants and application signer hints stay owned and
// unchanged; the complete manifest-derived release identity is recomputed.
func supplementCandidate(wire *releaseWire, roots map[string][]byte) (*releaseCandidate, error) {
	c := restoreCandidate(wire)
	for name, raw := range roots {
		if previous := c.records[name]; previous != nil && !bytes.Equal(previous, raw) {
			return nil, errors.New("repository: retained root conflicts with candidate metadata")
		}
		c.records[name] = bytes.Clone(raw)
	}
	pack, err := delivery.NewPackage(c.records)
	if err != nil {
		return nil, err
	}
	if _, err = restorePinPolicy(c, wire); err != nil {
		return nil, err
	}
	updated := *wire
	updated.Files = make(map[string][]byte, len(wire.Files))
	for name, raw := range wire.Files {
		updated.Files[name] = raw // immutable owned input bytes, never mutated
	}
	for _, artifact := range wire.Manifest.Artifacts {
		if artifact.Kind == delivery.Profile {
			payload, err := onboarding.DecodePublicProfile(wire.Files[artifact.ClientPackage])
			if err != nil || payload.Delivery == nil {
				return nil, delivery.ErrTrust
			}
			original, err := payload.ExactProfileBytes()
			if err != nil || digest(original) != artifact.SHA256 {
				return nil, delivery.ErrTrust
			}
			payload.Delivery.Package = pack
			raw, err := onboarding.EncodePublicProfile(payload)
			if err != nil {
				return nil, err
			}
			updated.Files[artifact.ClientPackage] = raw
		} else if artifact.Kind == delivery.PinPolicy {
			data, err := candidateArtifact(wire, artifact)
			if err != nil {
				return nil, err
			}
			raw, err := onboarding.EncodePinPolicyCapsule(data, pack)
			if err != nil {
				return nil, err
			}
			updated.Files[artifact.ClientPackage] = raw
		} else {
			data, err := candidateArtifact(wire, artifact)
			if err != nil {
				return nil, err
			}
			transfer := &delivery.TargetPackage{TargetPath: artifact.Target, Package: pack}
			raw, err := transfer.Encode(artifact.Kind, data)
			if err != nil {
				return nil, err
			}
			updated.Files[artifact.ClientPackage] = raw
		}
		data, err := restoreTransfer(c, &updated, artifact)
		if err != nil {
			return nil, err
		}
		if existing := c.artifacts[artifact.Target].raw; existing != nil && !(artifact.Kind == delivery.PinPolicy && bytes.Equal(existing, data)) {
			return nil, errors.New("repository: duplicate supplemented target")
		}
		c.artifacts[artifact.Target] = releaseArtifact{kind: artifact.Kind, raw: data}
	}
	return c, nil // caller repeats complete SDK/local-role/layout verification
}
