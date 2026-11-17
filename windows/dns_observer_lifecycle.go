package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

var errServiceDNSObserverUnavailable = errors.New("сетевой DNS-обработчик недоступен; исходная задача должна завершиться до повторного запуска")

// Lock order at publication: runtime -> Manager -> App -> epoch -> poller -> DNS.
// This lease covers value-only publication, never IPC/Engine/Manager calls.
func (owner *serviceDNSHandoff) guardRunning(app *App) (func(), bool) {
	if owner == nil {
		return nil, false
	}
	releasePoller, live := owner.poller.guardRunning(app)
	if !live {
		return nil, false
	}
	owner.mu.Lock()
	if owner.app != app || !owner.running || !owner.source.Current() || owner.fault != nil || owner.ctx.Err() != nil {
		owner.mu.Unlock()
		releasePoller()
		return nil, false
	}
	return func() { owner.mu.Unlock(); releasePoller() }, true
}

func (owner *serviceDNSHandoff) stop() {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	owner.running = false
	owner.source.Revoke()
	owner.stopping = true
	owner.pending = nil
	owner.mu.Unlock()
	owner.cancel() // no cancellation callbacks under the publication lease
}

func (w *netWatcher) networkOwners(app *App) (*networkPollerOwner, *serviceDNSHandoff, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.started || w.app != app || w.stop == nil || w.poller == nil || w.dns == nil || w.dns.poller != w.poller {
		return nil, nil, errors.Join(errNetworkPollerUnavailable, errServiceDNSObserverUnavailable)
	}
	select {
	case <-w.stop:
		return nil, nil, errors.Join(errNetworkPollerUnavailable, errServiceDNSObserverUnavailable)
	default:
		return w.poller, w.dns, nil
	}
}

// A failed task's finalizer may need operationMu. Join BEFORE acquiring it;
// never turn a wait timeout into permission to replace the original owner.
func (a *App) joinTerminalNetworkObserversBeforeStart(intent uint64) error {
	if !a.appLifetimeCurrent() || !a.operationCurrent(intent) {
		return errors.Join(errServiceDNSObserverUnavailable, context.Canceled)
	}
	poller, owner, err := netWatch.networkOwners(a)
	if err != nil {
		return err
	}
	wait, cancel := context.WithTimeout(a.ctx, 2*time.Second)
	defer cancel()
	poller.mu.Lock()
	pollerTerminal := poller.fault != nil || poller.ctx.Err() != nil
	poller.mu.Unlock()
	if err := joinTerminalNetworkTask(wait, pollerTerminal, poller.done, errNetworkPollerUnavailable); err != nil {
		return err
	}
	owner.mu.Lock()
	terminal := owner.fault != nil || owner.ctx.Err() != nil
	owner.mu.Unlock()
	if err := joinTerminalNetworkTask(wait, terminal, owner.done, errServiceDNSObserverUnavailable); err != nil {
		return err
	}
	if !a.appLifetimeCurrent() || !a.operationCurrent(intent) {
		return errors.Join(errServiceDNSObserverUnavailable, context.Canceled)
	}
	return nil
}

func (owner *serviceDNSHandoff) awaitStarted(ctx context.Context) error {
	return awaitNetworkTaskEntry(ctx, owner.started, owner.done, owner.ctx.Done(), errServiceDNSObserverUnavailable,
		func() (func(), bool) { return owner.guardRunning(owner.app) })
}

