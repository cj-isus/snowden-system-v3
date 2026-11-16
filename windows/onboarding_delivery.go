package main

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

type bundleDelivery struct {
	source   *delivery.State
	progress *delivery.Progress
	path     string
	grantRaw []byte
	pinRaw   []byte
	enroll   bool
}

// Public verification only. Neither a package nor a self-signed root is an
// enrollment decision: the separate API must supply the out-of-band hash.
func prepareBundleDelivery(ctx context.Context, payload *onboarding.BundlePayload, policy *metadata.TrustPolicy, approvedRoot string, raw []byte, now time.Time) (*bundleDelivery, error) {
	if payload.Delivery == nil {
		if policy.Delivery != nil || approvedRoot != "" || len(payload.PinPolicy) != 0 {
			return nil, errors.New("onboarding: enrolled delivery requires a proof package")
		}
		return nil, nil
	}
	state := policy.Delivery
	enroll := state == nil
	if enroll {
		if approvedRoot == "" || payload.Delivery.Package == nil {
			return nil, errors.New("onboarding: root enrollment requires a separate explicit out-of-band approval")
		}
		var err error
		state, err = delivery.EnrollmentSource(payload.Delivery.Package.Metadata["root.json"], approvedRoot, now)
		if err != nil {
			return nil, err
		}
	} else if approvedRoot != "" {
		return nil, errors.New("onboarding: root already enrolled; rotation must follow the verified root chain")
	}
	path := payload.Delivery.TargetPath
	requests := []delivery.Request{{Kind: delivery.Profile, Path: path, Bytes: raw}}
	if len(payload.EpochGrant) != 0 {
		// Verification does not authorize enrollment/recovery. Application
		// intent and clean-source/independent approval gates run separately.
		requests = append(requests, delivery.Request{Kind: delivery.ProfileEpoch, Path: delivery.EpochPolicyPath, Bytes: payload.EpochGrant})
	}
	if len(payload.PinPolicy) != 0 {
		requests = append(requests, delivery.Request{Kind: delivery.PinPolicy, Path: delivery.PinPolicyPath, Bytes: payload.PinPolicy})
	}
	progress, err := core.PrepareDeliveryArtifacts(ctx, state, payload.Delivery.Package, requests, now)
	return &bundleDelivery{source: state, progress: progress, path: path, grantRaw: bytes.Clone(payload.EpochGrant), pinRaw: bytes.Clone(payload.PinPolicy), enroll: enroll}, err
}

func (d *bundleDelivery) check(policy *metadata.TrustPolicy, raw []byte, now time.Time) error {
	_, err := d.validUntil(policy, raw, now)
	return err
}

// validUntil verifies the same complete private batch as check and reports
// every proof/body deadline. It does not infer authority from unsigned dates.
func (d *bundleDelivery) validUntil(policy *metadata.TrustPolicy, raw []byte, now time.Time) (time.Time, error) {
	if policy == nil {
		return time.Time{}, delivery.ErrTrust
	}
	if d == nil {
		if policy.Delivery != nil {
			return time.Time{}, delivery.ErrTrust
		}
		return time.Time{}, nil // legacy profile without a delivery proof
	}
	if d.source == nil || d.progress == nil || d.progress.ProofFor(delivery.Profile) == nil {
		return time.Time{}, delivery.ErrTrust
	}
	checking := policy
	if d.enroll {
		checking = policy.Scoped()
		checking.Delivery = d.source.Clone()
	}
	if err := metadata.CheckDeliveryProof(checking, d.progress.ProofFor(delivery.Profile), delivery.Profile, d.path, raw, now); err != nil {
		return time.Time{}, err
	}
	deadline := d.progress.ProofFor(delivery.Profile).ValidUntil()
	if d.enroll {
		root, err := tuf.Root().FromBytes(d.source.Root)
		if err != nil {
			return time.Time{}, err
		}
		// A first-enrollment bootstrap root must remain fresh. An already
		// retained old root may still authorize its ordinary forward rotation.
		deadline = earliestImportDeadline(deadline, root.Signed.Expires)
	}
	if len(d.grantRaw) != 0 {
		grant, err := metadata.CheckEpochProofs(checking, d.progress.ProofFor(delivery.Profile), d.path, raw, d.progress.ProofFor(delivery.ProfileEpoch), d.grantRaw, now)
		if err != nil {
			return time.Time{}, err
		}
		deadline = earliestImportDeadline(deadline, d.progress.ProofFor(delivery.ProfileEpoch).ValidUntil())
		deadline = earliestImportDeadline(deadline, grant.ExpiresAt)
	}
	pin, err := d.verifiedPinPolicy(checking, raw, now)
	if err != nil {
		return time.Time{}, err
	}
	if pin != nil {
		deadline = earliestImportDeadline(deadline, pin.ValidUntil())
	}
	if deadline.IsZero() || !now.Before(deadline) {
		return time.Time{}, errImportPreviewExpired
	}
	return deadline.UTC(), nil
}

func (d *bundleDelivery) verifiedPinPolicy(policy *metadata.TrustPolicy, raw []byte, now time.Time) (*metadata.VerifiedPinPolicy, error) {
	if d == nil || d.progress == nil || d.progress.ProofFor(delivery.Profile) == nil {
		return nil, delivery.ErrTrust
	}
	profile := d.progress.ProofFor(delivery.Profile)
	if len(d.pinRaw) == 0 {
		if profile.RequiresPinPolicy() {
			return nil, delivery.ErrTrust
		}
		return nil, nil
	}
	verified, err := metadata.PreparePinPolicy(policy, d.progress.ProofFor(delivery.PinPolicy), d.pinRaw, now)
	if err != nil {
		return nil, err
	}
	if err = verified.CheckProfile(policy, profile, d.path, raw, now); err != nil {
		return nil, err
	}
	return verified, nil
}

func (d *bundleDelivery) applyPinPolicy(policy *metadata.TrustPolicy, accepted *metadata.AcceptedProfileState, envelope *metadata.Envelope, raw []byte, now time.Time) (*metadata.AcceptedProfileState, error) {
	if d == nil {
		return accepted, nil
	}
	verified, err := d.verifiedPinPolicy(policy, raw, now)
	if err != nil {
		return nil, err
	}
	if verified == nil {
		return accepted, nil
	}
	return verified.ApplyProfile(policy, accepted, d.progress.ProofFor(delivery.Profile), d.path, raw, envelope, now)
}

// Root-only legacy entry points cannot express an independent policy baseline.
// Keep their names for old bindings, but fail before KDF/SDK or any mutation.
func (a *App) PreviewDeliveryEnrollment(transport, passphrase, rootSHA256 string) (*OnboardingPreview, error) {
	return nil, errors.New("onboarding: обновите клиент и выполните первую установку с независимой сверкой SHA256 root.json и списка отзывов")
}

func (a *App) ApplyDeliveryEnrollment(transport, passphrase, rootSHA256 string, confirm bool) (*OnboardingPreview, error) {
	return nil, errors.New("onboarding: обновите клиент и подтвердите оба отпечатка — root.json и списка отзывов — после нового превью первой установки")
}
