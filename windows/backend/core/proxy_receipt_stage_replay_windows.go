//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

type proxyJournalInput struct {
	raw     []byte
	info    os.FileInfo
	record  proxyDurableReceipt
	digest  [sha256.Size]byte
	missing bool
}

func (store *proxyReceiptStore) decodeJournal(raw []byte) (record proxyDurableReceipt, digest [sha256.Size]byte, result error) {
	if len(raw) <= len(proxyCipherPurpose) || len(raw) > maxProxyReceiptCipher+len(proxyCipherPurpose) || !bytes.HasPrefix(raw, []byte(proxyCipherPurpose)) {
		return record, digest, ErrProxyReceiptUnavailable
	}
	plain, err := store.crypto.transform(raw[len(proxyCipherPurpose):], false)
	if err != nil {
		return record, digest, err
	}
	record, err = decodeProxyReceipt(plain)
	digest = sha256.Sum256(plain)
	clear(plain)
	if err != nil || !bytes.Equal(record.sid, store.record.sid) {
		return proxyDurableReceipt{}, digest, ErrProxyReceiptUnavailable
	}
	return record, digest, nil
}

func (store *proxyReceiptStore) readJournal() (input proxyJournalInput, result error) {
	raw, info, err := store.reader.ReadSnapshot(store.path, maxProxyReceiptCipher+int64(len(proxyCipherPurpose)))
	if errors.Is(err, os.ErrNotExist) && store.reader.Retired() {
		input.missing = true
		return input, nil
	}
	if err != nil {
		return input, err
	}
	input.raw, input.info = raw, info
	input.record, input.digest, err = store.decodeJournal(raw)
	return input, err
}

func (intent proxyStageIntent) context(store *proxyReceiptStore) bool {
	return intent.valid() && bytes.Equal(intent.sid, store.record.sid) && intent.path == store.anchorPath()
}

func (intent proxyStageIntent) completedBy(state proxyAnchorState) bool {
	return !state.pending && state.sequence >= 2 && state.sequence-2 == intent.parentSequence && state.committed == intent.target
}

func (intent proxyStageIntent) parentOf(state proxyAnchorState) bool {
	return !state.pending && state.sequence == intent.parentSequence && state.committed == intent.parent
}

func (intent proxyStageIntent) prepares(state proxyAnchorState) bool {
	return state.pending && state.sequence > 0 && state.sequence-1 == intent.parentSequence && state.committed == intent.parent && state.target == intent.target
}

// At most one sidecar replay per intake. Committed backups, pre-anchor intents
// and prepared intents share the same original source guards; unrelated bytes
// are never replaced merely because they occupy the canonical path.
func (store *proxyReceiptStore) loadJournal() (input proxyJournalInput, result error) {
	input, err := store.readJournal()
	if err != nil {
		return input, err
	}
	if !input.missing && store.anchor.loaded && store.anchor.state.pending && store.anchor.state.target == proxyAnchorFor(input.record, input.raw) {
		if err := store.admitAnchoredFile(input.record, input.raw, input.info); err != nil {
			return input, err
		}
	}
	if err := store.stage.load(); err != nil {
		return input, err
	}
	if store.stage.present {
		intent := store.stage.intent
		if !intent.context(store) {
			return input, ErrProxyStageConflict
		}
		anchor := store.anchor
		if anchor.loaded && (!bytes.Equal(anchor.state.sid, store.record.sid) || anchor.state.path != store.anchorPath()) {
			store.anchorConflict = true
			return input, ErrProxyAnchorUnavailable
		}
		if anchor.loaded && intent.completedBy(anchor.state) && !input.missing && proxyAnchorFor(input.record, input.raw) == intent.target {
			// The bounded authenticated stage is an already completed predecessor;
			// retain it for the next checked overwrite, not another publication.
		} else {
			targetPresent := !input.missing && proxyAnchorFor(input.record, input.raw) == intent.target
			parent := input.missing || targetPresent || (proxyAnchorFor(input.record, input.raw) == intent.parent && intent.file.matches(input.info))
			initial := intent.parentSequence == 0 && intent.parent.empty() && !intent.file.present
			bootstrap := initial && (anchor.missingKey && input.missing || anchor.emptyValue && (input.missing || targetPresent))
			if !parent || !(bootstrap || anchor.loaded && (intent.completedBy(anchor.state) || intent.parentOf(anchor.state) || intent.prepares(anchor.state))) {
				return input, ErrProxyStageConflict
			}
			target := proxyJournalInput{raw: bytes.Clone(intent.raw)}
			target.record, target.digest, err = store.decodeJournal(target.raw)
			if err != nil || proxyAnchorFor(target.record, target.raw) != intent.target || bootstrap && !proxyReceiptInitialNoWrites(target.record) {
				return input, ErrProxyStageConflict
			}
			if !input.missing && input.record.nonce == target.record.nonce && !input.record.original.equal(target.record.original) {
				return input, ErrProxyStageConflict
			}
			input, err = store.replayStaged(input, target, bootstrap)
			if err != nil {
				return input, err
			}
		}
	}
	if input.missing {
		if store.anchor.emptyValue {
			return input, ErrProxyAnchorMigrationRequired // no initial stage, no bootstrap or history reset
		}
		if store.anchor.missingKey {
			store.freshAnchor = true
			return input, nil
		}
		if store.anchor.loaded && store.anchor.state.pending {
			return input, store.missingAnchoredFile()
		}
		store.anchorConflict = true
		return input, ErrProxyAnchorUnavailable
	}
	if err := store.admitAnchoredFile(input.record, input.raw, input.info); err != nil {
		return input, err
	}
	return input, nil
}

