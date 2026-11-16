//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
)

type proxyAnchorReconcileKind uint8

const (
	proxyAnchorReconcileUnknown proxyAnchorReconcileKind = iota
	proxyAnchorTargetObserved
	proxyAnchorCommittedObserved
	proxyAnchorNeitherObserved
	proxyAnchorMissingInitial
	proxyAnchorMissingKnown
)

// One retained journal-only reconciliation. It cannot open Internet Settings,
// change a proxy value, infer process death or issue a restore capability.
// The caller retains it before I/O, under the existing original proxy locks.
type proxyAnchorReconciliation struct {
	store                                     *proxyReceiptStore
	anchor                                    *proxyReceiptAnchor
	lease                                     *filetxn.WindowsWriterLease
	reader                                    *filetxn.BoundedReadSlot
	path                                      string
	before                                    proxyAnchorState
	beforeCipher, afterCipher                 [sha256.Size]byte
	file                                      proxyAnchorSubject
	plain                                     [sha256.Size]byte
	raw                                       []byte
	info                                      os.FileInfo
	record                                    proxyDurableReceipt
	kind                                      proxyAnchorReconcileKind
	attempted, returned, committed, completed bool
	failure                                   error
}

func (store *proxyReceiptStore) reconciliationReturned() bool {
	return store != nil && (store.reconciliation == nil || store.reconciliation.returned)
}

func (store *proxyReceiptStore) reconcilePending(record proxyDurableReceipt, raw []byte, info os.FileInfo) error {
	if store.reconciliation != nil || store.anchor == nil || !store.anchor.loaded || !store.anchor.state.pending {
		return ErrProxyStateUncertain
	}
	owner := &proxyAnchorReconciliation{store: store, anchor: store.anchor, lease: store.lease, reader: store.reader, path: store.path,
		before: store.anchor.state, beforeCipher: sha256.Sum256(store.anchor.current), file: proxyAnchorFor(record, raw),
		raw: bytes.Clone(raw), info: info, record: record}
	store.reconciliation = owner // before any further file/anchor/native work
	return owner.run()
}

func (owner *proxyAnchorReconciliation) matches(after bool) bool {
	if owner == nil || owner.store == nil || owner.anchor == nil || owner.lease == nil || owner.reader == nil ||
		owner.store.reconciliation != owner || owner.store.anchor != owner.anchor || owner.store.lease != owner.lease ||
		owner.store.reader != owner.reader || owner.store.path != owner.path || owner.store.active || owner.store.opened || owner.store.closed ||
		owner.store.anchorConflict || owner.store.subjectConflict || !owner.anchor.loaded || !owner.anchor.idle() ||
		!bytes.Equal(owner.anchor.state.sid, owner.store.record.sid) || owner.anchor.state.path != owner.store.anchorPath() ||
		!owner.before.valid() || !owner.before.pending || owner.before.target != owner.file || proxyAnchorFor(owner.record, owner.raw) != owner.file ||
		!filetxn.SameWindowsFile(owner.info, owner.info) {
		return false
	}
	plain, err := encodeProxyReceipt(owner.record)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(plain)
	clear(plain)
	if digest != owner.plain {
		return false
	}
	state := owner.anchor.state
	if after {
		return owner.committed && owner.before.sequence != ^uint64(0) && state.sequence == owner.before.sequence+1 &&
			!state.pending && state.committed == owner.file && state.target.empty() && sha256.Sum256(owner.anchor.current) == owner.afterCipher
	}
	return state.pending && state.sequence == owner.before.sequence && state.committed == owner.before.committed &&
		state.target == owner.before.target && sha256.Sum256(owner.anchor.current) == owner.beforeCipher
}

func (owner *proxyAnchorReconciliation) verify(after bool) error {
	if !owner.matches(after) || !owner.reader.Retired() {
		return ErrProxyAnchorUnavailable
	}
	if err := owner.lease.Validate(); err != nil {
		return errors.Join(ErrProxyAnchorUnavailable, err)
	}
	if err := owner.anchor.checkCurrent(); err != nil {
		return err
	}
	raw, info, err := owner.reader.ReadSnapshot(owner.path, maxProxyReceiptCipher+int64(len(proxyCipherPurpose)))
	if err != nil || !sameProxyReceiptFile(owner.info, info) || sha256.Sum256(raw) != owner.file.cipher || !owner.matches(after) {
		return ErrProxyAnchorUnavailable
	}
	if err := owner.anchor.checkCurrent(); err != nil {
		return err
	}
	return owner.lease.Validate()
}

func (owner *proxyAnchorReconciliation) run() (result error) {
	if owner == nil || owner.attempted {
		return ErrProxyStateUncertain
	}
	owner.attempted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyAnchorUnavailable, ErrProxyStateUncertain)
		}
		owner.returned, owner.failure = returned, result
		if result != nil && !errors.Is(result, ErrProxyAnchorOldFile) && owner.store != nil {
			owner.store.anchorConflict = true
		}
	}()
	plain, err := encodeProxyReceipt(owner.record)
	if err != nil {
		returned = true
		return err
	}
	owner.plain = sha256.Sum256(plain)
	clear(plain)
	switch owner.file {
	case owner.before.target:
		owner.kind = proxyAnchorTargetObserved
	case owner.before.committed:
		owner.kind, returned = proxyAnchorCommittedObserved, true
		return errors.Join(ErrProxyAnchorPending, ErrProxyAnchorOldFile) // present old bytes cannot prove the target was never published
	default:
		owner.kind, returned = proxyAnchorNeitherObserved, true
		return ErrProxyAnchorUnavailable
	}
	if err := owner.verify(false); err != nil {
		returned = true
		return err
	}
	// The exact authenticated target is already present. Complete only the
	// existing anchor step; do not rewrite the file or any traffic settings.
	if err := owner.store.commitAnchor(owner.record, owner.raw); err != nil {
		returned = true
		return err
	}
	owner.committed, owner.afterCipher = true, sha256.Sum256(owner.anchor.current)
	if err := owner.verify(true); err != nil {
		returned = true
		return err
	}
	owner.completed, returned = true, true
	return nil
}

// Called only on the original reader's known os.ErrNotExist plus Retired
// branch. This records a refusal; it never cancels enrollment or writes data.
func (store *proxyReceiptStore) missingAnchoredFile() error {
	anchor := store.anchor
	if store.reconciliation != nil || anchor == nil || !anchor.loaded || !anchor.idle() || !anchor.state.pending ||
		!bytes.Equal(anchor.state.sid, store.record.sid) || anchor.state.path != store.anchorPath() {
		store.anchorConflict = true
		return ErrProxyAnchorUnavailable
	}
	owner := &proxyAnchorReconciliation{store: store, anchor: anchor, lease: store.lease, reader: store.reader, path: store.path,
		before: anchor.state, beforeCipher: sha256.Sum256(anchor.current), attempted: true, returned: true, kind: proxyAnchorMissingKnown}
	store.reconciliation = owner
	if anchor.state.sequence == 1 && anchor.state.committed.empty() && anchor.state.target.generation == 1 && anchor.state.target.phase == proxyReceiptActive {
		owner.kind, owner.failure = proxyAnchorMissingInitial, errors.Join(ErrProxyAnchorPending, ErrProxyAnchorMissingInitial)
		return owner.failure
	}
	store.anchorConflict, owner.failure = true, ErrProxyAnchorUnavailable
	return owner.failure
}
