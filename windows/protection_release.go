package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

var (
	errProtectionReleaseCommitted   = errors.New("снятие защиты уже выполняется; дождитесь результата")
	errProtectionReleaseInterrupted = errors.New("снятие защиты прервано; очистка не подтверждена")
)

// One committed manual Disconnect phase, not another WFP/resource owner.
// Caller keeps operationMu through Stop publication and defers finish. App.mu
// guards every field; no App lock is held over the actual native policy call.
type protectionReleaseCommit struct {
	app            *App
	registration   intentRegistration
	policy         protectionPolicy // captured original policy, not a caller-supplied replacement
	started        bool
	returned       bool // function return only, never independent native enforcement proof
	closeRequested bool
}

func (a *App) protectionReleaseCurrentLocked(registration intentRegistration) bool {
	return registration.intent != 0 && registration.currentLocked(a) && a.appLifetimeCurrentLocked() && !a.closing.Load() &&
		!a.protectionDesired && !a.profileTransition && a.restartCommit == nil
}

// Only stopCoreRegisteredLocked calls this after original Manager/helper cleanup
// succeeded. Source revocation still permits negative teardown, but cannot
// authorize opening direct traffic or a successor registration during Release.
func (a *App) beginProtectionRelease(registration intentRegistration) (*protectionReleaseCommit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.releaseCommit != nil {
		return nil, errProtectionReleaseCommitted
	}
	if !a.protectionReleaseCurrentLocked(registration) {
		return nil, context.Canceled
	}
	commit := &protectionReleaseCommit{app: a, registration: registration, policy: a.protectionPolicy()}
	a.releaseCommit = commit // linearize native authorization before releasing the value lock
	a.cancelFailClosedRetryLocked()
	return commit, nil
}

func (commit *protectionReleaseCommit) call() (err error) {
	a := commit.app
	a.mu.Lock()
	if a.releaseCommit != commit || commit.started || !a.protectionReleaseCurrentLocked(commit.registration) {
		a.mu.Unlock()
		return context.Canceled
	}
	commit.started = true
	a.mu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(core.ErrEngineCleanupFailed, errProtectionReleaseInterrupted)
		}
		a.mu.Lock()
		commit.returned = completed
		a.mu.Unlock()
	}()
	err = commit.policy.Release()
	completed = true
	if err != nil {
		err = errors.Join(core.ErrEngineCleanupFailed, err) // retain custody uncertainty across root error gates
	}
	return err
}

func (commit *protectionReleaseCommit) finish() {
	a := commit.app
	a.mu.Lock()
	if a.releaseCommit != commit {
		a.mu.Unlock()
		return
	}
	publish := false
	if commit.registration.sameLocked(a) && a.state.State == "stopping" {
		// A panic/Goexit later in Stop must not leave a permanent busy phase or
		// turn an unfinished native call into a successful Disconnect receipt.
		a.state.State, a.state.CoreReady, a.state.ProbeRunning = "error", false, false
		a.state.BlockedReason, a.state.Error = "cleanup_unconfirmed", errProtectionReleaseInterrupted.Error()
		if !commit.started {
			a.state.BlockedReason, a.state.Error = "protection_retained", "снятие защиты не было разрешено"
		} else if commit.returned {
			a.state.Error = "остановка прервана после вызова снятия защиты; итог требует проверки"
		}
		a.cancelFailClosedRetryLocked()
		a.sm.markBlocked(time.Now())
		publish = true
	}
	closeRequested := commit.closeRequested
	if closeRequested {
		a.closing.Store(true) // transfer pending Close before any successor may register
		a.cancelFailClosedRetryLocked()
	}
	a.releaseCommit = nil
	a.mu.Unlock()
	if closeRequested {
		a.closeAfterLifecycle() // its task waits for caller-owned operationMu after registering Quit
	}
	if publish {
		a.pushState()
	}
}
