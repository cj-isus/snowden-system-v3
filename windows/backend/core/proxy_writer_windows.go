//go:build windows

package core

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

// Fixed local profile domain, not APPDATA/working-directory input. Its lock
// coordinates clients using this same storage namespace; it cannot attest
// profile redirection, legacy scripts or uncooperative registry writers.
func (pm *ProxyManager) acquireWriterLocked() error {
	return pm.acquireWriterForLocked(true)
}

func (pm *ProxyManager) acquireWriterForLocked(automaticRecovery bool) error {
	if pm.writerLease != nil {
		return ErrProxyStateUncertain
	}
	pm.pending = true // retain the ProxyManager before native path/lease acquisition
	base, err := windows.GetCurrentProcessToken().KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		pm.pending = false
		return errors.Join(ErrProxyStateUncertain, err)
	}
	if base == "" || len(base) > 32000 || strings.ContainsRune(base, 0) || !filepath.IsAbs(base) {
		pm.pending = false
		return ErrProxyStateUncertain
	}
	path := filepath.Join(base, "snowden-system", "proxy-recovery", "session")
	pm.writerLease = filetxn.NewWindowsWriterLease(path) // before any lock/parent/file native acquisition
	err = pm.writerLease.Acquire()
	pm.pending = false
	if errors.Is(err, filetxn.ErrWriterBusy) {
		return ErrProxyWriterBusy
	}
	if err != nil {
		return errors.Join(ErrProxyStateUncertain, err)
	}
	if err := pm.validateWriterLocked(); err != nil {
		return err
	}
	pm.receiptStore = newProxyReceiptStore(filepath.Join(base, "snowden-system", "proxy-recovery", "receipt.v1.bin"), pm.writerLease)
	pm.pending = true // store is retained before SID/process/file/DPAPI acquisition
	err = pm.receiptStore.open()
	if err == nil && automaticRecovery && pm.receiptStore.crashRecovery != nil {
		err = pm.receiptStore.recoverPrior(pm)
	}
	if pm.receiptStore.operationsReturned() {
		pm.pending = false
	} else if err == nil {
		err = ErrProxyStateUncertain
	}
	if err != nil {
		if pm.receiptStore.recoveredForeign(pm) {
			return ErrProxyOwnershipConflict // known component preservation; cleanup still owns both original handles
		}
		return errors.Join(ErrProxyStateUncertain, ErrProxyReceiptUnavailable, err)
	}
	return nil
}

func (pm *ProxyManager) validateWriterLocked() error {
	if pm.writerLease == nil {
		return ErrProxyStateUncertain
	}
	if err := pm.writerLease.Validate(); err != nil {
		return errors.Join(ErrProxyStateUncertain, err)
	}
	return nil
}
