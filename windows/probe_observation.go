package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Capture the factory with the config, so a retained rollback probe cannot
// silently adopt a different factory from a later session.
type appProtectedProbe struct {
	app          *App
	config       *render.ClientConfig
	factory      func(*render.ClientConfig) core.Probe
	boundary     tunnelBoundary
	lifecycle    bool
	subject      appProbeSubject
	nativeCommit *appNativeCommitSlot
}

func (a *App) protectedProbe(cc *render.ClientConfig) core.Probe {
	return a.newProtectedProbe(cc, a.appliedProbeSubject(cc))
}

func (a *App) protectedOperationProbe(ctx context.Context, cc *render.ClientConfig, manager *core.Manager) core.Probe {
	intent, valid := a.operationIntent(ctx)
	var source *core.OperationSource
	if !valid {
		intent = 0 // no fallback to the App's current, possibly newer operation
	} else {
		scope, _ := ctx.Value(appOperationContextKey{}).(*appOperationScope)
		source = scope.intentSource
	}
	return a.newProtectedProbe(cc, appProbeSubject{operation: true, manager: manager, intent: intent, intentSource: source})
}

func (a *App) newProtectedProbe(cc *render.ClientConfig, subject appProbeSubject) core.Probe {
	if cc != nil {
		cc = cc.Clone()
	}
	return appProtectedProbe{app: a, config: cc, factory: a.probeFactory, boundary: a.sessionTunnelBoundary(), subject: subject, nativeCommit: &appNativeCommitSlot{}}
}

func (a *App) sessionTunnelBoundary() tunnelBoundary {
	if a.tunnelBoundary != nil {
		return a.tunnelBoundary
	}
	if a.probeFactory != nil {
		return nil // no implicit native calls for local/custom probe fixtures
	}
	return nativeTunnelBoundary{policy: a.protectionPolicy(), app: a}
}

func (p appProtectedProbe) BeforeEngineChange(ctx context.Context) error {
	p.nativeCommit.clear() // every new preparation invalidates any earlier lease
	if err := p.ValidateLifecycle(ctx); err != nil {
		return err
	}
	return p.restrictBeforeEngineStop(ctx)
}

func (p appProtectedProbe) ValidateLifecycle(ctx context.Context) error {
	subject, valid := p.lifecycleSubject(ctx)
	if !valid || !subject.current(p.app, ctx, p.config) {
		return errSessionSuperseded
	}
	return nil
}

// Negative cleanup uses the retained boundary even after the App intent or
// original operation ctx has retired. It cannot prepare/commit forwarding.
func (p appProtectedProbe) BeforeEngineStop(ctx context.Context) error {
	p.nativeCommit.clear()
	return p.restrictBeforeEngineStop(ctx)
}

func (p appProtectedProbe) restrictBeforeEngineStop(ctx context.Context) error {
	if p.config == nil {
		return fmt.Errorf("missing lifecycle protection configuration")
	}
	if p.config.TUNName == "" {
		return nil
	}
	if p.boundary == nil {
		return fmt.Errorf("TUN lifecycle requires an explicit protection boundary")
	}
	if err := p.boundary.BeforeChange(ctx); err != nil {
		return err
	}
	return nil
}

func (p appProtectedProbe) RunLifecycle(ctx context.Context) *core.ProbeReport {
	p.lifecycle = true // value copy; periodic Run never adds external traffic
	return p.Run(ctx)
}

func (p appProtectedProbe) LifetimeRemaining() (time.Duration, error) {
	if p.app == nil {
		return 0, &core.TrustFaultError{Reason: "trust_unavailable"}
	}
	return p.app.sessionLifetime(p.config)
}

func tunnelProtectionFailure(causes ...error) *core.ProbeReport {
	report := &core.ProbeReport{ProtectionUnavailable: true, Results: []core.ProbeResult{{Name: "защита TUN/WFP", Status: "unavailable", Detail: "защита применённой сессии и внешний IPv4-путь не подтверждены"}}}
	for _, cause := range causes {
		if errors.Is(cause, core.ErrGUIAncestorUnavailable) || errors.Is(cause, core.ErrHelperCleanup) || errors.Is(cause, core.ErrHelperBusy) || errors.Is(cause, core.ErrEngineCleanupFailed) {
			report.CleanupUnconfirmed = true
			report.Results[0].Detail = "владение или завершение native-ресурсов не подтверждены; автоматическая смена канала заблокирована"
		}
	}
	return report
}

var probeObservationSequence atomic.Uint64

