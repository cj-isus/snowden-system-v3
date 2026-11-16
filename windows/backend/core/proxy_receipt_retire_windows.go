//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
)

// A prepared intent can have reached the native setter even with written == 0.
// Only a fully confirmed active record may establish that this session has no
// outstanding confirmed writes and no pending setter. This says nothing about
// process death, past attempted setters or the current proxy.
func proxyReceiptHasNoWrites(record proxyDurableReceipt) bool {
	return record.valid() && record.phase == proxyReceiptActive && record.index == proxyReceiptNoIndex &&
		record.written == 0 && record.generation < ^uint64(0) &&
		record.original.equal(record.current) && record.current.equal(record.target)
}

func proxyReceiptClosesNoWrites(source, target proxyDurableReceipt) bool {
	return proxyReceiptHasNoWrites(source) && target.valid() && target.phase == proxyReceiptClosed &&
		target.index == proxyReceiptNoIndex && target.written == 0 && target.generation == source.generation+1 &&
		target.nonce == source.nonce && target.pid == source.pid && target.creation == source.creation && bytes.Equal(target.sid, source.sid) &&
		target.original.equal(source.original) && target.current.equal(source.current) && target.target.equal(source.target)
}

type proxyNoWriteRetirement struct {
	store               *proxyReceiptStore
	lease               *filetxn.WindowsWriterLease
	reader              *filetxn.BoundedReadSlot
	writer              *filetxn.BoundedWriteSlot
	anchor              *proxyReceiptAnchor
	stage               *proxyReceiptStage
	before              proxyAnchorState
	anchorDigest        [sha256.Size]byte
	stageDigest         [sha256.Size]byte
	stageInfo           os.FileInfo
	stagePresent        bool
	fresh               proxyDurableReceipt
	source, output      proxyJournalInput
	target              proxyDurableReceipt
	attempted, returned bool
	completed           bool
	failure             error
}

func (store *proxyReceiptStore) noWriteReturned() bool {
	return store != nil && (store.noWriteRetirement == nil || store.noWriteRetirement.returned)
}

func (store *proxyReceiptStore) retireNoWrite(source proxyJournalInput) (proxyJournalInput, error) {
	if store.noWriteRetirement != nil || source.missing || !proxyReceiptHasNoWrites(source.record) {
		return source, ErrProxyReceiptUnavailable
	}
	owner := &proxyNoWriteRetirement{store: store, lease: store.lease, reader: store.reader, writer: store.writer,
		anchor: store.anchor, stage: store.stage, before: store.anchor.state, anchorDigest: sha256.Sum256(store.anchor.current),
		stageDigest: store.stage.digest, stageInfo: store.stage.info, stagePresent: store.stage.present,
		fresh: store.record, source: source, target: source.record}
	owner.target.phase, owner.target.generation = proxyReceiptClosed, source.record.generation+1
	store.noWriteRetirement = owner // before I/O; never replace an uncertain original owner
	return owner.run()
}

func (owner *proxyNoWriteRetirement) matchesOwner() bool {
	store := owner.store
	if store == nil || store.noWriteRetirement != owner || owner.lease == nil || owner.reader == nil || owner.writer == nil || owner.anchor == nil || owner.stage == nil ||
		store.lease != owner.lease || store.reader != owner.reader || store.writer != owner.writer || store.anchor != owner.anchor || store.stage != owner.stage ||
		!store.pending || store.opened || store.active || store.closed || store.failure != nil || store.previous != nil || store.priorProcess != nil || store.subject != nil ||
		store.subjectConflict || store.anchorConflict || !store.replayReturned() || !store.reconciliationReturned() ||
		!store.crypto.retired() || !owner.reader.Retired() || owner.writer.Check() != nil || !owner.anchor.idle() || !owner.stage.idle() ||
		!sameProxyReceiptFile(owner.source.info, store.fileInfo) {
		return false
	}
	current := store.record
	return current.phase == 0 && current.index == 0 && current.written == 0 && current.generation == 0 &&
		current.nonce == owner.fresh.nonce && current.pid == owner.fresh.pid && current.creation == owner.fresh.creation &&
		bytes.Equal(current.sid, owner.fresh.sid) && current.original.equal(proxyRegistryState{}) &&
		current.current.equal(proxyRegistryState{}) && current.target.equal(proxyRegistryState{})
}

