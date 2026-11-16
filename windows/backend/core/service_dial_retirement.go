package core

import (
	"context"
	"errors"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// worker.mu held. Domain calls may use only addresses observed by this original
// task's exact service Lookup. Membership is not authentication of DNS or TLS.
func (task *serviceDialTask) nativeDestinationLocked(network string, destination M.Socksaddr) bool {
	peer := task.outbound.spec.peer
	destination = destination.Unwrap()
	if !destination.IsIP() || !peer.AllowsServicePort(destination.Port) ||
		peer.Type == "vless" && N.NetworkName(network) != N.NetworkTCP ||
		peer.Type == "hysteria2" && N.NetworkName(network) != N.NetworkUDP {
		return false
	}
	if literal := net.ParseIP(peer.Server); literal != nil {
		return literal.Equal(net.IP(destination.Addr.AsSlice()))
	}
	for _, address := range task.resolved {
		if address.Unmap() == destination.Addr.Unmap() {
			return true
		}
	}
	return false
}

func (task *serviceDialTask) fenceLocked(cause error) {
	worker := task.outbound.worker
	if task.cleanupErr == nil {
		task.cleanupErr = errors.Join(ErrPreflightCleanup, cause)
	}
	if worker.fault == nil {
		worker.fault = task.cleanupErr
	}
	worker.admission.open.Store(false)
}

// Passive metadata progress from existing native return/Close/control paths.
// Never starts a reaper, waits for Close, or refunds a slot on early winner.
func (task *serviceDialTask) progressLocked() {
	complete := true
	for attempt := range task.attempts {
		retired, err := attempt.owner.retirementState()
		if err != nil || attempt.fault != nil {
			task.fenceLocked(errors.Join(err, attempt.fault))
		}
		if attempt.active || attempt.closes != 0 || !attempt.callbackJoined || !retired {
			complete = false
			continue
		}
		if err == nil && attempt.fault == nil {
			delete(task.attempts, attempt)
		}
	}
	if task.sealed && task.pathJoined && task.outerReturned && complete {
		if !task.doneClosed {
			task.doneClosed = true
			close(task.done) // all tracked native calls/callbacks/Close attempts returned; cleanupErr separate
		}
		if task.cleanupErr == nil {
			delete(task.outbound.worker.serviceDials, task)
		}
	}
}

func (worker *bootstrapWorker) serviceDialProgressLocked() {
	for task := range worker.serviceDials {
		task.progressLocked()
	}
}

func (attempt *serviceNativeAttempt) close() (finalErr error) {
	task, worker := attempt.task, attempt.task.outbound.worker
	worker.mu.Lock()
	if attempt.released {
		worker.mu.Unlock()
		return nil // this scope no longer owns the winner; its SDK caller has the original object
	}
	attempt.closes++
	worker.mu.Unlock()
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			finalErr = errors.Join(ErrPreflightCleanup, finalErr)
		}
		worker.mu.Lock()
		attempt.closes--
		if !returned {
			attempt.fault = ErrPreflightCleanup
		}
		task.progressLocked()
		fault := task.cleanupErr
		worker.mu.Unlock()
		if fault != nil {
			worker.poison(fault) // observes SDK-ignored late losing Close error
		}
	}()
	finalErr = attempt.owner.close()
	returned = true
	return finalErr
}

func (task *serviceDialTask) closeAttempts() {
	worker := task.outbound.worker
	worker.mu.Lock()
	attempts := make([]*serviceNativeAttempt, 0, len(task.attempts))
	for attempt := range task.attempts {
		attempts = append(attempts, attempt)
	}
	worker.mu.Unlock()
	for _, attempt := range attempts {
		_ = attempt.close() // acquiring is not yet retirement; its original callback/native return remains counted
	}
}

func (worker *bootstrapWorker) retireServiceDials(ctx context.Context, target *serviceDialOutbound) (finalErr error) {
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			finalErr = errors.Join(ErrPreflightCleanup, finalErr)
		}
		if errors.Is(finalErr, ErrPreflightCleanup) {
			worker.poison(finalErr)
		}
	}()
	worker.mu.Lock()
	worker.serviceDialProgressLocked()
	if target == nil && !worker.closed && !worker.paused || target != nil && (target.worker != worker || !target.closed) {
		worker.mu.Unlock()
		returned = true
		return ErrPreflightScope
	}
	tasks := make([]*serviceDialTask, 0, len(worker.serviceDials))
	for task := range worker.serviceDials {
		if target == nil || task.outbound == target {
			tasks = append(tasks, task)
		}
	}
	worker.mu.Unlock()
	for _, task := range tasks {
		task.cancel()
	}
	for _, task := range tasks {
		task.closeAttempts()
	}
	var result error
	for _, task := range tasks {
		select {
		case <-task.done:
			result = errors.Join(result, task.cleanupErr)
		case <-ctx.Done():
			result = errors.Join(result, ErrPreflightCleanup, ctx.Err())
		}
	}
	returned = true
	return errors.Join(result, worker.failure())
}