func (a *App) probeIntent() (uint64, bool) {
	if a == nil {
		return 0, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.operationID, a.protectionDesired && !a.profileTransition && !a.closing.Load()
}

func probeFailure(detail string) *core.ProbeReport {
	return &core.ProbeReport{Results: []core.ProbeResult{{Name: "probe", Status: "fail", Detail: detail}}}
}

// Retain the original attempted subject for a failure in the report finalizer
// itself. It cannot become positive evidence or adopt a newer global subject.
type protectedProbeAttempt struct {
	started      time.Time
	observation  uint64
	subject      appProbeSubject
	subjectValid bool
	workStarted  bool
	scope        context.Context // original source-bound descendant, never a new subject
	native       *appNativeProbePublication
}

func (p appProtectedProbe) Run(ctx context.Context) (report *core.ProbeReport) {
	attempt := &protectedProbeAttempt{started: time.Now(), observation: probeObservationSequence.Add(1)}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			report = probeFailure(fmt.Sprintf("protected probe did not return normally (%T)", payload))
			report.CleanupUnconfirmed = true
			report.Stale = true // only original restrictive custody may consume this failed completion
			subject := attempt.subject
			report.BindOperationSource(subject.intentSource)
			if subject.path != nil {
				report.BindObservationPath(&subject.path.current, attempt.subjectValid && attempt.workStarted)
			}
			report.StartedAt, report.CompletedAt = attempt.started.UTC(), time.Now().UTC()
			report.OwnerID, report.OwnerGeneration = subject.owner, subject.generation
			report.IntentGeneration, report.ObservationID = subject.intent, attempt.observation
			report.NetworkEpoch = subject.epoch
			if p.config != nil {
				report.SessionID = p.config.SessionID
			}
		}
	}()
	defer func() {
		if !returned {
			attempt.native.discard() // independent of abnormal report formatting/getters
		}
	}()
	if p.lifecycle && p.config != nil && p.config.TUNName != "" {
		attempt.native = p.nativeCommit.reserveProbe() // before caller/factory/native callbacks
	}
	report = p.runProtectedProbe(ctx, attempt)
	if err := p.publishNativeProbe(attempt, report); err != nil {
		attempt.native.discard()
		failure := tunnelProtectionFailure(err)
		report = report.Clone()
		if report == nil {
			report = &core.ProbeReport{}
		}
		report.AllPassed, report.Stale, report.ProtectionUnavailable = false, true, true
		report.CleanupUnconfirmed = report.CleanupUnconfirmed || failure.CleanupUnconfirmed
		report.Results = append(report.Results, failure.Results...)
		if attempt.subject.path != nil {
			report.BindObservationPath(&attempt.subject.path.current, attempt.subjectValid && attempt.workStarted)
		}
	}
	returned = true
	return report
}

