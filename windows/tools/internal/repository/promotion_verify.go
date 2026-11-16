package repository

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/tools/internal/deliverypack"
)

// promotionProgress separates a durably retainable SDK prefix from a complete
// publication. An initial root is not persisted on incomplete artifact import.
// The state writer fences sourceDigest before retaining next.
type promotionProgress struct {
	sourceDigest string
	next         *delivery.State
	accepted     *verifiedRelease
	rootSource   []byte            // exact enrolled/retained root before SDK advancement
	rootRecords  map[string][]byte // untrusted until forward continuity verification
}

type verifiedRelease struct {
	candidate    *releaseCandidate
	sourceSHA256 string
	policy       *metadata.TrustPolicy
	proofs       []promotionProof
	pinPolicy    *metadata.RetainedPinPolicy
}

type promotionProof struct {
	kind     delivery.Kind
	target   string
	raw      []byte
	profile  *delivery.Prepared
	grant    *delivery.Prepared
	grantRaw []byte
	pin      *metadata.VerifiedPinPolicy
	source   *delivery.State
}

func verifyRelease(ctx context.Context, raw []byte, candidateApproval string, baseline *delivery.State, initialRootApproval string, policy *metadata.TrustPolicy, previousPin *metadata.RetainedPinPolicy, now time.Time) (result *promotionProgress, finalErr error) {
	if ctx == nil {
		return nil, errors.New("repository: operation context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := metadata.EncodeTrustPolicy(policy); err != nil {
		return nil, err
	}
	policy = policy.Scoped() // owned role authority; delivery proof source supplied separately
	policy.Delivery = nil
	wire, err := decodeCandidate(raw, candidateApproval)
	if err != nil {
		return nil, err
	}
	c := restoreCandidate(wire)
	pinRaw, err := restorePinPolicy(c, wire)
	if err != nil {
		return nil, err
	}
	root, _, err := metadataRecord("root.json", c.records["root.json"])
	if err != nil {
		return nil, err
	}
	if err = root.fresh(now); err != nil {
		return nil, err
	}
	c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, root.expires())
	state := baseline
	if state == nil {
		if initialRootApproval == "" {
			return nil, errors.New("repository: independent first-root approval required")
		}
		state, err = delivery.Enroll(c.records["root.json"], initialRootApproval, now)
		if err != nil {
			return nil, err
		}
	} else {
		if initialRootApproval != "" {
			return nil, errors.New("repository: retained publisher trust cannot be re-enrolled")
		}
		if err = state.Validate(); err != nil {
			return nil, err
		}
		if err = metadata.CheckClockFloor(now, state.ClockFloor); err != nil {
			return nil, err
		}
		if now.Before(state.ClockFloor) {
			now = state.ClockFloor
		}
		state = state.Clone()
	}
	pack, err := delivery.NewPackage(c.records)
	if err != nil {
		return nil, err
	}
	sourceDigest, err := state.Digest()
	if err != nil {
		return nil, err
	}
	result = &promotionProgress{sourceDigest: sourceDigest, next: state.Clone(), rootSource: bytes.Clone(state.Root), rootRecords: c.records}
	defer func() {
		if finalErr != nil && baseline == nil {
			result = nil
		}
	}()
	verified := &verifiedRelease{candidate: c, sourceSHA256: candidateApproval, policy: policy}
	for _, artifact := range wire.Manifest.Artifacts {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		data, err := restoreTransfer(c, wire, artifact)
		if err != nil {
			return result, err
		}
		if existing := c.artifacts[artifact.Target].raw; existing != nil && !(artifact.Kind == delivery.PinPolicy && bytes.Equal(existing, data)) {
			return result, errors.New("repository: duplicate decoded target")
		}
		c.artifacts[artifact.Target] = releaseArtifact{kind: artifact.Kind, raw: data}
		requests := []delivery.Request{{Kind: artifact.Kind, Path: artifact.Target, Bytes: data}}
		grantRaw := c.transferGrants[artifact.Target] // do not impose another profile's migration intent
		if artifact.Kind == delivery.Profile && len(grantRaw) != 0 {
			requests = append(requests, delivery.Request{Kind: delivery.ProfileEpoch, Path: delivery.EpochPolicyPath, Bytes: grantRaw})
		}
		if len(pinRaw) != 0 && artifact.Kind != delivery.PinPolicy {
			requests = append(requests, delivery.Request{Kind: delivery.PinPolicy, Path: delivery.PinPolicyPath, Bytes: pinRaw})
		}
		before := state.Clone()
		progress, verifyErr := delivery.PrepareArtifacts(ctx, state, pack, requests, now)
		if progress != nil && progress.NextState() != nil {
			expected, digestErr := before.Digest()
			if digestErr != nil || progress.SourceDigest() != expected {
				return result, errors.New("repository: verifier progress source changed")
			}
			state = progress.NextState()
			result.next = state.Clone() // safe prefix can survive a later local-role failure
		}
		if verifyErr != nil {
			return result, verifyErr
		}
		if progress == nil || progress.ProofFor(artifact.Kind) == nil {
			return result, delivery.ErrTrust
		}
		proof := promotionProof{kind: artifact.Kind, target: artifact.Target, raw: data, profile: progress.ProofFor(artifact.Kind), source: before}
		if artifact.Kind == delivery.Profile && len(grantRaw) != 0 {
			proof.grant, proof.grantRaw = progress.ProofFor(delivery.ProfileEpoch), grantRaw
		}
		if len(pinRaw) != 0 {
			checking := policy.Scoped()
			checking.Delivery = before.Clone()
			proof.pin, err = metadata.PreparePinPolicy(checking, progress.ProofFor(delivery.PinPolicy), pinRaw, now)
			if err != nil {
				return result, err
			}
			verified.pinPolicy, err = proof.pin.Retain(checking, previousPin, now)
			if err != nil {
				return result, err
			}
		} else if err = metadata.CheckPinPolicyContinuity(previousPin, nil); err != nil {
			return result, err // keep the verified prefix, never remove accepted bans
		}
		if proof.profile.RootVersion() != wire.Manifest.RootVersion {
			return result, errors.New("repository: candidate current root differs from retained-trust result")
		}
		facts, err := proof.check(policy, now)
		if err != nil {
			return result, err
		}
		if facts.Profile != nil {
			if err = checkTransferSigner(wire.Files[artifact.ClientPackage], facts, policy); err != nil {
				return result, err
			}
			c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, facts.Profile.ExpiresAt)
		}
		c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, proof.profile.ValidUntil())
		if proof.grant != nil {
			c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, proof.grant.ValidUntil())
			grant, err := metadata.DecodeEpochGrant(proof.grantRaw)
			if err != nil {
				return result, err
			}
			c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, grant.ExpiresAt)
		}
		if proof.pin != nil {
			c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, proof.pin.ValidUntil())
		}
		verified.proofs = append(verified.proofs, proof)
	}
	if err = c.buildLayout(now); err != nil {
		return result, err
	}
	if result.next == nil || !bytes.Equal(c.wire.Files["metadata/root.json"], result.next.Root) || c.wire.Manifest.PublishedRootSHA256 != digest(result.next.Root) {
		return result, errors.New("repository: published root bytes differ from retained-trust SDK root")
	}
	if _, err = c.encode(); err != nil {
		return result, err
	}
	expectedManifest, err := json.Marshal(wire.Manifest)
	if err != nil {
		return result, err
	}
	actualManifest, err := json.Marshal(c.wire.Manifest)
	if err != nil {
		return result, err
	}
	if c.wire.ReleaseID != wire.ReleaseID || !bytes.Equal(expectedManifest, actualManifest) || !sameMetadata(c.wire.Files, wire.Files) {
		return result, errors.New("repository: rebuilt release inventory/validity differs from candidate")
	}
	if err = verified.check(ctx, now); err != nil {
		return result, err
	}
	result.accepted = verified
	return result, nil
}

