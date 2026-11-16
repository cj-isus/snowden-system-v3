//go:build windows

package core

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func (owner *proxyCrashRecovery) observe() (proxyRegistryState, error) {
	if err := owner.checkSource(); err != nil {
		return proxyRegistryState{}, err
	}
	if !owner.native.ready() {
		return proxyRegistryState{}, ErrProxyStateUncertain
	}
	return owner.native.readState()
}

func (owner *proxyCrashRecovery) resolvePrepared(changed bool) error {
	next := owner.current.record
	if next.phase != proxyReceiptPrepared || next.generation == ^uint64(0) {
		return ErrProxyStateUncertain
	}
	if changed {
		next.current = next.target
		next.written |= 1 << next.index
	}
	next.phase, next.index, next.target = proxyReceiptActive, proxyReceiptNoIndex, next.current
	next.generation++
	return owner.push(next)
}

func (owner *proxyCrashRecovery) preserveForeign() error {
	if !owner.native.ready() || !owner.phasesKnown() {
		return ErrProxyStateUncertain
	}
	if owner.current.record.phase == proxyReceiptPrepared {
		// Cancel only pending restoration authority. Keep historical current;
		// foreign observed bytes are never adopted or attributed to our setter.
		if err := owner.resolvePrepared(false); err != nil {
			return err
		}
	}
	next := owner.current.record
	next.generation++
	next.phase, next.index, next.target = proxyReceiptForeign, proxyReceiptNoIndex, next.current
	if err := owner.push(next); err != nil {
		return err
	}
	owner.outcome = proxyForeignPreserved
	return nil // no setter or WinINET notification after observing a foreign tuple
}

func (owner *proxyCrashRecovery) restoreValue(index int) error {
	current := owner.current.record
	if current.phase != proxyReceiptActive || index < 0 || index >= len(proxyValueNames) {
		return ErrProxyStateUncertain
	}
	after := current.current
	after[index] = current.original[index]
	if after.equal(current.current) {
		return nil
	}
	if owner.writes[index] != nil {
		return ErrProxyStateUncertain // only a fresh grant may retire an unchanged failed callback
	}
	if current.written&(1<<index) == 0 {
		return ErrProxyReceiptUnavailable // unchanged original fields never earn restore permission
	}
	observed, err := owner.observe()
	if err != nil {
		if errors.Is(err, ErrProxyOwnershipConflict) && owner.native.ready() {
			return owner.preserveForeign()
		}
		return err
	}
	if !observed.equal(current.current) {
		return owner.preserveForeign()
	}
	next := current
	next.generation++
	next.phase, next.index, next.target = proxyReceiptPrepared, byte(index), after
	if err := owner.push(next); err != nil {
		return err
	}
	// Repeat the whole tuple fence after journal I/O, directly before the setter.
	observed, err = owner.observe()
	if err != nil {
		if errors.Is(err, ErrProxyOwnershipConflict) && owner.native.ready() {
			return owner.preserveForeign()
		}
		return err
	}
	if !observed.equal(current.current) {
		return owner.preserveForeign()
	}
	if err := owner.nativeIntentAllowed(); err != nil {
		owner.pauseContext(err, proxyRecoveryPauseBeforeWrite, index)
		return err
	}
	attempt := &proxyWriteAttempt{index: index, before: current.current, target: after}
	owner.writes[index] = attempt // before the original setter; this bounded slot is never replaced
	writeErr := attempt.call(registry.Key(owner.native.handle))
	if !attempt.callReturned {
		return ErrProxyStateUncertain // no alias read/Close after an unreturned original setter
	}
	attempt.readStarted = true
	attempt.observed, attempt.readErr = owner.native.readState()
	attempt.readReturned = true
	if writeErr != nil && attempt.readErr == nil && attempt.observed.equal(current.current) && owner.native.ready() {
		attempt.outcome = proxyWriteUnchanged
		if err := owner.resolvePrepared(false); err != nil {
			return err
		}
		if !errors.Is(writeErr, ErrProxyStateUncertain) {
			owner.pause, owner.pauseIndex = proxyRecoveryPauseWriteUnchanged, index
		}
		return writeErr // confirmed unchanged, but outstanding original writes still require recovery
	}
	if writeErr != nil || attempt.readErr != nil || !attempt.observed.equal(after) || !owner.native.ready() {
		attempt.outcome = proxyWriteUnknown
		return errors.Join(ErrProxyStateUncertain, writeErr, attempt.readErr)
	}
	attempt.outcome = proxyWriteConfirmed
	return owner.resolvePrepared(true)
}

func (owner *proxyCrashRecovery) restoreValues() error {
	observed, err := owner.observe()
	if err != nil {
		if errors.Is(err, ErrProxyOwnershipConflict) && owner.native.ready() {
			return owner.preserveForeign()
		}
		return err
	}
	current := owner.current.record
	if current.phase == proxyReceiptPrepared {
		switch {
		case observed.equal(current.target):
			if err := owner.resolvePrepared(true); err != nil {
				return err
			}
		case observed.equal(current.current):
			if err := owner.resolvePrepared(false); err != nil {
				return err
			}
		default:
			return owner.preserveForeign()
		}
		current = owner.current.record
	}
	if !observed.equal(current.original) {
		if !observed.equal(current.current) {
			return owner.preserveForeign()
		}
		for _, index := range [3]int{proxyEnableIndex, proxyPACIndex, proxyServerIndex} {
			if err := owner.restoreValue(index); err != nil {
				return err
			}
			if owner.outcome == proxyForeignPreserved {
				return nil
			}
		}
	}
	observed, err = owner.observe()
	if err != nil {
		return err
	}
	if !observed.equal(owner.current.record.original) {
		return owner.preserveForeign()
	}
	if owner.current.record.written != 0 {
		if err := owner.nativeIntentAllowed(); err != nil {
			owner.pauseContext(err, proxyRecoveryPauseBeforeNotify, -1)
			return err
		}
		if owner.notification != nil {
			return ErrProxyStateUncertain // an old callback cannot be overwritten by an ordinary re-entry
		}
		owner.notification = &proxyNotificationAttempt{state: observed} // one original existing notification owner
		if err := owner.notification.run(); err != nil {
			if owner.notification.returned && !errors.Is(err, ErrProxyStateUncertain) {
				owner.pause, owner.pauseIndex = proxyRecoveryPauseNotification, -1
			}
			return err
		}
		observed, err = owner.observe()
		if err != nil {
			return err
		}
		if !observed.equal(owner.current.record.original) {
			return owner.preserveForeign()
		}
	}
	next := owner.current.record
	next.generation++
	next.phase, next.index = proxyReceiptClosed, proxyReceiptNoIndex
	next.current, next.target = next.original, next.original
	if err := owner.push(next); err != nil {
		return err
	}
	owner.outcome = proxyRestored
	if next.written == 0 {
		owner.outcome = proxyNotWritten
	}
	return nil
}
