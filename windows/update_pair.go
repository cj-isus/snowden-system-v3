package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

// updateMu is held by ApplyUpdate. Public install admission intentionally still
// rejects schema2 while native installer/child ownership is incomplete. This
// bridge supplies the real private source/clock/lifecycle guard for that seam;
// no test callback or journal hash can enable it.
func (a *App) stageVerifiedPair(source, root string, verified *verifiedUpdate, intent intentRegistration) (finalErr error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, update.ErrPairRecoveryRequired)
		}
		if finalErr != nil {
			finalErr = errors.Join(finalErr, a.discardUnpublishedPair())
		}
	}()
	if !a.restartIntentCurrent(intent) {
		return context.Canceled
	}
	ctx, cancel, err := a.beginUpdateOperation(intent, false)
	if err != nil {
		return err
	}
	defer a.endOperation(cancel)
	ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
	defer timeout()
	if err := a.discardUnpublishedPair(); err != nil {
		return err // an old partial owner must not be overwritten by a new one
	}
	if err := a.acquirePairRoot(ctx, root); err != nil {
		return err
	}
	if err := a.withVerifiedUpdateSourceLocked(verified, func() error {
		if !lifecyclePublicationCurrent(ctx) {
			return context.Canceled
		}
		return nil
	}); err != nil {
		return err
	}
	a.pairTransaction, err = update.StagePair(ctx, root, source, verified.raw, verified.sidecar, a.pairRoot)
	if err != nil {
		return err // constructor returns the exact partial owner even on error
	}
	if !a.restartIntentCurrent(intent) {
		return context.Canceled
	}
	if err := a.validatePairRoot(ctx, root); err != nil {
		return err
	}
	if err := a.withVerifiedUpdateSourceLocked(verified, func() error {
		if !lifecyclePublicationCurrent(ctx) {
			return context.Canceled
		}
		return nil
	}); err != nil {
		return err
	}
	completed = true
	return nil
}

// Caller owns updateMu. A published owner cannot be discarded or reconstructed
// from public recovery material. Failed cleanup keeps its exact pointer.
func (a *App) discardUnpublishedPair() error {
	if a.pairImage != nil {
		return core.ErrHelperCleanup // do not abandon the scope of an uncertain image owner
	}
	if a.pairTransaction != nil {
		if a.pairTransaction.State().Pending {
			return update.ErrPairRecoveryRequired
		}
		if err := a.pairTransaction.Discard(); err != nil {
			return err
		}
		a.pairTransaction = nil
	}
	return a.closePairRoot()
}

// All three helpers require updateMu. Acquisition always stores the partial
// native owner; only a confirmed Close may clear it or permit replacement.
func (a *App) acquirePairRoot(ctx context.Context, root string) error {
	if a.pairRoot != nil {
		return core.ErrInstallationRootBusy
	}
	return core.RetainInstallationRoot(ctx, &a.pairRoot, root)
}

func (a *App) validatePairRoot(ctx context.Context, root string) error {
	if a.pairRoot == nil {
		return core.ErrInstallationRootUnavailable
	}
	return a.pairRoot.Validate(ctx, root)
}

func (a *App) closePairRoot() error {
	if a.pairRecovery != nil {
		if err := a.pairRecovery.Retire(); err != nil {
			return err // never release the root under an uncertain recovery writer
		}
		a.pairRecovery = nil // all Go writers retired; payload originals remain in root custody
	}
	if a.pairRoot == nil {
		return nil
	}
	if err := a.pairRoot.Close(); err != nil {
		return err
	}
	a.pairRoot = nil
	return nil
}

func (h *restartHandoff) updateCurrent() bool {
	a := h.app
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.restartCommit == h && h.request.currentLocked(a) && !a.closing.Load() && !a.installedImageChanged
}

// After installed-name mutation the old image is deliberately restricted.
// Its committed updater token may finish only the fixed code-acceptance phase;
// this gate never authorizes a worker, profile operation or GUI Ready.
func (h *restartHandoff) codeAcceptanceCurrent() bool {
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	a := h.app
	a.mu.Lock()
	defer a.mu.Unlock()
	return h.codeAcceptanceCurrentLocked()
}

// runtime_.mu and App.mu held; values only, no SDK/native/Manager work.
func (h *restartHandoff) codeAcceptanceCurrentLocked() bool {
	a := h.app
	return a.restartCommit == h && a.releaseCommit == nil && h.request.currentLocked(a) && a.appLifetimeCurrentLocked() && !a.closing.Load() && !h.launched &&
		(h.rendererEpoch == 0 || a.installedImageChanged && a.updateGUIRendererEpoch == h.rendererEpoch) &&
		a.protectionDesired == h.request.resumeRequested && a.profileTransition == h.request.profileTransition &&
		runtime_.manager == nil && runtime_.engine == nil && runtime_.cc == nil
}

func (a *App) updateCodeSource(ctx context.Context, h *restartHandoff, verified *verifiedUpdate) error {
	if !lifecyclePublicationCurrent(ctx) || !h.codeAcceptanceCurrent() {
		return context.Canceled
	}
	return a.withVerifiedUpdateSourceLocked(verified, func() error {
		if !lifecyclePublicationCurrent(ctx) {
			return context.Canceled
		}
		return nil
	})
}

func (a *App) closeUpdateCodeAcceptance() error {
	if a.pairAcceptance == nil {
		return nil
	}
	if err := a.pairAcceptance.Close(); err != nil {
		return err
	}
	a.pairAcceptance = nil
	return nil
}

