package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

type importCommitFn func(*filetxn.Store, []filetxn.File) error

func importTargets(store *metadata.Store, vaultPath string) []filetxn.File {
	return []filetxn.File{{Name: "keys", Path: store.KeysPath()}, {Name: "envelope", Path: store.EnvelopePath()}, {Name: "version", Path: filepath.Join(store.Dir, "version.txt")}, {Name: "vault", Path: vaultPath}}
}

// Recovery precedes Seed/migration and any startup profile consumers. An
// incomplete/corrupt journal stays present and blocks metadata and vault reads.
func (a *App) recoverOnboardingImport() error {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	if a.vault == nil {
		return errors.New("onboarding: vault unavailable")
	}
	return a.vault.RecoverImport(store.ImportJournal(), func(path string) error {
		tx := &filetxn.Store{Dir: store.ImportJournal()}
		err := tx.Recover(importTargets(store, path))
		return errors.Join(err, profileStorageQuarantineError(tx))
	})
}

func readImportFloor(dir string) (uint64, error) {
	state, err := metadata.LoadAcceptedProfile(dir)
	if err != nil {
		return 0, err
	}
	if state.ProfileEpoch != 0 {
		return 0, metadata.ErrProfileEpoch // legacy scalar reader cannot discard the epoch
	}
	return state.Version, nil
}

// Both entrypoints share exactly the same validation path. Decryption is
// outside operationMu; lifecycle is serialized and staged source fences detect
// intervening vault/public-state writes. Preview performs no disk recovery/write.
func (a *App) importOnboarding(transport, passphrase string, commit bool) (result *OnboardingPreview, finalErr error) {
	return a.importProfileBundle(transport, passphrase, commit, false, "")
}

// Recovery is a distinct user intent. Ordinary profile import never interprets
// a bundled key or permit as authority to replace installed publisher trust.
func (a *App) importProfileBundle(transport, passphrase string, commit, recovery bool, approvedTarget string) (result *OnboardingPreview, finalErr error) {
	return a.importProfileBundleIntent(transport, passphrase, profileImportIntent{commit: commit, recovery: recovery, approvedTarget: approvedTarget})
}

func (a *App) importProfileBundleIntent(transport, passphrase string, intent profileImportIntent) (result *OnboardingPreview, finalErr error) {
	ctx, _, err := a.captureProfileRequest(intent.requestContext)
	if err != nil {
		return nil, err
	}
	intent.requestContext = ctx
	payload, err := onboarding.Open(transport, passphrase)
	if err != nil {
		return nil, err
	}
	return a.importPreparedProfilePayload(payload, intent)
}

