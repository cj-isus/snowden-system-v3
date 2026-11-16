package core

import (
	"context"
	"errors"
	"sync"

	box "github.com/sagernet/sing-box"
)

type admissionBox struct {
	*box.Box
	admission  *forwardingAdmission
	bootstrap  *bootstrapWorker
	closeMu    sync.Mutex // metadata only; never held across SDK/cancellation/drain
	closeOwner *admissionBoxClose
}

type admissionBoxClose struct {
	done     chan struct{}
	sequence lifecycleCleanup
}

// SDK Box.done closes before child cleanup. Only this original owner records
// our full close sequence; a repeated Close never replays SDK/native calls.
func (b *admissionBox) Close() (result error) {
	if b.admission != nil {
		b.admission.open.Store(false)
	}
	b.closeMu.Lock()
	if previous := b.closeOwner; previous != nil {
		b.closeMu.Unlock()
		select {
		case <-previous.done:
			if b.admission != nil {
				if fault := b.admission.localRouteFault(); fault != nil {
					return errors.Join(previous.sequence.result, fault)
				}
			}
			return previous.sequence.result
		default:
			return ErrEngineCleanupFailed // pending/reentrant is restrictive, not a second close receipt
		}
	}
	owner := &admissionBoxClose{done: make(chan struct{})}
	b.closeOwner = owner
	b.closeMu.Unlock()
	defer func() {
		if b.admission != nil {
			owner.sequence.result = errors.Join(owner.sequence.result, b.admission.localRouteFault())
		}
		if owner.sequence.result != nil {
			owner.sequence.result = errors.Join(ErrEngineCleanupFailed, owner.sequence.result)
		}
		owner.sequence.drained, result = true, owner.sequence.result
		close(owner.done)
	}()
	// Register the whole original order before any callback. Goexit in one
	// stage cannot skip later stages; blocked native calls remain pending.
	defer lifecycleCleanupStep("drain original forwarding admission", &owner.sequence, func() error {
		if b.admission == nil {
			return ErrEngineCleanupFailed
		}
		b.admission.mu.Lock()
		snapshot := b.admission.closeScope
		b.admission.mu.Unlock()
		if snapshot == nil {
			return ErrEngineCleanupFailed // do not manufacture an original revoked cohort
		}
		cleanup, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
		defer cancel()
		return b.admission.finishClose(cleanup, *snapshot)
	})
	defer lifecycleCleanupStep("close original SDK box", &owner.sequence, b.Box.Close)
	if b.bootstrap != nil {
		defer lifecycleCleanupStep("retire original bootstrap worker", &owner.sequence, b.bootstrap.close)
	}
	defer lifecycleCleanupStep("revoke original forwarding admission", &owner.sequence, func() error {
		b.admission.beginClose()
		return nil
	})
	if b.bootstrap != nil {
		defer lifecycleCleanupStep("revoke original bootstrap work", &owner.sequence, func() error {
			b.bootstrap.beginClose()
			return nil
		})
	}
	return nil
}
