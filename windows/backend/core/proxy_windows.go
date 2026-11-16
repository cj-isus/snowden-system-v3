//go:build windows

package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows/registry"
)

const (
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxyEnableValue    = `ProxyEnable`
	proxyServerValue    = `ProxyServer`
	autoConfigURLValue  = `AutoConfigURL`
)

var (
	ErrProxyOwnershipConflict  = errors.New("system proxy changed outside this owner; settings were not overwritten")
	ErrProxyStateUncertain     = errors.New("system proxy write/readback or original handle retirement is unconfirmed")
	ErrProxyReceiptUnavailable = errors.New("proxy recovery requires a verified original write receipt; a loopback port is not ownership")
	proxyOwnerMu               sync.Mutex
	proxyOwner                 *ProxyManager // original writer/handle, including failed cleanup
)

// ProxySnapshot is a display projection, not a restore or ownership receipt.
type ProxySnapshot struct {
	ProxyEnable   uint32
	ProxyServer   string
	AutoConfigURL string
}

// Process-local receipts and the encrypted write journal do not authorize
// crash/startup recovery without the separate original-exit or manual admission.
// Tuple fences are cooperative observations, not atomic OS CAS or a barrier
// against another process/MDM changing settings between native calls.
type ProxyManager struct {
	mu                           sync.Mutex
	key                          registry.Key
	native                       *proxyRegistryKeyOwner // actual original HKEY; key is a borrowed read/write alias
	writerLease                  *filetxn.WindowsWriterLease
	receiptStore                 *proxyReceiptStore
	write                        *proxyWriteAttempt
	notification                 *proxyNotificationAttempt
	conflictObserved             bool // sticky for this original snapshot; later matching bytes cannot erase it
	lastRestore                  *proxyRestoreReceipt
	manualRecovery               *proxyManualRecovery
	original, current            proxyRegistryState
	originalSet, writes, pending bool
	applied                      bool // full SetProxy/notification phase; not a restore write receipt
	uncertain, closeErr          error
}

// Snapshot reserves cleanup ownership but grants no restore write permission.
// A failed/unused snapshot closes only its own original handle.
func (pm *ProxyManager) Snapshot() error {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.reserveSnapshotLocked(); err != nil {
		return err
	}
	if err := pm.acquireWriterLocked(); err != nil {
		return err
	}
	if err := pm.validateWriterLocked(); err != nil {
		return err
	}
	if err := pm.native.open(registry.QUERY_VALUE | registry.SET_VALUE); err != nil {
		return fmt.Errorf("open system proxy: %w", err)
	}
	pm.key = registry.Key(pm.native.handle)
	state, err := pm.native.readState()
	if err != nil {
		pm.observeConflictLocked(err)
		return err
	}
	pm.original, pm.current, pm.originalSet = state, state, true
	pm.pending = true // the initial durable record precedes every possible registry mutation
	if err := pm.receiptStore.begin(state); err != nil {
		pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
		return pm.uncertain
	}
	pm.pending = false
	return nil
}

// Both original proxy locks held; bookkeeping only. Preview shares the exact
// reservation and storage domain without invoking automatic system recovery.
func (pm *ProxyManager) reserveSnapshotLocked() error {
	if proxyOwner != nil || pm.key != 0 || pm.originalSet || pm.pending || proxyObservation != nil || pm.native != nil && !pm.native.closed || pm.writerLease != nil && !pm.writerLease.Retired() || pm.receiptStore != nil && !pm.receiptStore.closed || pm.manualRecovery != nil && !pm.manualRecovery.finished {
		return ErrProxyStateUncertain
	}
	pm.conflictObserved, pm.lastRestore = false, nil
	pm.applied = false                                   // a fresh native snapshot cannot inherit a previous SetProxy phase
	pm.writerLease = nil                                 // only an already confirmed retired predecessor can be replaced
	pm.receiptStore = nil                                // only an already closed predecessor can be replaced
	pm.manualRecovery = nil                              // only a finished/cancelled predecessor can be replaced
	pm.native, proxyOwner = &proxyRegistryKeyOwner{}, pm // retain before the original native Open
	return nil
}

func (pm *ProxyManager) SetProxy(socksAddr string) error {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.readyLocked(); err != nil {
		return err
	}
	pm.applied = false // a later/failed application cannot reuse a previous positive phase
	if socksAddr != "127.0.0.1:1080" {
		return errors.New("unsupported system proxy endpoint")
	}
	server, err := proxyStringValue("socks="+socksAddr, registry.SZ)
	if err != nil {
		return err
	}
	// Address/PAC first, enable last. Each write needs complete tuple readback.
	for _, change := range []struct {
		index int
		value proxyRegistryValue
	}{
		{proxyServerIndex, server}, {proxyPACIndex, proxyRegistryValue{}},
		{proxyEnableIndex, proxyDWORDValue(1)},
	} {
		if err := pm.writeLocked(change.index, change.value); err != nil {
			return err
		}
	}
	if err := pm.notifyLocked(); err != nil {
		return err
	}
	pm.applied = true
	return nil
}