func (a *App) beginCommittedUpdate(h *restartHandoff) (context.Context, context.CancelFunc, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if h == nil || h.app != a || a.restartCommit != h || !h.request.currentLocked(a) || a.closing.Load() || a.installedImageChanged {
		return nil, nil, context.Canceled
	}
	return a.bindUpdateOperationLocked(h.request.intentRegistration)
}

// Caller holds updateMu. No engine/native helper remains after the existing
// restart retirement, and that handoff owns operationMu through mutation/launch.
func (a *App) applyVerifiedPair(source string, verified *verifiedUpdate, intent intentRegistration) (message string, finalErr error) {
	defer func() { finalErr = errors.Join(finalErr, a.closePairRecoveryCheck()) }()
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if filepath.Base(exe) != update.ApplicationFile {
		return "", update.ErrPairRecoveryRequired
	}
	if err := a.stageVerifiedPair(source, filepath.Dir(exe), verified, intent); err != nil {
		return "", err
	}
	tx := a.pairTransaction
	defer func() {
		if !tx.State().Pending {
			finalErr = errors.Join(finalErr, a.discardUnpublishedPair())
		}
	}()
	handoff, err := a.beginRestartHandoff(intent)
	if err != nil {
		return "", err
	}
	defer handoff.release()
	defer func() {
		state := tx.State()
		if state.Pending || state.InstalledNamesTouched || update.CheckPairPending(filepath.Dir(exe)) != nil {
			handoff.requireFreshImage() // before the lifecycle token is released
		}
	}()
	if err := handoff.commit(); err != nil {
		return "", err
	}
	ctx, cancel, err := a.beginCommittedUpdate(handoff)
	if err != nil {
		return "", err
	}
	defer a.endOperation(cancel)
	ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
	defer timeout()
	err = tx.Apply(ctx, func(commit func() error) error {
		if !lifecyclePublicationCurrent(ctx) || !handoff.updateCurrent() {
			return context.Canceled
		}
		if err := a.validatePairRoot(ctx, filepath.Dir(exe)); err != nil {
			return err // native validation outside metadata journal, handles remain held
		}
		if a.pairRecoveryCheck != nil {
			if err := a.validatePairRecoveryReady(ctx); err != nil {
				return err
			}
		}
		return a.withVerifiedUpdateSourceLocked(verified, commit)
	}, a.pairReadiness(filepath.Dir(exe), verified, updateInstallPurpose, handoff.updateCurrent))
	if err != nil {
		if tx.State().Pending {
			rollbackErr := func() error {
				cleanup, stopCleanup := context.WithTimeout(context.Background(), 30*time.Second)
				defer stopCleanup()
				if err := a.validatePairRoot(cleanup, filepath.Dir(exe)); err != nil {
					return err
				}
				return tx.Rollback(cleanup)
			}()
			err = errors.Join(err, rollbackErr)
		}
		handoff.failed(err)
		return "", err // even restored files keep pending and cannot run old code
	}
	if err := a.validatePairRoot(ctx, filepath.Dir(exe)); err != nil {
		handoff.failed(err)
		return "", err
	}
	if a.pairAcceptance != nil {
		return "", core.ErrHelperCleanup // no replacement of an uncertain original owner
	}
	// The original payload handles include DELETE access and cannot overlap the
	// deny-delete prospective image lease. Retire them under the retained writer
	// root, then acquire the actual signed image before releasing that root.
	if err := a.pairRoot.ReleasePayloads(); err != nil {
		handoff.failed(err)
		return "", err
	}
	defer func() {
		if !a.pairAcceptance.GUITransferred() {
			finalErr = errors.Join(finalErr, a.closeUpdateCodeAcceptance())
		}
	}()
	sum := sha256.Sum256(verified.raw)
	guard := func(ctx context.Context) error { return a.updateCodeSource(ctx, handoff, verified) }
	err = core.RetainUpdateCodeAcceptance(ctx, &a.pairAcceptance, verified.manifest, hex.EncodeToString(sum[:]), guard)
	if err != nil {
		handoff.failed(err)
		return "", err
	}
	if err := a.closePairRecoveryCheck(); err != nil {
		handoff.failed(err)
		return "", err // prospective Main image held; keep root if companion retirement is uncertain
	}
	handoff.requireFreshImage() // old mapped App is restricted before releasing writer scope
	if err := a.closePairRoot(); err != nil {
		handoff.failed(err)
		return "", err
	}
	if err := a.pairAcceptance.Verify(ctx, guard); err != nil {
		handoff.failed(err)
		return "", err
	}
	if err := a.pairAcceptance.StartGUI(ctx, tunRequested(), handoff.request.resumeRequested, guard); err != nil {
		handoff.failed(err)
		return "", err
	}
	receipt, err := a.pairAcceptance.CommitGUI(ctx, guard, func(commit core.UpdateGUICommit) error {
		return handoff.acceptOwnedLaunch(ctx, commit)
	})
	if finishErr := handoff.finishOwnedLaunch(receipt); finishErr != nil {
		err = errors.Join(err, finishErr)
		handoff.failed(err)
		return "", err
	}
	// The new process owns its non-inheritable lifetime Job reference and image;
	// it will wait this original process, recheck SDK authority, then enter Wails.
	// Keep our original process/Job/image owner until OS exit, no Process.Release.
	a.appendLog("info", "update: передача новому процессу подготовлена; его готовность подтвердится после выхода текущего приложения")
	a.requestQuit(700 * time.Millisecond)
	return "передача обновления подготовлена — приложение перезапускается", err
}
