package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
)

type verifiedUpdate struct {
	manifest   *update.Manifest
	raw        []byte
	sidecar    []byte // exact owned public delivery package, never journal authority
	proof      *delivery.Prepared
	targetPath string
	policy     []byte
	schema     int
	checkpoint []byte
}

// App and offline publishers use one strict application manifest decoder.
func decodeUpdateManifest(raw []byte) (*update.Manifest, error) {
	return update.DecodeStrict(raw)
}

// Caller owns operationMu. Capture App-owned values before the journal; no
// App/runtime/Manager mutex is held across public I/O, TUF or vault retention.
func (a *App) prepareVerifiedUpdate(updateDir string, retain bool) (result *verifiedUpdate, finalErr error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	return a.prepareVerifiedUpdateContext(ctx, updateDir, retain)
}

// Preview and apply keep their original registered context. Neither path revives
// missing/canceled lifetime on Background.
func (a *App) prepareVerifiedUpdateContext(appContext context.Context, updateDir string, retain bool) (result *verifiedUpdate, finalErr error) {
	return a.prepareVerifiedUpdateContextApproved(appContext, updateDir, retain, nil)
}

func (a *App) prepareVerifiedUpdateContextApproved(appContext context.Context, updateDir string, retain bool, approval *updatePreviewApproval) (result *verifiedUpdate, finalErr error) {
	if !lifecyclePublicationCurrent(appContext) {
		return nil, context.Canceled
	}
	clock := a.trustClock()
	a.mu.Lock()
	vaultManager, committer, operationID := a.vault, a.onboardingCommit, a.operationID
	a.mu.Unlock()
	prefixRetained := false
	defer func() {
		if finalErr != nil && prefixRetained {
			finalErr = errors.Join(errors.New("update: проверенная история доставки уже сохранена; применение обновления не завершено"), finalErr)
		}
		if errors.Is(finalErr, errProfileStorageQuarantine) {
			transition, cause := a.retireProfileStorageFailure(operationID, 0, finalErr)
			finalErr = cause
			if transition != 0 {
				a.finishProfileTransition(transition, false, finalErr)
			}
		}
	}() // retention's local journal/vault scopes have already been released
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	source, err := func() (*metadata.TrustSnapshot, error) {
		unlock := filetxn.Serialize(store.ImportJournal())
		defer unlock()
		return store.InspectTrustSnapshot()
	}()
	if err != nil {
		return nil, err
	}
	if !source.PolicyPresent() {
		return nil, errors.New("update: local update authority unavailable")
	}
	policy, state := source.TrustPolicy(), source.AcceptedProfile()
	now, err := clock.Now(deliveryClockFloor(policy, state))
	if err != nil {
		return nil, err
	}
	raw, err := readDeliveryFile(filepath.Join(updateDir, "update.json"), delivery.MaxManifestBytes)
	if err != nil {
		return nil, err
	}
	if approval != nil && !approval.matchesManifest(raw) {
		return nil, errUpdateApproval // before delivery SDK/history retention for another target
	}
	verified := &verifiedUpdate{raw: raw}
	sidecar := filepath.Join(updateDir, "delivery.json")
	if policy.Delivery == nil {
		if _, err := os.Lstat(sidecar); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("update: delivery sidecar cannot enroll root; use explicit profile enrollment first")
		}
		if approval != nil && !approval.matchesSidecar(nil) {
			return nil, errUpdateApproval
		}
	} else {
		data, err := readDeliveryFile(sidecar, delivery.MaxEncodedPackageBytes+512)
		if err != nil {
			return nil, errors.Join(errors.New("update: enrolled delivery requires readable delivery.json"), err)
		}
		if approval != nil && !approval.matchesSidecar(data) {
			return nil, errUpdateApproval // exact public package before prepare or prefix writes
		}
		pack, err := delivery.DecodeTargetPackage(data)
		if err != nil {
			return nil, err
		}
		verified.sidecar = bytes.Clone(data)
		ctx, cancel := context.WithTimeout(appContext, 30*time.Second)
		progress, deliveryErr := core.PrepareDeliveryArtifacts(ctx, policy.Delivery, pack.Package, []delivery.Request{{Kind: delivery.UpdateManifest, Path: pack.TargetPath, Bytes: raw}}, now)
		cancel()
		if retain && progress != nil {
			if vaultManager == nil {
				return nil, errors.New("update: delivery prefix vault owner unavailable")
			}
			policy, err = func() (*metadata.TrustPolicy, error) {
				unlock := filetxn.Serialize(store.ImportJournal())
				defer unlock()
				var vaultPath string
				if err := vaultManager.RecoverImport(store.ImportJournal(), func(path string) error { vaultPath = path; return filetxn.Check(store.ImportJournal()) }); err != nil {
					return nil, err
				}
				next, committed, err := retainDeliveryProgressWithOutcome(store, policy, state, progress, vaultManager, vaultPath, committer)
				prefixRetained = committed
				return next, err
			}()
			if err != nil {
				return nil, errors.Join(deliveryErr, err)
			}
		}
		if deliveryErr != nil {
			return nil, deliveryErr
		}
		if progress == nil {
			return nil, delivery.ErrTrust
		}
		verified.proof, verified.targetPath = progress.Proof(), pack.TargetPath
	}
	manifest, err := decodeUpdateManifest(raw)
	if err != nil {
		return nil, err
	}
	verified.policy, err = metadata.EncodeTrustPolicy(policy)
	if err != nil {
		return nil, err
	}
	verified.checkpoint, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	verified.manifest, verified.schema = manifest, policy.Schema
	// Verification and prefix retention ran outside the native read scope.
	// Do not publish preparation from a superseded or uncertain trust source.
	err = func() error {
		unlock := filetxn.Serialize(store.ImportJournal())
		defer unlock()
		return verified.withTrustSource(store, func(policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState) error {
			if a.closing.Load() {
				return context.Canceled
			}
			if !lifecyclePublicationCurrent(appContext) {
				return context.Canceled
			}
			return verified.checkFresh(policy, state, clock)
		})
	}()
	if err != nil {
		return nil, err
	}
	return verified, nil
}

