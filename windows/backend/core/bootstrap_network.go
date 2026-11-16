package core

import (
	"context"
	"errors"
)

// One original DNS-work generation. It is not a physical network identity or
// forwarding permit. All fields except ctx/cancel are guarded by worker.mu.
type bootstrapPath struct {
	ctx         context.Context
	cancel      context.CancelFunc
	live        bool
	callers     int
	drained     chan struct{}
	drainClosed bool
}

func newBootstrapPath(parent context.Context) *bootstrapPath {
	ctx, cancel := context.WithCancel(parent)
	return &bootstrapPath{ctx: ctx, cancel: cancel, live: true, drained: make(chan struct{})}
}

func (path *bootstrapPath) signalDrainedLocked() {
	if !path.live && path.callers == 0 && !path.drainClosed {
		path.drainClosed = true
		close(path.drained)
	}
}

func (worker *bootstrapWorker) pathSource() *bootstrapPath {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.path
}

func (worker *bootstrapWorker) currentPath(ctx context.Context, path *bootstrapPath) bool {
	if path == nil || contextError(ctx) != nil || contextError(worker.ctx) != nil || contextError(path.ctx) != nil {
		return false
	}
	worker.mu.Lock()
	valid := !worker.closed && !worker.paused && worker.fault == nil && worker.path == path && path.live
	worker.mu.Unlock()
	worker.admission.mu.Lock()
	valid = valid && !worker.admission.closed
	worker.admission.mu.Unlock()
	return valid
}

// Pause first, then join original workers/borrowers and inspect the retained
// work domain. No new generation may bypass an uncertain original owner.
func (worker *bootstrapWorker) retireNetwork(ctx context.Context) (*bootstrapPath, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	worker.mu.Lock()
	if worker.closed || worker.fault != nil || worker.path == nil {
		err := errors.Join(ErrPreflightScope, worker.fault)
		worker.mu.Unlock()
		return nil, err
	}
	path, flight := worker.path, worker.flight
	worker.paused, path.live = true, false
	path.signalDrainedLocked()
	worker.mu.Unlock()
	path.cancel() // after revocation, outside the value lock
	wait, cancel := context.WithTimeout(ctx, workerCloseTimeout)
	defer cancel()
	retired := false
	defer func() {
		if !retired {
			worker.poison(ErrPreflightCleanup)
		}
	}()
	if err := worker.retireServiceDials(wait, nil); err != nil {
		return nil, err
	}
	if err := worker.retireDNSQueries(wait, false); err != nil {
		return nil, err
	}
	if flight != nil {
		if flight.path != path {
			return nil, ErrPreflightCleanup
		}
		select {
		case <-flight.done:
		case <-wait.Done():
			return nil, errors.Join(ErrPreflightCleanup, wait.Err())
		}
	}
	select {
	case <-path.drained:
	case <-wait.Done():
		return nil, errors.Join(ErrPreflightCleanup, wait.Err())
	}
	if !worker.work.mu.TryLock() {
		return nil, ErrPreflightCleanup
	}
	retained := worker.work.owner != nil
	worker.work.mu.Unlock()
	if retained {
		return nil, ErrPreflightCleanup
	}
	if err := worker.failure(); err != nil {
		return nil, err
	}
	retired = true
	return path, contextError(ctx)
}

func (worker *bootstrapWorker) pauseNetwork(ctx context.Context) error {
	_, err := worker.retireNetwork(ctx)
	return err
}

func (worker *bootstrapWorker) resumeNetwork(ctx context.Context) error {
	path, err := worker.retireNetwork(ctx)
	if err != nil {
		return err
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.closed || worker.fault != nil || worker.path != path || !worker.paused || contextError(ctx) != nil || contextError(worker.ctx) != nil {
		return errors.Join(ErrPreflightScope, worker.fault, contextError(ctx))
	}
	worker.path = newBootstrapPath(worker.ctx)
	worker.paused = false
	return nil // permits DNS work only; no forwarding/native state is opened
}
