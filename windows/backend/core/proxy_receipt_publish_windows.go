//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
)

// Shared by the active writer, no-write retirement and original-exit recovery.
// Their admission and return witnesses stay with the caller. This only publishes
// private metadata; it neither adopts a session nor accesses Internet Settings.
func (store *proxyReceiptStore) publishJournal(next proxyDurableReceipt, retirement *proxyNoWriteRetirement, recovery *proxyCrashRecovery) (published proxyJournalInput, result error) {
	if store == nil || !store.pending || store.closed || store.failure != nil ||
		store.subjectConflict || store.anchorConflict || !store.replayReturned() || !store.stage.idle() ||
		!store.priorProcessRetired() || !store.reader.Retired() || !store.crypto.retired() || !store.anchor.idle() ||
		!bytes.Equal(next.sid, store.record.sid) {
		return published, ErrProxyReceiptUnavailable
	}
	if recovery != nil {
		if retirement != nil || !recovery.admits(store, next) {
			return published, ErrProxyReceiptUnavailable
		}
	} else if store.previous != nil || !store.crashReturned() {
		return published, ErrProxyReceiptUnavailable
	} else if retirement == nil {
		if !store.opened || !store.noWriteReturned() {
			return published, ErrProxyReceiptUnavailable
		}
	} else if store.opened || store.noWriteRetirement != retirement || !retirement.attempted || retirement.returned || retirement.completed ||
		!retirement.matchesOwner() || !proxyReceiptClosesNoWrites(retirement.source.record, next) {
		return published, ErrProxyReceiptUnavailable
	}
	if err := store.writer.Check(); err != nil {
		return published, ErrProxyStateUncertain
	}
	if err := store.validateLease(); err != nil {
		return published, err
	}
	if err := store.checkFileSource(); err != nil {
		return published, err
	}
	plain, err := encodeProxyReceipt(next)
	if err != nil {
		return published, err
	}
	digest := sha256.Sum256(plain)
	cipher, err := store.crypto.transform(plain, true)
	if store.crypto.retired() {
		clear(plain)
	}
	if err != nil {
		return published, err
	}
	if err := store.validateLease(); err != nil {
		return published, err
	}
	raw := append([]byte(proxyCipherPurpose), cipher...)
	store.writeRaw = raw // exact borrowed input survives an unknown original operation
	if err := store.stageIntent(next, raw); err != nil {
		return published, err
	}
	if err := store.checkFileSource(); err != nil {
		return published, err
	}
	if err := store.prepareAnchor(next, raw); err != nil {
		return published, errors.Join(ErrProxyAnchorUnavailable, err)
	}
	if err := store.validateLease(); err != nil {
		return published, err
	}
	if err := store.checkFileSource(); err != nil {
		return published, err
	}
	if err := store.writer.Write(store.path, raw); err != nil {
		return published, ErrProxyStateUncertain // possible publication is not a local commit
	}
	info, err := store.publishedFile(raw)
	if err != nil {
		return published, err
	}
	if err := store.commitAnchor(next, raw); err != nil {
		return published, errors.Join(ErrProxyAnchorUnavailable, err)
	}
	return proxyJournalInput{raw: raw, info: info, record: next, digest: digest}, nil
}