// Read through the same original writer handle, never a display snapshot/new
// handle or a guessed loopback port. This does not create restore authority:
// a no-op SetProxy still has no own-write receipt. External ABA/OS CAS and
// consumers refreshing after WinINET notification remain outside this check.
func (pm *ProxyManager) ValidateAppliedProxy() error {
	if pm == nil {
		return errProxyAppliedStateUnavailable
	}
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.readyLocked(); err != nil {
		pm.applied = false
		return err
	}
	if !pm.applied {
		return errProxyAppliedStateUnavailable
	}
	pm.applied = false // a panic/unknown read cannot retain the earlier positive phase
	observed, err := pm.native.readState()
	if err != nil {
		pm.observeConflictLocked(err)
		return err
	}
	if !observed.equal(pm.current) {
		pm.conflictObserved = true
		return ErrProxyOwnershipConflict
	}
	pm.applied = true // only this completed, exact original-handle observation restores the phase
	return nil
}

func (pm *ProxyManager) readyLocked() error {
	if pm.closeErr != nil || pm.pending {
		return errors.Join(ErrProxyStateUncertain, pm.closeErr, pm.uncertain)
	}
	if err := pm.phaseErrorLocked(); err != nil {
		return err
	}
	if pm.conflictObserved {
		return ErrProxyOwnershipConflict
	}
	if proxyOwner != pm || !pm.native.ready() || pm.key != registry.Key(pm.native.handle) || !pm.originalSet {
		return errors.New("system proxy has no original snapshot owner")
	}
	if err := pm.validateWriterLocked(); err != nil {
		return err
	}
	if !pm.receiptStore.ready(pm.current) {
		return errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable)
	}
	return nil
}

func (pm *ProxyManager) writeLocked(index int, value proxyRegistryValue) error {
	if err := pm.readyLocked(); err != nil {
		return err
	}
	if index < 0 || index >= len(proxyValueNames) {
		return ErrProxyStateUncertain
	}
	before, err := pm.native.readState()
	if err != nil {
		pm.observeConflictLocked(err)
		return err
	}
	if !before.equal(pm.current) {
		pm.conflictObserved = true
		return ErrProxyOwnershipConflict
	}
	after := pm.current
	after[index] = value
	if after.equal(before) {
		return nil
	}
	attempt := &proxyWriteAttempt{index: index, before: before, target: after}
	pm.write = attempt // retain the exact mutation and observations before the adapter callback
	pm.pending = true  // panic/unknown native outcome retains this owner
	if err := pm.receiptStore.prepare(index, before, after); err != nil {
		pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
		return pm.uncertain // never call the registry setter without a published encrypted intent
	}
	if err := pm.validateWriterLocked(); err != nil {
		pm.uncertain = err
		return err // original lease must still validate after receipt file I/O
	}
	// Journal I/O can take time. Repeat the full tuple fence immediately before
	// the setter; this narrows the race, but does not provide atomic OS CAS.
	checked, checkErr := pm.native.readState()
	if checkErr != nil || !checked.equal(before) {
		pm.observeConflictLocked(checkErr)
		if checkErr == nil {
			pm.conflictObserved, checkErr = true, ErrProxyOwnershipConflict
		}
		if !pm.native.ready() {
			pm.uncertain = errors.Join(ErrProxyStateUncertain, checkErr)
			return pm.uncertain
		}
		// The setter never started. Cancel only this prepared intent, retaining
		// the last confirmed own tuple; do not adopt the newly observed tuple.
		if err := pm.receiptStore.confirm(before, false); err != nil {
			pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
			return pm.uncertain
		}
		pm.write, pm.pending = nil, false
		return checkErr
	}
	writeErr := attempt.call(pm.key)
	if !attempt.callReturned {
		pm.uncertain = ErrProxyStateUncertain
		return pm.uncertain // no readback or cleanup I/O after unknown original callback
	}
	attempt.readStarted = true
	attempt.observed, attempt.readErr = pm.native.readState()
	attempt.readReturned = true
	observed, readErr := attempt.observed, attempt.readErr
	if writeErr != nil || readErr != nil || !observed.equal(after) {
		if writeErr != nil && readErr == nil && observed.equal(before) {
			if err := pm.receiptStore.confirm(observed, false); err != nil {
				pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
				return pm.uncertain
			}
			attempt.outcome = proxyWriteUnchanged
			pm.pending = false // confirmed unchanged, no new write authority
			return writeErr
		}
		attempt.outcome = proxyWriteUnknown
		pm.uncertain = errors.Join(ErrProxyStateUncertain, writeErr, readErr)
		return pm.uncertain
	}
	if err := pm.receiptStore.confirm(observed, true); err != nil {
		pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
		return pm.uncertain // native mutation is known, durable confirmation is not
	}
	attempt.outcome = proxyWriteConfirmed
	pm.current, pm.pending, pm.writes = after, false, true
	return nil
}

