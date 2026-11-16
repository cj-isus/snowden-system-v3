package core

import (
	"context"
	"errors"
)

var errProxyAppliedStateUnavailable = errors.New("system proxy applied-state verification unavailable")

// A running session's original local proxy observation failed. This is neither
// a channel-health measurement nor, by itself, a resource-retirement result.
var ErrProxyObservationUnavailable = errors.New("running system proxy observation unavailable")

// ErrProxyWriterBusy describes local coordination contention, not a failed channel.
var ErrProxyWriterBusy = errors.New("system proxy storage writer is busy")

// Finite refusal reasons for authenticated unresolved journal intake. None is a
// restoration grant or a public original-process/registry ownership receipt.
var (
	ErrProxyPriorOwnerRunning          = errors.New("recorded system proxy owner was observed running")
	ErrProxyPriorOwnerChanged          = errors.New("recorded system proxy PID belongs to a different process creation")
	ErrProxyPriorOwnerUnresolved       = errors.New("recorded system proxy process lifetime could not be confirmed")
	ErrProxyRecoveryRequired           = errors.New("recorded system proxy owner exited; recovery admission is required")
	ErrProxyReceiptSubjectChanged      = errors.New("original system proxy recovery receipt changed or could not be revalidated")
	ErrProxyAnchorUnavailable          = errors.New("system proxy journal registry anchor is missing, inconsistent or unconfirmed")
	ErrProxyAnchorPending              = errors.New("system proxy journal has an unresolved registry anchor transition")
	ErrProxyAnchorMigrationRequired    = errors.New("existing system proxy receipt requires explicit registry anchor migration")
	ErrProxyAnchorOldFile              = errors.New("pending proxy journal currently contains the old committed file; target publication remains unconfirmed")
	ErrProxyAnchorMissingInitial       = errors.New("initial proxy journal file was not observed after pending anchor enrollment")
	ErrProxyStageUnavailable           = errors.New("encrypted proxy journal staging operation is unavailable or unconfirmed")
	ErrProxyStageConflict              = errors.New("encrypted proxy journal stage does not match its original parent or file source")
	ErrProxyRecoveryConsentUnavailable = errors.New("manual proxy recovery requires an available source-bound confirmation")
	ErrProxyRecoveryConsentExpired     = errors.New("manual proxy recovery confirmation expired or was already used")
	ErrProxyRecoveryConsentInvalid     = errors.New("manual proxy recovery confirmation does not match this operation")
)

// Optional refinement keeps explicit injected/noop adapters on their existing
// contract. The actual native ProxyManager supplies original-handle readback;
// a display ProxySnapshot or success from an unrelated adapter is not that proof.
type systemProxyValidation interface {
	ValidateAppliedProxy() error
}

// Native/adapter work stays outside Manager/Engine locks. Callers still own the
// exact lifecycle operation; final completion repeats context/source/owner checks.
// This is a bounded observation, not atomic registry CAS or a new OS writer.
func (manager *Manager) validateAppliedProxy(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	manager.mu.RLock()
	proxy, active, conflict := manager.proxy, manager.proxyActive, manager.proxyConflict
	manager.mu.RUnlock()
	if conflict {
		return ErrProxyOwnershipConflict
	}
	if proxy == nil {
		return errProxyAppliedStateUnavailable
	}
	validator, supported := proxy.(systemProxyValidation)
	if !supported {
		return contextError(ctx) // explicit custom/TUN noop contract, not native proxy ownership
	}
	if !active {
		return errProxyAppliedStateUnavailable
	}
	err := cleanupCall("validate applied system proxy", validator.ValidateAppliedProxy)
	if errors.Is(err, ErrProxyOwnershipConflict) {
		manager.mu.Lock()
		manager.proxyConflict = true // negative evidence only; cannot authorize retirement or forwarding
		manager.mu.Unlock()
	}
	if err != nil {
		return errors.Join(errProxyAppliedStateUnavailable, err, contextError(ctx))
	}
	return contextError(ctx)
}

// Runs after the original lifecycle finalizer. Cleanup may discover a foreign
// change while retiring a failed candidate for another reason. Do not lose that
// local conflict behind a network/probe error and then retry another channel.
func (manager *Manager) retainProxyConflict(result *error) {
	manager.mu.RLock()
	conflict := manager.proxyConflict
	manager.mu.RUnlock()
	if conflict {
		*result = errors.Join(revokeVerifiedRollback(*result), ErrProxyOwnershipConflict)
	}
}
