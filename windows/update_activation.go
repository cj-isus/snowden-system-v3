package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/update"
)

// Release builds must not silently run below an accepted floor, or with an
// unreadable floor. Dev is an explicit nonrelease mode, never an update target.
func checkRunningVersionFloor() error {
	if runningVersion == "dev" {
		return nil
	}
	version, err := update.ParseVersion(runningVersion)
	if err != nil {
		return errors.Join(update.ErrFloorUnavailable, err)
	}
	floor, err := update.ReadFloor()
	if err != nil || version.Compare(floor) < 0 {
		return errors.Join(update.ErrFloorUnavailable, err)
	}
	return nil
}

// Equal floor is allowed only for acceptance by the new running code. This
// also permits retry after floor commit succeeded but barrier archival failed.
// An update installation still requires a strictly greater version.
func checkInstalledUpdateVersion(manifest *update.Manifest) error {
	if manifest == nil || manifest.Validate() != nil || manifest.Schema != update.PairSchema ||
		nativeProbeHelperSHA256 == "" || manifest.NativeHelper.SHA256 != nativeProbeHelperSHA256 {
		return update.ErrPairRecoveryRequired
	}
	current, err := update.ParseVersion(runningVersion)
	if err != nil {
		return errors.Join(update.ErrPairRecoveryRequired, err)
	}
	next, err := update.ParseVersion(manifest.Version)
	if err != nil || next.Compare(current) != 0 {
		return update.ErrPairRecoveryRequired
	}
	return checkRunningVersionFloor()
}

// Caller owns operationMu. Update work is legal with VPN stopped; it must not
// borrow beginOperation's protectionDesired=true requirement. Stop/epoch/Close
// still cancel the existing single operation slot, without a second registry.
func (a *App) beginUpdateOperation(intent intentRegistration, recovery bool) (context.Context, context.CancelFunc, error) {
	preparation, err := func() (*appOperationPreparation, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closing.Load() || !intent.currentLocked(a) || a.restartCommit != nil || a.profileTransition || a.installedImageChanged != recovery {
			return nil, context.Canceled
		}
		return a.reserveOperationContextLocked(intent, appUpdateOperation, nil, nil, recovery)
	}()
	if err != nil {
		return nil, nil, err
	}
	return preparation.build()
}

func (a *App) activationCurrent(intent intentRegistration, rendererEpoch uint64) bool {
	// Match the existing runtime -> App value-only lock order. An old process
	// must never activate a replacement while any engine owner remains.
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	return rendererEpoch != 0 && a.updateGUIRendererEpoch == rendererEpoch &&
		intent.currentLocked(a) && !a.closing.Load() && a.restartCommit == nil && a.installedImageChanged &&
		!a.profileTransition && runtime_.manager == nil && runtime_.engine == nil && runtime_.cc == nil
}

// Called only during restricted startup, after vault/import recovery and before
// network producers. Journal contents select no old code and supply no trust:
// the exact manifest/sidecar must pass today's actual local private SDK state.
// A failed native close retains the original owner in App until process exit.
func (a *App) activateInstalledUpdate(rendererEpoch uint64) (finalErr error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	a.operationMu.Lock()
	releaseOperation := func() { a.operationMu.Unlock() }
	defer func() { releaseOperation() }()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, update.ErrPairRecoveryRequired)
		}
		finalErr = errors.Join(finalErr, a.closeInstalledUpdateOwners())
		if finalErr != nil {
			a.mu.Lock()
			a.installedImageChanged = true
			a.mu.Unlock()
		}
	}()
	a.mu.Lock()
	intent := a.intentRegistrationLocked()
	a.mu.Unlock()
	ctx, cancel, err := a.beginUpdateOperation(intent, true)
	if err != nil {
		return err
	}
	defer a.endOperation(cancel)
	ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
	defer timeout()
	if !a.activationCurrent(intent, rendererEpoch) {
		return context.Canceled
	}
	expectedSHA := ""
	if a.updateGUISession != nil {
		expectedSHA = a.updateGUISession.ManifestSHA()
	}
	root, verified, restart, err := a.prepareForwardRecovery(ctx, intent, rendererEpoch, expectedSHA)
	if err != nil {
		return err
	}
	if restart {
		handoff, err := a.commitRecoveryHandoff(intent, rendererEpoch)
		if err != nil {
			return err
		}
		releaseOperation = handoff.release // transfer actual operationMu ownership, no recursive acquisition
		a.pushState()                      // token release ownership is installed before any UI callback can panic
		if err := a.launchRecoveredUpdate(ctx, verified, handoff); err != nil {
			handoff.failed(err)
			return err
		}
		completed = true
		return errUpdateRecoveryRestarting // never proceed to producers in the old mapped process
	}
	if a.updateGUISession != nil {
		if err := a.updateGUISession.PrepareAcceptance(ctx); err != nil {
			return err // actual App image lease remains held, pending is not archived
		}
	}
	authorize := func(commit func() error) error {
		if !lifecyclePublicationCurrent(ctx) || !a.activationCurrent(intent, rendererEpoch) {
			return context.Canceled
		}
		if err := a.validatePairRoot(ctx, root); err != nil {
			return err
		}
		if err := a.pairImage.Validate(ctx); err != nil {
			return err
		}
		if !lifecyclePublicationCurrent(ctx) || !a.activationCurrent(intent, rendererEpoch) {
			return context.Canceled
		}
		return a.withVerifiedUpdatePurposeLocked(verified, true, func() error {
			if !lifecyclePublicationCurrent(ctx) {
				return context.Canceled // pure original source/path fence, no App lock under journal
			}
			return commit() // short filesystem/registry mutation; no flush under journal
		})
	}
	archive, err := update.FinalizePair(ctx, root, verified.raw, verified.sidecar, a.pairRoot, authorize)
	if archive != "" {
		a.pairArchive = archive
	}
	if err != nil {
		return err
	}
	if err := a.pairImage.Validate(ctx); err != nil {
		return err
	}
	if err := a.validatePairRoot(ctx, root); err != nil {
		return err
	}
	if err := update.CompletePairIntent(ctx, root, verified.raw, verified.sidecar, archive, authorize); err != nil {
		return err // visible Set/uncertain flush never clears this App's restriction
	}
	if err := a.pairImage.Validate(ctx); err != nil {
		return err
	}
	if err := a.pairImage.Close(); err != nil {
		return err
	}
	a.pairImage = nil
	if err := a.validatePairRoot(ctx, root); err != nil {
		return err
	}
	if err := a.closePairRoot(); err != nil {
		return err
	}
	if err := checkRunningVersionFloor(); err != nil {
		return err
	}
	if a.updateGUISession != nil {
		if err := a.updateGUISession.AcceptStartup(ctx); err != nil {
			return err // native image uncertainty cannot clear the App restriction
		}
	}
	// Install acceptance never publishes VPN Ready. Ordinary startup/Start must
	// still pass the complete fresh protected path admission.
	a.mu.Lock()
	if !intent.currentLocked(a) || a.closing.Load() || !lifecyclePublicationCurrent(ctx) || a.updateGUIRendererEpoch != rendererEpoch {
		a.mu.Unlock()
		return context.Canceled
	}
	a.installedImageChanged = false
	a.mu.Unlock()
	completed = true
	return nil
}
