package main

import (
	"context"

	"github.com/snowden-system/windows/backend/core"
)

// Telemetry identity only. No copied probe PASS, new intent, path permission,
// secret/config clone, native check or protection grant is manufactured here.
type metricsScope struct {
	app     *App
	parent  context.Context
	metrics *metricsState
	subject appProbeSubject
	control *core.OperationSource
}

func (a *App) captureMetricsScope() *metricsScope {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	parent := a.ctx
	a.mu.Unlock()
	if parent == nil || parent.Err() != nil || !clashAPIBuilt {
		return nil
	}
	runtime_.mu.Lock()
	cc, metrics := runtime_.cc, runtime_.metrics
	runtime_.mu.Unlock()
	if cc == nil || metrics == nil {
		return nil
	}
	scope := &metricsScope{app: a, parent: parent, metrics: metrics, subject: a.appliedProbeSubject(cc)}
	release, ok := scope.subject.lockCurrent(a, parent, cc)
	if !ok {
		return nil
	}
	defer release()
	scope.control = netEpoch.controlSource
	if !scope.currentLocked() {
		return nil
	}
	return scope
}

// Caller holds runtime -> Manager -> App -> epoch, then optionally the metrics
// registry/state lock. Only pointer/value reads run under this lease.
func (scope *metricsScope) currentLocked() bool {
	a := scope.app
	return scope.metrics != nil && scope.metrics.id != "" && runtime_.metrics == scope.metrics && a.state.State == "running" && a.ctx == scope.parent && scope.parent.Err() == nil &&
		a.startupErr == nil && !a.watchdogBlocked.Load() && !a.quitRequested && !a.metricsSealed.Load() &&
		netEpoch.controlSource == scope.control && (scope.control == nil || scope.control.Current())
}

func (scope *metricsScope) lockCurrent() (func(), bool) {
	if scope == nil || scope.app == nil || scope.parent == nil {
		return nil, false
	}
	release, ok := scope.subject.lockCurrent(scope.app, scope.parent, scope.subject.applied)
	if !ok {
		return nil, false
	}
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()
	if !scope.currentLocked() {
		return nil, false
	}
	handedOff = true
	return release, true
}
