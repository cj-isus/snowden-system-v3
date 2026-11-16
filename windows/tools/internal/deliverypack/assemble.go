package deliverypack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/update"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

type RecordSummary struct {
	Name   string `json:"name"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// Report describes this issuer snapshot only, never installed-client rollback,
// credentials, actual protected traffic, payload EXE or current OS acceptance.
type Report struct {
	Schema                   int                      `json:"schema"`
	Kind                     delivery.Kind            `json:"kind"`
	TargetPath               string                   `json:"target_path"`
	ArtifactSHA256           string                   `json:"artifact_sha256"`
	ArtifactBytes            int                      `json:"artifact_bytes"`
	ApprovedIssuerRootSHA256 string                   `json:"approved_issuer_root_sha256"`
	IssuerRootExpiresAt      time.Time                `json:"issuer_root_expires_at"`
	RootVersion              int64                    `json:"root_version"`
	DeliveryValidUntil       time.Time                `json:"delivery_valid_until"`
	IssuerObservedAt         time.Time                `json:"issuer_observed_at"`
	SignerKeyID              string                   `json:"signer_key_id"`
	Metadata                 []RecordSummary          `json:"metadata"`
	OutputBytes              int                      `json:"output_bytes"`
	RequiresClientAcceptance bool                     `json:"requires_client_acceptance"`
	ProfileVersion           *metadata.ProfileVersion `json:"profile_version,omitempty"`
	ProfileContentSHA256     string                   `json:"profile_content_sha256,omitempty"`
	EpochGrantSHA256         string                   `json:"epoch_grant_sha256,omitempty"`
	PinPolicySHA256          string                   `json:"pin_policy_sha256,omitempty"`
	PinPolicyVersion         string                   `json:"pin_policy_version,omitempty"`
	PinPolicyRequired        bool                     `json:"pin_policy_required"`
	PinPolicyBytes           int                      `json:"pin_policy_bytes"`
}

type assembled struct {
	records      map[string][]byte
	data         []byte
	artifact     []byte
	proof        *delivery.Prepared
	grantProof   *delivery.Prepared
	grantRaw     []byte
	pinRaw       []byte
	pinPolicy    *metadata.VerifiedPinPolicy
	pinAuthority *metadata.TrustPolicy
	report       Report
}

func assemble(ctx context.Context, plan *Plan, root, approvedRoot string, now time.Time) (*assembled, error) {
	records := map[string][]byte{}
	remaining := delivery.MaxPackageBytes
	for _, name := range sortedNames(plan.Metadata) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		limit := min(delivery.MaxMetadataBytes, remaining)
		if limit <= 0 {
			return nil, delivery.ErrBudget
		}
		data, err := readRelative(root, plan.Metadata[name], int64(limit))
		if err != nil {
			return nil, errors.New("delivery-pack: bounded metadata input unavailable")
		}
		remaining -= len(data)
		records[name] = data
	}
	pack, err := delivery.NewPackage(records)
	if err != nil {
		return nil, err
	}
	state, err := delivery.Enroll(pack.Metadata["root.json"], approvedRoot, now)
	if err != nil {
		return nil, err
	}
	issuerRoot, err := tuf.Root().FromBytes(state.Root)
	if err != nil {
		return nil, err
	}
	limit := int64(delivery.MaxProfileBytes)
	if plan.Kind == delivery.UpdateManifest {
		limit = min(update.MaxManifestSize, delivery.MaxManifestBytes)
	} else if plan.Kind == delivery.PinPolicy {
		limit = delivery.MaxPinPolicyBytes
	}
	raw, err := readRelative(root, plan.Artifact, limit)
	if err != nil {
		return nil, errors.New("delivery-pack: bounded artifact unavailable")
	}
	requests := []delivery.Request{{Kind: plan.Kind, Path: plan.Target, Bytes: raw}}
	var grantRaw []byte
	if plan.EpochGrant != "" {
		grantRaw, err = readRelative(root, plan.EpochGrant, delivery.MaxEpochGrantBytes)
		if err != nil {
			return nil, errors.New("delivery-pack: public epoch grant unavailable")
		}
		requests = append(requests, delivery.Request{Kind: delivery.ProfileEpoch, Path: delivery.EpochPolicyPath, Bytes: grantRaw})
	}
	var pinRaw []byte
	if plan.PinPolicy != "" {
		pinRaw, err = readRelative(root, plan.PinPolicy, delivery.MaxPinPolicyBytes)
		if err != nil {
			return nil, errors.New("delivery-pack: public pin policy unavailable")
		}
		requests = append(requests, delivery.Request{Kind: delivery.PinPolicy, Path: delivery.PinPolicyPath, Bytes: pinRaw})
	}
	progress, err := delivery.PrepareArtifacts(ctx, state, pack, requests, now)
	if err != nil {
		// No client trust state exists here to advance. A partial prefix cannot
		// be published as artifact proof; the client retains its own progress.
		return nil, err
	}
	proof := progress.Proof()
	if err := proof.Check(plan.Kind, plan.Target, raw, now); err != nil {
		return nil, err
	}
	policyBytes, err := readRelative(root, plan.Authority, metadata.MaxTrustPolicySize)
	if err != nil {
		return nil, errors.New("delivery-pack: public application authority unavailable")
	}
	policy, err := metadata.DecodeTrustPolicy(policyBytes)
	if err != nil {
		return nil, err
	}
	if plan.Kind == delivery.PinPolicy {
		return assemblePolicyOnly(plan, records, raw, pack, state, proof, policy, approvedRoot, issuerRoot.Signed.Expires, now)
	}
	facts, err := CheckApplicationArtifact(plan.Kind, plan.Target, raw, policy, now)
	if err != nil {
		return nil, err
	}
	out := &assembled{records: records, artifact: raw, proof: proof, report: Report{
		Schema: 1, Kind: plan.Kind, TargetPath: plan.Target, ArtifactSHA256: digest(raw), ArtifactBytes: len(raw),
		ApprovedIssuerRootSHA256: approvedRoot, RootVersion: proof.RootVersion(), DeliveryValidUntil: proof.ValidUntil(),
		IssuerObservedAt:         now,
		RequiresClientAcceptance: true, Metadata: []RecordSummary{},
	}}
	out.report.IssuerRootExpiresAt = issuerRoot.Signed.Expires.UTC()
	out.report.PinPolicyRequired = plan.Kind == delivery.Profile && proof.RequiresPinPolicy()
	if len(pinRaw) != 0 {
		checking := policy.Scoped()
		checking.Delivery = state.Clone()
		pin, err := metadata.PreparePinPolicy(checking, progress.ProofFor(delivery.PinPolicy), pinRaw, now)
		if err != nil {
			return nil, err
		}
		if err = pin.CheckArtifact(checking, proof, plan.Kind, plan.Target, raw, now); err != nil {
			return nil, err
		}
		retained, err := pin.Retain(checking, nil, now) // issuer has no installed device history
		if err != nil {
			return nil, err
		}
		if facts.Profile != nil {
			if err = retained.CheckEnvelope(facts.Profile); err != nil {
				return nil, err
			}
		}
		out.pinRaw, out.pinPolicy, out.pinAuthority = pinRaw, pin, checking
		out.report.PinPolicyBytes = len(pinRaw)
		out.report.PinPolicySHA256, out.report.PinPolicyVersion = retained.SHA256, fmt.Sprint(retained.Version)
		if pin.ValidUntil().Before(out.report.DeliveryValidUntil) {
			out.report.DeliveryValidUntil = pin.ValidUntil()
		}
	}
	if out.report.IssuerRootExpiresAt.Before(out.report.DeliveryValidUntil) {
		out.report.DeliveryValidUntil = out.report.IssuerRootExpiresAt
	}
	switch plan.Kind {
	case delivery.Profile:
		envelope := facts.Profile
		keys := policy.VerificationKeys(metadata.RoleProfile)
		if envelope.ExpiresAt.Before(out.report.DeliveryValidUntil) {
			out.report.DeliveryValidUntil = envelope.ExpiresAt
		}
		if len(grantRaw) != 0 {
			checking := policy.Scoped()
			checking.Delivery = state.Clone() // issuer baseline, not installed device state
			grantProof := progress.ProofFor(delivery.ProfileEpoch)
			grant, err := metadata.CheckEpochProofs(checking, proof, plan.Target, raw, grantProof, grantRaw, now)
			if err != nil {
				return nil, err
			}
			if envelope.ProfileEpoch != grant.ToEpoch || hex.EncodeToString(keys[envelope.KeyID]) != grant.ProfilePublic {
				return nil, errors.New("delivery-pack: epoch grant does not authorize this profile/signer")
			}
			out.grantProof, out.grantRaw = grantProof, grantRaw
			out.report.EpochGrantSHA256 = digest(grantRaw)
			if grantProof.ValidUntil().Before(out.report.DeliveryValidUntil) {
				out.report.DeliveryValidUntil = grantProof.ValidUntil()
			}
			if grant.ExpiresAt.Before(out.report.DeliveryValidUntil) {
				out.report.DeliveryValidUntil = grant.ExpiresAt
			}
		}
		identity := envelope.ProfileVersion()
		out.report.ProfileVersion = &identity
		out.report.ProfileContentSHA256, err = envelope.ContentSHA256()
		if err != nil {
			return nil, err
		}
		payload := &onboarding.BundlePayload{
			Schema: 1, CreatedAt: envelope.IssuedAt,
			TrustedKeys: []onboarding.TrustedKey{{KeyID: envelope.KeyID, Public: hex.EncodeToString(keys[envelope.KeyID])}},
			Envelope:    json.RawMessage(raw), Secrets: []onboarding.SecretSlot{},
			Delivery:   &onboarding.ProfileDelivery{TargetPath: plan.Target, ProfileBytes: raw, Package: pack},
			EpochGrant: grantRaw,
			PinPolicy:  pinRaw,
		}
		out.data, err = onboarding.EncodePublicProfile(payload)
		if err != nil {
			return nil, err
		}
		out.report.SignerKeyID = envelope.KeyID
	case delivery.UpdateManifest:
		manifest := facts.Update
		out.data, err = (&delivery.TargetPackage{TargetPath: plan.Target, Package: pack}).Encode(delivery.UpdateManifest, raw)
		if err != nil {
			return nil, err
		}
		out.report.SignerKeyID = manifest.KeyID
	default:
		return nil, delivery.ErrTrust
	}
	for _, name := range sortedNames(records) {
		data := records[name]
		out.report.Metadata = append(out.report.Metadata, RecordSummary{Name: name, Bytes: len(data), SHA256: digest(data)})
	}
	out.report.OutputBytes = len(out.data)
	return out, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