// Caller owns operationMu and has just confirmed previous.Stop and original
// capture-helper retirement. StateStopped/done alone are NOT that receipt.
// Each replacement requires its original task's full done. A failed producer
// and its linked DNS task are replaced together, never with overlapping loops.
func (a *App) reconcileServiceDNSAfterRetirement(ctx context.Context, previous *core.Manager, previousEngine *core.Engine) (*serviceDNSHandoff, error) {
	appLifetime := a.appLifetimeFence() // before runtime/owner/registry locks
	if ctx == nil || ctx.Err() != nil || !appLifetime.Current() || a.closing.Load() {
		return nil, errServiceDNSObserverUnavailable
	}
	runtime_.mu.Lock()
	same := runtime_.manager == previous && runtime_.engine == previousEngine && (previous != nil || previousEngine == nil)
	runtime_.mu.Unlock()
	if !same {
		return nil, errors.Join(errServiceDNSObserverUnavailable, errRuntimeOwnerUnavailable)
	}
	poller, owner, err := netWatch.networkOwners(a)
	if err != nil {
		return nil, err
	}
	pollerErr := poller.awaitStarted(ctx)
	if pollerErr == nil && owner.awaitStarted(ctx) == nil {
		return owner, nil
	}
	if ctx.Err() != nil {
		return nil, errors.Join(errServiceDNSObserverUnavailable, ctx.Err())
	}
	select {
	case <-owner.done:
	default:
		return nil, errServiceDNSObserverUnavailable // original may still need operationMu
	}
	owner.mu.Lock()
	dnsCleanupKnown := owner.retirement.cleanupKnown()
	owner.mu.Unlock()
	if !dnsCleanupKnown {
		return nil, errServiceDNSObserverUnavailable // done alone cannot authorize a replacement
	}
	if pollerErr != nil {
		select {
		case <-poller.done:
		default:
			return nil, errNetworkPollerUnavailable
		}
		poller.mu.Lock()
		pollerCleanupKnown := poller.retirement.cleanupKnown()
		poller.mu.Unlock()
		if !pollerCleanupKnown {
			return nil, errNetworkPollerUnavailable
		}
		if err := poller.retirePowerNotifications(); err != nil {
			return nil, err // original native handle must retire before one replacement
		}
	}
	netWatch.mu.Lock()
	if netWatch.app != a || netWatch.poller != poller || netWatch.dns != owner || a.closing.Load() || !appLifetime.Current() || ctx.Err() != nil {
		netWatch.mu.Unlock()
		return nil, errServiceDNSObserverUnavailable
	}
	select {
	case <-netWatch.stop:
		netWatch.mu.Unlock()
		return nil, errServiceDNSObserverUnavailable
	default:
	}
	if pollerErr == nil {
		releasePoller, live := poller.guardRunning(a)
		if !live {
			netWatch.mu.Unlock()
			return nil, errNetworkPollerUnavailable
		}
		releasePoller() // final source check before swapping DNS under watcher lock
	}
	if pollerErr != nil {
		nextPoller := newNetworkPollerOwner(a)
		netWatch.retiredPoller, netWatch.poller = poller, nextPoller
		poller = nextPoller
		netWatch.last, netWatch.lastAt = netFingerprint{}, time.Time{} // no synthetic OS event/epoch
	}
	next := newServiceDNSHandoff(a, poller) // lifetime is App.ctx, not temporary Start ctx
	netWatch.retiredDNS = owner             // bounded one confirmed predecessor; never an unfinished owner
	netWatch.dns = next
	netWatch.mu.Unlock()
	if pollerErr != nil {
		go poller.run(netWatch)
	}
	go next.run()
	if err := poller.awaitStarted(ctx); err != nil {
		// A producer may fail after the earlier check. Do not strand a new
		// DNS task on an already-failed source whose finalizer saw its predecessor.
		if releasePoller, live := poller.guardRunning(a); live {
			releasePoller()
		} else {
			if faultErr := netWatch.failNetworkPoller(poller); faultErr != nil {
				return nil, errors.Join(err, faultErr)
			}
		}
		return nil, err
	}
	if err := next.awaitStarted(ctx); err != nil {
		return nil, err // keep this exact task; no rollback/replacement on wait failure
	}
	return next, nil
}

// Private negative-fault registration for one actual runtime actor. It is not
// a probe/native receipt or authorization to adopt another App request.
type serviceDNSFaultScope struct {
	intent  uint64
	source  *core.OperationSource // original registered request, not a later applied-report fallback
	manager *core.Manager
	engine  *core.Engine
	config  *render.ClientConfig
	cause   error // first local fault registration; never reinterpret it as channel evidence
}

func (a *App) invalidateFailedServiceDNS(owner *serviceDNSHandoff) context.CancelFunc {
	_, cancel := a.invalidateFailedServiceDNSForScope(owner)
	return cancel
}

// Negative value fence before waiting for operationMu. Lock order is runtime
// -> App -> original DNS owner; callers release poller/DNS locks before entry.
// The first scope is immutable; a repeated finalizer cannot adopt a successor.
func (a *App) invalidateFailedServiceDNSForScope(owner *serviceDNSHandoff) (serviceDNSFaultScope, context.CancelFunc) {
	if owner == nil || owner.app != a {
		return serviceDNSFaultScope{}, nil
	}
	runtime_.mu.Lock()
	defer runtime_.mu.Unlock()
	if runtime_.dns != owner || runtime_.manager == nil {
		return serviceDNSFaultScope{}, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.failureScope == nil {
		owner.failureScope = &serviceDNSFaultScope{intent: a.operationID, source: a.intentSource, manager: runtime_.manager, engine: runtime_.engine, config: runtime_.cc, cause: owner.fault}
	}
	scope := *owner.failureScope
	if a.operationID != scope.intent || a.intentSource != scope.source || !scope.source.Current() || runtime_.manager != scope.manager || runtime_.engine != scope.engine || runtime_.cc != scope.config {
		return scope, nil
	}
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.nativeObservation = nil
	if a.protectionDesired && !a.closing.Load() {
		a.state.BlockedReason = "cleanup_unconfirmed"
		a.cancelFailClosedRetryLocked() // canceled lifecycle work must not schedule channel retry
		a.sm.markBlocked(time.Now())
	}
	return scope, a.operationCancel // callback is invoked only after all leases are released
}

func (a *App) retireFailedServiceDNS(owner *serviceDNSHandoff) (err error) {
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			err = core.ErrEngineCleanupFailed
		}
	}()
	scope, cancel := a.invalidateFailedServiceDNSForScope(owner)
	if cancel != nil {
		cancel()
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	runtime_.mu.Lock()
	current := scope.manager != nil && runtime_.dns == owner && runtime_.manager == scope.manager && runtime_.engine == scope.engine && runtime_.cc == scope.config
	runtime_.mu.Unlock()
	a.mu.Lock()
	current = current && a.operationID == scope.intent && a.intentSource == scope.source && scope.source.Current()
	a.mu.Unlock()
	if current && a.operationCurrent(scope.intent) {
		cause := scope.cause
		if cause == nil {
			cause = errServiceDNSObserverUnavailable
		}
		stage := "обработчик сетевого DNS завершился без подтверждения"
		if errors.Is(cause, errPowerContinuityChanged) {
			stage = "непрерывность после сна требует нового запуска"
		}
		a.failCoreForIntent(scope.intent, stage, cause)
	}
	completed = true
	return nil
}
