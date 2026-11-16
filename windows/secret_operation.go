package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/secretvault"
)

// Shared original owner for explicit GUI secret/network/local diagnostics,
// independent of the serialized engine operation. Historical filename only.
// It owns no forwarding/probe/trust authority. App.mu only reserves/retires the
// original pointer; vault, network I/O, cancellation and joins run outside it.
type manualDiagnosticTask struct {
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
	a.mu.Lock()
	defer a.mu.Unlock()
	registration := a.intentRegistrationLocked()
	if timeout <= 0 || timeout > 90*time.Second || requests < 0 || bodyBytes < 0 ||
		a.diagnosticTask != nil || a.ctx == nil || a.ctx.Err() != nil || a.vault == nil ||
		a.closing.Load() || a.restartCommit != nil || a.releaseCommit != nil || !registration.currentLocked(a) {
		return nil, nil, errSecretTestUnavailable
	}
	base := a.ctx
	var path *networkPathSource
	if network {
		base = netEpoch.scopeContext(base)
		path, _ = base.Value(networkPathContextKey{}).(*networkPathSource)
		if path == nil || !path.current.Load() {
			return nil, nil, errSecretTestUnavailable
		}
	}
	contextView := a.diagnosticContextLocked()
	if contextView == nil {
		return nil, nil, errSecretTestUnavailable // presentation preparation cannot strand admission
	}
	ctx, cancel := context.WithTimeout(base, timeout)
	handedOff := false
	var release func()
	// Register separately: an abnormal cancellation must not skip unused budget
	// retirement. After handoff only the original task finisher may release it.
	defer func() {
		if !handedOff && release != nil {
			release()
		}
	}()
	defer func() {
		if !handedOff {
			cancel()
		}
	}()
	if requests != 0 {
		var err error
		ctx, release, err = core.AdmitDiagnostic(ctx, core.DiagnosticEndpoint, requests, bodyBytes, timeout)
		if err != nil {
			return nil, nil, errSecretTestUnavailable
		}
	}
	if ctx.Err() != nil || a.closing.Load() || !registration.currentLocked(a) ||
		path != nil && (netEpoch.pathValue.Load() != path || !path.current.Load()) {
		return nil, nil, errSecretTestUnavailable
	}
	task := &manualDiagnosticTask{ctx: ctx, parent: a.ctx, contextView: contextView, cancel: cancel, registration: registration, closing: &a.closing,
		path: path, release: release, done: make(chan struct{}), watchDone: make(chan struct{}), resultRejected: true}
	a.diagnosticTask = task // busy until this original call AND its watcher return
	handedOff = true        // exact original task owns cancellation/budget before its watcher starts
	go task.watch()
	return task, a.vault, nil
}

func (task *manualDiagnosticTask) currentError() error {
	if task.ctx.Err() != nil || task.cleanupFailed.Load() || task.dialLimitReached.Load() || task.closing == nil || task.closing.Load() ||
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
	task.cancel()
	dials := task.sealDials()
	cancelManualDials(dials)
	task.closeDials(dials)
}

func (a *App) finishManualDiagnostic(task *manualDiagnosticTask) error {
	currentBeforeCancel := task.currentError() == nil
	task.cancel()
	<-task.watchDone // cancellation/deadline is not completion
	task.retireDials()
	if !task.bodyReturned || !task.watchReturned.Load() || task.cleanupFailed.Load() {
		return errSecretTestUnavailable // original pointer/done retained; no replacement or false success
	}
	task.retirementConfirmed = true
	a.mu.Lock()
	defer a.mu.Unlock()
	deadline, bounded := task.ctx.Deadline()
	publicationCurrent := currentBeforeCancel && task.parent != nil && task.parent.Err() == nil && task.registration.currentLocked(a) && !a.closing.Load() &&
		(task.path == nil || task.path.current.Load()) && (!bounded || time.Now().Before(deadline)) &&
		sameDiagnosticContext(task.contextView, a.diagnosticContextLocked())
	task.publicationCurrent = publicationCurrent
	if !publicationCurrent {
		return errSecretTestUnavailable // resources retired, but the old result is not current
	}
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
