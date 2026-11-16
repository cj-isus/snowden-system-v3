//go:build windows

package core

import (
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
)

const (
	proxyRestored         = "restored"
	proxyNotWritten       = "not_written"
	proxyForeignPreserved = "foreign_preserved"
)

// Opaque component receipt only. No public boolean/error/tuple/port can create
// it. It never attests Engine, WFP, all-process cleanup or durable recovery.
type proxyRestoreReceipt struct {
	proxy       *ProxyManager
	native      *proxyRegistryKeyOwner
	writerLease *filetxn.WindowsWriterLease
	store       *proxyReceiptStore
	outcome     string
	consumed    bool // exceptional conflict conversion is one-shot
}

func (pm *ProxyManager) observeConflictLocked(err error) {
	if errors.Is(err, ErrProxyOwnershipConflict) {
		pm.conflictObserved = true
	}
}

func (pm *ProxyManager) retireForeignLocked() (*proxyRestoreReceipt, error) {
	if !pm.conflictObserved || pm.pending || pm.phaseErrorLocked() != nil || !pm.native.ready() {
		return nil, ErrProxyStateUncertain // conflict cannot override any unknown original phase
	}
	receipt, err := pm.retireKeyLocked(proxyForeignPreserved) // no registry write or WinINET notify
	if err != nil {
		return nil, err
	}
	return receipt, ErrProxyOwnershipConflict // keep public Restore's conflict outcome
}

// Short value-only lease. Manager may clear its original proxyActive under
// this lease; no native work/events/callbacks occur there. Order: proxy owner
// -> ProxyManager -> Manager. Manager never holds its lock over proxy I/O.
func (receipt *proxyRestoreReceipt) guardForeign(proxy *ProxyManager) (func(), bool) {
	if receipt == nil || proxy == nil || receipt.proxy != proxy || receipt.native == nil {
		return nil, false
	}
	proxyOwnerMu.Lock()
	proxy.mu.Lock()
	release := func() { proxy.mu.Unlock(); proxyOwnerMu.Unlock() }
	native := receipt.native
	if receipt.consumed || receipt.outcome != proxyForeignPreserved || proxy.lastRestore != receipt || proxy.native != native ||
		receipt.writerLease == nil || proxy.writerLease != receipt.writerLease || !receipt.writerLease.Retired() ||
		receipt.store == nil || proxy.receiptStore != receipt.store || !receipt.store.closed || receipt.store.pending || receipt.store.active ||
		!proxy.conflictObserved || proxyOwner == proxy || proxy.key != 0 || proxy.originalSet || proxy.writes || proxy.pending ||
		!native.closed || native.handle != 0 || !native.closeStarted || !native.closeReturned || native.closeErr != nil || native.read != nil {
		release()
		return nil, false
	}
	return release, true
}

// Last component result, never a forwarding/whole-cleanup permission.
func (manager *Manager) ProxyRestoreOutcome() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.proxyRestoreOutcome
}
