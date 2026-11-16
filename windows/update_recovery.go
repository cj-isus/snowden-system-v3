package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

var errUpdateRecoveryRestarting = errors.New("update: verified forward recovery transferred to new GUI")

// Forward repair may finish the signed target already equal to the floor, but
// never place code below the floor or the currently running version. An older
// GUI can place a future pair; only that NEW code can accept itself afterwards.
func checkForwardRecoveryVersion(manifest *update.Manifest) error {
	if manifest == nil || manifest.Validate() != nil || manifest.Schema != update.PairSchema {
		return update.ErrPairRecoveryRequired
	}
	current, ce := update.ParseVersion(runningVersion)
	next, ne := update.ParseVersion(manifest.Version)
	floor, fe := update.ReadFloor()
	if ce != nil || ne != nil || fe != nil || next.Compare(current) < 0 || next.Compare(floor) < 0 {
		return errors.Join(update.ErrPairRecoveryRequired, ce, ne, fe)
	}
	if next.Compare(current) == 0 {
		return checkInstalledUpdateVersion(manifest) // equal version also needs this build's helper pin
	}
	return nil
}

// Caller owns updateMu+operationMu and the restricted startup intent. No engine
// or network producer is admitted. This reuses exact candidate/private SDK
// preparation; the backend facade supplies no rollback or acceptance authority.
func (a *App) prepareForwardRecovery(ctx context.Context, intent intentRegistration, rendererEpoch uint64, expectedSHA string) (root string, verified *verifiedUpdate, restart bool, finalErr error) {
	if a.pairRecovery != nil || acceptedStartupImage != nil {
		return "", nil, false, core.ErrHelperCleanup
	}
	root, _, verified, err := a.prepareUpdatePairCandidate(ctx, true, expectedSHA)
	if err != nil {
		return root, nil, false, err
	}
	if err := checkForwardRecoveryVersion(verified.manifest); err != nil {
		return root, nil, false, err
	}
	authorize := func(commit func() error) error {
		if !lifecyclePublicationCurrent(ctx) || !a.activationCurrent(intent, rendererEpoch) {
			return context.Canceled
		}
		if err := a.validatePairRoot(ctx, root); err != nil {
			return err
		}
		if a.pairRecoveryCheck != nil {
			if err := a.validatePairRecoveryReady(ctx); err != nil {
				return err
			}
		}
		if !lifecyclePublicationCurrent(ctx) || !a.activationCurrent(intent, rendererEpoch) {
			return context.Canceled
		}
		return a.withVerifiedUpdateModeLocked(verified, updateForwardRecoveryPurpose, func() error {
			if !lifecyclePublicationCurrent(ctx) {
				return context.Canceled // pure source/path gate, no App callback under journal
			}
			return commit()
		})
	}
	a.pairRecovery, err = update.OpenPairRecovery(ctx, root, verified.raw, verified.sidecar, a.pairRoot)
	if err != nil {
		return root, nil, false, err // keep the exact partial writer/root owner
	}
	if err := a.pairRecovery.Apply(ctx, authorize, a.pairReadiness(root, verified, updateForwardRecoveryPurpose,
		func() bool { return a.activationCurrent(intent, rendererEpoch) })); err != nil {
		return root, nil, false, err // never old-code rollback on public recovery facts
	}
	restart = a.pairRecovery.InstalledNamesTouched()
	if err := a.pairRecovery.Retire(); err != nil {
		return root, nil, false, err
	}
	if err := a.pairRoot.ReleasePayloads(); err != nil {
		return root, nil, false, err // deny-delete prospective image must follow confirmed retirement
	}
	a.pairRecovery = nil
	current, ce := update.ParseVersion(runningVersion)
	next, ne := update.ParseVersion(verified.manifest.Version)
	if ce != nil || ne != nil {
		return root, nil, false, update.ErrPairRecoveryRequired
	}
	restart = restart || next.Compare(current) != 0
	if restart {
		return root, verified, true, nil // root remains held until prospective signed image acquisition
	}
	if err := checkInstalledUpdateVersion(verified.manifest); err != nil {
		return root, nil, false, err
	}
	m := verified.manifest
	err = core.RetainInstalledPair(ctx, &a.pairImage, m.SHA256, m.Size, m.NativeHelper.SHA256, m.NativeHelper.Size)
	if err != nil {
		return root, nil, false, err
	}
	if err := a.closePairRecoveryCheck(); err != nil {
		return root, nil, false, err
	}
	return root, verified, false, nil
}