func (p appProtectedProbe) runProtectedProbe(ctx context.Context, attempt *protectedProbeAttempt) (report *core.ProbeReport) {
	started, observation := attempt.started, attempt.observation
	var subject appProbeSubject
	var subjectValid bool
	workStarted := false
	defer func() {
		if recovered := recover(); recovered != nil {
			// Panic values can contain private configuration. Retain the fault
			// type in diagnostics, without publishing the arbitrary payload.
			report = probeFailure(fmt.Sprintf("protected probe panic (%T)", recovered))
			report.CleanupUnconfirmed = true // local callback failure is not remote channel evidence
			if p.lifecycle && p.config != nil && p.config.TUNName != "" {
				report.ProtectionUnavailable = true
			}
		}
		if ctx != nil && ctx.Err() != nil {
			if report == nil {
				report = &core.ProbeReport{}
			}
			out := *report
			out.AllPassed = false
			out.Results = append(append([]core.ProbeResult(nil), report.Results...), core.ProbeResult{Name: "контекст", Status: "fail", Detail: ctx.Err().Error()})
			report = &out
		}
		report = normalizeProbeReport(report)
		report.BindOperationSource(subject.intentSource)
		if subject.path != nil {
			report.BindObservationPath(&subject.path.current, workStarted && unsafeObservation(report))
		}
		if _, err := p.LifetimeRemaining(); err != nil {
			report.AllPassed, report.TrustUnavailable = false, true
			report.TrustReason = trustFailureCode(err)
			report.Results = append(report.Results, trustFailureReport(err).Results...)
		}
		report.StartedAt, report.CompletedAt = started.UTC(), time.Now().UTC()
		report.NetworkEpoch = subject.epoch
		report.OwnerID, report.OwnerGeneration = subject.owner, subject.generation
		report.IntentGeneration, report.ObservationID = subject.intent, observation
		if p.config != nil {
			report.SessionID = p.config.SessionID
		}
		if !subjectValid || !subject.current(p.app, ctx, p.config) || report.CompletedAt.Before(report.StartedAt) {
			report.Stale = true
			report.AllPassed = false
			report.Results = append(report.Results, core.ProbeResult{Name: "владелец наблюдения", Status: "unavailable", Detail: "intent/network/session/clock changed during probe; observation is stale"})
		}
	}()
	if ctx == nil {
		return probeFailure("probe context is nil")
	}
	if err := ctx.Err(); err != nil {
		return &core.ProbeReport{} // finalizer adds the context failure once
	}
	subject, subjectValid = p.runSubject(ctx)
	attempt.subject, attempt.subjectValid = subject, subjectValid
	if subjectValid && subject.path != nil && p.app != nil {
		ctx = core.WithOperationSources(ctx, core.OperationSources{Intent: subject.intentSource, Path: &subject.path.current, Closing: &p.app.closing, Blocked: &p.app.watchdogBlocked})
	}
	attempt.scope = ctx
	if !subjectValid || !subject.current(p.app, ctx, p.config) {
		return &core.ProbeReport{ProtectionUnavailable: true, Results: []core.ProbeResult{{Name: "владелец наблюдения", Status: "unavailable", Detail: "исходная операция, сессия или сетевой путь отозваны; наблюдение не запускается"}}}
	}
	if p.config == nil {
		return probeFailure("probe config is not available")
	}
	if p.lifecycle && p.config.TUNName != "" && attempt.native == nil {
		return tunnelProtectionFailure() // another preparation/revocation cannot be replaced
	}
	if _, err := p.LifetimeRemaining(); err != nil {
		return trustFailureReport(err)
	}
	var probe core.Probe
	if p.factory != nil {
		probe = p.factory(p.config)
	} else {
		probe = newAppProbe(p.config)
	}
	if probe == nil {
		return probeFailure("probe is not configured")
	}
	if err := ctx.Err(); err != nil {
		return &core.ProbeReport{}
	}
	if !subject.current(p.app, ctx, p.config) {
		return &core.ProbeReport{} // a factory cannot move work to a successor
	}
	workStarted = true // early source-gap reports above are not observed safety faults
	attempt.workStarted = true
	if !p.lifecycle || p.config.TUNName == "" {
		return probe.Run(ctx)
	}
	if p.boundary == nil {
		return tunnelProtectionFailure()
	}
	lease, err := p.boundary.Establish(ctx, p.config)
	if err != nil {
		return tunnelProtectionFailure(err)
	}
	if lease.native != nil {
		if p.factory != nil {
			return tunnelProtectionFailure() // custom report cannot confirm original native work
		}
		probe = newAppEngineProbe(p.config, &lease.native.observation)
	}
	report = normalizeProbeReport(probe.Run(ctx))
	if err := p.boundary.Validate(ctx, p.config, lease); err != nil {
		report.AllPassed, report.ProtectionUnavailable = false, true
		failure := tunnelProtectionFailure(err)
		report.CleanupUnconfirmed = report.CleanupUnconfirmed || failure.CleanupUnconfirmed
		report.Results = append(report.Results, failure.Results...)
		return report
	}
	name := "защита TUN/WFP"
	detail := "граница платформы проверена в рамках lifecycle; не native DNS/UDP/IPv6 proof"
	if lease.native != nil {
		name = "Правила блокировки"
		detail = "Правила блокировки повторно проверены для текущей операции после диагностики. Общий трафик ещё не разрешён."
		if err := attempt.native.stage(lease); err != nil {
			return tunnelProtectionFailure(err)
		}
	}
	report.Results = append([]core.ProbeResult{{Name: name, Passed: true, Status: "pass", Detail: detail}}, report.Results...)
	return report
}

func currentProbeOwner() (uint64, uint64) {
	runtime_.mu.Lock()
	manager := runtime_.manager
	runtime_.mu.Unlock()
	if manager == nil {
		return 0, 0
	}
	return manager.Owner()
}

func (a *App) currentProbeReport(cc *render.ClientConfig, report *core.ProbeReport) bool {
	if report == nil || report.Stale || report.RestrictionObserved() || unsafeObservation(report) {
		return false // network/selection consumers never accept actor-only retirement scope
	}
	return a.currentProbeSubject(cc, report, true)
}

