package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/secretvault"
)

// Shared original owner for explicit GUI secret/network/local diagnostics,
// independent of the serialized engine operation. Historical filename only.
// It owns no forwarding/probe/trust authority. App.mu only reserves/retires the
// original pointer; vault, network I/O, cancellation and joins run outside it.
type manualDiagnosticTask struct {
	app                 *App
	contextState        appTaskContext // immutable fences/deadline after contextReady publication
	contextReady        atomic.Bool
	contextDeadline     time.Time
	contextBounded      bool
	preparation         appProducerReceipt // App.mu; failed original setup only, never an RPC/body return
	ctx                 context.Context
	parent              context.Context        // original App lifetime; own cancel never cancels this parent
	contextView         *DiagnosticContextView // immutable original presentation receipt; never authority
	cancel              context.CancelFunc
	registration        intentRegistration
	closing             *atomic.Bool // original App lifetime negative fence, never authority
	cleanupFailed       atomic.Bool  // unexpected original socket-close failure stays pending
	dialLimitReached    atomic.Bool  // local admission rejection, not a remote network failure or uncertain cleanup
	credentialsChanged  atomic.Bool  // facade mutation cannot retag a pending credential observation
	done                chan struct{}
	watchDone           chan struct{}
	watchReturned       atomic.Bool             // original watcher ordinary-return witness, separate from watchDone
	mu                  sync.Mutex              // original dial registry only; never acquire App.mu here
	dials               []*manualDiagnosticDial // bounded originals, never replaced by endpoint/address
	sealed              bool
	bodyReturned        bool // synchronous RPC frame only
	retirementConfirmed bool // same original RPC; no native/SDK global retirement claim
	publicationCurrent  bool
	releaseStarted      bool
	releaseReturned     bool
	completed           bool               // App.mu only; original done closes once
	path                *networkPathSource // optional original negative path fence; no route attestation
	release             func()             // original budget; synchronous owner only, retained on uncertain cleanup
	resultRejected      bool               // App.mu only; historical cache cannot publish a failed completion fence
}

const secretTestTimeout = 30 * time.Second

var errSecretTestUnavailable = errors.New("диагностика недоступна: запрос отменён, источник изменился, занят бюджет или проверка уже выполняется")

func (a *App) beginSecretTest() (*manualDiagnosticTask, *secretvault.Manager, error) {
	// One endpoint request; max application response is the CF16KiB+1 body.
	// SSH fixed script +4096 output is smaller. Protocol headers/auth/handshake
	// internals are outside this application-payload budget, not a wire/RSS bound.
	return a.beginManualDiagnostic(secretTestTimeout, 1, (16<<10)+1, true)
}

func (a *App) beginManualDiagnostic(timeout time.Duration, requests int, bodyBytes int64, network bool) (task *manualDiagnosticTask, vault *secretvault.Manager, result error) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			task, vault, result = nil, nil, errSecretTestUnavailable
		}
	}()
	task, vault, result = a.prepareManualDiagnostic(timeout, requests, bodyBytes, network)
	returned = true // includes preparation's unused-resource guards and App.mu unlock
	return
}

func (a *App) prepareManualDiagnostic(timeout time.Duration, requests int, bodyBytes int64, network bool) (*manualDiagnosticTask, *secretvault.Manager, error) {
	preparation, err := a.reserveManualDiagnostic(timeout, requests, bodyBytes, network)
	if err != nil {
		return nil, nil, err
	}
	return preparation.prepare()
}

func (task *manualDiagnosticTask) currentError() error {
	if task == nil || !task.contextReady.Load() || !task.contextState.cancellation.operation.Current() || !task.contextState.parentFence.Current() ||
		!task.contextState.childFence.Current() || task.cleanupFailed.Load() || task.dialLimitReached.Load() || task.closing == nil || task.closing.Load() ||
		!task.registration.source.Current() || task.path != nil && !task.path.current.Load() {
		return errSecretTestUnavailable
	}
	return nil
}

func (task *manualDiagnosticTask) watch() {
	defer close(task.watchDone)
	defer func() {
		if payload := recover(); payload != nil || !task.watchReturned.Load() {
			task.cleanupFailed.Store(true)
		}
	}()
	task.watchBody()
	task.watchReturned.Store(true)
}

func (task *manualDiagnosticTask) watchBody() {
	select {
	case <-task.ctx.Done():
	case <-task.registration.source.Done(): // Start/Stop/profile successor revokes the original intent
	}
	dials := task.sealDials()
	sequence := new(appCleanupSequence)
	defer func() {
		if sequence.result != nil {
			task.cleanupFailed.Store(true)
		}
	}()
	defer appCleanupStep("close original watched diagnostic sockets", sequence, func() error { task.closeDials(dials); return nil })
	defer appCleanupStep("cancel original watched diagnostic dials", sequence, func() error { cancelManualDials(dials); return nil })
	defer appCleanupStep("cancel original watched diagnostic context", sequence, func() error { task.cancel(); return nil })
}

func (a *App) finishManualDiagnostic(task *manualDiagnosticTask) (result error) {
	currentBeforeCancel := task.currentError() == nil
	sequence := new(appCleanupSequence)
	defer func() {
		if result != nil || !task.bodyReturned || !task.watchReturned.Load() || task.cleanupFailed.Load() {
			task.cleanupFailed.Store(true)
			result = errSecretTestUnavailable
			return // exact task/budget/done retained; no completion inferred from cancellation
		}
		task.retirementConfirmed = true
		a.mu.Lock()
		defer a.mu.Unlock()
		task.publicationCurrent = currentBeforeCancel && task.originalParentCurrent() && task.registration.currentLocked(a) && !a.closing.Load() &&
			(task.path == nil || task.path.current.Load()) && (!task.contextBounded || time.Now().Before(task.contextDeadline)) &&
			sameDiagnosticContext(task.contextView, a.diagnosticContextLocked())
		if !task.publicationCurrent {
			result = errSecretTestUnavailable // resources retired; stale observation never becomes current
		}
	}()
	defer sequence.finish(&result)
	defer appCleanupStep("join original diagnostic context cancellation", sequence, task.contextState.cancellation.join)
	defer appCleanupStep("retire original diagnostic dials", sequence, func() error { task.retireDials(); return nil })
	defer appCleanupStep("join original diagnostic watcher", sequence, func() error {
		<-task.watchDone
		if !task.watchReturned.Load() {
			return errSecretTestUnavailable
		}
		return nil
	})
	defer appCleanupStep("cancel original diagnostic context", sequence, func() error { task.cancel(); return nil })
	return nil
}

func (a *App) cancelManualDiagnostic() <-chan struct{} {
	a.mu.Lock()
	task := a.diagnosticTask
	a.mu.Unlock()
	if task == nil {
		return nil
	}
	task.cancel() // does not release admission or erase the original task
	return task.done
}
