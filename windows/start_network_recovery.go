package main

import (
	"context"
	"errors"
	"time"
)

// A source-flow receipt from actual stopCoreGenerationLocked, never a probe,
// public report, timer, native admission token or inferred empty-runtime state.
type lifecycleRetirementReceipt struct {
	intent  uint64
	retired bool
}

const networkRetryBurstLimit = 2

// Caller owns operationMu. Preserve the real attempt error through cancel so
// local cleanup/trust failures cannot become ordinary network retry evidence.
func (a *App) finishStartCancellation(ctx context.Context, intent uint64, cause error) {
	retirement := a.finishCanceledLifecycleCause(ctx, intent, cause)
	if !retirement.retired || retirement.intent != intent || ctx == nil || !errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	previous, ok := ctx.Value(networkPathContextKey{}).(*networkPathSource)
	if !ok || previous == nil || previous.current.Load() {
		return // not an observed-path cancellation; no deadline/user-cancel acceleration
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil || a.ctx.Err() != nil || a.operationID != intent || !a.protectionDesired || a.closing.Load() ||
		!a.failClosedState() || a.retry.timer == nil || a.retry.done || a.retry.intent != intent {
		return
	}
	path := netEpoch.pathSource()
	if path == previous || !a.takeNetworkRetryBurstLocked(path) {
		return
	}
	if a.armFailClosedRetryLocked(time.Now(), 0, path) {
		a.pushStateLocked()
		a.appendLogLocked("info", "запуск: сетевой контекст изменился; очистка подтверждена, запланирован новый запуск с полной проверкой")
	}
}

// a.mu is held. Count issued immediate attempts, at most one for each source
// and two across the automatic intent chain. Normal backoff does not reset it.
func (a *App) takeNetworkRetryBurstLocked(path *networkPathSource) bool {
	if path == nil || !path.current.Load() {
		return false
	}
	if a.retry.networkIntent != a.operationID {
		a.retry.networkIntent = a.operationID
		a.retry.networkBursts = 0
		a.retry.networkPath = nil
	}
	if a.retry.networkBursts >= networkRetryBurstLimit || a.retry.networkPath == path {
		return false
	}
	a.retry.networkBursts++
	a.retry.networkPath = path
	return true
}