// Adopt the already-held operationMu into the SAME restart token used by normal
// updates. No second lifecycle/process owner; caller switches its deferred
// operation release to h.release only after this value-only commit succeeds.
func (a *App) commitRecoveryHandoff(intent intentRegistration, rendererEpoch uint64) (*restartHandoff, error) {
	h := &restartHandoff{app: a, rendererEpoch: rendererEpoch}
	accepted := func() bool {
		runtime_.mu.Lock()
		defer runtime_.mu.Unlock()
		a.mu.Lock()
		defer a.mu.Unlock()
		if rendererEpoch == 0 || a.updateGUIRendererEpoch != rendererEpoch || !intent.currentLocked(a) ||
			a.closing.Load() || a.restartCommit != nil || !a.installedImageChanged || a.profileTransition ||
			runtime_.manager != nil || runtime_.engine != nil || runtime_.cc != nil {
			return false
		}
		h.request = stopRequest{intentRegistration: intent, resumeRequested: a.protectionDesired, profileTransition: a.profileTransition}
		a.restartCommit, a.state.State = h, "stopping"
		return true
	}()
	if !accepted {
		return nil, context.Canceled
	}
	return h, nil
}

func (a *App) launchRecoveredUpdate(ctx context.Context, verified *verifiedUpdate, h *restartHandoff) (finalErr error) {
	if a.pairAcceptance != nil || a.pairImage != nil || a.pairRecovery != nil {
		return core.ErrHelperCleanup
	}
	guard := func(ctx context.Context) error {
		if !lifecyclePublicationCurrent(ctx) || !h.codeAcceptanceCurrent() {
			return context.Canceled
		}
		return a.withVerifiedUpdateModeLocked(verified, updateForwardRecoveryPurpose, func() error {
			if !lifecyclePublicationCurrent(ctx) {
				return context.Canceled
			}
			return nil
		})
	}
	defer func() {
		if !a.pairAcceptance.GUITransferred() {
			finalErr = errors.Join(finalErr, a.closeUpdateCodeAcceptance())
		}
	}()
	sum := sha256.Sum256(verified.raw)
	var err error
	err = core.RetainUpdateCodeAcceptance(ctx, &a.pairAcceptance, verified.manifest, hex.EncodeToString(sum[:]), guard)
	if err != nil {
		return err
	}
	if err := a.closePairRecoveryCheck(); err != nil {
		return err // new Main image is pinned; root cannot be released on uncertainty
	}
	if err := a.closePairRoot(); err != nil {
		return err
	}
	if err := a.pairAcceptance.Verify(ctx, guard); err != nil {
		return err // the new code's own SDK/compiled pin/image gate must succeed
	}
	if err := a.pairAcceptance.StartGUI(ctx, tunRequested(), h.request.resumeRequested, guard); err != nil {
		return err
	}
	receipt, err := a.pairAcceptance.CommitGUI(ctx, guard, func(commit core.UpdateGUICommit) error {
		return h.acceptOwnedLaunch(ctx, commit)
	})
	if finishErr := h.finishOwnedLaunch(receipt); finishErr != nil {
		return errors.Join(err, finishErr)
	}
	a.requestQuit(700 * time.Millisecond)
	return err // a completed receipt still requires exit even alongside a late diagnostic error
}
