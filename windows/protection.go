package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

type protectionPolicy interface {
	Arm() error
	BlockDirect() error
	Release() error
}

// Optional negative stage for original actor retirement. Nil means general
// permits were revoked, not that a complete direct-traffic barrier was proved.
type protectionRetirementPolicy interface {
	RevokeDirect() error
}

type strictWFPPolicy struct{}

var errRuntimeOwnerUnavailable = errors.New("engine has no lifecycle owner; cleanup unconfirmed")

func localRetirementFailure(err error) bool {
	return errors.Is(err, errRuntimeOwnerUnavailable) ||
		errors.Is(err, errWatchdogUnavailable) ||
		errors.Is(err, errNetworkPollerUnavailable) ||
		errors.Is(err, errServiceDNSObserverUnavailable) ||
		errors.Is(err, core.ErrGUIAncestorUnavailable) ||
		errors.Is(err, core.ErrHelperCleanup) ||
		errors.Is(err, core.ErrEngineCleanupFailed) ||
		errors.Is(err, core.ErrPreflightCleanup)
}

// Local proxy coordination/state failures apply to every channel; switching
// candidates cannot fix them. They do not themselves assert failed retirement.
func channelRetryBlocked(err error) bool {
	return localRetirementFailure(err) || errors.Is(err, core.ErrProxyOwnershipConflict) || errors.Is(err, core.ErrProxyWriterBusy) ||
		errors.Is(err, core.ErrProxyStateUncertain) || errors.Is(err, core.ErrProxyObservationUnavailable)
}

func (strictWFPPolicy) Arm() error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	return armWFPKillSwitch(exe)
}

func (strictWFPPolicy) ArmContext(ctx context.Context) error {
	if err := wfpOperationScope(ctx); err != nil {
		return err
	}
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	return armWFPKillSwitchContext(ctx, exe)
}

func (strictWFPPolicy) BlockDirect() error { return revokeWFPTUN() }
func (strictWFPPolicy) Release() error     { return disarmKillSwitch() }

func (a *App) protectionPolicy() protectionPolicy {
	if a.policy != nil {
		return a.policy
	}
	return strictWFPPolicy{}
}

// StopReason determines policy cleanup, independently of the engine state.
// Only explicit Disconnect authorizes opening direct traffic (FR-009/012).
type StopReason uint8

const (
	stopFailure StopReason = iota
	stopDisconnect
	stopQuit
	stopRestart
)

func (r StopReason) releasesProtection() bool { return r == stopDisconnect }

// Private compatibility adapter. Product callers preserve the actual error
// through beginOperationContext; a boolean is not a cleanup outcome.
func (a *App) beginOperation(id uint64, original ...*core.OperationSource) (context.Context, context.CancelFunc, bool) {
	ctx, cancel, err := a.beginOperationContext(id, original...)
	return ctx, cancel, err == nil
}

// Caller holds operationMu. a.mu is never held while calling Manager/network.
func (a *App) beginOperationContext(id uint64, original ...*core.OperationSource) (context.Context, context.CancelFunc, error) {
	preparation, err := func() (*appOperationPreparation, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(original) > 1 || (len(original) == 1 && (original[0] == nil || original[0] != a.intentSource)) {
			return nil, context.Canceled // captured Start/Switch cannot adopt a successor's source
		}
		if !a.appLifetimeCurrentLocked() || a.startupErr != nil || a.watchdogBlocked.Load() || !a.protectionDesired || a.profileTransition || a.proxyConflict || a.closing.Load() || a.restartCommit != nil || a.installedImageChanged || a.operationID != id || id == 0 || !a.intentSource.Current() {
			return nil, context.Canceled
		}
		return a.reserveOperationContextLocked(a.intentRegistrationLocked(), appForegroundOperation, nil, nil, false)
	}()
	if err != nil {
		return nil, nil, err
	}
	return preparation.build()
}

// App.mu held. Revoke the original source before registering a successor; an
// Engine/pipe waiter sees this without calling App or waiting for ctx cancel.
func (a *App) advanceOperationLocked() error {
	if a.releaseCommit != nil {
		return errProtectionReleaseCommitted
	}
	if a.restartCommit != nil {
		return errRestartCommitted
	}
	a.intentSource.Revoke()
	a.operationID++
	a.intentSource = core.NewOperationSource()
	if a.operationID == 0 {
		a.intentSource.Revoke() // zero/wrap never authorizes a positive operation
	}
	return nil
}