func (pm *ProxyManager) Restore() error {
	_, err := pm.restoreOriginal()
	return err
}

func (pm *ProxyManager) restoreOriginal() (*proxyRestoreReceipt, error) {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.key == 0 && !pm.originalSet && (pm.native == nil || pm.native.closed) && (pm.writerLease == nil || pm.writerLease.Retired()) && (pm.receiptStore == nil || pm.receiptStore.closed) {
		if pm.lastRestore != nil && pm.lastRestore.outcome == proxyForeignPreserved {
			return pm.lastRestore, ErrProxyOwnershipConflict
		}
		return pm.lastRestore, nil
	}
	if pm.closeErr != nil {
		return nil, errors.Join(ErrProxyStateUncertain, pm.closeErr)
	}
	if proxyOwner != pm {
		return nil, ErrProxyStateUncertain
	}
	pm.applied = false // retirement cannot leave an earlier application phase usable
	if pm.native == nil || pm.key != 0 && pm.key != registry.Key(pm.native.handle) {
		return nil, ErrProxyStateUncertain // a borrowed numeric alias cannot mint native custody
	}
	if (pm.native.closeStarted || pm.originalSet && !pm.native.ready()) &&
		!(pm.receiptStore != nil && pm.receiptStore.recoveredForeign(pm) && !pm.originalSet) {
		return nil, ErrProxyStateUncertain // never read/write through an alias after an unknown Close
	}
	if err := pm.phaseErrorLocked(); err != nil {
		return nil, err
	}
	if pm.pending {
		return nil, errors.Join(ErrProxyStateUncertain, pm.uncertain)
	}
	if pm.originalSet {
		if err := pm.validateWriterLocked(); err != nil {
			return nil, err
		}
	}
	if !pm.originalSet && pm.receiptStore != nil && pm.receiptStore.recoveredForeign(pm) {
		pm.conflictObserved = true
		receipt, err := pm.retireKeyLocked(proxyForeignPreserved)
		return receipt, errors.Join(ErrProxyOwnershipConflict, err)
	}
	if pm.conflictObserved {
		return pm.retireForeignLocked()
	}
	outcome := proxyNotWritten
	if pm.originalSet && pm.writes {
		outcome = proxyRestored
		observed, err := pm.native.readState()
		if err != nil {
			pm.observeConflictLocked(err)
			if pm.conflictObserved {
				return pm.retireForeignLocked()
			}
			return nil, err
		}
		if !observed.equal(pm.original) {
			if !observed.equal(pm.current) {
				pm.conflictObserved = true
				return pm.retireForeignLocked()
			}
			// Reverse only confirmed own writes. Partial restore earns receipts.
			for _, index := range []int{proxyEnableIndex, proxyPACIndex, proxyServerIndex} {
				if err := pm.writeLocked(index, pm.original[index]); err != nil {
					if pm.conflictObserved && !pm.pending && pm.phaseErrorLocked() == nil {
						return pm.retireForeignLocked()
					}
					return nil, err
				}
			}
		}
		if err := pm.notifyLocked(); err != nil {
			return nil, err
		}
	}
	return pm.retireKeyLocked(outcome)
}

// Both existing proxy locks held. Persist the final encrypted component result
// before HKEY/lease retirement. This never mutates registry settings; unknown
// native/query/write/notify phases were checked before entering it.
func (pm *ProxyManager) retireKeyLocked(outcome string) (*proxyRestoreReceipt, error) {
	if pm.receiptStore != nil {
		pm.pending = true
		if err := pm.receiptStore.finish(outcome, pm.current); err != nil {
			pm.uncertain = errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
			return nil, pm.uncertain
		}
		pm.pending = false
	}
	if err := pm.native.close(); err != nil {
		pm.closeErr = err // no second Close/replacement used as retirement proof
		return nil, errors.Join(ErrProxyStateUncertain, err)
	}
	if pm.writerLease != nil {
		if err := pm.writerLease.Close(); err != nil {
			pm.closeErr = err
			return nil, errors.Join(ErrProxyStateUncertain, err)
		}
		if !pm.writerLease.Retired() {
			pm.closeErr = ErrProxyStateUncertain
			return nil, pm.closeErr
		}
	}
	receipt := &proxyRestoreReceipt{proxy: pm, native: pm.native, writerLease: pm.writerLease, store: pm.receiptStore, outcome: outcome}
	pm.key, pm.originalSet, pm.writes = 0, false, false
	pm.applied = false
	pm.original, pm.current = proxyRegistryState{}, proxyRegistryState{}
	pm.write, pm.notification = nil, nil // after confirmed restore/preservation and original Close; not secure erasure
	proxyOwner = nil
	if pm.manualRecovery != nil && !pm.manualRecovery.consumed && !pm.manualRecovery.executing {
		// Ordinary Stop may retire an unused preparation without its UI token.
		// Mark it finished only after this exact native/store/lease cleanup.
		pm.manualRecovery.invalidated, pm.manualRecovery.finished = true, true
	}
	pm.lastRestore = receipt
	return receipt, nil
}