type proxyStageReplay struct {
	lease                                      *filetxn.WindowsWriterLease
	reader                                     *filetxn.BoundedReadSlot
	writer                                     *filetxn.BoundedWriteSlot
	before                                     proxyAnchorState
	beforeCipher                               [sha256.Size]byte
	store                                      *proxyReceiptStore
	stage                                      *proxyReceiptStage
	anchor                                     *proxyReceiptAnchor
	anchorHandle                               windows.Handle
	initialEmptyValue                          bool
	intent                                     proxyStageIntent
	source, target                             proxyJournalInput
	stageDigest, intentDigest                  [sha256.Size]byte
	publishedInfo                              os.FileInfo
	bootstrap, prepare, commit, reusePublished bool
	attempted, returned, published, completed  bool
	failure                                    error
}

func sameProxyAnchorState(a, b proxyAnchorState) bool {
	return a.sequence == b.sequence && a.path == b.path && bytes.Equal(a.sid, b.sid) && a.committed == b.committed && a.target == b.target && a.pending == b.pending
}

func (store *proxyReceiptStore) replayReturned() bool {
	return store != nil && (store.stageReplay == nil || store.stageReplay.returned)
}

func (store *proxyReceiptStore) replayStaged(source, target proxyJournalInput, bootstrap bool) (proxyJournalInput, error) {
	if store.stageReplay != nil {
		return source, ErrProxyStateUncertain
	}
	owner := &proxyStageReplay{store: store, stage: store.stage, anchor: store.anchor, source: source, target: target,
		lease: store.lease, reader: store.reader, writer: store.writer, before: store.anchor.state, beforeCipher: sha256.Sum256(store.anchor.current),
		intent: store.stage.intent, stageDigest: store.stage.digest, bootstrap: bootstrap,
		anchorHandle: store.anchor.handle, initialEmptyValue: store.anchor.emptyValue}
	owner.prepare = bootstrap || owner.intent.parentOf(store.anchor.state)
	owner.commit = owner.prepare || owner.intent.prepares(store.anchor.state)
	owner.reusePublished = !source.missing && bytes.Equal(source.raw, target.raw)
	store.stageReplay = owner // before any replay I/O; never replace an unknown original attempt
	return owner.run()
}

