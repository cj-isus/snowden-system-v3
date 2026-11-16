package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

var errRestartCommitted = errors.New("перезапуск уже применяется; дождитесь результата")

// One App transaction token, not another engine/process owner. It keeps the
// existing operationMu from successful Stop through source/apply/launch. Its
// commit is the point after which conflicting UI lifecycle actions are busy.
// Fields exposed to OnBeforeClose are guarded by App.mu.
type restartHandoff struct {
	app            *App
	request        stopRequest
	closeRequested bool
	launched       bool
	launchCommit   core.UpdateGUICommit   // original authorization, matched by the final wire receipt
	elevatedCommit core.ElevatedGUICommit // original U1 authorization, not a native launch report
	rendererEpoch  uint64                 // nonzero only for adoption by restricted forward recovery
}

func (a *App) restartIntent() (intentRegistration, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.restartIntentLocked()
}

// Caller owns App.mu; ordered UI admission captures this exact source before
// any serializer wait. A later Stop cannot be adopted as the restart's intent.
func (a *App) restartIntentLocked() (intentRegistration, error) {
	if a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || !a.intentSource.Current() {
		return intentRegistration{}, context.Canceled
	}
	if a.installedImageChanged {
		return intentRegistration{}, errInstalledImageChanged
	}
	if a.restartCommit != nil {
		return intentRegistration{}, errRestartCommitted
	}
	if a.releaseCommit != nil {
		return intentRegistration{}, errProtectionReleaseCommitted
	}
	return a.intentRegistrationLocked(), nil
}

func (a *App) restartIntentCurrent(registration intentRegistration) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx != nil && a.ctx.Err() == nil && !a.closing.Load() && a.restartCommit == nil && a.releaseCommit == nil && !a.installedImageChanged && registration.currentLocked(a)
}

// A caller retains its initiating intent through update serialization/staging.
// On success it owns operationMu and MUST defer handoff.release().
func (a *App) beginRestartHandoff(expected intentRegistration) (*restartHandoff, error) {
	request, err := func() (stopRequest, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !expected.currentLocked(a) || a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() {
			return stopRequest{}, context.Canceled
		}
		return a.registerStopLocked(stopRestart)
	}()
	if err != nil {
		return nil, err
	}
	a.operationMu.Lock()
	transferred := false
	defer func() {
		if !transferred {
			a.operationMu.Unlock() // also on panic before a caller can own the token
		}
	}()
	if err := a.stopCoreRegisteredLocked(stopRestart, request.intentRegistration); err != nil {
		return nil, err
	}
	handoff := &restartHandoff{app: a, request: request}
	transferred = true
	return handoff, nil
}

// Final value-only gate, before final journal/source work or OS launch.
// No App lock is held over file I/O, crypto, UAC or a native process call.
func (h *restartHandoff) commit() error {
	a := h.app
	accepted := func() bool {
		runtime_.mu.Lock()
		defer runtime_.mu.Unlock()
		a.mu.Lock()
		defer a.mu.Unlock()
		if !h.request.currentLocked(a) || a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || a.restartCommit != nil || a.releaseCommit != nil || a.installedImageChanged || a.protectionDesired != h.request.resumeRequested ||
			a.profileTransition != h.request.profileTransition || a.state.State != "stopped" || runtime_.manager != nil || runtime_.engine != nil || runtime_.cc != nil {
			return false
		}
		a.restartCommit = h
		a.state.State = "stopping" // existing busy UI state; no new public binding
		return true
	}()
	if !accepted {
		return context.Canceled
	}
	a.pushState() // no runtime/Manager/epoch lock over UI events
	return nil
}

func (h *restartHandoff) markLaunched() {
	a := h.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.restartCommit == h && h.request.currentLocked(a) {
		h.launched = true
		a.closing.Store(true) // no lifecycle action between accepted launch and Quit
	}
}

