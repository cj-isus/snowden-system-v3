package main

import (
	"context"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/secretvault"
)

// The original setup frame owns all partial children/admission. It is not a
// worker or second diagnostic queue; App.mu already retains its exact task.
type manualDiagnosticPreparation struct {
	app               *App
	task              *manualDiagnosticTask
	vault             *secretvault.Manager
	parent            appTaskParent
	timeout           time.Duration
	requests          int
	bodyBytes         int64
	contextEntered    bool
	admissionEntered  bool
	admissionReturned bool
	transferred       bool
}

func (a *App) reserveManualDiagnostic(timeout time.Duration, requests int, bodyBytes int64, network bool) (*manualDiagnosticPreparation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	registration := a.intentRegistrationLocked()
	if timeout <= 0 || timeout > 90*time.Second || requests < 0 || bodyBytes < 0 || a.diagnosticTask != nil ||
		!a.appLifetimeCurrentLocked() || a.vault == nil || a.closing.Load() || a.restartCommit != nil || a.releaseCommit != nil || !registration.currentLocked(a) {
		return nil, errSecretTestUnavailable
	}
	var path *networkPathSource
	if network {
		path = netEpoch.pathSource() // App -> epoch, passive source only
		if path == nil || !path.current.Load() {
			return nil, errSecretTestUnavailable
		}
	}
	view := a.diagnosticContextLocked()
	if view == nil {
		return nil, errSecretTestUnavailable
	}
	state := newAppTaskContext()
	state.dependencies[0] = registration.source // a pending factory still observes the exact request's revocation
	task := &manualDiagnosticTask{app: a, contextState: state, parent: a.ctx, contextView: view, cancel: state.cancellation.request,
		registration: registration, closing: &a.closing, path: path, done: make(chan struct{}), watchDone: make(chan struct{}), resultRejected: true}
	preparation := &manualDiagnosticPreparation{app: a, task: task, vault: a.vault, parent: appTaskParent{app: a, ctx: a.ctx, fence: a.ctxFence}, timeout: timeout, requests: requests, bodyBytes: bodyBytes}
	a.diagnosticTask = task // before any real Value/timeout/admission/cancel/release callback
	return preparation, nil
}

func (preparation *manualDiagnosticPreparation) restrictionsCurrent() bool {
	task := preparation.task
	return preparation.parent.fence.Current() && task.contextState.cancellation.operation.Current() && task.registration.source.Current() &&
		!task.closing.Load() && (task.path == nil || task.path.current.Load() && netEpoch.pathValue.Load() == task.path)
}

func (preparation *manualDiagnosticPreparation) prepare() (task *manualDiagnosticTask, vault *secretvault.Manager, result error) {
	returned := false
	defer preparation.finish(&returned, &task, &vault, &result)
	result = preparation.build()
	returned = true
	if result != nil {
		return nil, nil, errSecretTestUnavailable
	}
	preparation.transferred = true
	go preparation.task.watch() // existing original watcher, only after exact publication
	return preparation.task, preparation.vault, nil
}