func (a *App) currentProbeSubject(cc *render.ClientConfig, report *core.ProbeReport, enforceOrder bool) bool {
	if cc == nil || report == nil || report.ObservationID == 0 || cc.SessionID == 0 || report.SessionID != cc.SessionID {
		return false
	}
	restrictive := restrictiveObservation(report)
	if (report.Stale || unsafeObservation(report) || report.RestrictionObserved()) && !restrictive {
		return false
	}
	if !restrictive {
		if report.TrustUnavailable {
			return false
		}
		if _, err := a.sessionLifetime(cc); err != nil {
			return false
		}
	}
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	manager, engine := runtime_.manager, runtime_.engine
	if runtime_.cc != cc || manager == nil || engine == nil {
		return false
	}
	var releaseOwner func()
	var ok bool
	if restrictive {
		releaseOwner, ok = manager.GuardRetirementOwner(report.OwnerID, report.OwnerGeneration, engine)
	} else {
		releaseOwner, ok = manager.GuardRunningOwner(report.OwnerID, report.OwnerGeneration)
	}
	if !ok {
		return false
	}
	defer releaseOwner()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.reportIntentCurrentLocked(report) || cc.OwnerID != report.OwnerID || cc.OwnerGeneration != report.OwnerGeneration ||
		!restrictive && (a.trustBlocked || a.restartCommit != nil || a.installedImageChanged || enforceOrder && report.ObservationID < a.lastObservation) {
		return false
	}
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	return restrictive || reportPathCurrentLocked(report)
}

// Facts outrank the aggregate flag. Work on a copy: a caller's report and result
// slice must not be rewritten by UI conversion or another consumer.
func normalizeProbeReport(report *core.ProbeReport) *core.ProbeReport {
	if report == nil {
		return probeFailure("empty report")
	}
	out := *report
	out.Results = append([]core.ProbeResult(nil), report.Results...)
	if out.Stale || out.ProtectionUnavailable || out.CleanupUnconfirmed || out.TrustUnavailable || len(out.Results) == 0 {
		out.AllPassed = false
	}
	leakStep := false
	failedStep := false
	required := 0
	for i := range out.Results {
		step := &out.Results[i]
		*step = step.NormalizeDiagnostic()
		if !step.Informational() {
			required++
		}
		if step.Status == "leak" {
			out.LeakConfirmed = true
			leakStep = true
		}
		if step.Status != "" && step.Status != "pass" {
			step.Passed = false
		}
		if step.Observation.Schema != 0 && (step.Observation.Schema != 1 || !step.Observation.BodyComplete || step.Observation.ErrorKind != "") {
			step.Passed = false
		}
		if !step.Passed && !step.Informational() {
			out.AllPassed = false
			failedStep = true
			if step.Status == "pass" {
				step.Status = "fail"
			}
		}
	}
	if required == 0 {
		out.AllPassed = false
	}
	if out.LeakConfirmed {
		out.AllPassed = false
		if !leakStep {
			out.Results = append(out.Results, core.ProbeResult{Name: "защищённый путь", Status: "leak", Detail: "Подтверждена утечка защищённого пути"})
		}
	}
	if !out.AllPassed && !out.LeakConfirmed && !failedStep {
		out.Results = append(out.Results, core.ProbeResult{Name: "probe", Status: "fail", Detail: "protected probe did not confirm the path"})
	}
	return &out
}

// Preserve the declared mode; IP comparison itself is not local leak evidence.
// All App entrypoints execute
// this configuration through protectedProbe's shared guard.
func newAppProbe(cc *render.ClientConfig) core.Probe {
	return newAppEngineProbe(cc, nil)
}

func newAppEngineProbe(cc *render.ClientConfig, native *core.NativeTUNObservation) core.Probe {
	if cc == nil {
		return nil
	}
	targets := cc.ProbeTargets
	if len(targets) < 2 {
		targets = render.DefaultProbeTargets
	}
	settings := core.ProtectedProbe{
		Targets:                 append([]string(nil), targets...),
		DNSProbeTargets:         append([]string(nil), cc.DNSProbeTargets...),
		Timeout:                 10 * time.Second,
		ExpectedEgress:          cc.ExpectedEgress,
		DirectLeakThroughTunnel: cc.TUNName != "",
	}
	runtime_.mu.Lock()
	engine := runtime_.engine
	runtime_.mu.Unlock()
	if native != nil {
		return core.NewEngineProbeWithNativeObservation(engine, cc.Raw, cc.SelectorDefault, settings, *native)
	}
	return core.NewEngineProbe(engine, cc.Raw, cc.SelectorDefault, settings)
}

