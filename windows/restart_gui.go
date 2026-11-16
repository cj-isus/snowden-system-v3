package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

var activeRestartGUISession *core.RestartGUISession // including partial dispatch, retained to terminal OS exit

// operationMu held. This owner is unrelated to installation authority and is
// published in App before Prepare acquires any native resource.
func (a *App) launchOwnedModeGUI(ctx context.Context, h *restartHandoff, tun, resume bool, guard func(context.Context) error) (transferred bool, finalErr error) {
	if h == nil || h.app != a || a.restartGUI != nil {
		return false, core.ErrHelperCleanup
	}
	a.restartGUI = core.NewRestartGUIOwner()
	original := a.restartGUI
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, core.ErrUpdateGUIBootstrap)
		}
		if original.GUITransferred() {
			if !completed {
				// Actual wire custody is a negative terminal obligation even if
				// local receipt finalization failed/panicked/Goexit interrupted.
				// Do not reopen this App or infer GUI Ready from that fact.
				a.closing.Store(true)
				transferred = true
				finalErr = errors.Join(finalErr, core.ErrUpdateGUICommitted)
				a.requestQuit(300 * time.Millisecond)
			}
			return // original ownership survives parent exit, no abort/replacement
		}
		if a.restartGUI != original {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup)
			return
		}
		if err := original.Close(); err != nil {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup, err)
		} else {
			a.restartGUI = nil
		}
	}()
	if err := original.Prepare(ctx, guard); err != nil {
		return false, err
	}
	if err := original.StartGUI(ctx, tun, resume, guard); err != nil {
		return false, err
	}
	receipt, err := original.CommitGUI(ctx, guard, func(commit core.UpdateGUICommit) error { return h.acceptOwnedLaunch(ctx, commit) })
	if finishErr := h.finishOwnedLaunch(receipt); finishErr != nil {
		return false, errors.Join(err, finishErr)
	}
	// Actual final custody must cause parent exit even alongside a late error.
	transferred, completed = true, true
	return true, err
}

// operationMu held, after original engine retirement and before a new engine
// or positive protection Release. Retry only the exact retained preparation;
// a transferred GUI is intentionally kept alive until this parent exits.
// A failure in one preparation must not skip another known negative owner.
func (a *App) retireCaptureHelpers() (result error) {
	sequence := &appCleanupSequence{}
	originalRestart, originalElevation := a.restartGUI, a.elevatedGUI
	defer sequence.finish(&result)
	// Register all original groups before the first callback; one abnormal
	// group cannot skip the later known owners. No transferred GUI is closed.
	defer appCleanupStep("retire original elevation preparation", sequence, func() error {
		if a.elevatedGUI != originalElevation {
			return core.ErrHelperCleanup
		}
		if originalElevation == nil || originalElevation.Transferred() {
			return nil
		}
		if err := originalElevation.Close(); err != nil {
			return errors.Join(core.ErrHelperCleanup, err)
		}
		if a.elevatedGUI != originalElevation {
			return core.ErrHelperCleanup
		}
		a.elevatedGUI = nil
		return nil
	})
	defer appCleanupStep("retire original restart preparation", sequence, func() error {
		if a.restartGUI != originalRestart {
			return core.ErrHelperCleanup
		}
		if originalRestart == nil || originalRestart.GUITransferred() {
			return nil
		}
		if err := originalRestart.Close(); err != nil {
			return errors.Join(core.ErrHelperCleanup, err)
		}
		if a.restartGUI != originalRestart {
			return core.ErrHelperCleanup
		}
		a.restartGUI = nil
		return nil
	})
	defer appCleanupStep("retire original capture lanes", sequence, core.RetireCaptureHelpers)
	return nil
}
