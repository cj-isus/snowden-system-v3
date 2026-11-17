package main

import (
	"context"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// Captured at event delivery, before waiting for operationMu. Neither a later
// Start nor a replacement worker/observer may become this event's subject.
type serviceDNSChange struct {
	kind       string
	path       *networkPathSource
	event      *core.OperationSource
	observer   *serviceDNSHandoff
	intent     uint64
	source     *core.OperationSource
	config     *render.ClientConfig
	manager    *core.Manager
	engine     *core.Engine
	owner      uint64
	generation uint64
}

func (a *App) captureServiceDNSChange(observer *serviceDNSHandoff, kind string) *serviceDNSChange {
	if observer == nil || observer.app != a || !observer.source.Current() || !observer.contextCurrent() {
		return nil
	}
	// Reuse the periodic probe's applied-identity reader. Its historical report
	// supplies identity only; the actual delivered event is bound separately.
	runtime_.mu.Lock()
	cc, busy := runtime_.cc, runtime_.startBusy
	runtime_.mu.Unlock()
	var subject appProbeSubject
	if !busy {
		subject = a.appliedProbeSubject(cc)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	change := &serviceDNSChange{kind: kind, observer: observer, intent: a.operationID, source: a.intentSource}
	if !change.intentCurrentLocked(a) {
		return nil
	}
	if cc != nil && subject.manager != nil && subject.engine != nil && subject.intent == change.intent && subject.intentSource == change.source &&
		subject.owner == cc.OwnerID && subject.generation == cc.OwnerGeneration {
		change.config, change.manager, change.engine = cc, subject.manager, subject.engine
		change.owner, change.generation = subject.owner, subject.generation
	}
	return change // no actor remains no actor; apply never looks up a successor
}

func (change *serviceDNSChange) bindEvent(event networkControlEvent) *serviceDNSChange {
	if change == nil || change.kind != event.kind {
		return nil
	}
	bound := *change
	bound.path, bound.event = event.path, event.source
	return &bound // original request/actor and exact delivered event, no recapture
}

// App.mu held. These are request/custody restrictions, never path evidence.
func (change serviceDNSChange) intentCurrentLocked(a *App) bool {
	return a.appLifetimeCurrentLocked() && change.intent != 0 && a.operationID == change.intent &&
		a.intentSource == change.source && change.source.Current() && a.protectionDesired && !a.profileTransition &&
		!a.closing.Load() && !a.trustBlocked && a.restartCommit == nil && !a.installedImageChanged
}

// epoch.mu held. Down may control DNS with a revoked physical path; it never
// authorizes a probe or positive publication. Power loss denies both directions.
func (change serviceDNSChange) eventCurrentLocked() bool {
	return (change.kind == "down" || change.kind == "up" || change.kind == "change") &&
		change.path != nil && change.path == netEpoch.path && change.event != nil && change.event == netEpoch.controlSource &&
		change.event.Current() && !netEpoch.powerBlocked.Load() && (change.kind == "down") == netEpoch.knownDown &&
		(change.kind == "down" || change.path.current.Load())
}

func (change serviceDNSChange) current() bool {
	if change.observer == nil || !change.observer.source.Current() || !change.observer.contextCurrent() || !change.source.Current() {
		return false
	}
	netEpoch.mu.Lock()
	defer netEpoch.mu.Unlock()
	return change.eventCurrentLocked()
}

// Value-only actor check. Actor-only mode permits cleanup of the same failed worker
// after the event/task is canceled, without adopting a newer request or actor.
func (change serviceDNSChange) actorCurrent(a *App, requireEvent bool) bool {
	if change.config == nil || change.manager == nil || change.engine == nil {
		return false
	}
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	if runtime_.cc != change.config || runtime_.manager != change.manager || runtime_.engine != change.engine {
		return false
	}
	owner, generation := change.manager.Owner()
	if owner != change.owner || generation != change.generation || requireEvent && change.manager.State() != core.StateRunning {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return change.intentCurrentLocked(a) && (!requireEvent || change.current())
}

func (change serviceDNSChange) controlContext(ctx context.Context, a *App) context.Context {
	if change.observer == nil {
		return core.WithServiceDNSControlSources(ctx, nil, nil, nil, &a.closing, &netEpoch.powerBlocked)
	}
	return core.WithServiceDNSControlSources(ctx, change.source, change.event, change.observer.source, &a.closing, &netEpoch.powerBlocked)
}

// Fresh measurement retains the actual physical path, plus the original event
// and task as additional negative fences. No lifecycle operation scope is made.
func (change serviceDNSChange) reactionContext(a *App) context.Context {
	if change.kind == "down" || !change.current() || change.path == nil || change.observer == nil {
		return nil
	}
	ctx := context.WithValue(change.observer.ctx, networkPathContextKey{}, change.path)
	return core.WithOperationSources(ctx, core.OperationSources{Intent: change.source, Operation: change.event,
		Dependency: change.observer.source, Path: &change.path.current, Closing: &a.closing})
}

func (a *App) reactToServiceDNSChange(change serviceDNSChange) {
	ctx := change.reactionContext(a)
	if !lifecyclePublicationCurrent(ctx) {
		return
	}
	if change.config != nil {
		if change.actorCurrent(a, true) {
			a.netChangeReprobeFor(ctx, change.config)
		}
		return
	}
	// The existing retry owns the episode and per-path budget. A pending Start
	// or newly appearing actor is never measured by this actor-less event.
	a.wakeFailClosedRetryFor(&change)
}
