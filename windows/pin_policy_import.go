package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
)

// Approval is a source/intent fence, never signing or enrollment authority.
// The exact public capsule must still pass the already enrolled SDK verifier.
type PinPolicyApproval struct {
	SourceTrustSHA256      string `json:"source_trust_sha256"`
	SourceCheckpointSHA256 string `json:"source_checkpoint_sha256"`
	CapsuleSHA256          string `json:"capsule_sha256"`
	PolicySHA256           string `json:"policy_sha256"`
}

type PinPolicyPreview struct {
	Approval           PinPolicyApproval `json:"approval"`
	ProfileIdentity    string            `json:"profile_identity"`
	PolicyVersion      string            `json:"policy_version"`
	AppliedVersion     string            `json:"applied_version,omitempty"`
	AppliedSHA256      string            `json:"applied_sha256,omitempty"`
	ValidUntil         string            `json:"valid_until"`
	RootVersion        string            `json:"root_version"`
	WillRetireSession  bool              `json:"will_retire_session"`
	Committed          bool              `json:"committed"`
	RequiresFreshStart bool              `json:"requires_fresh_start"`
}

func (a *App) PreviewPinPolicy(capsuleJSON string) (*PinPolicyPreview, error) {
	return a.importPinPolicy(capsuleJSON, PinPolicyApproval{}, false)
}

func (a *App) ApplyPinPolicy(capsuleJSON string, approval PinPolicyApproval, confirm bool) (*PinPolicyPreview, error) {
	if !confirm {
		return nil, errors.New("pin policy: подтвердите применение списка отзывов после превью")
	}
	return a.importPinPolicy(capsuleJSON, approval, true)
}

// This operation accepts denials independently of old profile validity. It
// never loads/decrypts credential values, replaces signed bytes, enrolls a root,
// assigns a profile signer or changes profile epoch/counter/history.
func (a *App) importPinPolicy(capsuleJSON string, approval PinPolicyApproval, commit bool) (result *PinPolicyPreview, finalErr error) {
	return a.importPinPolicyUntil(capsuleJSON, approval, commit, time.Time{})
}

func (a *App) importPinPolicyUntil(capsuleJSON string, approval PinPolicyApproval, commit bool, approvedUntil time.Time) (result *PinPolicyPreview, finalErr error) {
	return a.importPinPolicyContext(nil, capsuleJSON, approval, commit, approvedUntil)
}

