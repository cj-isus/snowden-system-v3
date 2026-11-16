package deliverypack

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

// Run accepts an explicit public plan and separately approved root hash.
// No CLI private key argument, key generation, file discovery or network path.
// A package is not installation acceptance; the client repeats its own gates.
func Run(ctx context.Context, kind delivery.Kind, args []string) ([]byte, error) {
	if len(args) != 3 || ctx == nil || kind != delivery.Profile && kind != delivery.UpdateManifest && kind != delivery.PinPolicy {
		return nil, errors.New("usage: delivery-pack <public-plan.json> <out-of-band-root-sha256> <new-output.json>")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	prepared, err := PreparePublication(ctx, kind, args[0], args[1])
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	report, err := json.MarshalIndent(prepared.Summary(), "", "  ")
	if err != nil {
		return nil, err
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return prepared.Check(time.Now().UTC())
	}
	if err := publisherfiles.PublishNewChecked(args[2], prepared.ClientPackage(), check); err != nil {
		return nil, err
	}
	return report, nil
}

func (p *assembled) check(kind delivery.Kind, target string, now time.Time) error {
	if !now.Before(p.report.IssuerRootExpiresAt) {
		return errors.New("delivery-pack: issuer bootstrap root expired")
	}
	if err := p.proof.Check(kind, target, p.artifact, now); err != nil {
		return err
	}
	var envelope *metadata.Envelope
	if kind == delivery.Profile {
		var err error
		envelope, err = metadata.DecodeStrict(p.artifact)
		if err != nil {
			return err
		}
		if err := envelope.ValidateCandidate(now); err != nil {
			return err
		}
	}
	if p.grantProof != nil {
		if err := p.grantProof.Check(delivery.ProfileEpoch, delivery.EpochPolicyPath, p.grantRaw, now); err != nil {
			return err
		}
		grant, err := metadata.DecodeEpochGrant(p.grantRaw)
		if err != nil {
			return err
		}
		if err = grant.Validate(now); err != nil {
			return err
		}
	}
	if p.pinPolicy != nil {
		var err error
		if kind == delivery.PinPolicy {
			err = p.pinPolicy.Check(p.pinAuthority, now)
		} else {
			err = p.pinPolicy.CheckArtifact(p.pinAuthority, p.proof, kind, target, p.artifact, now)
		}
		if err != nil {
			return err
		}
		retained, err := p.pinPolicy.Retain(p.pinAuthority, nil, now)
		if err != nil {
			return err
		}
		if envelope != nil {
			return retained.CheckEnvelope(envelope)
		}
	} else if kind == delivery.PinPolicy {
		return delivery.ErrTrust
	}
	return nil
}