// One-use App authorization before U1 final USE. Runtime/App values only;
// no native work or closing publication until the final wire receipt.
func (h *restartHandoff) authorizeElevatedRequest(ctx context.Context, commit core.ElevatedGUICommit) error {
	a := h.app
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !lifecyclePublicationCurrent(ctx) || !h.codeAcceptanceCurrentLocked() {
		return context.Canceled
	}
	if err := commit.Accept(); err != nil {
		return err
	}
	h.elevatedCommit = commit
	return nil
}

func (h *restartHandoff) finishElevatedRequest(receipt core.ElevatedGUIReceipt) error {
	a := h.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.restartCommit != h || !h.request.sameLocked(a) || h.launched || a.releaseCommit != nil {
		return context.Canceled
	}
	if err := receipt.Consume(a.elevatedGUI, h.elevatedCommit); err != nil {
		return err
	}
	h.launched = true
	a.closing.Store(true)
	return nil
}

// CommitGUI holds its original owner/pipe lane. This callback takes only the
// established downstream Runtime/App value locks. The opaque grant performs
// atomic/value checks, never native I/O or an App callback under these locks.
func (h *restartHandoff) acceptOwnedLaunch(ctx context.Context, commit core.UpdateGUICommit) error {
	if h == nil || h.app == nil {
		return context.Canceled
	}
	a := h.app
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !lifecyclePublicationCurrent(ctx) || !h.codeAcceptanceCurrentLocked() {
		return context.Canceled
	}
	if err := commit.Accept(); err != nil {
		return err
	}
	h.launchCommit = commit // final native exchange still needs the original live context
	return nil
}

// Finalize only the matching completed wire transfer. This is an ownership
// fact, so a later ctx/path loss cannot authorize reuse of the old App or child.
func (h *restartHandoff) finishOwnedLaunch(receipt core.UpdateGUIReceipt) error {
	a := h.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.restartCommit != h || !h.request.sameLocked(a) || h.launched || a.releaseCommit != nil {
		return context.Canceled
	}
	if err := receipt.Consume(h.launchCommit); err != nil {
		return err
	}
	h.launched = true
	a.closing.Store(true) // after final exchange; no new positive work or registration before Quit
	return nil
}

// An error cannot overwrite a Start/Disconnect that won before commit.
func (h *restartHandoff) failed(cause error) {
	a := h.app
	cleanupUnconfirmed := localRetirementFailure(cause)
	message := lifecycleFailureText("перезапуск", cause, cleanupUnconfirmed)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !h.request.sameLocked(a) || a.closing.Load() || h.launched {
		return
	}
	a.state.State = "error"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.state.BlockedReason = "restart_failed"
	if cleanupUnconfirmed {
		a.state.BlockedReason = "cleanup_unconfirmed" // original process/image custody is still retained
	}
	a.state.Error = message
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
	a.pushStateLocked()
}

func (h *restartHandoff) release() {
	a := h.app
	a.mu.Lock()
	closeRequested := false
	if a.restartCommit == h {
		if !h.launched && h.request.sameLocked(a) && a.state.State == "stopping" {
			// An interrupted native/apply path cannot leave a permanent busy UI.
			// Its panic still propagates; this does not convert it into success.
			a.state.State = "error"
			a.state.CoreReady, a.state.ProbeRunning = false, false
			a.state.BlockedReason, a.state.Error = "restart_failed", "перезапуск прерван"
			a.cancelFailClosedRetryLocked()
			a.sm.markBlocked(time.Now())
		}
		a.restartCommit = nil
		closeRequested = h.closeRequested && !h.launched
		if closeRequested {
			a.closing.Store(true) // transfer pending close before any new Start can register
			a.cancelFailClosedRetryLocked()
		}
	}
	a.mu.Unlock()
	a.operationMu.Unlock()
	a.pushState()
	if closeRequested {
		// Reenter terminal cleanup only after releasing lifecycle locks.
		a.closeAfterLifecycle()
	}
}
