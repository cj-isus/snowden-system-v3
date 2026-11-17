package core

import "context"

// Private original-worker/lifecycle receipt, never a decoded public report.
// The temporary diagnostic context is deliberately not the operation source:
// its normal release must not revoke a still-current lifecycle handoff.
type engineForwardingPermit struct {
	process                  *engineProcess
	operation                *nativeTUNOperation // existing metadata-only original scope; no new observer
	instance                 uint64
	subject, selector, token string
	epoch                    uint64
}

func (permit *engineForwardingPermit) currentOperation(operation *nativeTUNOperation) bool {
	return permit != nil && permit.operation != nil && operation != nil && permit.operation.Done() != nil &&
		permit.operation.Done() == operation.Done() && contextError(permit.operation) == nil && contextError(operation) == nil
}

// Only Manager commits this private observation after fresh lifecycle gates.
// Burn before context callbacks and every outcome. Capture handoff metadata
// outside Engine.mu while the exact permit remains revocable. The worker RPC
// uses a separately prepared stdlib child, never the metadata view as parent.
func (e *Engine) admitForwarding(ctx context.Context) error {
	e.mu.Lock()
	permit := e.forwardingPermit
	e.clearForwardingPermitsLocked()
	e.forwardingConsume = permit
	e.mu.Unlock()
	defer func() {
		if permit == nil {
			return
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.forwardingConsume == permit {
			e.forwardingConsume = nil // never discard a successor's in-flight receipt
		}
	}()
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return err
	}
	if !permit.currentOperation(operation) {
		return ErrForwardingNotAdmitted
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true // never retry an interrupted parent unlink from a defer
			cancel()
		}
	}
	defer cancelRPC() // fallback cleanup also runs outside Engine.mu
	if err := e.admitForwardingPrepared(rpc, operation, permit); err != nil {
		return err
	}
	cancelRPC() // ordinary return is required BEFORE the final original handoff check
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.forwardingConsume != permit || !permit.currentOperation(operation) || !e.isolated || e.process == nil || e.cleanupErr != nil ||
		e.process != permit.process || e.process.instance != permit.instance || e.process.unusable || e.process.exited() {
		return ErrForwardingNotAdmitted // never retire a successor from an old metadata attempt
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || subject != permit.subject {
		return ErrForwardingNotAdmitted
	}
	return nil // normal RPC-budget cancellation does not revoke the original operation
}

// The existing process/pipe owner remains serialized during the RPC. Its
// context is already prepared; constructor and final cancel are outside this
// lock. A native syscall or an unknown future Value lookup is not preempted.
func (e *Engine) admitForwardingPrepared(rpc *workerRPCContext, operation *nativeTUNOperation, permit *engineForwardingPermit) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.forwardingConsume != permit || !permit.currentOperation(operation) || !e.isolated || e.process == nil || e.cleanupErr != nil ||
		e.process != permit.process || e.process.instance != permit.instance {
		return ErrForwardingNotAdmitted
	}
	if err := contextError(rpc); err != nil {
		return err // expired prepared budget issued no command and reserved no protocol ID
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || subject != permit.subject {
		return ErrForwardingNotAdmitted
	}
	process := permit.process
	command := workerRequest{Action: "admit", Selector: "proxy", Tag: permit.selector, Subject: permit.subject, AdmissionEpoch: permit.epoch, AdmissionToken: permit.token}
	reply, err := process.request(rpc, command, workerControlTimeout)
	if err != nil {
		return e.retireIsolatedFailureLocked(err)
	}
	if e.forwardingConsume != permit || !permit.currentOperation(operation) || contextError(rpc) != nil || e.process != process || process.instance != permit.instance || process.exited() || !validWorkerControlReply(&command, reply) {
		return e.retireIsolatedFailureLocked(ErrForwardingNotAdmitted, contextError(permit.operation), contextError(operation), contextError(rpc))
	}
	return nil
}