func (a *App) operationCurrent(id uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.startupErr == nil && !a.watchdogBlocked.Load() && a.protectionDesired && !a.profileTransition && !a.closing.Load() && a.operationID == id
}

func (a *App) endOperation(cancel context.CancelFunc) {
	// operationMu retains the original constructed operation through retirement.
	// App.mu never encloses an actual CancelFunc or a cancellation join.
	a.mu.Lock()
	owner := a.operationCancellation
	valid := owner != nil && a.operationCancel != nil && cancel != nil
	a.mu.Unlock()
	if !valid {
		panic(core.ErrEngineCleanupFailed)
	}
	cancel()
	if err := owner.join(); err != nil {
		panic(core.ErrEngineCleanupFailed) // retain original slot on abnormal/unknown cancellation
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.operationCancellation != owner || a.operationCancel == nil {
		panic(core.ErrEngineCleanupFailed) // never clear a different owner's slot
	}
	a.operationCancel = nil
	a.operationCancellation = nil
}

// Caller still owns operationMu. Network cancellation has no separate Stop
// owner, unlike explicit Disconnect: retire any partially applied lifecycle
// work instead of leaving an orphaned starting/reloading state. A successful
// already-published session remains detached from the operation context.
func (a *App) finishCanceledLifecycle(ctx context.Context, intent uint64) {
	a.finishCanceledLifecycleCause(ctx, intent, nil)
}

func (a *App) finishCanceledLifecycleCause(ctx context.Context, intent uint64, cause error) lifecycleRetirementReceipt {
	if ctx == nil || ctx.Err() == nil {
		return lifecycleRetirementReceipt{}
	}
	a.mu.Lock()
	needsCleanup := a.operationID == intent && a.protectionDesired && !a.profileTransition && !a.closing.Load() && (a.state.State == "starting" || a.state.State == "reloading")
	a.mu.Unlock()
	if needsCleanup {
		err := errors.Join(ctx.Err(), cause)
		if tunRequested() {
			// Cancellation can precede the first render-time Arm. The pending
			// Connect still requests protection until an explicit Disconnect.
			err = errors.Join(err, a.armProtection(ctx))
		}
		return a.failCoreForIntent(intent, "операция жизненного цикла отменена", err)
	}
	return lifecycleRetirementReceipt{}
}

func (a *App) stopCoreFor(reason StopReason) error {
	request, err := func() (stopRequest, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if reason.releasesProtection() {
			a.retireUILifecycleLocked()
		}
		return a.registerStopLocked(reason)
	}()
	if err != nil {
		return err
	}
	if err := a.acquireStopRequest(request); err != nil {
		return err // cancellation uncertainty cannot authorize cleanup/release
	}
	defer a.operationMu.Unlock()
	return a.stopCoreRegisteredLocked(reason, request.intentRegistration)
}

type stopRequest struct {
	intentRegistration
	wasRunning        bool
	resumeRequested   bool
	profileTransition bool
	cancellation      *appOperationCancellation
	cancelUnavailable bool
}

func (request stopRequest) cancelOriginal() {
	if request.cancelUnavailable {
		panic(core.ErrEngineCleanupFailed)
	}
	if request.cancellation != nil {
		request.cancellation.request()
	}
}

// operationMu held. The original operation may already have removed its own
// slot; this captured record still proves the exact callback's actual return.
func (request stopRequest) joinOriginal() error {
	if request.cancelUnavailable {
		return core.ErrEngineCleanupFailed
	}
	if request.cancellation != nil {
		return request.cancellation.join()
	}
	return nil
}

// Caller holds App.mu. Capture resume facts with the exact stop registration,
// before state becomes stopping. Restart and ordinary Stop share this path.
func (a *App) registerStopLocked(reason StopReason) (stopRequest, error) {
	if _, err := a.stopRejectionLocked(reason); err != nil {
		return stopRequest{}, err
	}
	request := stopRequest{wasRunning: a.state.State == "running", resumeRequested: a.protectionDesired, profileTransition: a.profileTransition}
	if err := a.advanceOperationLocked(); err != nil {
		return stopRequest{}, err
	}
	request.intentRegistration = a.intentRegistrationLocked()
	request.cancellation = a.operationCancellation
	request.cancelUnavailable = (a.operationCancel == nil) != (request.cancellation == nil)
	if reason.releasesProtection() {
		a.protectionDesired = false
		a.profileTransition = false
		a.trustBlocked = false
	}
	a.cancelFailClosedRetryLocked()
	a.state.State = "stopping"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.nativeObservation = nil
	// Registration cannot depend on a Wails event. The serialized Stop publishes
	// stopping after all negative cleanup stages have been registered.
	return request, nil
}

func (a *App) sessionCurrent(cc *render.ClientConfig) bool {
	runtime_.mu.Lock()
	ours := cc != nil && runtime_.cc == cc && runtime_.manager != nil
	runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	return ours && a.startupErr == nil && !a.watchdogBlocked.Load() && a.protectionDesired && !a.profileTransition && !a.closing.Load()
}

// A watchdog result belongs to its captured session. Recheck after acquiring
// the lifecycle lock: it must never tear down a new session or undo Disconnect.
func (a *App) failSession(cc *render.ClientConfig, report *core.ProbeReport, reason string, exhausted bool) bool {
	return a.stopObservedSession(cc, report, reason, exhausted, observationStopping)
}

func (a *App) failUnsafeSession(cc *render.ClientConfig, report *core.ProbeReport, reason string) bool {
	return a.stopObservedSession(cc, report, reason, false, observationSafetyStopping)
}

func (a *App) stopObservedSession(cc *render.ClientConfig, report *core.ProbeReport, reason string, exhausted bool, phase observationPhase) bool {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if !a.sessionCurrent(cc) {
		return false
	}
	var appliedIntent uint64
	var appliedSource *core.OperationSource
	var lifetimeCause error
	if report != nil {
		if !a.publishObservationLocked(cc, report, phase, "") {
			return false
		}
	} else {
		// A missing probe is not permission to stop a healthy owner. This
		// report-free path is reserved for actual engine/lifecycle failure.
		runtime_.mu.Lock()
		failedOwner := runtime_.cc == cc && runtime_.manager != nil && runtime_.manager.State() == core.StateError
		var scope *core.ProbeReport
		if failedOwner {
			scope = runtime_.manager.LastProbeReport()
			lifetimeCause = runtime_.manager.LifetimeError()
		}
		runtime_.mu.Unlock()
		if !failedOwner || scope == nil || scope.SessionID != cc.SessionID {
			return false
		}
		appliedIntent = scope.IntentGeneration // identity only, not reused health proof
		appliedSource = scope.OperationSource()
	}
	a.mu.Lock()
	intent := a.operationID
	valid := report != nil && a.reportIntentCurrentLocked(report) || report == nil && appliedIntent == intent && appliedSource == a.intentSource && appliedSource.Current()
	a.mu.Unlock()
	if !valid {
		return false
	}
	if lifetimeCause != nil {
		a.failTrustCoreIntentLocked(intent, lifetimeCause)
		return true
	}
	err := a.stopCoreGenerationLocked(stopFailure, intent)
	if report != nil && report.CleanupUnconfirmed {
		err = errors.Join(core.ErrEngineCleanupFailed, err) // the observed fault is independent of a later Stop returning nil
	}
	if err != nil {
		err = errors.Join(core.ErrEngineCleanupFailed, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.protectionDesired || a.profileTransition || a.closing.Load() || a.operationID != intent {
		return false
	}
	a.state.State = "error"
	a.state.Error = reason
	if err != nil {
		a.state.Error += "; cleanup: " + err.Error()
	} else if exhausted {
		a.state.State = "stopped"
		a.state.BlockedReason = "all_channels_failed"
	}
	netEpoch.mu.Lock()
	pathCurrent := reportPathCurrentLocked(report)
	netEpoch.mu.Unlock()
	if report != nil && !pathCurrent {
		report = report.Clone()
		report.Stale, report.AllPassed = true, false
		report.Results = append(report.Results, core.ProbeResult{Name: "путь наблюдения", Status: "unavailable", Detail: "исходный сетевой путь отозван; остановка принята только для прежнего владельца ресурсов"})
	}
	a.state.Probe = toView(report)
	if err != nil {
		a.state.BlockedReason = "cleanup_unconfirmed"
		a.cancelFailClosedRetryLocked()
		a.sm.markBlocked(time.Now())
	} else {
		a.scheduleFailClosedRetryLocked(time.Now())
		a.sm.markRecovering(time.Now())
	}
	a.pushStateLocked()
	return true
}