func (a *App) importPreparedProfilePayload(payload *onboarding.BundlePayload, intent profileImportIntent) (result *OnboardingPreview, finalErr error) {
	if payload == nil {
		return nil, onboarding.ErrBadFormat
	}
	commit, recovery := intent.commit, intent.recovery
	appContext, requestScope, err := a.captureProfileRequest(intent.requestContext)
	if err != nil {
		return nil, err
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	if !requestScope.currentLocked(a) {
		a.mu.Unlock()
		return nil, context.Canceled
	}
	if a.vault == nil {
		a.mu.Unlock()
		return nil, errors.New("onboarding: vault unavailable")
	}
	vaultManager := a.vault
	committer := a.onboardingCommit
	operationID := a.operationID
	pendingTransition := a.profileTransition
	a.mu.Unlock()
	if !requestScope.current() {
		return nil, context.Canceled
	}
	clock := a.trustClock() // capture before filetxn serialization; no App lock inside clock sampling
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	lockHeld := true
	releaseImport := func() {
		if lockHeld {
			unlock()
			lockHeld = false
		}
	}
	defer releaseImport()
	var transitionID uint64
	profileCommitted, prefixRetained := false, false
	defer func() {
		if recovered := recover(); recovered != nil {
			releaseImport()
			result = nil
			finalErr = fmt.Errorf("onboarding: import interrupted by panic (%T)", recovered)
		}
		if finalErr != nil && profileCommitted {
			finalErr = errors.Join(errors.New("onboarding: изменения профиля уже сохранены; последующее завершение операции не подтверждено"), finalErr)
		} else if finalErr != nil && prefixRetained {
			finalErr = errors.Join(errors.New("onboarding: проверенная история доставки уже сохранена; применение профиля не завершено"), finalErr)
		}
		if errors.Is(finalErr, errProfileStorageQuarantine) {
			releaseImport()
			transitionID, finalErr = a.retireProfileStorageFailure(operationID, transitionID, finalErr)
		}
		if finalErr != nil && transitionID != 0 {
			releaseImport() // lifecycle publication must not take App locks under the journal
			a.finishProfileTransition(transitionID, false, finalErr)
		}
	}()
	var vaultPath string
	if err := vaultManager.RecoverImport(store.ImportJournal(), func(path string) error { vaultPath = path; return filetxn.Check(store.ImportJournal()) }); err != nil {
		return nil, err
	}
	source, err := captureProfileImportSource(store)
	if err != nil {
		return nil, err
	}
	policy, retained := source.policy, source.state
	current := retained.Version
	sourceTrustDigest, err := policy.Digest()
	if err != nil {
		return nil, err
	}
	if intent.expectedPreview != nil && sourceTrustDigest != intent.expectedPreview.SourceTrustSHA256 {
		return nil, errors.New("onboarding: доверие изменилось после превью; проверьте профиль заново")
	}
	if err = checkBootstrapPinApproval(policy, payload, intent); err != nil {
		return nil, err
	}
	rawProfile, err := payload.ExactProfileBytes()
	if err != nil {
		return nil, err
	}
	now, err := clock.Now(deliveryClockFloor(policy, retained))
	if err != nil {
		return nil, err
	}
	if err = checkImportApprovalDeadline(now, intent.approvedUntil); err != nil {
		return nil, err
	}
	releaseImport() // SDK verification uses the owned source, no profile scope
	ctx, cancelDelivery := context.WithTimeout(appContext, 30*time.Second)
	delivered, deliveryErr := prepareBundleDelivery(ctx, payload, policy, intent.approvedRoot, rawProfile, now)
	cancelDelivery()
	unlock = filetxn.Serialize(store.ImportJournal())
	lockHeld = true
	if commit && delivered != nil && !delivered.enroll && delivered.progress != nil {
		var retirementErr error
		if metadata.CheckAppliedPinPolicy(delivered.progress.NextState(), retained.PinPolicy) != nil {
			// The authenticated target hash itself creates a durable obligation.
			// Retire before retaining that prefix, even if artifact validation failed.
			releaseImport()
			transitionID, retirementErr = a.retireProfileRequest(requestScope)
			unlock = filetxn.Serialize(store.ImportJournal())
			lockHeld = true
		}
		policy, prefixRetained, err = retainDeliveryProgressWithOutcome(store, policy, retained, delivered.progress, vaultManager, vaultPath, committer)
		if retirementErr != nil {
			return nil, errors.Join(deliveryErr, retirementErr, err)
		}
		if err != nil {
			return nil, errors.Join(deliveryErr, fmt.Errorf("onboarding: delivery prefix retention failed: %w", err))
		}
		if err := source.expectPolicy(policy); err != nil {
			return nil, err
		}
	}
	if deliveryErr != nil {
		return nil, deliveryErr
	}
	env, err := metadata.DecodeStrict(rawProfile)
	if err != nil {
		return nil, err
	}
	var authority *bundleAuthority
	profileChanged := true // explicit recovery does not consult the damaged old file
	err = source.withCurrentSource(func() error {
		if !requestScope.current() {
			return context.Canceled
		}
		if !recovery && !intent.epochRecovery {
			if err := source.bindEnvelopeInScope(); err != nil {
				return err
			}
		}
		now, err = clock.Now(deliveryClockFloor(policy, retained))
		if err != nil {
			return err
		}
		if err := checkImportApprovalDeadline(now, intent.approvedUntil); err != nil {
			return err
		}
		authority, err = (bundleAuthorityInput{
			store: store, payload: payload, envelope: env, retained: retained, policy: policy, now: now,
			intent: intent, delivered: delivered, rawProfile: rawProfile,
		}).prepare()
		if err != nil {
			return err
		}
		if source.envelopeBound && source.envelopePresent {
			installed, err := metadata.DecodeStrict(source.envelope)
			if err != nil {
				return err
			}
			profileChanged = !bytes.Equal(installed.Canonical(), env.Canonical())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	accepted, nextPolicy := authority.accepted, authority.policy
	if commit {
		accepted, err = metadata.WithClockObservation(accepted, now)
		if err != nil {
			return nil, err
		}
	}
	acceptedBytes, err := metadata.EncodeAcceptedProfile(accepted)
	if err != nil {
		return nil, err
	}
	oldTrustDigest, err := policy.Digest()
	if err != nil {
		return nil, err
	}
	newTrustDigest, err := nextPolicy.Digest()
	if err != nil {
		return nil, err
	}
	trustChanged := oldTrustDigest != newTrustDigest
	pinPolicyChanged := accepted.PinPolicy != nil && (retained.PinPolicy == nil || accepted.PinPolicy.SHA256 != retained.PinPolicy.SHA256)
	channels, err := render.DescriptorsFromEnvelopeAt(env, now)
	if err != nil {
		return nil, err
	}
	deadline, err := profileImportDeadline(delivered, policy, env, rawProfile, payload.Recovery, now)
	if err != nil {
		return nil, err
	}
	preview := &OnboardingPreview{CreatedAt: payload.CreatedAt.Format(time.RFC3339), DeviceName: payload.DeviceName, Channels: len(channels), Secrets: len(payload.Secrets), SplitDirect: len(env.SplitDirect), BundleVer: env.MetadataVersion, CurrentVer: current, Keys: []OnboardingKeyPreview{}, ValidUntil: deadline.Format(time.RFC3339Nano)}
	preview.BootstrapPolicySHA256 = intent.approvedPinPolicy
	preview.AuthorityMode, preview.SignerKeyID, preview.TargetSHA256 = authority.mode, env.KeyID, accepted.EnvelopeSHA256
	preview.BundleEpoch, preview.CurrentEpoch = env.ProfileEpoch, retained.ProfileEpoch
	preview.BundleIdentity, preview.CurrentIdentity = env.ProfileVersion().String(), retained.ProfileVersion().String()
	preview.SourceTrustSHA256 = sourceTrustDigest
	preview.EpochGrantSHA256 = accepted.LastEpochGrantSHA256
	if len(payload.EpochGrant) != 0 {
		preview.EpochGrantSHA256 = publicSHA(payload.EpochGrant) // incoming reference, not a rewritten transition marker
	}
	if retained.PinPolicy != nil {
		preview.CurrentPinPolicySHA256 = retained.PinPolicy.SHA256
	}
	if accepted.PinPolicy != nil {
		preview.PinPolicyVersion = fmt.Sprint(accepted.PinPolicy.Version)
		preview.PinPolicySHA256 = accepted.PinPolicy.SHA256
		preview.PinPolicyValidUntil = accepted.PinPolicy.ExpiresAt.Format(time.RFC3339)
	}
	preview.RetiredPublishers = authority.retiredPublishers
	preview.SignerFingerprint = onboarding.KeyFingerprint(fmt.Sprintf("%x", authority.policy.VerificationKeys(metadata.RoleProfile)[env.KeyID]))
	preview.RecoveryIssuerKeyID = authority.recoveryIssuer
	if nextPolicy.Delivery != nil {
		preview.PinPolicyRequired = delivered.progress.Proof().RequiresPinPolicy()
		preview.DeliveryAnchorSHA256 = nextPolicy.Delivery.AnchorSHA256
		preview.DeliveryRootVersion = delivered.progress.Proof().RootVersion()
		preview.DeliveryValidUntil = deadline.Format(time.RFC3339Nano)
	}
	for _, k := range payload.TrustedKeys {
		preview.Keys = append(preview.Keys, OnboardingKeyPreview{KeyID: k.KeyID, Fingerprint: onboarding.KeyFingerprint(k.Public), Comment: k.Comment})
	}
	if intent.expectedPreview != nil && !samePublicProfileApproval(preview, intent.expectedPreview) {
		return nil, errors.New("onboarding: профиль или контекст подтверждения изменился; требуется новое превью")
	}
	slots := make([]secretvault.Slot, len(payload.Secrets))
	for i, slot := range payload.Secrets {
		slots[i] = secretvault.Slot{ID: slot.ID, Kind: slot.Kind, Value: slot.Value}
	}
	defer clear(slots) // release references, not Go string zeroization
	releaseImport()    // vault credential encryption/validation takes no profile scope
	prepared, err := vaultManager.PrepareImportSnapshot(slots)
	if err != nil {
		return nil, err
	}
	if err := render.ValidateCredentials(channels, prepared); err != nil {
		return nil, err
	}
	// Reuse this fence after lifecycle retirement AND after all four targets
	// have been staged/synced. Caller already holds a native scope. Preview
	// supplies its read-only vault scope; Store.BeforeCommit owns the writer.
	sourceFenceInScope := func() (time.Time, error) {
		if !requestScope.current() {
			return time.Time{}, context.Canceled
		}
		if err := source.checkInScope(); err != nil {
			return time.Time{}, err
		}
		if intent.epochEnrollment {
			if err := store.CheckFreshEpochSource(policy); err != nil {
				return time.Time{}, err
			}
		}
		if err := checkBootstrapPinApproval(policy, payload, intent); err != nil {
			return time.Time{}, err
		}
		freshNow, err := clock.Now(deliveryClockFloor(policy, retained))
		if err != nil {
			return time.Time{}, err
		}
		if err = checkImportApprovalDeadline(freshNow, intent.approvedUntil); err != nil {
			return time.Time{}, err
		}
		if intent.epochRecovery || intent.epochEnrollment {
			err = env.ValidateCandidate(freshNow)
		} else {
			err = env.ValidateVersion(freshNow, retained.ProfileVersion())
		}
		if err == nil {
			err = delivered.check(policy, rawProfile, freshNow)
		}
		if err == nil && recovery {
			permit, permitErr := metadata.DecodeRecoveryPermit(payload.Recovery)
			err = permitErr
			if err == nil {
				err = permit.Validate(freshNow)
			}
		}
		if err == nil && !requestScope.current() {
			err = context.Canceled
		}
		return freshNow, err
	}
	sourceFence := func() (observed time.Time, finalErr error) {
		finalErr = filetxn.WithRead(store.ImportJournal(), func() error {
			var err error
			observed, err = sourceFenceInScope()
			return err
		})
		if finalErr != nil {
			return time.Time{}, finalErr
		}
		return observed, nil
	}
	if !commit {
		unlock = filetxn.Serialize(store.ImportJournal())
		lockHeld = true
		err := vaultManager.WithPreparedReadSnapshot(prepared, store.ImportJournal(), func() error {
			_, err := sourceFenceInScope()
			return err
		})
		releaseImport()
		if err != nil {
			return nil, err
		}
		return preview, nil
	}
	keys, err := metadata.EncodeTrustPolicy(nextPolicy)
	if err != nil {
		return nil, err
	}
	vault, err := prepared.Bytes()
	if err != nil {
		return nil, err
	}
	files := importTargets(store, vaultPath)
	files[0].Data, files[1].Data = keys, bytes.Clone(rawProfile)
	files[2].Data, files[3].Data = acceptedBytes, vault
	// Lifecycle can acquire App locks; do not hold journal/vault locks while
	// retiring. Reacquire and compare the exact prepared public/vault source.
	releaseImport()
	if transitionID == 0 && (profileChanged || trustChanged || pinPolicyChanged || prepared.Changed() || pendingTransition) {
		transitionID, err = a.retireProfileRequest(requestScope)
	}
	if err == nil {
		unlock = filetxn.Serialize(store.ImportJournal())
		lockHeld = true
		var freshNow time.Time
		freshNow, err = sourceFence()
		if err == nil {
			accepted, err = metadata.WithClockObservation(accepted, freshNow)
			if err == nil {
				files[2].Data, err = metadata.EncodeAcceptedProfile(accepted)
			}
		}
		if err == nil {
			tx := &filetxn.Store{Dir: store.ImportJournal(), BeforeCommit: func() error { _, err := sourceFenceInScope(); return err }}
			err = vaultManager.WithPreparedTransaction(prepared, tx, func() error {
				var commitErr error
				if committer != nil {
					commitErr = committer(tx, files)
				} else {
					commitErr = tx.Commit(files)
				}
				profileCommitted = commitErr == nil
				return commitErr
			})
			err = errors.Join(err, profileStorageQuarantineError(tx))
		}
	}
	releaseImport() // error publication/finish may take App locks
	if err != nil {
		return nil, fmt.Errorf("onboarding: import rejected: %w", err)
	}
	if commit {
		preview.CurrentVer = env.MetadataVersion
		preview.CurrentEpoch = env.ProfileEpoch
		preview.CurrentIdentity = env.ProfileVersion().String()
		if accepted.PinPolicy != nil {
			preview.CurrentPinPolicySHA256 = accepted.PinPolicy.SHA256
		}
		a.appendLog("info", fmt.Sprintf("onboarding: импорт применён (каналов: %d, секретов: %d, профиль: %d:%d)", len(channels), len(slots), env.ProfileEpoch, env.MetadataVersion))
		if transitionID != 0 {
			if launch := a.finishProfileTransition(transitionID, true, nil); launch != nil {
				launch()
			}
		}
	}
	return preview, nil
}

func (a *App) PreviewOnboardingString(transport, passphrase string) (*OnboardingPreview, error) {
	return a.importOnboarding(transport, passphrase, false)
}
func (a *App) ApplyOnboardingString(transport, passphrase string, confirm bool) (*OnboardingPreview, error) {
	return a.importOnboarding(transport, passphrase, confirm)
}

func (a *App) PreviewProfileRecovery(transport, passphrase string) (*OnboardingPreview, error) {
	return a.importProfileBundle(transport, passphrase, false, true, "")
}

func (a *App) ApplyProfileRecovery(transport, passphrase, targetSHA256 string, confirm bool) (*OnboardingPreview, error) {
	if !confirm {
		return nil, errors.New("onboarding: восстановление требует явного подтверждения после превью")
	}
	return a.importProfileBundle(transport, passphrase, true, true, targetSHA256)
}
