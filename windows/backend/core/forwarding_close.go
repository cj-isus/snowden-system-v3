package core

import (
	"context"
	"errors"
)

// Original drain slot, reserved before post-SDK cohort snapshots. Sequence
// result is read by another caller only after done; neither is native proof.
type forwardingDrain struct {
	done       chan struct{}
	sequence   lifecycleCleanup
	closing    *forwardingClose
	rejections []*forwardingRejection
	inputs     []*mixedIngress
}

// Revoke/cancel before SDK Close, drain afterwards: SDK callbacks may need
// Close itself. Publish the exact cohort before a cancellation can unwind.
func (admission *forwardingAdmission) beginClose() forwardingClose {
	admission.open.Store(false)
	closing := func() forwardingClose {
		admission.mu.Lock()
		defer admission.mu.Unlock()
		if admission.closeScope == nil {
			admission.closeScope = &forwardingClose{admission: admission,
				routes: admission.sealRoutesLocked(), inputs: admission.mixedInputsLocked(), scope: admission.tunScope}
			admission.clearPendingLocked()
			admission.nativeReceipt = nil
			if admission.tunScope != nil {
				admission.tunScope.deactivateLocked()
			}
		}
		return *admission.closeScope
	}()
	for _, route := range closing.routes {
		route.cancel()
	}
	for _, input := range closing.inputs {
		input.cancel()
	}
	return closing
}

func (admission *forwardingAdmission) finishClose(ctx context.Context, closing forwardingClose) (result error) {
	admission.mu.Lock()
	if closing.admission != admission || admission.closeScope == nil {
		admission.mu.Unlock()
		return ErrEngineCleanupFailed
	}
	if previous := admission.drainOwner; previous != nil {
		admission.mu.Unlock()
		select {
		case <-previous.done:
			if fault := admission.localRouteFault(); fault != nil {
				return errors.Join(previous.sequence.result, fault)
			}
			return previous.sequence.result
		default:
			return ErrEngineCleanupFailed // no reentrant wait/retry against the original native scope
		}
	}
	owner := &forwardingDrain{done: make(chan struct{}), closing: admission.closeScope}
	admission.drainOwner = owner // before snapshots or a callback; one original drain per admission
	admission.mu.Unlock()
	captured := false
	defer func() {
		admission.mu.Lock()
		defer admission.mu.Unlock()
		if admission.routeFault {
			owner.sequence.result = errors.Join(owner.sequence.result, ErrEngineCleanupFailed)
		}
		if !captured {
			owner.sequence.result = errors.Join(owner.sequence.result, ErrEngineCleanupFailed)
		}
		if owner.sequence.result != nil {
			owner.sequence.result = errors.Join(ErrEngineCleanupFailed, owner.sequence.result)
		}
		owner.sequence.drained, result = true, owner.sequence.result
		close(owner.done) // after every registered stage returned/unwound; errors remain separate
	}()
	// Reverse registration preserves scope -> routes -> rejections -> inputs.
	defer lifecycleCleanupStep("drain original mixed inputs", &owner.sequence, func() error {
		return admission.drainMixedInputs(ctx, owner.inputs)
	})
	defer lifecycleCleanupStep("drain original rejected inputs", &owner.sequence, func() error {
		return admission.drainRejections(ctx, owner.rejections)
	})
	defer lifecycleCleanupStep("drain original forwarding routes", &owner.sequence, func() error {
		return admission.drainRoutes(ctx, owner.closing.routes)
	})
	defer lifecycleCleanupStep("retire original TUN diagnostic scope", &owner.sequence, func() error {
		scope := owner.closing.scope
		if scope == nil {
			return nil
		}
		if err := scope.close(); err != nil {
			return err
		}
		admission.mu.Lock()
		defer admission.mu.Unlock()
		if admission.tunScope != scope {
			return ErrNativeScopeUnavailable
		}
		admission.tunScope = nil // only the successful original close can release this slot
		return nil
	})
	owner.rejections = admission.sealRejections() // remains AFTER the SDK Close attempt
	func() {
		admission.mu.Lock()
		defer admission.mu.Unlock()
		owner.inputs = admission.mixedInputsLocked()
	}()
	captured = true
	return nil
}