// Caller owns local Serialize. Source remains protected from cooperative
// profile writers throughout use, including a separate installer mutation.
func (v *verifiedUpdate) withTrustSource(store *metadata.Store, use func(*metadata.TrustPolicy, *metadata.AcceptedProfileState) error) error {
	if v == nil || v.manifest == nil || store == nil || use == nil {
		return metadata.ErrTrustRole
	}
	return store.WithTrustSnapshot(func(source *metadata.TrustSnapshot) error {
		policy, state := source.TrustPolicy(), source.AcceptedProfile()
		encoded, err := metadata.EncodeTrustPolicy(policy)
		if err != nil {
			return err
		}
		checkpoint, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if !source.PolicyPresent() || policy.Schema != v.schema || !bytes.Equal(encoded, v.policy) || !bytes.Equal(checkpoint, v.checkpoint) {
			return errors.New("update: prepared authority/checkpoint superseded; retry")
		}
		return use(policy, state)
	})
}

func (v *verifiedUpdate) checkFresh(policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState, clock *metadata.ObservedClock) error {
	if err := v.manifest.Validate(); err != nil {
		return err
	}
	if err := v.manifest.Verify(policy.VerificationKeys(metadata.RoleUpdate)); err != nil {
		return err
	}
	// Take the last local observation after structure/signature and floor I/O.
	now, err := clock.Now(deliveryClockFloor(policy, state))
	if err != nil {
		return err
	}
	if policy.Delivery != nil {
		if err := metadata.CheckDeliveryProof(policy, v.proof, delivery.UpdateManifest, v.targetPath, v.raw, now); err != nil {
			return err
		}
	}
	if v.manifest.ReleasedAt.After(now.Add(metadata.MaxClockSkew)) {
		return errors.New("update: release time is ahead of observed local time")
	}
	return nil
}

// Only the short filesystem swap belongs in apply. No App/lifecycle callback
// under journal serialization. This fences local profile/authority writes and
// rechecks freshness; it does not prove installer/Windows lifecycle correctness.
func (a *App) withVerifiedUpdateSource(verified *verifiedUpdate, apply func() error) error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	return a.withVerifiedUpdateSourceLocked(verified, apply)
}

// Caller owns operationMu, including any committed restart handoff. The
// callback remains filesystem-only: no App/lifecycle calls under the journal.
func (a *App) withVerifiedUpdateSourceLocked(verified *verifiedUpdate, apply func() error) error {
	return a.withVerifiedUpdatePurposeLocked(verified, false, apply)
}

func (a *App) withVerifiedUpdatePurposeLocked(verified *verifiedUpdate, activating bool, apply func() error) error {
	purpose := updateInstallPurpose
	if activating {
		purpose = updateCurrentCodePurpose
	}
	return a.withVerifiedUpdateModeLocked(verified, purpose, apply)
}

type updateApplyPurpose uint8

const (
	updateInstallPurpose updateApplyPurpose = iota
	updateCurrentCodePurpose
	updateForwardRecoveryPurpose
)

func (a *App) withVerifiedUpdateModeLocked(verified *verifiedUpdate, purpose updateApplyPurpose, apply func() error) error {
	if verified == nil || apply == nil {
		return metadata.ErrTrustRole
	}
	clock := a.trustClock()
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	return verified.withTrustSource(store, func(policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState) error {
		if a.closing.Load() {
			return context.Canceled
		}
		// Floor reads can stall. Sample freshness only after this purpose gate.
		switch purpose {
		case updateCurrentCodePurpose:
			if err := checkInstalledUpdateVersion(verified.manifest); err != nil {
				return err
			}
		case updateInstallPurpose:
			if err := checkVersionPolicy(verified.manifest); err != nil {
				return err
			}
		case updateForwardRecoveryPurpose:
			if err := checkForwardRecoveryVersion(verified.manifest); err != nil {
				return err
			}
		default:
			return update.ErrPairRecoveryRequired
		}
		if err := verified.checkFresh(policy, state, clock); err != nil {
			return err
		}
		if a.closing.Load() {
			return context.Canceled
		}
		return apply()
	})
}
