package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

func (owner *serviceDNSHandoff) contextCurrent() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.contextState.currentLocked()
}

// A single original setup frame, never an executor or replacement scheduler.
// The registry already owns these passive tasks before any real constructor.
type networkObserverPreparation struct {
	app                       *App
	parent                    appTaskParent
	interest                  core.OperationFence
	poller                    *networkPollerOwner
	dns                       *serviceDNSHandoff
	newPoller                 bool
	pollerEntered, dnsEntered bool
	transferred               bool
}

func (a *App) prepareNetworkObservers(parent appTaskParent, interest core.OperationFence, poller *networkPollerOwner, dns *serviceDNSHandoff, newPoller bool) (result error) {
	preparation := &networkObserverPreparation{app: a, parent: parent, interest: interest, poller: poller, dns: dns, newPoller: newPoller}
	returned := false
	defer preparation.finish(&returned, &result)
	result = preparation.build()
	returned = true
	return result
}

func (preparation *networkObserverPreparation) build() error {
	if !preparation.interest.Current() || !preparation.parent.fence.Current() || preparation.app.closing.Load() {
		return context.Canceled
	}
	var pollerContext context.Context
	var pollerFence core.OperationFence
	if preparation.newPoller {
		preparation.pollerEntered = true
		var err error
		pollerContext, pollerFence, err = preparation.poller.contextState.construct(preparation.parent)
		if err != nil {
			return errors.Join(errNetworkPollerUnavailable, err)
		}
	}
	if !preparation.interest.Current() || !preparation.parent.fence.Current() {
		return context.Canceled
	}
	preparation.dnsEntered = true
	dnsContext, dnsFence, err := preparation.dns.contextState.construct(preparation.parent)
	if err != nil {
		return errors.Join(errServiceDNSObserverUnavailable, err)
	}
	if !preparation.publish(pollerContext, pollerFence, dnsContext, dnsFence) {
		return errors.Join(errNetworkPollerUnavailable, errServiceDNSObserverUnavailable)
	}
	preparation.transferred = true
	if preparation.newPoller {
		go preparation.poller.run(netWatch)
	}
	go preparation.dns.run()
	return nil
}

// Value-only final gate, App -> watcher -> poller -> DNS. Parent capture takes
// App.mu before registry ownership; no Context/cancel/native callback runs here.
func (preparation *networkObserverPreparation) publish(pollerContext context.Context, pollerFence core.OperationFence, dnsContext context.Context, dnsFence core.OperationFence) bool {
	a, poller, dns := preparation.app, preparation.poller, preparation.dns
	a.mu.Lock()
	defer a.mu.Unlock()
	netWatch.mu.Lock()
	defer netWatch.mu.Unlock()
	if preparation.parent.app != a || !preparation.parent.fence.Current() || !preparation.interest.Current() ||
		!a.appLifetimeCurrentLocked() || a.closing.Load() || a.quitRequested || a.startupErr != nil || a.installedImageChanged ||
		netWatch.app != a || netWatch.poller != poller || netWatch.dns != dns || dns.poller != poller {
		return false
	}
	select {
	case <-netWatch.stop:
		return false
	default:
	}
	poller.mu.Lock()
	defer poller.mu.Unlock()
	dns.mu.Lock()
	defer dns.mu.Unlock()
	if poller.stopping || poller.fault != nil || dns.stopping || dns.fault != nil || !dns.source.Current() ||
		!dns.contextState.cancellation.operation.Current() || !dnsFence.Current() {
		return false
	}
	if preparation.newPoller {
		if !poller.contextState.cancellation.operation.Current() || !pollerFence.Current() {
			return false
		}
		poller.ctx = pollerContext
		poller.contextState.parentFence, poller.contextState.childFence, poller.contextState.ready = preparation.parent.fence, pollerFence, true
	} else if !poller.running || !poller.contextState.currentLocked() {
		return false
	}
	dns.ctx = dnsContext
	dns.contextState.parentFence, dns.contextState.childFence, dns.contextState.ready = preparation.parent.fence, dnsFence, true
	return true
}

// A failed creator owns both unscheduled frames through their actual cleanup.
// Never reenter creator-held operationMu via running-task fault finalization.
func (preparation *networkObserverPreparation) finish(returned *bool, result *error) {
	if preparation.transferred {
		return
	}
	if !*returned {
		_ = recover()
		*result = errors.Join(*result, core.ErrEngineCleanupFailed)
	}
	dns, poller := preparation.dns, preparation.poller
	dns.mu.Lock()
	dns.running = false
	dns.source.Revoke()
	dns.pending = nil
	dns.fault = errors.Join(dns.fault, errServiceDNSObserverUnavailable, *result)
	dns.mu.Unlock()
	if !preparation.dnsEntered {
		dns.contextState.skipConstruction()
	}
	if preparation.newPoller {
		poller.mu.Lock()
		poller.running, poller.observed = false, false
		poller.fault = errors.Join(poller.fault, errNetworkPollerUnavailable, *result)
		poller.mu.Unlock()
		if !preparation.pollerEntered {
			poller.contextState.skipConstruction()
		}
	}
	sequence := new(appCleanupSequence)
	defer func() { *result = errors.Join(*result, sequence.result) }()
	defer appCleanupStep("retire original DNS preparation", sequence, func() error { return dns.finishContextPreparation(*returned) })
	if preparation.newPoller {
		defer appCleanupStep("retire original poller preparation", sequence, func() error { return poller.finishContextPreparation(*returned) })
	}
}

func (owner *networkPollerOwner) finishContextPreparation(returned bool) (result error) {
	defer close(owner.done) // actual original failed setup frame, never polling/native entry
	sequence := new(appCleanupSequence)
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.retirement = networkProducerReceipt{preparationOnly: true, preparationReturned: returned, cleanupReturned: sequence.drained && result == nil, cleanupResult: result}
		owner.fault = errors.Join(owner.fault, result)
	}()
	defer sequence.finish(&result)
	defer appCleanupStep("cancel unstarted original poller context", sequence, owner.contextState.cancellation.join)
	return nil
}

func (owner *serviceDNSHandoff) finishContextPreparation(returned bool) (result error) {
	defer close(owner.done) // actual original failed setup frame, never DNS-loop entry
	sequence := new(appCleanupSequence)
	defer func() {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.retirement = networkProducerReceipt{preparationOnly: true, preparationReturned: returned, cleanupReturned: sequence.drained && result == nil, cleanupResult: result}
		owner.fault = errors.Join(owner.fault, result)
	}()
	defer sequence.finish(&result)
	defer appCleanupStep("cancel unstarted original DNS context", sequence, owner.contextState.cancellation.join)
	return nil
}
