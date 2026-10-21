package main

import (
	"context"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Capture the factory with the config, so a retained rollback probe cannot
// silently adopt a different factory from a later session.
type appProtectedProbe struct {
	config  *render.ClientConfig
	factory func(*render.ClientConfig) core.Probe
}

func (a *App) protectedProbe(cc *render.ClientConfig) core.Probe {
	if cc != nil {
		snapshot := *cc
		snapshot.ProbeTargets = append([]string(nil), cc.ProbeTargets...)
		cc = &snapshot
	}
	return appProtectedProbe{config: cc, factory: a.probeFactory}
}

func probeFailure(detail string) *core.ProbeReport {
	return &core.ProbeReport{Results: []core.ProbeResult{{Name: "probe", Status: "fail", Detail: detail}}}
}

func (p appProtectedProbe) Run(ctx context.Context) (report *core.ProbeReport) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// Panic values can contain private configuration. Retain the fault
			// type in diagnostics, without publishing the arbitrary payload.
			report = probeFailure(fmt.Sprintf("protected probe panic (%T)", recovered))
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
	}()
	if ctx == nil {
		return probeFailure("probe context is nil")
	}
	if err := ctx.Err(); err != nil {
		return &core.ProbeReport{} // finalizer adds the context failure once
	}
	if p.config == nil {
		return probeFailure("probe config is not available")
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
	return probe.Run(ctx)
}

// Facts outrank the aggregate flag. Work on a copy: a caller's report and result
// slice must not be rewritten by UI conversion or another consumer.
func normalizeProbeReport(report *core.ProbeReport) *core.ProbeReport {
	if report == nil {
		return probeFailure("empty report")
	}
	out := *report
	out.Results = append([]core.ProbeResult(nil), report.Results...)
	leakStep := false
	failedStep := false
	for i := range out.Results {
		step := &out.Results[i]
		if step.Status == "leak" {
			out.LeakConfirmed = true
			leakStep = true
		}
		if step.Status != "" && step.Status != "pass" {
			step.Passed = false
		}
		if !step.Passed {
			out.AllPassed = false
			failedStep = true
			if step.Status == "pass" {
				step.Status = "fail"
			}
		}
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

// Preserve the established direct-leak semantics; all App entrypoints execute
// this configuration through protectedProbe's shared guard.
func newAppProbe(cc *render.ClientConfig) *core.ProtectedProbe {
	targets := cc.ProbeTargets
	if len(targets) < 2 {
		targets = render.DefaultProbeTargets
	}
	return &core.ProtectedProbe{
		SOCKSAddr:               "127.0.0.1:1080",
		Targets:                 append([]string(nil), targets...),
		Timeout:                 10 * time.Second,
		ExpectedEgress:          cc.ExpectedEgress,
		DirectLeakThroughTunnel: tunRequested(),
	}
}

func toView(report *core.ProbeReport) *ProbeReportView {
	if report == nil {
		return nil
	}
	r := normalizeProbeReport(report)
	out := &ProbeReportView{OK: r.AllPassed, Steps: []ProbeStepView{}}
	for _, res := range r.Results {
		status := "pass"
		if !res.Passed {
			status = "fail"
		}
		if res.Status != "" {
			status = res.Status
		}
		out.Steps = append(out.Steps, ProbeStepView{Name: res.Name, Status: status, Detail: res.Detail})
	}
	return out
}

func (a *App) probeCore() error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	id := a.operationID
	a.mu.Unlock()
	ctx, cancel, ok := a.beginOperation(id)
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
	a.mu.Lock()
	if a.operationID != id || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.ProbeRunning = true
	a.pushStateLocked()
	a.mu.Unlock()
	defer func() {
		ours := currentSession()
		a.mu.Lock()
		defer a.mu.Unlock()
		if ours && a.operationID == id && a.state.ProbeRunning {
			a.state.ProbeRunning = false
			a.pushStateLocked()
		}
	}()

	report := a.protectedProbe(cc).Run(ctx)
	if !currentSession() || manager.State() != core.StateRunning {
		return errSessionSuperseded
	}
	a.mu.Lock()
	if a.operationID != id || ctx.Err() != nil {
		a.mu.Unlock()
		return context.Canceled
	}
	a.state.ProbeRunning = false
	a.state.Probe = toView(report)
	if report.AllPassed {
		a.state.ProbeLastAt = time.Now().Format(time.RFC3339)
	}
	a.pushStateLocked()
	a.mu.Unlock()
	level := "info"
	if !report.AllPassed {
		level = "warn"
	}
	a.appendLog(level, "probe: "+report.Summary())
	if report.LeakConfirmed {
		a.failCore("probe: подтверждён direct leak", fmt.Errorf("%s", report.Summary()))
		return fmt.Errorf("probe: unsafe protected path")
	}
	if !report.AllPassed {
		return fmt.Errorf("probe: %s", report.Summary())
	}
	return nil
}