func (preparation *manualDiagnosticPreparation) build() error {
	if !preparation.restrictionsCurrent() {
		return errSecretTestUnavailable
	}
	parent, task := preparation.parent, preparation.task
	if task.path != nil {
		value := parent.ctx.Value(networkPathContextKey{})
		if value != nil {
			original, ok := value.(*networkPathSource)
			if !ok || original != task.path {
				return errSecretTestUnavailable
			}
		} else {
			parent.ctx = context.WithValue(parent.ctx, networkPathContextKey{}, task.path)
		}
	}
	if !preparation.restrictionsCurrent() {
		return errSecretTestUnavailable
	}
	preparation.contextEntered = true
	ctx, _, err := task.contextState.constructTimeout(parent, preparation.timeout)
	if err != nil || !preparation.restrictionsCurrent() {
		return errSecretTestUnavailable
	}
	if preparation.requests != 0 {
		preparation.admissionEntered = true
		admitted, release, admissionErr := core.AdmitDiagnostic(ctx, core.DiagnosticEndpoint, preparation.requests, preparation.bodyBytes, preparation.timeout)
		task.release = release // original partial lease before any later original getter or gate
		preparation.admissionReturned = true
		if admissionErr != nil || admitted == nil || release == nil {
			return errSecretTestUnavailable
		}
		ctx = admitted
	}
	if !preparation.restrictionsCurrent() {
		return errSecretTestUnavailable
	}
	fence, err := core.CaptureOperationFence(ctx)
	if err != nil {
		return errSecretTestUnavailable
	}
	deadline, bounded := ctx.Deadline() // actual admitted child, outside App/task locks
	a := preparation.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.diagnosticTask != task || a.vault != preparation.vault || task.completed || !preparation.restrictionsCurrent() ||
		!a.appLifetimeCurrentLocked() || a.restartCommit != nil || a.releaseCommit != nil || !task.registration.currentLocked(a) ||
		!fence.Current() || !bounded || !time.Now().Before(deadline) || !sameDiagnosticContext(task.contextView, a.diagnosticContextLocked()) {
		return errSecretTestUnavailable
	}
	task.ctx, task.contextDeadline, task.contextBounded = ctx, deadline, bounded
	task.contextState.parentFence, task.contextState.childFence, task.contextState.ready = preparation.parent.fence, fence, true
	task.contextReady.Store(true) // release immutable context/fences/deadline to value-only readers
	return nil
}

// No RPC/native dial/watcher was handed off. Clear only the exact slot after
// ordinary original setup and actual cancellation/budget return are known.
func (preparation *manualDiagnosticPreparation) finish(returned *bool, taskResult **manualDiagnosticTask, vaultResult **secretvault.Manager, result *error) {
	if preparation.transferred {
		return
	}
	if !*returned {
		_ = recover()
	}
	task := preparation.task
	*taskResult, *vaultResult, *result = nil, nil, errSecretTestUnavailable
	if !preparation.contextEntered {
		task.contextState.skipConstruction()
	}
	if !*returned || preparation.admissionEntered && !preparation.admissionReturned {
		task.cleanupFailed.Store(true)
	}
	sequence := new(appCleanupSequence)
	var cleanupResult error
	defer func() {
		a := preparation.app
		a.mu.Lock()
		defer a.mu.Unlock()
		task.preparation = appProducerReceipt{preparationOnly: true, preparationReturned: *returned, cleanupReturned: sequence.drained && cleanupResult == nil, cleanupResult: cleanupResult}
		if !task.preparation.cleanupKnown() || task.cleanupFailed.Load() || a.diagnosticTask != task || task.completed {
			task.cleanupFailed.Store(true)
			return // exact original slot/done remain pending, never replay unknown work
		}
		task.retirementConfirmed, task.completed = true, true
		a.diagnosticTask = nil
		close(task.done) // known original preparation retirement only; no fake body/watch return
	}()
	defer sequence.finish(&cleanupResult)
	defer appCleanupStep("release original unused diagnostic admission", sequence, func() error {
		if sequence.result != nil || task.cleanupFailed.Load() || preparation.admissionEntered && !preparation.admissionReturned {
			return errSecretTestUnavailable
		}
		return task.releaseOriginalBudget()
	})
	defer appCleanupStep("retire original unused diagnostic context", sequence, task.contextState.cancellation.join)
}

func (task *manualDiagnosticTask) originalParentCurrent() bool {
	return task != nil && task.contextReady.Load() && task.parent != nil && task.contextState.parentFence.Current()
}

// The original setup/RPC alone owns this stage. Never retry an interrupted
// release or run its callback under App/task/value locks.
func (task *manualDiagnosticTask) releaseOriginalBudget() error {
	if task.releaseStarted || task.releaseReturned {
		return errSecretTestUnavailable
	}
	if task.release != nil {
		task.releaseStarted = true
		task.release()
	}
	task.releaseReturned = true
	if task.cleanupFailed.Load() {
		return errSecretTestUnavailable
	}
	return nil
}