func (a *App) importPinPolicyContext(requestContext context.Context, capsuleJSON string, approval PinPolicyApproval, commit bool, approvedUntil time.Time) (result *PinPolicyPreview, finalErr error) {
	if len(capsuleJSON) == 0 || len(capsuleJSON) > onboarding.MaxPinPolicyCapsuleSize {
		return nil, onboarding.ErrTooLarge
	}
	base, requestScope, err := a.captureProfileRequest(requestContext)
	if err != nil {
		return nil, err
	}
	capsule, err := onboarding.DecodePinPolicyCapsule([]byte(capsuleJSON))
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
	vault, committer := a.vault, a.onboardingCommit
	operationID, pendingTransition := a.operationID, a.profileTransition
	a.mu.Unlock()
	if a.closing.Load() || commit && vault == nil {
		return nil, errors.New("pin policy: application/vault unavailable")
	}
	if !requestScope.current() {
		return nil, context.Canceled
	}
	clock := a.trustClock() // no App mutex acquisition inside journal/vault scope
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	locked := true
	releaseJournal := func() {
		if locked {
			unlock()
			locked = false
		}
	}
	defer releaseJournal()
	var transitionID uint64
	prefixRetained := false
	policyCommitted := false
	defer func() {
		if recovered := recover(); recovered != nil {
			releaseJournal()
			result = nil
			finalErr = fmt.Errorf("pin policy: interrupted (%T)", recovered)
		}
		if finalErr != nil && policyCommitted {
			finalErr = errors.Join(errors.New("pin policy: policy committed; subsequent restart/publication did not finish"), finalErr)
		} else if finalErr != nil && prefixRetained {
			finalErr = errors.Join(errors.New("pin policy: verified delivery prefix retained; policy activation did not finish"), finalErr)
		}
		if errors.Is(finalErr, errProfileStorageQuarantine) {
			releaseJournal()
			transitionID, finalErr = a.retireProfileStorageFailure(operationID, transitionID, finalErr)
		}
		if finalErr != nil && transitionID != 0 {
			releaseJournal()
			a.finishProfileTransition(transitionID, false, finalErr)
		}
	}()
	source, err := captureProfileImportSource(store)
	if err != nil {
		return nil, err
	}
	policy, state := source.policy, source.state
	if policy.Delivery == nil || !state.HasCheckpoint() {
		return nil, errors.New("pin policy: an already enrolled installation and intact checkpoint are required")
	}
	sourceTrust, err := policy.Digest()
	if err != nil {
		return nil, err
	}
	expected := PinPolicyApproval{sourceTrust, publicSHA(source.checkpoint), publicSHA([]byte(capsuleJSON)), publicSHA(capsule.Policy)}
	if commit && approval != expected {
		return nil, errors.New("pin policy: source or capsule changed; obtain a new preview")
	}
	now, err := clock.Now(deliveryClockFloor(policy, state))
	if err != nil {
		return nil, err
	}
	if !approvedUntil.IsZero() && !now.Before(approvedUntil) {
		return nil, errors.New("pin policy: срок подтверждённого превью истёк")
	}
	releaseJournal() // SDK uses the owned source, not a held profile scope
	ctx, cancel := context.WithTimeout(base, 30*time.Second)
	progress, verifyErr := core.PrepareDeliveryArtifacts(ctx, policy.Delivery, capsule.Package, []delivery.Request{{Kind: delivery.PinPolicy, Path: delivery.PinPolicyPath, Bytes: capsule.Policy}}, now)
	cancel()
	unlock = filetxn.Serialize(store.ImportJournal())
	locked = true
	willRetire := pendingTransition
	if progress != nil {
		willRetire = willRetire || metadata.CheckAppliedPinPolicy(progress.NextState(), state.PinPolicy) != nil
	}
	var vaultPath string
	if commit && progress != nil {
		var retirementErr error
		if willRetire {
			releaseJournal()
			transitionID, retirementErr = a.retireProfileRequest(requestScope)
			unlock = filetxn.Serialize(store.ImportJournal())
			locked = true
		}
		if err = vault.RecoverImport(store.ImportJournal(), func(path string) error { vaultPath = path; return filetxn.Check(store.ImportJournal()) }); err != nil {
			return nil, errors.Join(verifyErr, retirementErr, err)
		}
		policy, prefixRetained, err = retainDeliveryProgressWithOutcome(store, policy, state, progress, vault, vaultPath, committer)
		if err != nil {
			return nil, errors.Join(verifyErr, retirementErr, err)
		}
		if err := source.expectPolicy(policy); err != nil {
			return nil, errors.Join(verifyErr, retirementErr, err)
		}
		if retirementErr != nil {
			return nil, errors.Join(verifyErr, retirementErr)
		}
	}
	if verifyErr != nil {
		return nil, verifyErr
	}
	if progress == nil || progress.ProofFor(delivery.PinPolicy) == nil {
		return nil, delivery.ErrTrust
	}
	var verified *metadata.VerifiedPinPolicy
	var accepted *metadata.AcceptedProfileState
	checkFresh := func() error {
		if !requestScope.current() {
			return context.Canceled
		}
		fresh, err := clock.Now(deliveryClockFloor(policy, state))
		if err != nil {
			return err
		}
		if !approvedUntil.IsZero() && !fresh.Before(approvedUntil) {
			return errors.New("pin policy: срок подтверждённого превью истёк до принятия")
		}
		if err := verified.Check(policy, fresh); err != nil {
			return err
		}
		if !requestScope.current() {
			return context.Canceled
		}
		return nil
	}
	err = source.withCurrentSource(func() error {
		now, err = clock.Now(deliveryClockFloor(policy, state))
		if err != nil {
			return err
		}
		verified, err = metadata.PreparePinPolicy(policy, progress.ProofFor(delivery.PinPolicy), capsule.Policy, now)
		if err != nil {
			return err
		}
		accepted, err = verified.Apply(policy, state, now)
		if err != nil {
			return err
		}
		if err := metadata.CheckAppliedPinPolicy(progress.NextState(), accepted.PinPolicy); err != nil {
			return err
		}
		return checkFresh()
	})
	if err != nil {
		return nil, err
	}
	preview := &PinPolicyPreview{Approval: expected, ProfileIdentity: state.ProfileVersion().String(), PolicyVersion: fmt.Sprint(accepted.PinPolicy.Version), ValidUntil: verified.ValidUntil().UTC().Format(time.RFC3339), RootVersion: fmt.Sprint(progress.ProofFor(delivery.PinPolicy).RootVersion()), WillRetireSession: willRetire, RequiresFreshStart: willRetire}
	if state.PinPolicy != nil {
		preview.AppliedVersion, preview.AppliedSHA256 = fmt.Sprint(state.PinPolicy.Version), state.PinPolicy.SHA256
	}
	if !commit {
		return preview, nil
	}
	// Prefix activation cannot change state. Preserve the exact whole source
	// after that separate transaction, and fence again after synced staging.
	releaseJournal()
	prepared, err := vault.PrepareImportSnapshot(nil) // encrypted snapshot only
	if err != nil {
		return nil, err
	}
	files := importTargets(store, vaultPath)
	for i := range files {
		files[i].Keep = true
	}
	files[2].Data, err = metadata.EncodeAcceptedProfile(accepted)
	files[2].Keep = false
	if err != nil {
		return nil, err
	}
	fenceInScope := func() error {
		if err := source.checkInScope(); err != nil {
			return err
		}
		return checkFresh()
	}
	unlock = filetxn.Serialize(store.ImportJournal())
	locked = true
	if err = filetxn.WithRead(store.ImportJournal(), fenceInScope); err != nil {
		return nil, err
	}
	tx := &filetxn.Store{Dir: store.ImportJournal(), BeforeCommit: fenceInScope}
	err = vault.WithPreparedTransaction(prepared, tx, func() error {
		var commitErr error
		if committer != nil {
			commitErr = committer(tx, files)
		} else {
			commitErr = tx.Commit(files)
		}
		policyCommitted = commitErr == nil
		return commitErr
	})
	err = errors.Join(err, profileStorageQuarantineError(tx))
	if err != nil {
		return nil, err
	}
	preview.Committed = true
	preview.AppliedVersion, preview.AppliedSHA256 = preview.PolicyVersion, accepted.PinPolicy.SHA256
	releaseJournal()
	if transitionID != 0 {
		if launch := a.finishProfileTransition(transitionID, true, nil); launch != nil {
			launch() // ordinary fresh runtime projection and protected probe, no old-session rollback
		}
	}
	return preview, nil
}
