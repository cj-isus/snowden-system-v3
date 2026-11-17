package main

import (
	"context"
	"errors"
)

// Preserve the existing private network receipt contract while sharing the
// same actual setup/body/cleanup distinction with other App-owned producers.
type networkProducerReceipt = appProducerReceipt

func (owner *networkPollerOwner) finishPoller(watcher *netWatcher, completed *bool) {
	payload := recover()
	bodyReturned := *completed && payload == nil
	sequence := &appCleanupSequence{}
	var result error
	unexpected := false
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.retirement = networkProducerReceipt{bodyReturned: bodyReturned, cleanupReturned: sequence.drained && result == nil, cleanupResult: result}
		if result != nil {
			owner.fault = errors.Join(owner.fault, errNetworkPollerUnavailable, result)
		}
	}()
	defer sequence.finish(&result)
	defer appCleanupStep("retire failed original network poller scope", sequence, func() error {
		if unexpected || sequence.result != nil {
			return watcher.failNetworkPoller(owner)
		}
		return nil
	})
	defer appCleanupStep("cancel original network poller", sequence, owner.contextState.cancellation.join)
	defer appCleanupStep("retire original power subscription", sequence, owner.retirePowerNotifications)
	defer appCleanupStep("revoke original linked DNS control", sequence, func() error {
		watcher.revokeServiceDNSControl(owner)
		return nil
	})
	defer appCleanupStep("fence original network poller outcome", sequence, func() error {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		unexpected = !bodyReturned || !owner.stopping && !owner.app.closing.Load()
		owner.running, owner.observed = false, false
		if unexpected && (!owner.powerRetirement || !bodyReturned) {
			owner.fault = errors.Join(owner.fault, errNetworkPollerUnavailable)
		}
		return nil
	})
}

func (owner *serviceDNSHandoff) finishDNS(completed *bool) {
	payload := recover()
	bodyReturned := *completed && payload == nil
	sequence := &appCleanupSequence{}
	var result error
	unexpected := false
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.retirement = networkProducerReceipt{bodyReturned: bodyReturned, cleanupReturned: sequence.drained && result == nil, cleanupResult: result}
		if result != nil {
			owner.fault = errors.Join(owner.fault, errServiceDNSObserverUnavailable, result)
		}
	}()
	defer sequence.finish(&result)
	defer appCleanupStep("retire failed original DNS observer scope", sequence, func() error {
		if unexpected || sequence.result != nil {
			return owner.app.retireFailedServiceDNS(owner)
		}
		return nil
	})
	defer appCleanupStep("cancel original DNS observer", sequence, owner.contextState.cancellation.join)
	defer appCleanupStep("fence original DNS observer outcome", sequence, func() error {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		unexpected = !bodyReturned || !owner.stopping && !owner.app.closing.Load()
		owner.running = false
		owner.source.Revoke()
		owner.pending = nil
		if unexpected && (!owner.powerRetirement || !bodyReturned) {
			owner.fault = errors.Join(owner.fault, errServiceDNSObserverUnavailable)
		}
		return nil
	})
}

func (a *App) joinNetworkPoller(ctx context.Context, done <-chan struct{}) error {
	netWatch.mu.Lock()
	owner, app := netWatch.poller, netWatch.app
	netWatch.mu.Unlock()
	if owner == nil {
		if done == nil && (app == nil || app == a) {
			return nil
		}
		return errNetworkPollerUnavailable
	}
	if app != a || owner.app != a || done != owner.done {
		return errNetworkPollerUnavailable
	}
	if err := joinAppProducerContext(ctx, done, errNetworkPollerUnavailable); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.retirement.terminalResult(errNetworkPollerUnavailable)
}

func (a *App) joinServiceDNSObserver(ctx context.Context, done <-chan struct{}) error {
	netWatch.mu.Lock()
	owner, app := netWatch.dns, netWatch.app
	netWatch.mu.Unlock()
	if owner == nil {
		if done == nil && (app == nil || app == a) {
			return nil
		}
		return errServiceDNSObserverUnavailable
	}
	if app != a || owner.app != a || done != owner.done {
		return errServiceDNSObserverUnavailable
	}
	if err := joinAppProducerContext(ctx, done, errServiceDNSObserverUnavailable); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.retirement.terminalResult(errServiceDNSObserverUnavailable)
}
