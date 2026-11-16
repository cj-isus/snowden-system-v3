package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

// One sampler belongs to the original lifecycle operation until actual return.
// Stopping sampling must not cancel a successful operation before its terminal
// source/trust/proxy gate. Parent cancellation and a trust fault still cancel it.
type operationLifetime struct {
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

func observeOperationLifetime(parent context.Context, probe Probe) (context.Context, *operationLifetime) {
	if _, ok := probe.(LifetimeProbe); !ok {
		return parent, nil
	}
	ctx, cancel := context.WithCancelCause(parent)
	watch := &operationLifetime{stop: make(chan struct{}), done: make(chan struct{})}
	go watch.run(ctx, probe, cancel) // reserved by this original caller before it can finish
	return ctx, watch
}

func (watch *operationLifetime) run(ctx context.Context, probe Probe, cancel context.CancelCauseFunc) {
	defer close(watch.done) // after the abnormal-exit restriction and original sampling frame
	returned := false
	defer func() {
		if !returned {
			_ = recover() // panic(nil)/Goexit cannot silently stop trust observation
			cancel(&TrustFaultError{Reason: "trust_unavailable"})
		}
	}()
	watch.observe(ctx, probe, cancel)
	returned = true
}

func (watch *operationLifetime) observe(ctx context.Context, probe Probe, cancel context.CancelCauseFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-watch.stop:
			return
		default:
		}
		remaining, err := lifetimeRemaining(probe)
		if err != nil {
			cancel(err) // an actual sampled fault is retained even during a stop request
			return
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-watch.stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (watch *operationLifetime) requestStop() {
	watch.stopOnce.Do(func() { close(watch.stop) })
}

func (operation *lifecycleOperation) lifetimeReturned() bool {
	if operation == nil {
		return false
	}
	if operation.lifetime == nil {
		return true
	}
	select {
	case <-operation.lifetime.done:
		return true
	default:
		return false
	}
}

// No Manager lock is held while stopping/joining the sampler. If its callback
// has not returned when the original context is revoked, perform negative
// engine/protection cleanup first, then retain this active operation until the
// original sampler returns. No timeout or replacement waiter creates a receipt.
func (manager *Manager) retireOperationLifetime(ctx context.Context, operation *lifecycleOperation, engine *Engine, proxy systemProxy, probes ...Probe) (error, bool) {
	if operation == nil {
		return ErrInvalidState, false
	}
	if operation.lifetime == nil {
		return nil, false
	}
	watch := operation.lifetime
	watch.requestStop()
	cause := terminalContextCause(ctx)
	if cause == nil {
		select {
		case <-watch.done:
			return nil, false
		case <-ctx.Done():
			cause = terminalContextCause(ctx)
		}
	}
	manager.mu.Lock()
	ours := manager.active == operation
	if ours {
		manager.state = StateStopping // negative state while the original callback still owns work
	}
	manager.mu.Unlock()
	if !ours {
		<-watch.done
		return ErrInvalidState, false // never clean up a successor for an obsolete operation
	}
	cleanupErr := manager.cleanup(engine, proxy, probes...)
	<-watch.done // original callback return, independent of the expired interest budget
	return errors.Join(cause, cleanupErr), true
}
