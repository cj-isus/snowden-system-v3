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
	ctx                               context.Context
	cancel                            context.CancelFunc
	deadline                          time.Time
	pending, returned, cancelReturned bool
}

// One original native-query frame, not a native OS/RSS/latency attestation.
// A stopped UI or expired deadline cannot replace an unreturned query.
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
	if parent == nil || parent.Err() != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx == parent && !a.closing.Load() && !a.quitRequested
}

func observeAdapterFacts(parent context.Context) (snapshot adapterSnapshot, err error) {
	if !lifecyclePublicationCurrent(parent) || !adapterObservations.TryLock() {
		return snapshot, errAdapterObservation
	}
	defer adapterObservations.Unlock()
	if adapterObservations.owner != nil {
		return snapshot, errAdapterObservation
	}
	owner := &adapterObservationTask{}
	adapterObservations.owner = owner
	defer func() {
		if !owner.returned {
			_ = recover()
			snapshot, err = adapterSnapshot{}, errAdapterObservation
		}
		if owner.retire() != nil {
			snapshot, err = adapterSnapshot{}, errAdapterObservation
			return
		}
		adapterObservations.owner = nil
		if err != nil || !lifecyclePublicationCurrent(parent) || !time.Now().Before(owner.deadline) {
			snapshot, err = adapterSnapshot{}, errAdapterObservation
		}
	}()
	owner.pending = true
	owner.ctx, owner.cancel = context.WithTimeout(parent, adapterObservationBudget)
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
