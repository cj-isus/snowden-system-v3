package core

import (
	"context"
	"errors"
	"time"
)

// Remains closed until the documented Windows/WFP/process acceptance is run
// and reviewed. No runtime setting, report, caller boolean or linker override
// can enable this unverified path. Source implementation is not acceptance.
const nativeTUNForwardingAccepted = false

func NativeTUNForwardingAvailable() bool { return nativeTUNForwardingAccepted }

// A platform commit must return an opaque receipt from the actual native
// consume RPC. A successful callback/report alone is insufficient for Manager.
type NativeForwardingProbe interface {
	LifecycleProbe
	CommitNativeForwarding(context.Context, *Engine) (NativeTUNAdmission, error)
}

type nativeForwardingCommitKey struct{}
type nativeForwardingCommit struct {
	manager           *Manager
	engine            *Engine
	owner, generation uint64
	done              <-chan struct{}
}

func (commit *nativeForwardingCommit) current(ctx context.Context, engine *Engine) bool {
	if commit == nil || commit.manager == nil || commit.engine != engine || contextError(ctx) != nil || commit.done == nil || ctx.Done() != commit.done || ctx.Value(nativeForwardingCommitKey{}) != commit {
		return false
	}
	release, ok := commit.manager.GuardLifecycleOwner(commit.owner, commit.generation, engine)
	if ok {
		release() // never hold Manager locks across Engine/IPC/platform work
	}
	return ok && contextError(ctx) == nil
}

func (engine *Engine) nativePreparationCurrent(ctx context.Context) bool {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.nativePreparationLocked(ctx, nil)
}

// Caller holds Engine.mu. Source identity and the private worker verdict are
// independent of any public report. The worker rechecks its own deadlines.
func (engine *Engine) nativePreparationLocked(ctx context.Context, source *nativeTUNObservationSource) bool {
	permit := engine.nativePermit
	if !permit.busyLocked(engine) || contextError(ctx) != nil || ctx.Done() == nil || permit.source.operation.Done() != ctx.Done() ||
		source != nil && source != permit.source || !probePassed(permit.diagnostic) {
		return false
	}
	actual, err := configurationSubject(engine.rawJSON)
	origin := permit.source
	return err == nil && actual == permit.subject && origin.engine == engine && origin.scope != nil && origin.scope.closed && !origin.scope.retired &&
		origin.scope.process == engine.process && origin.scope.instance == permit.instance && engine.tunScope == nil &&
		origin.runtime.Instance == permit.instance && origin.runtime.AdmissionEpoch == permit.epoch && !origin.runtime.ForwardingAdmitted &&
		origin.scope.request.Subject == permit.subject && origin.scope.request.Selector == permit.selector && origin.scope.request.Epoch == permit.epoch
}

// Read-only precondition before the compiled platform attempts a WFP grant.
// Only Manager creates the private commit context, for this original operation.
func (engine *Engine) ValidateNativeForwardingContext(ctx context.Context, observation NativeTUNObservation) error {
	if engine == nil || !NativeTUNForwardingAvailable() || contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	commit, _ := ctx.Value(nativeForwardingCommitKey{}).(*nativeForwardingCommit)
	if !commit.current(ctx, engine) || observation.source == nil {
		return ErrNativeScopeUnavailable
	}
	engine.mu.Lock()
	valid := engine.nativePreparationLocked(ctx, observation.source)
	engine.mu.Unlock()
	if !valid || !commit.current(ctx, engine) {
		return ErrNativeScopeUnavailable
	}
	return nil
}

// No exported fields/constructor. A copy cannot mint another admission or
// survive the original operation/Manager owner/process/config/phase change.
type NativeTUNAdmission struct {
	engine *Engine
	permit *engineNativeTUNPermit
	commit *nativeForwardingCommit
}

func (admission NativeTUNAdmission) current(ctx context.Context, manager *Manager, engine *Engine) bool {
	if admission.engine != engine || admission.permit == nil || admission.commit == nil || admission.commit.manager != manager || !admission.commit.current(ctx, engine) {
		return false
	}
	permit := admission.permit
	engine.mu.Lock()
	original := engine.isolated && engine.cleanupErr == nil && engine.process == permit.process && engine.process != nil && !engine.process.exited()
	engine.mu.Unlock()
	if !original || permit.source == nil {
		return false
	}
	identity, err := engine.CurrentRuntime(ctx)
	expected := permit.source.runtime
	expected.ForwardingAdmitted = true // explicit completed phase; all other fields stay exact
	return err == nil && identity == expected && admission.commit.current(ctx, engine)
}

// Called by the compiled platform only after its exact WFP-ready phase. Both
// parent lanes are burned before every outcome; ordinary admit cannot do this.
func (engine *Engine) CommitPreparedNative(ctx context.Context, observation NativeTUNObservation) (NativeTUNAdmission, error) {
	if engine == nil {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable
	}
	var commit *nativeForwardingCommit
	if ctx != nil {
		commit, _ = ctx.Value(nativeForwardingCommitKey{}).(*nativeForwardingCommit)
	}
	managerCurrent := commit.current(ctx, engine)
	engine.mu.Lock()
	valid := engine.nativePreparationLocked(ctx, observation.source)
	permit := engine.nativePermit
	engine.clearForwardingPermitsLocked()
	defer engine.mu.Unlock()
	if !NativeTUNForwardingAvailable() || !managerCurrent || !valid || observation.source == nil || permit.source != observation.source {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable
	}
	process := engine.process
	command := workerRequest{Action: "admit_native", Selector: "proxy", Subject: permit.subject, Tag: permit.selector, AdmissionEpoch: permit.epoch,
		ScopeNonce: permit.source.scope.request.Nonce, NativeAdmissionToken: permit.token}
	reply, err := process.request(ctx, command, workerControlTimeout)
	if err != nil || contextError(ctx) != nil || engine.process != process || process.exited() || !validWorkerControlReply(&command, reply) {
		return NativeTUNAdmission{}, engine.retireIsolatedFailureLocked(ErrForwardingNotAdmitted, err, contextError(ctx))
	}
	return NativeTUNAdmission{engine: engine, permit: permit, commit: commit}, nil
}

