package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// One original observer task and one pending value, not one goroutine per
// network event. Active IPC finishes before the latest pending event is used.
type serviceDNSHandoff struct {
	app          *App
	poller       *networkPollerOwner // immutable original producer dependency
	contextState appTaskContext
	ctx          context.Context
	source       *core.OperationSource // original task revocation; done alone confirms task return
	cancel       context.CancelFunc
	done         chan struct{}
	started      chan struct{}
	wake         chan struct{}
	mu           sync.Mutex
	pending      *serviceDNSChange
	running      bool                   // short publication lease; never held over IPC, cancellation or cleanup
	stopping     bool                   // explicit task retirement, distinguished from unexpected parent cancellation
	fault        error                  // sticky on this original task; replacement requires done + actual engine retirement
	retirement   networkProducerReceipt // owner.mu; functional fault remains a separate readiness fence
	// First actual runtime/intent fence, retained across poller and DNS finalizers.
	failureScope    *serviceDNSFaultScope
	powerRetirement bool // expected suspend/resume continuity loss, not inferred network failure
}

func newServiceDNSHandoff(app *App, poller *networkPollerOwner) *serviceDNSHandoff {
	state := newAppTaskContext()
	return &serviceDNSHandoff{app: app, poller: poller, contextState: state, source: state.cancellation.operation, cancel: state.cancellation.request, done: make(chan struct{}), started: make(chan struct{}), wake: make(chan struct{}, 1)}
}

func (owner *serviceDNSHandoff) submit(kind string) {
	change := owner.app.captureServiceDNSChange(owner, kind)
	netEpoch.mu.Lock()
	event := networkControlEvent{kind: kind, path: netEpoch.path, source: netEpoch.controlSource}
	netEpoch.mu.Unlock()
	owner.submitChange(change.bindEvent(event))
}

func (owner *serviceDNSHandoff) submitChange(change *serviceDNSChange) {
	if change == nil || change.observer != owner {
		return
	}
	kind := change.kind
	if kind != "down" && kind != "up" && kind != "change" || !owner.contextCurrent() || owner.app.closing.Load() {
		return
	}
	select {
	case <-owner.done:
		return
	default:
	}
	if !change.current() {
		return
	}
	owner.mu.Lock()
	if owner.fault != nil || owner.stopping || !owner.source.Current() || !owner.contextState.currentLocked() {
		owner.mu.Unlock()
		return
	}
	owner.pending = change
	owner.mu.Unlock()
	select {
	case owner.wake <- struct{}{}:
	default:
	}
}

func (owner *serviceDNSHandoff) run() {
	defer close(owner.done) // close even if terminal cleanup itself fails; this is task retirement, not engine proof
	completed := false
	defer owner.finishDNS(&completed)
	owner.mu.Lock()
	owner.running = !owner.stopping && owner.source.Current() && owner.contextState.currentLocked()
	owner.mu.Unlock()
	close(owner.started) // task-entry receipt only, never engine/native readiness
	for {
		select {
		case <-owner.ctx.Done():
			completed = true
			return
		case <-owner.wake:
		}
		for {
			owner.mu.Lock()
			change := owner.pending
			owner.pending = nil
			owner.mu.Unlock()
			if change == nil {
				break
			}
			if owner.ctx.Err() != nil || owner.app.closing.Load() {
				completed = true
				return
			}
			if !change.current() {
				continue
			}
			react, err := owner.app.applyServiceDNSChange(owner.ctx, *change)
			if react && err == nil && change.kind != "down" && change.current() && owner.ctx.Err() == nil && !owner.app.closing.Load() {
				owner.app.reactToServiceDNSChange(*change) // same captured request/actor, synchronous within this task
			}
		}
	}
}

func (a *App) applyServiceDNSChange(ctx context.Context, change serviceDNSChange) (react bool, finalErr error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if ctx == nil {
		return false, core.ErrPreflightScope
	}
	ctx = change.controlContext(ctx, a)
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second) // I/O budget; acquiring operationMu is not a hard deadline
	defer cancel()
	cc, manager, engine := change.config, change.manager, change.engine
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			react = false
			finalErr = errors.Join(finalErr, core.ErrEngineCleanupFailed)
		}
		if finalErr == nil || !change.actorCurrent(a, false) {
			return
		}
		// Supersession before Engine/pipe work is not a policy failure. An
		// independently failed/retired original engine or custody fault still
		// requires actual cleanup even after the event/task was revoked.
		if errors.Is(finalErr, context.Canceled) && !channelRetryBlocked(finalErr) && engine.Status() == "running" {
			return
		}
		a.failCoreForIntent(change.intent, "смена сетевого DNS-контекста не подтверждена", finalErr)
	}()
	finalErr = func() error {
		if !lifecyclePublicationCurrent(ctx) || !change.current() {
			return nil
		}
		if cc == nil || manager == nil || engine == nil {
			react = true // only original scoped retry; never adopt an actor after queueing
			return nil
		}
		if !change.actorCurrent(a, true) {
			return nil
		}
		// Owner/State are value reads. Do not hold a Manager reader lease while
		// waiting for Engine.mu or the original worker's IPC/retirement.
		remaining, err := a.sessionLifetime(cc)
		if err != nil {
			return err
		}
		if remaining > 0 {
			bounded, cancelLifetime := context.WithTimeout(ctx, remaining)
			defer cancelLifetime()
			ctx = bounded
		}
		identity, err := engine.CurrentRuntime(ctx)
		if err != nil {
			return err
		}
		digest, err := core.RuntimeConfigurationDigest(cc.Raw)
		if err != nil || identity.ConfigDigest != digest || identity.DefaultOutbound != cc.SelectorDefault || identity.TUNName != cc.TUNName || !identity.ForwardingAdmitted {
			return errors.Join(core.ErrForwardingNotAdmitted, err)
		}
		if !lifecyclePublicationCurrent(ctx) || !change.actorCurrent(a, true) {
			return nil
		}
		if change.kind == "down" {
			err = engine.PauseServiceDNS(ctx, identity)
		} else {
			err = engine.ResumeServiceDNS(ctx, identity)
		}
		if err != nil {
			return err
		}
		react = change.actorCurrent(a, true) && lifecyclePublicationCurrent(ctx)
		return nil
	}()
	completed = true
	return react, finalErr
}
