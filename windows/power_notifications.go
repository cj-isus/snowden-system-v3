package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

var (
	errPowerContinuityChanged       = errors.New("система приостановлена или возобновлена; прежняя непрерывность не подтверждена")
	errPowerNotificationUnavailable = errors.New("уведомления Windows о сне недоступны")
)

type powerSubscription interface{ retire() error }

// Called only by the original native callback. No epoch/App/lifecycle locks,
// cancellation callbacks, I/O, logging or task creation in this notification.
func (owner *networkPollerOwner) notePowerNotification(event uintptr) {
	// RESUMESUSPEND (7) is user interaction after automatic resume, not another
	// network wake. Automatic resume (18) also covers a missed suspend notice.
	if event != 4 && event != 18 {
		return
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.stopping || owner.ctx.Err() != nil || owner.powerRetirement || owner.fault != nil {
		return
	}
	owner.powerRetirement = true
	owner.running, owner.observed = false, false
	owner.fault = errors.Join(context.Canceled, errPowerContinuityChanged)
	netEpoch.revokePowerContinuityValue()
	select {
	case owner.powerWake <- struct{}{}:
	default:
	}
}

func (owner *networkPollerOwner) failPowerNotification() {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.stopping {
		owner.running, owner.observed = false, false
		owner.fault = errors.Join(owner.fault, errNetworkPollerUnavailable, errPowerNotificationUnavailable)
		netEpoch.revokePowerContinuityValue()
	}
	select {
	case owner.powerWake <- struct{}{}:
	default:
	}
}

// Never discard an original native handle after an unsuccessful unregister.
// Called outside owner/registry/App locks; the original subscription serializes
// its own retirement and rejects replacement while ownership is uncertain.
func (owner *networkPollerOwner) retirePowerNotifications() (resultErr error) {
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			resultErr = errors.Join(errNetworkPollerUnavailable, core.ErrEngineCleanupFailed)
		}
	}()
	owner.mu.Lock()
	power := owner.power
	owner.mu.Unlock()
	if power == nil {
		completed = true
		return nil
	}
	if err := power.retire(); err != nil {
		completed = true
		return errors.Join(errNetworkPollerUnavailable, core.ErrEngineCleanupFailed, err)
	}
	completed = true
	return nil
}

// Caller holds operationMu, before binding a new operation to the revoked path.
// Only this actual Stop/helpers + original task/native retirement permits one
// replacement pair. Empty runtime/done alone never provide that authority.
func (a *App) recoverPowerContinuityBeforeOperation(intent uint64) error {
	if !netEpoch.powerBlocked.Load() {
		return nil
	}
	poller, dns, err := netWatch.networkOwners(a)
	if err != nil {
		return err
	}
	select {
	case <-poller.done:
	default:
		return errNetworkPollerUnavailable
	}
	select {
	case <-dns.done:
	default:
		return errServiceDNSObserverUnavailable
	}
	if !a.operationCurrent(intent) || a.ctx == nil || a.ctx.Err() != nil {
		return context.Canceled
	}
	if err := a.stopCoreGenerationLocked(stopFailure, intent); err != nil {
		return errors.Join(core.ErrEngineCleanupFailed, err)
	}
	if !a.operationCurrent(intent) {
		return context.Canceled
	}
	// App lifetime is for producer setup only; no render/probe/forwarding work.
	// A queued Stop can finish setup, but normal beginOperation still rejects
	// this old intent before a replacement engine is created.
	_, err = a.reconcileServiceDNSAfterRetirement(a.ctx, nil, nil)
	return err
}

// Existing watchdog only. This capability restores notifications while VPN is
// disconnected; it never calls Manager.Stop/Start, proxy/policy Arm/Release,
// render or probe. Existing helper retirement may revoke its own capture grants.
func (a *App) recoverDisconnectedPowerObservers(ctx context.Context) {
	if ctx == nil || ctx.Err() != nil || !netEpoch.powerBlocked.Load() {
		return
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	scope := func() (uint64, bool) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.operationID, ctx.Err() == nil && a.ctx != nil && a.ctx.Err() == nil && a.startupErr == nil && !a.protectionDesired &&
			!a.profileTransition && !a.closing.Load() && a.restartCommit == nil && !a.installedImageChanged &&
			(a.state.State == "stopped" || a.state.State == "error")
	}
	intent, allowed := scope()
	if !allowed || !netEpoch.powerBlocked.Load() {
		return
	}
	current := func() bool { id, valid := scope(); return valid && id == intent }
	runtime_.mu.Lock()
	noEngine := runtime_.manager == nil && runtime_.engine == nil && runtime_.cc == nil && runtime_.metrics == nil && !runtime_.startBusy
	runtime_.mu.Unlock()
	if !noEngine {
		return // never stop, detach or infer retirement of an existing runtime actor
	}
	poller, dns, err := netWatch.networkOwners(a)
	if err != nil {
		return
	}
	select {
	case <-poller.done:
	default:
		return
	}
	select {
	case <-dns.done:
	default:
		return
	}
	poller.mu.Lock()
	knownPower := poller.powerRetirement && errors.Is(poller.fault, errPowerContinuityChanged) && !localRetirementFailure(poller.fault)
	tried := poller.powerRecoveryTried && poller.powerRecoveryIntent == intent
	poller.mu.Unlock()
	dns.mu.Lock()
	knownPower = knownPower && dns.powerRetirement && errors.Is(dns.fault, errPowerContinuityChanged) && !localRetirementFailure(dns.fault)
	dns.mu.Unlock()
	if !knownPower || tried || !current() {
		return // independent native/custody faults require explicit reconciliation
	}
	claim := func() {
		poller.mu.Lock()
		poller.powerRecoveryTried, poller.powerRecoveryIntent = true, intent
		poller.mu.Unlock()
	}
	// TryLock inside this existing API does not cancel another active capture.
	// Busy defers to a later watchdog tick, without allocating a replacement.
	if err := a.retireCaptureHelpers(); err != nil {
		if !errors.Is(err, core.ErrHelperBusy) {
			claim()
			if current() {
				a.appendLog("warn", "восстановление наблюдателей после сна: возврат служебных ресурсов не подтверждён")
			}
		}
		return
	}
	claim() // at most one actual restart attempt per original producer/App intent
	if !current() {
		return
	}
	// No Manager exists to retire. This actual helper/native/task reconciliation
	// authorizes only observer setup; engine admission still needs normal Start.
	_, err = a.reconcileServiceDNSAfterRetirement(a.ctx, nil, nil)
	if !current() {
		return // a queued Start/Stop may use the pair; never overwrite its state
	}
	if err != nil {
		a.appendLog("warn", "наблюдатели после сна не восстановлены; автоматический запуск VPN не выполнялся")
		return
	}
	a.pushState() // existing state/intent untouched; publish the current continuity projection
	a.appendLog("info", "наблюдатели сети восстановлены после сна без запуска VPN")
}