func (owner *proxyNoWriteRetirement) verifySource() error {
	if !owner.matchesOwner() || !proxyReceiptClosesNoWrites(owner.source.record, owner.target) || owner.source.missing ||
		!owner.before.valid() || owner.before.pending || owner.before.sequence > ^uint64(0)-2 ||
		!bytes.Equal(owner.before.sid, owner.fresh.sid) || owner.before.path != owner.store.anchorPath() ||
		!sameProxyAnchorState(owner.anchor.state, owner.before) || sha256.Sum256(owner.anchor.current) != owner.anchorDigest ||
		owner.before.committed != proxyAnchorFor(owner.source.record, owner.source.raw) ||
		owner.stage.present != owner.stagePresent || owner.stage.digest != owner.stageDigest ||
		owner.stagePresent && !sameProxyReceiptFile(owner.stageInfo, owner.stage.info) ||
		!sameProxyReceiptFile(owner.source.info, owner.store.fileInfo) {
		return ErrProxyReceiptSubjectChanged
	}
	plain, err := encodeProxyReceipt(owner.source.record)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(plain)
	clear(plain)
	if digest != owner.source.digest {
		return ErrProxyReceiptSubjectChanged
	}
	if err := owner.store.validateLease(); err != nil {
		return err
	}
	if err := owner.anchor.check(); err != nil {
		return err
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	observed, err := owner.store.readJournal()
	if err != nil {
		return err
	}
	if observed.missing || !sameProxyReceiptFile(owner.source.info, observed.info) ||
		!bytes.Equal(owner.source.raw, observed.raw) || observed.digest != owner.source.digest {
		return ErrProxyReceiptSubjectChanged
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	if err := owner.anchor.check(); err != nil {
		return err
	}
	return owner.store.validateLease()
}

func (owner *proxyNoWriteRetirement) verifyPublished() error {
	if !owner.matchesOwner() || !proxyReceiptClosesNoWrites(owner.source.record, owner.target) || owner.output.missing || owner.output.info == nil || !owner.anchor.loaded ||
		owner.anchor.state.pending || owner.anchor.state.sequence != owner.before.sequence+2 ||
		owner.anchor.state.path != owner.before.path || !bytes.Equal(owner.anchor.state.sid, owner.before.sid) ||
		owner.anchor.state.committed != proxyAnchorFor(owner.target, owner.output.raw) ||
		!owner.stage.present || !owner.stage.intent.context(owner.store) ||
		owner.stage.intent.parentSequence != owner.before.sequence || owner.stage.intent.parent != owner.before.committed ||
		!owner.stage.intent.file.matches(owner.source.info) || owner.stage.intent.target != owner.anchor.state.committed {
		return ErrProxyReceiptSubjectChanged
	}
	if err := owner.store.validateLease(); err != nil {
		return err
	}
	if err := owner.anchor.check(); err != nil {
		return err
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	observed, err := owner.store.readJournal()
	if err != nil {
		return err
	}
	if observed.missing || !sameProxyReceiptFile(owner.output.info, observed.info) ||
		!bytes.Equal(owner.output.raw, observed.raw) || observed.digest != owner.output.digest ||
		observed.record.phase != proxyReceiptClosed || observed.record.written != 0 ||
		observed.record.nonce != owner.source.record.nonce || observed.record.generation != owner.source.record.generation+1 {
		return ErrProxyReceiptSubjectChanged
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	if err := owner.anchor.check(); err != nil {
		return err
	}
	return owner.store.validateLease()
}

func (owner *proxyNoWriteRetirement) run() (input proxyJournalInput, result error) {
	if owner == nil || owner.attempted {
		return input, ErrProxyReceiptUnavailable
	}
	owner.attempted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyReceiptUnavailable, ErrProxyStateUncertain)
		}
		owner.returned, owner.failure = returned, result
		if result != nil {
			input = proxyJournalInput{}
			owner.store.subjectConflict = true // preserve the original owner, including a known partial publication
		}
	}()
	if err := owner.verifySource(); err != nil {
		returned = true
		return input, err
	}
	var err error
	owner.output, err = owner.store.publishJournal(owner.target, owner, nil)
	if err != nil {
		returned = true
		return input, err
	}
	if err := owner.verifyPublished(); err != nil {
		returned = true
		return input, err
	}
	owner.store.fileInfo = owner.output.info
	owner.store.writeRaw = nil
	owner.completed, returned = true, true
	return owner.output, nil // original fresh nonce/PID are still untouched for the subsequent Snapshot
}