func (owner *proxyStageReplay) verify(state proxyAnchorState, anchorDigest [sha256.Size]byte, after bool) error {
	store := owner.store
	if store == nil || store.stageReplay != owner || store.stage != owner.stage || store.anchor != owner.anchor || owner.lease == nil || owner.reader == nil || owner.writer == nil ||
		store.lease != owner.lease || store.reader != owner.reader || store.writer != owner.writer || store.opened || store.active || store.closed ||
		store.anchorConflict || store.subjectConflict || !owner.stage.idle() || owner.stage.digest != owner.stageDigest || !owner.intent.context(store) ||
		owner.initialEmptyValue && owner.anchor.handle != owner.anchorHandle ||
		!sameProxyAnchorState(owner.anchor.state, state) || sha256.Sum256(owner.anchor.current) != anchorDigest {
		return ErrProxyStageConflict
	}
	plain, err := encodeProxyStage(owner.stage.intent)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(plain)
	clear(plain)
	if digest != owner.intentDigest {
		return ErrProxyStageConflict
	}
	plain, err = encodeProxyReceipt(owner.target.record)
	if err != nil {
		return err
	}
	digest = sha256.Sum256(plain)
	clear(plain)
	if digest != owner.target.digest || proxyAnchorFor(owner.target.record, owner.target.raw) != owner.intent.target {
		return ErrProxyStageConflict
	}
	if err := store.validateLease(); err != nil {
		return err
	}
	if state.sequence == 0 {
		if err := owner.checkInitialAnchor(); err != nil {
			return err
		}
	} else if err := owner.anchor.checkCurrent(); err != nil {
		return err
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	observed, err := store.readJournal()
	if err != nil {
		return err
	}
	if after {
		if !owner.published || observed.missing || !sameProxyReceiptFile(owner.publishedInfo, observed.info) || proxyAnchorFor(observed.record, observed.raw) != owner.intent.target {
			return ErrProxyStageConflict
		}
	} else if owner.source.missing {
		if !observed.missing {
			return ErrProxyStageConflict
		}
	} else if observed.missing || !sameProxyReceiptFile(owner.source.info, observed.info) || !bytes.Equal(owner.source.raw, observed.raw) {
		return ErrProxyStageConflict
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	if state.sequence == 0 {
		if err := owner.checkInitialAnchor(); err != nil {
			return err
		}
	} else {
		if err := owner.anchor.checkCurrent(); err != nil {
			return err
		}
	}
	return store.validateLease()
}

func (owner *proxyStageReplay) run() (input proxyJournalInput, result error) {
	if owner == nil || owner.attempted {
		return input, ErrProxyStateUncertain
	}
	owner.attempted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyStageUnavailable, ErrProxyStateUncertain)
		}
		owner.returned, owner.failure = returned, result
		if result != nil {
			input = proxyJournalInput{}
			owner.store.anchorConflict = true
		}
	}()
	plain, err := encodeProxyStage(owner.intent)
	if err != nil {
		returned = true
		return input, err
	}
	owner.intentDigest = sha256.Sum256(plain)
	clear(plain)
	state, digest := owner.before, owner.beforeCipher
	if err := owner.verify(state, digest, false); err != nil {
		returned = true
		return input, err
	}
	if owner.prepare {
		if owner.bootstrap {
			owner.store.freshAnchor = true
		}
		if err := owner.store.prepareAnchor(owner.target.record, owner.target.raw); err != nil {
			returned = true
			return input, err
		}
		state, digest = owner.anchor.state, sha256.Sum256(owner.anchor.current)
	}
	if err := owner.verify(state, digest, false); err != nil {
		returned = true
		return input, err
	}
	if owner.reusePublished {
		owner.publishedInfo = owner.source.info // verified exact target; no redundant replacement
	} else {
		if err := owner.store.writer.Check(); err != nil {
			returned = true
			return input, err
		}
		if owner.source.missing {
			err = owner.store.writer.WriteNew(owner.store.path, owner.target.raw)
		} else {
			err = owner.store.writer.Write(owner.store.path, owner.target.raw)
		}
		if err != nil {
			returned = true
			return input, errors.Join(ErrProxyStageUnavailable, err)
		}
		owner.publishedInfo, err = owner.store.publishedFile(owner.target.raw)
		if err != nil {
			returned = true
			return input, err
		}
	}
	owner.published = true
	if owner.commit {
		if err := owner.store.commitAnchor(owner.target.record, owner.target.raw); err != nil {
			returned = true
			return input, err
		}
	}
	state, digest = owner.anchor.state, sha256.Sum256(owner.anchor.current)
	if state.pending || state.committed != owner.intent.target {
		returned = true
		return input, ErrProxyStageConflict
	}
	if err := owner.verify(state, digest, true); err != nil {
		returned = true
		return input, err
	}
	owner.target.info, owner.completed, returned = owner.publishedInfo, true, true
	return owner.target, nil
}
