package core

import "context"

// Exact private consume attempt after the original prepared slot was burned.
// This metadata cannot be a forwarding grant; shared negative clear revokes it.
type workerTUNConsumption struct {
	permit          *workerTUNPreparedPermit
	originalReceipt *workerNativeTUNReceipt
}

func (engine *Engine) consumeLocalNativeForwarding(ctx context.Context, subject, selector string, epoch uint64, nonce, token string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	actual, selected, guarded, tunMode, err := engine.forwardingSubjectLocked()
	if err != nil || guarded == nil {
		return ErrForwardingNotAdmitted
	}
	gate := guarded.admission
	var original *workerTUNConsumption
	if err := func() error {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		permit := gate.nativePending
		valid := permit.currentLocked(gate, subject, selector, epoch, nonce)
		gate.clearPendingLocked() // burn original candidate BEFORE any context callback
		if !NativeTUNForwardingAvailable() || !tunMode || actual != subject || selected != selector || !valid ||
			!permit.emitted || !workerSubject(token) || permit.token != token {
			return ErrForwardingNotAdmitted
		}
		original = &workerTUNConsumption{permit: permit, originalReceipt: gate.nativeReceipt}
		gate.nativeConsuming = original
		return nil
	}(); err != nil {
		return err
	}
	defer func() {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		if gate.nativeConsuming == original {
			gate.nativeConsuming = nil // never erase a successor or an ordinary issuer
		}
	}()
	// The outer original Engine owner remains serialized. Admission close/fault
	// can still seal while caller/source context methods run outside gate.mu.
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return ErrForwardingNotAdmitted
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	permit := original.permit
	if gate.nativeConsuming != original || gate.nativePending != nil || gate.nativeReceipt != original.originalReceipt ||
		!permit.emitted || !permit.matchesLocked(gate, subject, selector, epoch, nonce) || contextError(operation) != nil {
		return ErrForwardingNotAdmitted
	}
	gate.nativeConsuming = nil
	gate.open.Store(true)
	if contextError(operation) != nil {
		gate.open.Store(false)
		return ErrForwardingNotAdmitted
	}
	return nil
}