func (engine *Engine) consumeLocalNativeForwarding(ctx context.Context, subject, selector string, epoch uint64, nonce, token string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	actual, selected, guarded, tunMode, err := engine.forwardingSubjectLocked()
	if err != nil || guarded == nil {
		return ErrForwardingNotAdmitted
	}
	gate := guarded.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	permit := gate.nativePending
	valid := permit.currentLocked(gate, subject, selector, epoch, nonce)
	gate.clearPendingLocked() // remove original slot before ANY consume outcome
	if !NativeTUNForwardingAvailable() || contextError(ctx) != nil || !tunMode || actual != subject || selected != selector || !valid ||
		!permit.emitted || !workerSubject(token) || permit.token != token || !time.Now().Before(permit.until) || !permit.native.current(subject, selector, epoch, nonce) {
		return ErrForwardingNotAdmitted
	}
	gate.open.Store(true)
	if contextError(ctx) != nil {
		gate.open.Store(false)
		return ErrForwardingNotAdmitted
	}
	return nil
}

// This recognizes only the explicit pending activation step. No independent
// protection/cleanup/trust/leak/stale flag is cleared or inferred away.
func nativeReportWithoutActivation(report *ProbeReport) *ProbeReport {
	if report == nil || report.AllPassed {
		return nil
	}
	out := report.Clone()
	out.Results = out.Results[:0]
	pending := 0
	for _, step := range report.Results {
		if step.Name == "TUN activation" {
			expected := ProbeResult{Name: "TUN activation", Status: "unavailable", Detail: nativeActivationPendingDetail, Requirement: step.Requirement}
			if step.Requirement != "" && step.Requirement != ProbeRequired || step != expected {
				return nil
			}
			pending++
			continue
		}
		out.Results = append(out.Results, step)
	}
	out.AllPassed = true
	if pending != 1 || !probePassed(out) {
		return nil
	}
	return out
}

func lifecycleProbePassed(ctx context.Context, engine *Engine, probe Probe, report *ProbeReport) bool {
	if probePassed(report) {
		return true
	}
	_, native := probe.(NativeForwardingProbe)
	return NativeTUNForwardingAvailable() && native && engine != nil && engine.nativePreparationCurrent(ctx) && nativeReportWithoutActivation(report) != nil
}

func (manager *Manager) commitEngineForwarding(ctx context.Context, engine *Engine, probe Probe, report *ProbeReport) (finalErr error) {
	if err := manager.validateAppliedProxy(ctx); err != nil {
		manager.mu.Lock()
		manager.markTerminalFailureLocked(err) // no stale positive route report after local proxy conflict
		manager.mu.Unlock()
		return err
	}
	if engine == nil || !engine.nativePreparationCurrent(ctx) {
		return commitEngineForwarding(ctx, engine, probe)
	}
	defer func() {
		if finalErr != nil {
			failure := report.Clone()
			if failure == nil {
				failure = workerProbeFailure()
			}
			failure.AllPassed, failure.ProtectionUnavailable = false, true
			failure.CleanupUnconfirmed = failure.CleanupUnconfirmed || errors.Is(finalErr, ErrGUIAncestorUnavailable) || errors.Is(finalErr, ErrEngineCleanupFailed) || errors.Is(finalErr, ErrHelperCleanup) || errors.Is(finalErr, ErrHelperBusy)
			manager.rememberProbe(failure)
		}
	}()
	if err := validateLifecycleSource(ctx, probe); err != nil {
		return err
	}
	committer, ok := probe.(NativeForwardingProbe)
	if !ok || !lifecycleProbePassed(ctx, engine, probe, report) {
		return ErrNativeScopeUnavailable
	}
	if _, err := lifetimeRemaining(probe); err != nil {
		return err
	}
	owner, generation := manager.Owner()
	commit := &nativeForwardingCommit{manager: manager, engine: engine, owner: owner, generation: generation, done: ctx.Done()}
	ctx = context.WithValue(ctx, nativeForwardingCommitKey{}, commit)
	if !commit.current(ctx, engine) {
		return ErrNativeScopeUnavailable
	}
	admission, err := committer.CommitNativeForwarding(ctx, engine)
	if err != nil {
		return err
	}
	if !admission.current(ctx, manager, engine) {
		return ErrForwardingNotAdmitted
	}
	if _, err := lifetimeRemaining(probe); err != nil {
		return err
	}
	if err := validateLifecycleSource(ctx, probe); err != nil {
		return err
	}
	completed := nativeReportWithoutActivation(report)
	if completed == nil {
		return ErrForwardingNotAdmitted
	}
	completed.Results = append(completed.Results, ProbeResult{Name: "TUN activation", Passed: true, Status: "pass", Detail: "Допуск исходного процесса и текущего туннеля подтверждён после проверки системной защиты."})
	completed.CompletedAt = time.Now().UTC()
	if completed.CompletedAt.Before(completed.StartedAt) || !probePassed(completed) || !commit.current(ctx, engine) {
		return ErrForwardingNotAdmitted
	}
	manager.rememberProbe(completed)
	return nil
}
