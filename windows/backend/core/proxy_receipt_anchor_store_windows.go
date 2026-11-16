//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (store *proxyReceiptStore) anchorPath() [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.ToLower(filepath.Clean(store.path))))
}

// Called only after the file's DPAPI/codec/SID checks. A missing anchor beside
// an existing receipt requires explicit migration, never implicit enrollment.
func (store *proxyReceiptStore) admitAnchoredFile(record proxyDurableReceipt, raw []byte, info os.FileInfo) error {
	anchor := store.anchor
	if anchor == nil || !anchor.idle() {
		return ErrProxyStateUncertain
	}
	if anchor.missingKey || anchor.emptyValue {
		return ErrProxyAnchorMigrationRequired
	}
	if !anchor.loaded || !bytes.Equal(anchor.state.sid, store.record.sid) || anchor.state.path != store.anchorPath() {
		store.anchorConflict = true
		return ErrProxyAnchorUnavailable
	}
	if anchor.state.pending {
		if err := store.reconcilePending(record, raw, info); err != nil {
			return err
		}
		if store.reconciliation == nil || !store.reconciliation.completed {
			return ErrProxyStateUncertain
		}
	}
	if anchor.state.committed != proxyAnchorFor(record, raw) {
		store.anchorConflict = true
		return ErrProxyAnchorUnavailable // includes file-only rollback to an older valid DPAPI receipt
	}
	if err := anchor.check(); err != nil {
		store.anchorConflict = true
		return err
	}
	return nil
}

func (store *proxyReceiptStore) prepareAnchor(record proxyDurableReceipt, raw []byte) error {
	anchor := store.anchor
	if anchor == nil || !anchor.idle() || anchor.failure != nil || anchor.state.pending || store.anchorConflict {
		return ErrProxyAnchorUnavailable
	}
	state := anchor.state
	if anchor.missingKey {
		if !store.freshAnchor || store.active || store.previous != nil {
			return ErrProxyAnchorMigrationRequired
		}
		state = proxyAnchorState{sid: bytes.Clone(store.record.sid), path: store.anchorPath()}
	} else if anchor.emptyValue {
		owner := store.stageReplay
		if !store.freshAnchor || store.active || store.previous != nil || owner == nil {
			return ErrProxyAnchorMigrationRequired
		}
		state = proxyAnchorState{sid: bytes.Clone(store.record.sid), path: store.anchorPath()}
		next := state
		next.sequence, next.pending, next.target = 1, true, proxyAnchorFor(record, raw)
		if !owner.admitsEmptyAnchor(next) {
			return ErrProxyAnchorMigrationRequired
		}
		anchor.enrollment = owner // retain the exact original replayer before native publication
	} else if !anchor.loaded || !bytes.Equal(state.sid, store.record.sid) || state.path != store.anchorPath() {
		return ErrProxyAnchorUnavailable
	}
	if state.sequence > ^uint64(0)-2 {
		return ErrProxyAnchorUnavailable
	}
	state.sequence++
	state.pending, state.target = true, proxyAnchorFor(record, raw)
	if !validProxyAnchorTransition(state.committed, state.target) {
		return ErrProxyAnchorUnavailable
	}
	return anchor.publish(state) // prepared anchor+readback+flush precedes file publication
}

func (store *proxyReceiptStore) commitAnchor(record proxyDurableReceipt, raw []byte) error {
	anchor := store.anchor
	if anchor == nil || !anchor.idle() || anchor.failure != nil || !anchor.loaded || !anchor.state.pending ||
		anchor.state.target != proxyAnchorFor(record, raw) || anchor.state.sequence == ^uint64(0) {
		return ErrProxyAnchorUnavailable
	}
	state := anchor.state
	state.sequence++
	state.pending, state.committed, state.target = false, state.target, proxyAnchorSubject{}
	if err := anchor.publish(state); err != nil {
		return err
	}
	store.freshAnchor = false
	return nil
}

// Anchor HKEY is intentionally held while the store is active. This checks
// only completion of transient calls, not closure of all store resources.
func (store *proxyReceiptStore) operationsReturned() bool {
	return store != nil && !store.pending && store.crashReturned() && store.noWriteReturned() && store.replayReturned() && store.stage.idle() && store.reconciliationReturned() && store.priorProcessRetired() && store.crypto.retired() &&
		store.reader.Retired() && store.anchor != nil && store.anchor.idle()
}

func (store *proxyReceiptStore) closeAnchor() error {
	if store.anchor == nil {
		return ErrProxyStateUncertain
	}
	if err := store.anchor.close(); err != nil {
		return errors.Join(ErrProxyStateUncertain, err)
	}
	return nil
}

// Check the prior file both before preparation and immediately before its
// replacement. A live owner does not silently overwrite a changed private file.
func (store *proxyReceiptStore) checkFileSource() error {
	anchor := store.anchor
	if anchor == nil || !anchor.idle() || anchor.failure != nil {
		return ErrProxyAnchorUnavailable
	}
	if anchor.loaded {
		if err := anchor.checkCurrent(); err != nil {
			store.anchorConflict = true
			return err
		}
	}
	raw, info, err := store.reader.ReadSnapshot(store.path, maxProxyReceiptCipher+int64(len(proxyCipherPurpose)))
	expected := anchor.state.committed
	if expected.empty() && store.freshAnchor && !store.active && store.fileInfo == nil &&
		errors.Is(err, os.ErrNotExist) && store.reader.Retired() {
		return nil
	}
	if err != nil || expected.empty() || !sameProxyReceiptFile(store.fileInfo, info) || sha256.Sum256(raw) != expected.cipher {
		store.anchorConflict = true
		return ErrProxyAnchorUnavailable
	}
	return nil
}

func (store *proxyReceiptStore) publishedFile(raw []byte) (os.FileInfo, error) {
	observed, info, err := store.reader.ReadSnapshot(store.path, maxProxyReceiptCipher+int64(len(proxyCipherPurpose)))
	if err != nil || !bytes.Equal(observed, raw) {
		store.anchorConflict = true
		return nil, ErrProxyAnchorUnavailable
	}
	return info, nil // original checked reader Close precedes anchor commit
}
