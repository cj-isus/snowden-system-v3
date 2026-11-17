package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errAdapterObservation = errors.New("local adapter observation unavailable")

const adapterObservationBudget = 700 * time.Millisecond

type adapterSnapshot struct {
	stats             NetStats
	address, netClass string
}

type adapterObservationTask struct {
	parent                            context.Context // retained original constructor parent on partial failure
	ctx                               context.Context
	cancel                            context.CancelFunc
	deadline                          time.Time
	pending, returned, cancelReturned bool
}

// One original native-query frame, not a native OS/RSS/latency attestation.
// A stopped UI or expired deadline cannot replace an unreturned query.
// The mutex protects the slot only; task fields belong to the original goroutine.
var adapterObservations struct {
	sync.Mutex
	owner *adapterObservationTask
}

func (a *App) observationParent() context.Context {
	a.mu.Lock()
	parent := a.ctx
	a.mu.Unlock()
	if !a.observationParentCurrent(parent) {
		return nil
	}
	return parent
}

// Shared negative App-lifetime fence for read-only observations only.
func (a *App) observationParentCurrent(parent context.Context) bool {
	if a == nil {
		return false
	}
	publication := capturePublicationContext(parent)
	if publication == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return sameContextIdentity(a.ctx, parent) && publication.current() && !a.closing.Load() && !a.quitRequested
}

func observeAdapterFacts(parent context.Context) (snapshot adapterSnapshot, err error) {
	publication := capturePublicationContext(parent)
	if publication == nil || !adapterObservations.TryLock() {
		return snapshot, errAdapterObservation
	}
	if adapterObservations.owner != nil {
		adapterObservations.Unlock()
		return snapshot, errAdapterObservation
	}
	owner := &adapterObservationTask{parent: parent}
	adapterObservations.owner = owner
	adapterObservations.Unlock() // exact slot reserved; no context/native/cancel work under registry mutex
	defer func() {
		if !owner.returned {
			_ = recover()
			snapshot, err = adapterSnapshot{}, errAdapterObservation
		}
		if owner.retire() != nil {
			snapshot, err = adapterSnapshot{}, errAdapterObservation
			return
		}
		adapterObservations.Lock()
		defer adapterObservations.Unlock()
		if adapterObservations.owner != owner {
			snapshot, err = adapterSnapshot{}, errAdapterObservation
			return // never clear a successor for this original query frame
		}
		adapterObservations.owner = nil
		if err != nil || !publication.current() || !time.Now().Before(owner.deadline) {
			snapshot, err = adapterSnapshot{}, errAdapterObservation
		}
	}()
	owner.pending = true
	owner.ctx, owner.cancel = context.WithTimeout(owner.parent, adapterObservationBudget)
	owner.deadline, _ = owner.ctx.Deadline()
	owner.pending = false
	owner.pending = true
	snapshot, err = collectAdapterFacts(owner.ctx)
	owner.pending, owner.returned = false, true
	return snapshot, err
}

func (owner *adapterObservationTask) retire() (result error) {
	result = errAdapterObservation
	sequence := &appCleanupSequence{}
	defer func() {
		if sequence.result == nil && owner.returned && !owner.pending && owner.cancelReturned {
			result = nil
		}
	}()
	defer appCleanupStep("original adapter query cancellation", sequence, func() error {
		if owner.cancel == nil {
			return errAdapterObservation
		}
		owner.cancel()
		owner.cancelReturned = true
		return nil
	})
	return result
}
