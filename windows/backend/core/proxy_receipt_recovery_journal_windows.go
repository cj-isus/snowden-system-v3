//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
)

func sameProxyRecoveryRecord(a, b proxyDurableReceipt) bool {
	return a.phase == b.phase && a.index == b.index && a.written == b.written && a.generation == b.generation &&
		a.pid == b.pid && a.creation == b.creation && a.nonce == b.nonce && bytes.Equal(a.sid, b.sid) &&
		a.original.equal(b.original) && a.current.equal(b.current) && a.target.equal(b.target)
}

func (owner *proxyCrashRecovery) admits(store *proxyReceiptStore, next proxyDurableReceipt) bool {
	current, source := owner.current.record, owner.source.record
	return owner.store == store && owner.matchesOwner() && owner.journalNext != nil &&
		owner.steps < owner.stepLimit && next.valid() && sameProxyRecoveryRecord(next, *owner.journalNext) &&
		current.generation < ^uint64(0) && next.generation == current.generation+1 &&
		next.pid == owner.fresh.pid && next.creation == owner.fresh.creation && next.nonce == source.nonce && bytes.Equal(next.sid, source.sid) &&
		next.original.equal(source.original) && next.written&current.written == current.written
}

func (owner *proxyCrashRecovery) checkSource() error {
	if (!owner.matchesOwner() && !owner.matchesPausedOwner()) || owner.current.missing || !owner.current.record.valid() ||
		owner.current.record.generation < owner.source.record.generation || owner.current.record.generation-owner.source.record.generation != owner.steps ||
		owner.anchorState.sequence < owner.subject.anchorSequence || (owner.anchorState.sequence-owner.subject.anchorSequence)%2 != 0 ||
		(owner.anchorState.sequence-owner.subject.anchorSequence)/2 != owner.steps ||
		owner.current.record.nonce != owner.source.record.nonce || !owner.current.record.original.equal(owner.source.record.original) ||
		!bytes.Equal(owner.current.record.sid, owner.source.record.sid) ||
		owner.steps == 0 && (owner.current.record.pid != owner.source.record.pid || owner.current.record.creation != owner.source.record.creation) ||
		owner.steps != 0 && (owner.current.record.pid != owner.fresh.pid || owner.current.record.creation != owner.fresh.creation) ||
		!sameProxyRecoveryRecord(*owner.previous, owner.source.record) || owner.subject.store != owner.store ||
		owner.subject.lease != owner.lease || owner.subject.reader != owner.reader || owner.subject.writer != owner.writer || owner.subject.anchor != owner.anchor ||
		owner.subject.path != owner.store.path || owner.subject.nonce != owner.source.record.nonce || owner.subject.generation != owner.source.record.generation ||
		owner.subject.plainSHA256 != owner.source.digest || owner.subject.cipherSHA256 != sha256.Sum256(owner.source.raw) ||
		!sameProxyReceiptFile(owner.subject.info, owner.source.info) || owner.observer.expectedPID != owner.source.record.pid ||
		owner.observer.expectedCreation != owner.source.record.creation || owner.observer.currentUser != owner.store.primaryUser ||
		!owner.anchor.loaded || owner.anchor.state.pending || !sameProxyAnchorState(owner.anchor.state, owner.anchorState) ||
		owner.anchorState.path != owner.store.anchorPath() || !bytes.Equal(owner.anchorState.sid, owner.fresh.sid) ||
		sha256.Sum256(owner.anchor.current) != owner.anchorDigest || owner.anchorState.committed != proxyAnchorFor(owner.current.record, owner.current.raw) ||
		owner.stage.present != owner.stagePresent || owner.stage.digest != owner.stageDigest ||
		owner.stagePresent && !sameProxyReceiptFile(owner.stageInfo, owner.stage.info) {
		return ErrProxyReceiptSubjectChanged
	}
	plain, err := encodeProxyReceipt(owner.current.record)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(plain)
	clear(plain)
	if digest != owner.current.digest {
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
	if observed.missing || !sameProxyReceiptFile(owner.current.info, observed.info) ||
		!bytes.Equal(owner.current.raw, observed.raw) || observed.digest != owner.current.digest {
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

// Never advance the original subject on a possible publication. Output, intended
// record and exact borrowed input remain owned until all postconditions return.
func (owner *proxyCrashRecovery) push(next proxyDurableReceipt) error {
	if owner.journalNext != nil || owner.steps >= owner.stepLimit || !owner.matchesOwner() {
		return ErrProxyStateUncertain
	}
	if err := owner.checkSource(); err != nil {
		return err
	}
	before, source := owner.anchorState, owner.current
	// Persist the responsible recovery writer before any setter, preserving the
	// old session nonce and exact original tuple. A crash must not name only the
	// already exited predecessor as the writer of a new pending native intent.
	next.pid, next.creation = owner.fresh.pid, owner.fresh.creation
	owner.journalNext = &next
	var err error
	owner.output, err = owner.store.publishJournal(next, nil, owner)
	if err != nil {
		return err
	}
	if !owner.matchesOwner() || owner.anchor.state.pending || owner.anchor.state.sequence != before.sequence+2 ||
		owner.anchor.state.path != before.path || !bytes.Equal(owner.anchor.state.sid, before.sid) ||
		owner.anchor.state.committed != proxyAnchorFor(next, owner.output.raw) || !owner.stage.present ||
		!owner.stage.intent.context(owner.store) || owner.stage.intent.parentSequence != before.sequence ||
		owner.stage.intent.parent != before.committed || !owner.stage.intent.file.matches(source.info) ||
		owner.stage.intent.target != owner.anchor.state.committed {
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
	if observed.missing || !sameProxyReceiptFile(owner.output.info, observed.info) || !bytes.Equal(owner.output.raw, observed.raw) ||
		observed.digest != owner.output.digest || !sameProxyRecoveryRecord(observed.record, next) {
		return ErrProxyReceiptSubjectChanged
	}
	if err := owner.stage.recheck(); err != nil {
		return err
	}
	if err := owner.anchor.check(); err != nil {
		return err
	}
	if err := owner.store.validateLease(); err != nil {
		return err
	}
	owner.current, owner.store.fileInfo = owner.output, owner.output.info
	owner.anchorState, owner.anchorDigest = owner.anchor.state, sha256.Sum256(owner.anchor.current)
	owner.stageDigest, owner.stageInfo, owner.stagePresent = owner.stage.digest, owner.stage.info, owner.stage.present
	owner.steps++
	owner.journalNext, owner.store.writeRaw = nil, nil
	return nil
}
