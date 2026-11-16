package core

import (
	"context"
	"sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
)

// One original timer/channel watcher per UDP DNS scope. Activity updates values;
// only this retained watcher resets/stops the original timer. No AfterFunc or
// opaque SDK timeout worker is manufactured without a joinable local owner.
type forwardingDNSIdle struct {
	scope                              *forwardingDNSScope
	mu                                 sync.Mutex // values only; timer/cancel/join outside
	timer                              *time.Timer
	until                              time.Time
	installed, done                    chan struct{}
	installReturned, started, returned bool
	stopStarted, stopReturned, failed  bool
	expired                            bool
}

func (scope *forwardingDNSScope) startIdle() error {
	scope.mu.Lock()
	if scope.closing || scope.idle != nil || scope.route.ctx.Err() != nil {
		scope.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	owner := &forwardingDNSIdle{scope: scope, until: time.Now().Add(C.DNSTimeout), installed: make(chan struct{}), done: make(chan struct{})}
	scope.idle = owner // BEFORE NewTimer, including any partial acquisition/panic
	scope.mu.Unlock()
	if err := owner.install(); err != nil {
		return err
	}
	owner.mu.Lock()
	owner.started = true // BEFORE Go; constructor frame remains covered by scope.frameDone
	owner.mu.Unlock()
	go owner.watch()
	return nil
}

func (owner *forwardingDNSIdle) install() (finalErr error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.installReturned, owner.failed = returned, owner.failed || fault || finalErr != nil
		owner.mu.Unlock()
		close(owner.installed)
		if fault || finalErr != nil {
			owner.scope.route.markFault()
		}
	}()
	owner.timer = time.NewTimer(time.Until(owner.until))
	returned = true
	if owner.timer == nil {
		return ErrEngineCleanupFailed
	}
	return nil
}

func (owner *forwardingDNSIdle) watch() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		stopped := owner.stop()
		owner.mu.Lock()
		owner.returned = returned
		owner.failed = owner.failed || fault || !stopped
		failed := owner.failed
		close(owner.done)
		owner.mu.Unlock()
		if failed {
			owner.scope.route.markFault()
		}
	}()
	for {
		select {
		case <-owner.scope.route.ctx.Done():
			returned = true
			return
		case <-owner.timer.C:
			owner.mu.Lock()
			until := owner.until
			// Decide under the same value lock as touch: an old snapshot must
			// never cancel activity that already extended the deadline.
			expired := !time.Now().Before(until)
			owner.expired = owner.expired || expired
			owner.mu.Unlock()
			if expired {
				owner.scope.route.cancel()
				returned = true
				return
			}
			owner.timer.Reset(time.Until(until)) // same timer, consumed tick; only this watcher controls it
		}
	}
}

func (owner *forwardingDNSIdle) touch() error {
	owner.mu.Lock()
	if owner.failed || owner.expired || !owner.installReturned || !owner.started || owner.returned || owner.scope.route.ctx.Err() != nil {
		owner.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	now := time.Now()
	if !now.Before(owner.until) {
		owner.expired = true
		owner.mu.Unlock()
		owner.scope.route.cancel() // late activity cannot resurrect an expired scope
		return errMixedHandshakeTimeout
	}
	owner.until = now.Add(C.DNSTimeout)
	owner.mu.Unlock()
	return nil
}

func (owner *forwardingDNSIdle) stop() (known bool) {
	defer func() {
		if recover() != nil {
			known = false
		}
	}()
	owner.mu.Lock()
	if owner.stopStarted {
		known = owner.stopReturned
		owner.mu.Unlock()
		return
	}
	owner.stopStarted = true
	owner.mu.Unlock()
	_ = owner.timer.Stop() // no callback; false does not manufacture a callback-return receipt
	owner.mu.Lock()
	owner.stopReturned = true
	owner.mu.Unlock()
	return true
}

func (owner *forwardingDNSIdle) retire(ctx context.Context) error {
	observed := mixedOriginalDone(ctx, owner.installed)
	owner.mu.Lock()
	started := owner.started
	owner.mu.Unlock()
	if observed {
		if started {
			observed = mixedOriginalDone(ctx, owner.done)
		} else {
			observed = owner.stop() // scope read frame returned: no future watcher can start
		}
	}
	owner.mu.Lock()
	unknown := !observed || !owner.installReturned || started && !owner.returned || !owner.stopReturned || owner.failed
	owner.failed = owner.failed || unknown
	owner.mu.Unlock()
	if unknown {
		owner.scope.route.markFault()
		return ErrEngineCleanupFailed
	}
	return nil // controlled timer/watcher frame, not literal whole goroutine/SDK/Process/Job retirement
}
