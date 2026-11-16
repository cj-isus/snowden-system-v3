package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
)

type enrolledExportSource struct {
	*profileExportSource
}

type preparedEnrolledExport struct {
	source       *enrolledExportSource
	progress     *delivery.Progress
	payload      *onboarding.BundlePayload // rebased public payload, no credential values
	portable     *bundleDelivery           // private fresh-origin profile/grant proofs
	portableRoot string
	preview      *OnboardingPreview
}

func publicSHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Caller owns operationMu and local journal serialization, never a.mu or a
// native scope. No vault or secret values are touched by public source capture.
func readEnrolledExportSource(store *metadata.Store) (*enrolledExportSource, error) {
	source, err := captureProfileExportSource(store, nil)
	if err != nil {
		return nil, err
	}
	if source.policy.Delivery == nil || !source.state.HasCheckpoint() {
		return nil, errors.New("onboarding: экспорт доставки требует принятый enrolled профиль")
	}
	return &enrolledExportSource{profileExportSource: source}, nil
}

func (a *App) captureEnrolledExportSource() (*enrolledExportSource, error) {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	return readEnrolledExportSource(store)
}

// Verify fresh delivery against existing source trust without changing its
// epoch/origin/pins. Rebase first-enrollment authority only to exact SDK-trusted
// next-root bytes; an ignored input root of the same version is not authority.
func (a *App) prepareEnrolledExport(ctx context.Context, payload *onboarding.BundlePayload, expected *OnboardingPreview, checkpoint string) (*preparedEnrolledExport, *enrolledExportSource, *delivery.Progress, error) {
	if ctx == nil || payload == nil || payload.Delivery == nil || len(payload.Recovery) != 0 || len(payload.Secrets) != 0 || len(payload.TrustedKeys) != 1 {
		return nil, nil, nil, errors.New("onboarding: нужен публичный пакет издателя без recovery permit и секретов")
	}
	if len(payload.PinPolicy) == 0 {
		return nil, nil, nil, errors.New("onboarding: перенос на новый ПК требует свежий пакет издателя со списком отзывов для независимого подтверждения")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	source, err := a.captureEnrolledExportSource()
	if err != nil {
		return nil, nil, nil, err
	}
	if expected != nil && (source.policySHA != expected.SourceTrustSHA256 || source.checkpoint != checkpoint) {
		return nil, source, nil, errors.New("onboarding: контекст экспорта изменился после превью")
	}
	raw, err := payload.ExactProfileBytes()
	if err != nil {
		return nil, source, nil, err
	}
	env, err := metadata.DecodeStrict(raw)
	if err != nil {
		return nil, source, nil, err
	}
	installed, err := metadata.DecodeStrict(source.raw)
	if err != nil || env.ProfileVersion() != source.state.ProfileVersion() || !bytes.Equal(installed.Canonical(), env.Canonical()) {
		return nil, source, nil, errors.New("onboarding: свежий пакет должен содержать именно установленный профиль")
	}
	keys := source.policy.VerificationKeys(metadata.RoleProfile)
	if err = env.Verify(keys); err != nil {
		return nil, source, nil, err
	}
	key := payload.TrustedKeys[0]
	if key.KeyID != env.KeyID || key.Public != hex.EncodeToString(keys[env.KeyID]) || key.Comment != "" {
		return nil, source, nil, errors.New("onboarding: пакет переносит только ключ текущего издателя профиля")
	}
	now, err := a.trustClock().Now(deliveryClockFloor(source.policy, source.state))
	if err != nil {
		return nil, source, nil, err
	}
	if err = env.ValidateVersion(now, source.state.ProfileVersion()); err != nil {
		return nil, source, nil, err
	}
	delivered, verifyErr := prepareBundleDelivery(ctx, payload, source.policy, "", raw, now)
	var progress *delivery.Progress
	if delivered != nil {
		progress = delivered.progress
	}
	if verifyErr != nil {
		return nil, source, progress, verifyErr
	}
	if err = delivered.check(source.policy, raw, now); err != nil {
		return nil, source, progress, err
	}
	if env.ProfileEpoch == 0 && len(payload.EpochGrant) != 0 || env.ProfileEpoch != 0 && len(payload.EpochGrant) == 0 {
		return nil, source, progress, errors.New("onboarding: для нового устройства нужен grant именно текущей эпохи")
	}
	if len(payload.EpochGrant) != 0 {
		grant, err := metadata.CheckEpochProofs(source.policy, progress.ProofFor(delivery.Profile), delivered.path, raw, progress.ProofFor(delivery.ProfileEpoch), payload.EpochGrant, now)
		if err != nil {
			return nil, source, progress, err
		}
		if grant.ToEpoch != env.ProfileEpoch || grant.ProfilePublic != key.Public {
			return nil, source, progress, errors.New("onboarding: grant не соответствует текущей эпохе и издателю")
		}
	}
	// Store acceptance checks installed history/pin continuity under fresh source
	// fences, but commits neither profile changes nor new epoch authority.
	err = func() error {
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		unlock := filetxn.Serialize(source.store.ImportJournal())
		defer unlock()
		return source.withCurrentSource(func() error {
			accepted, err := source.store.PrepareDeliveredAcceptance(env, keys, progress.ProofFor(delivery.Profile), delivered.path, raw, now)
			if err != nil {
				return err
			}
			checking := source.policy.Scoped()
			checking.Delivery = progress.NextState()
			_, err = delivered.applyPinPolicy(checking, accepted, env, raw, now)
			return err // preparation only: source pin history remains unchanged
		})
	}()
	if err != nil {
		return nil, source, progress, err
	}
	portable, portableProof, rootSHA, err := rebaseExportProfile(ctx, payload, progress, now)
	if err != nil {
		return nil, source, progress, err
	}
	channels, err := render.DescriptorsFromEnvelopeAt(env, now)
	if err != nil {
		return nil, source, progress, err
	}
	contentSHA, err := env.ContentSHA256()
	if err != nil {
		return nil, source, progress, err
	}
	preview := &OnboardingPreview{
		CreatedAt: payload.CreatedAt.Format(time.RFC3339),
		Channels:  len(channels), SplitDirect: len(env.SplitDirect),
		Keys:      []OnboardingKeyPreview{{KeyID: env.KeyID, Fingerprint: onboarding.KeyFingerprint(key.Public)}},
		BundleVer: env.MetadataVersion, CurrentVer: source.state.Version,
		BundleIdentity: env.ProfileVersion().String(), CurrentIdentity: source.state.ProfileVersion().String(),
		AuthorityMode: "installed_profile_export", SignerKeyID: env.KeyID,
		SignerFingerprint: onboarding.KeyFingerprint(key.Public), TargetSHA256: contentSHA,
		DeliveryAnchorSHA256: source.policy.Delivery.AnchorSHA256,
		DeliveryRootVersion:  portableProof.progress.ProofFor(delivery.Profile).RootVersion(),
		SourceTrustSHA256:    source.policySHA,
	}
	preview.PinPolicyRequired = portableProof.progress.ProofFor(delivery.Profile).RequiresPinPolicy()
	if source.state.PinPolicy != nil {
		preview.CurrentPinPolicySHA256 = source.state.PinPolicy.SHA256
	}
	deadline, err := profileImportDeadline(delivered, source.policy, env, raw, nil, now)
	if err != nil {
		return nil, source, progress, err
	}
	portablePolicy := source.policy.Scoped()
	portablePolicy.Delivery = portableProof.source.Clone()
	portableDeadline, err := profileImportDeadline(portableProof, portablePolicy, env, raw, nil, now)
	if err != nil {
		return nil, source, progress, err
	}
	deadline = earliestImportDeadline(deadline, portableDeadline)
	if len(payload.EpochGrant) != 0 {
		preview.EpochGrantSHA256 = publicSHA(payload.EpochGrant)
	}
	if len(payload.PinPolicy) != 0 {
		pin, err := metadata.DecodePinPolicy(payload.PinPolicy)
		if err != nil {
			return nil, source, progress, err
		}
		preview.PinPolicyVersion, preview.PinPolicySHA256 = fmt.Sprint(pin.Version), publicSHA(payload.PinPolicy)
		preview.PinPolicyValidUntil = pin.ExpiresAt.UTC().Format(time.RFC3339)
	}
	preview.ValidUntil = deadline.UTC().Format(time.RFC3339Nano)
	preview.DeliveryValidUntil = preview.ValidUntil
	preview.BundleEpoch, preview.CurrentEpoch = env.ProfileEpoch, source.state.ProfileEpoch
	if expected != nil && !samePublicProfileApproval(preview, expected) {
		return nil, source, progress, errors.New("onboarding: профиль экспорта изменился после подтверждения")
	}
	if err = ctx.Err(); err != nil {
		return nil, source, progress, err
	}
	return &preparedEnrolledExport{source: source, progress: progress, payload: portable, portable: portableProof, portableRoot: rootSHA, preview: preview}, source, progress, nil
}

func rebaseExportProfile(ctx context.Context, payload *onboarding.BundlePayload, verified *delivery.Progress, now time.Time) (*onboarding.BundlePayload, *bundleDelivery, string, error) {
	if verified == nil || verified.ProofFor(delivery.Profile) == nil {
		return nil, nil, "", delivery.ErrTrust
	}
	next := verified.NextState()
	if next == nil {
		return nil, nil, "", delivery.ErrTrust
	}
	root := next.Root // actual private SDK result, never input root.json
	rootSHA := publicSHA(root)
	initial, err := delivery.Enroll(root, rootSHA, now)
	if err != nil {
		return nil, nil, "", err
	}
	records := make(map[string][]byte, len(payload.Delivery.Package.Metadata))
	for name, data := range payload.Delivery.Package.Metadata {
		if name == "root.json" || strings.HasSuffix(name, ".root.json") {
			continue // a fresh destination starts exactly here, not unverified old roots
		}
		records[name] = bytes.Clone(data)
	}
	records["root.json"] = bytes.Clone(root)
	pack, err := delivery.NewPackage(records)
	if err != nil {
		return nil, nil, "", err
	}
	raw, err := payload.ExactProfileBytes()
	if err != nil {
		return nil, nil, "", err
	}
	copy := *payload
	copy.Recovery, copy.DeviceName, copy.Secrets = nil, "", []onboarding.SecretSlot{}
	copy.TrustedKeys = append([]onboarding.TrustedKey(nil), payload.TrustedKeys...)
	copy.Envelope, copy.EpochGrant = bytes.Clone(payload.Envelope), bytes.Clone(payload.EpochGrant)
	copy.PinPolicy = bytes.Clone(payload.PinPolicy)
	copy.Delivery = &onboarding.ProfileDelivery{TargetPath: payload.Delivery.TargetPath, ProfileBytes: bytes.Clone(raw), Package: pack}
	checking := &metadata.TrustPolicy{Schema: 2, Keys: []metadata.TrustKey{{KeyID: copy.TrustedKeys[0].KeyID, Public: copy.TrustedKeys[0].Public, Role: metadata.RoleProfile}}, Delivery: initial}
	proof, err := prepareBundleDelivery(ctx, &copy, checking, "", raw, now)
	if err != nil {
		return nil, nil, "", err
	}
	if err = proof.check(checking, raw, now); err != nil {
		return nil, nil, "", err
	}
	if _, err = onboarding.EncodePublicProfile(&copy); err != nil {
		return nil, nil, "", fmt.Errorf("onboarding: portable public package: %w", err)
	}
	return &copy, proof, rootSHA, nil
}

func (p *preparedEnrolledExport) checkFresh(now time.Time) error {
	if p == nil || p.source == nil || p.payload == nil || p.progress == nil || p.portable == nil {
		return delivery.ErrTrust
	}
	raw, err := p.payload.ExactProfileBytes()
	if err != nil {
		return err
	}
	env, err := metadata.DecodeStrict(raw)
	if err != nil {
		return err
	}
	if err = env.Verify(p.source.policy.VerificationKeys(metadata.RoleProfile)); err != nil {
		return err
	}
	if err = env.ValidateVersion(now, p.source.state.ProfileVersion()); err != nil {
		return err
	}
	if err = metadata.CheckDeliveryProof(p.source.policy, p.progress.ProofFor(delivery.Profile), delivery.Profile, p.payload.Delivery.TargetPath, raw, now); err != nil {
		return err
	}
	if len(p.payload.EpochGrant) != 0 {
		if _, err = metadata.CheckEpochProofs(p.source.policy, p.progress.ProofFor(delivery.Profile), p.payload.Delivery.TargetPath, raw, p.progress.ProofFor(delivery.ProfileEpoch), p.payload.EpochGrant, now); err != nil {
			return err
		}
	}
	checking := &metadata.TrustPolicy{Schema: 2, Keys: []metadata.TrustKey{{KeyID: env.KeyID, Public: p.payload.TrustedKeys[0].Public, Role: metadata.RoleProfile}}, Delivery: p.portable.source.Clone()}
	return p.portable.check(checking, raw, now)
}