func (p *promotionProof) check(policy *metadata.TrustPolicy, now time.Time) (*deliverypack.ApplicationFacts, error) {
	if p == nil || p.profile == nil || p.source == nil {
		return nil, delivery.ErrTrust
	}
	if err := p.profile.Check(p.kind, p.target, p.raw, now); err != nil {
		return nil, err
	}
	if p.kind == delivery.PinPolicy {
		if p.target != delivery.PinPolicyPath || p.pin == nil || p.grant != nil || len(p.grantRaw) != 0 {
			return nil, delivery.ErrTrust
		}
		checking := policy.Scoped()
		checking.Delivery = p.source.Clone()
		if _, err := p.pin.Retain(checking, nil, now); err != nil {
			return nil, err
		}
		return &deliverypack.ApplicationFacts{}, nil // no application payload or signer accepted
	}
	facts, err := deliverypack.CheckApplicationArtifact(p.kind, p.target, p.raw, policy, now)
	if err != nil {
		return nil, err
	}
	if len(p.grantRaw) != 0 {
		if p.grant == nil {
			return nil, delivery.ErrTrust
		}
		checking := policy.Scoped()
		checking.Delivery = p.source.Clone()
		grant, err := metadata.CheckEpochProofs(checking, p.profile, p.target, p.raw, p.grant, p.grantRaw, now)
		if err != nil {
			return nil, err
		}
		if facts.Profile == nil || facts.Profile.ProfileEpoch != grant.ToEpoch || hex.EncodeToString(policy.VerificationKeys(metadata.RoleProfile)[facts.KeyID]) != grant.ProfilePublic {
			return nil, errors.New("repository: epoch grant/profile authority mismatch")
		}
	}
	if p.pin != nil {
		checking := policy.Scoped()
		checking.Delivery = p.source.Clone()
		if err := p.pin.CheckArtifact(checking, p.profile, p.kind, p.target, p.raw, now); err != nil {
			return nil, err
		}
		retained, err := p.pin.Retain(checking, nil, now)
		if err != nil {
			return nil, err
		}
		if facts.Profile != nil {
			if err = retained.CheckEnvelope(facts.Profile); err != nil {
				return nil, err
			}
		}
	} else if p.kind == delivery.Profile && p.profile.RequiresPinPolicy() {
		return nil, delivery.ErrTrust
	}
	return facts, nil
}

func checkTransferSigner(raw []byte, facts *deliverypack.ApplicationFacts, policy *metadata.TrustPolicy) error {
	payload, err := onboarding.DecodePublicProfile(raw)
	if err != nil {
		return err
	}
	if facts.Profile == nil || len(payload.TrustedKeys) != 1 || !payload.CreatedAt.Equal(facts.Profile.IssuedAt) {
		return errors.New("repository: profile transfer identity changed")
	}
	key := payload.TrustedKeys[0]
	expected := hex.EncodeToString(policy.VerificationKeys(metadata.RoleProfile)[facts.KeyID])
	if key.KeyID != facts.KeyID || key.Public != expected || key.Comment != "" {
		return errors.New("repository: unsigned transfer keys are outside independent role authority")
	}
	return nil
}

func (v *verifiedRelease) check(ctx context.Context, now time.Time) error {
	if v == nil || v.candidate == nil || ctx == nil || len(v.proofs) == 0 {
		return delivery.ErrTrust
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !now.Before(v.candidate.wire.Manifest.ValidUntil) {
		return errors.New("repository: verified release expired before promotion")
	}
	for _, proof := range v.proofs {
		if _, err := proof.check(v.policy, now); err != nil {
			return err
		}
	}
	return nil
}