func toView(report *core.ProbeReport) *ProbeReportView {
	if report == nil {
		return nil
	}
	r := normalizeProbeReport(report)
	out := &ProbeReportView{OK: r.AllPassed, Steps: []ProbeStepView{}, Scope: core.DescribeProbeScope(r)}
	for _, res := range r.Results {
		status := "pass"
		if !res.Passed {
			status = "fail"
		}
		if res.Status != "" {
			status = res.Status
		}
		comparison := res.Comparison
		if comparison != (core.ProbeComparison{}) && !comparison.Valid() {
			comparison = core.ProbeComparison{}
			status, out.OK = "unavailable", false
		}
		out.Steps = append(out.Steps, ProbeStepView{Name: res.Name, Status: status, Detail: res.Detail, Comparison: comparison, Requirement: res.Requirement})
	}
	return out
}

func (a *App) probeCore() (resultErr error) {
	a.mu.Lock()
	outcome := &appLifecycleOutcome{app: a, registration: a.intentRegistrationLocked(), stage: "проверка защищённого пути"}
	a.mu.Unlock()
	defer outcome.finish(&resultErr)
	resultErr = a.probeCoreBody(outcome)
	outcome.returned = true
	return resultErr
}

func (a *App) probeCoreBody(outcome *appLifecycleOutcome) error {
	if !outcome.acquire() {
		return core.ErrEngineCleanupFailed
	}
	if !outcome.originalCurrent() {
		return context.Canceled
	}
	id := outcome.registration.intent
	ctx, cancel, ok := a.beginOperation(id, outcome.registration.source)
	if !ok {
		return fmt.Errorf("probe: защита не включена")
	}
	defer a.endOperation(cancel)
	runtime_.mu.Lock()
	cc, manager := runtime_.cc, runtime_.manager
	runtime_.mu.Unlock()
	if cc == nil || manager == nil || manager.State() != core.StateRunning {
		return fmt.Errorf("probe: VPN не запущен")
	}
	currentSession := func() bool {
		runtime_.mu.Lock()
		defer runtime_.mu.Unlock()
		return runtime_.cc == cc && runtime_.manager == manager
	}
	published := false
	defer func() {
		if !published {
			return
		}
		ours := currentSession()
		a.mu.Lock()
		defer a.mu.Unlock()
		if ours && outcome.registration.sameLocked(a) && a.state.ProbeRunning {
			a.state.ProbeRunning = false
			a.pushStateLocked()
		}
	}()
	a.mu.Lock()
	if !outcome.registration.currentLocked(a) || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.ProbeRunning, published = true, true
	a.mu.Unlock()
	a.pushState() // original busy cleanup was registered before any UI callback

	report := a.protectedProbe(cc).Run(ctx)
	current := a.currentProbeReport(cc, report)
	if restrictiveObservation(report) {
		current = a.currentProbeSubject(cc, report, false)
	}
	if !currentSession() || !current {
		return errSessionSuperseded
	}
	phase := observationMeasured
	if restrictiveObservation(report) {
		phase = observationSafetyStopping
	}
	if (phase == observationMeasured && ctx.Err() != nil) || !a.publishObservationLocked(cc, report, phase, "") {
		return errSessionSuperseded
	}
	level := "info"
	if !report.AllPassed {
		level = "warn"
	}
	a.appendLog(level, "probe: "+report.Summary())
	if report.CleanupUnconfirmed {
		a.failCoreForIntent(id, "probe: возврат ресурсов не подтверждён", errors.Join(core.ErrEngineCleanupFailed, errors.New("probe cleanup unconfirmed")))
		return fmt.Errorf("probe: cleanup unconfirmed")
	}
	if report.LeakConfirmed {
		a.failCoreForIntent(id, "probe: подтверждён direct leak", fmt.Errorf("%s", report.Summary()))
		return fmt.Errorf("probe: unsafe protected path")
	}
	if report.ProtectionUnavailable {
		a.failCoreForIntent(id, "probe: защита TUN/WFP не подтверждена", fmt.Errorf("session protection unavailable"))
		return fmt.Errorf("probe: session protection unavailable")
	}
	if observedEgressDivergence(report) {
		a.failCoreForIntent(id, "probe: egress-адреса не соответствуют сравнению", fmt.Errorf("egress comparison mismatch; cause unknown"))
		return fmt.Errorf("probe: egress comparison mismatch")
	}
	if !report.AllPassed {
		return fmt.Errorf("probe: %s", report.Summary())
	}
	return nil
}
