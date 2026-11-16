//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
)

// Initial publication has no restoration authority. Later active/prepared or
// unanchored legacy receipts cannot be used to reconstruct missing history.
func proxyReceiptInitialNoWrites(record proxyDurableReceipt) bool {
	return record.generation == 1 && proxyReceiptHasNoWrites(record)
}

// Read only through the retained key; never turn an existing key into missingKey
// or reopen a path alias. Repeated absence is consistency, not global freshness.
func (anchor *proxyReceiptAnchor) checkEmptyValue() (result error) {
	if anchor == nil || !anchor.idle() || anchor.failure != nil || !anchor.opened || !anchor.emptyValue || anchor.missingKey ||
		anchor.created || anchor.loaded || anchor.closeStarted || !validProxyRegistryHandle(anchor.handle) || len(anchor.current) != 0 ||
		!sameProxyAnchorState(anchor.state, proxyAnchorState{}) {
		return ErrProxyAnchorUnavailable
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			anchor.pending = true
			result = ErrProxyStateUncertain
		}
		if result != nil {
			anchor.failure = result
		}
	}()
	raw, err := anchor.readCipher(true)
	returned = true
	if err != nil || len(raw) != 0 {
		return ErrProxyAnchorUnavailable
	}
	return nil
}

func (owner *proxyStageReplay) checkInitialAnchor() error {
	if owner == nil || !owner.bootstrap || owner.anchor.loaded || !proxyReceiptInitialNoWrites(owner.target.record) ||
		owner.intent.parentSequence != 0 || !owner.intent.parent.empty() || owner.intent.file.present {
		return ErrProxyStageConflict
	}
	if owner.initialEmptyValue {
		if owner.anchor.handle != owner.anchorHandle {
			return ErrProxyStageConflict
		}
		return owner.anchor.checkEmptyValue()
	}
	if !owner.anchor.missingKey || owner.anchor.emptyValue || owner.anchor.handle != 0 || owner.anchorHandle != 0 {
		return ErrProxyStageConflict
	}
	return nil
}

// Private single-publication capability, minted by the actual staged replayer.
// No settings key, process-death inference, active-session adoption, counter
// downgrade of a loaded anchor, or arbitrary metadata reset is authorized.
func (owner *proxyStageReplay) admitsEmptyAnchor(next proxyAnchorState) bool {
	if owner == nil || !owner.bootstrap || !owner.initialEmptyValue || !owner.attempted || owner.returned || owner.completed || owner.published ||
		owner.store == nil || owner.anchor == nil || owner.stage == nil || owner.lease == nil || owner.reader == nil || owner.writer == nil ||
		!proxyReceiptInitialNoWrites(owner.target.record) {
		return false
	}
	store, anchor := owner.store, owner.anchor
	if store.stageReplay != owner || store.anchor != anchor || store.stage != owner.stage || store.lease != owner.lease || store.reader != owner.reader || store.writer != owner.writer ||
		!store.pending || store.opened || store.active || store.closed || !store.freshAnchor || store.previous != nil || store.priorProcess != nil || store.subject != nil ||
		store.subjectConflict || store.anchorConflict || !owner.stage.idle() || owner.stage.digest != owner.stageDigest || !owner.intent.context(store) ||
		!anchor.idle() || anchor.failure != nil || !anchor.opened || !anchor.emptyValue || anchor.missingKey || anchor.created || anchor.loaded || anchor.closeStarted ||
		anchor.handle != owner.anchorHandle || !validProxyRegistryHandle(anchor.handle) || anchor.enrollment != nil && anchor.enrollment != owner ||
		len(anchor.current) != 0 || !sameProxyAnchorState(anchor.state, proxyAnchorState{}) || !sameProxyAnchorState(owner.before, proxyAnchorState{}) ||
		owner.beforeCipher != sha256.Sum256(nil) || owner.intentDigest == [sha256.Size]byte{} || owner.intent.parentSequence != 0 || !owner.intent.parent.empty() || owner.intent.file.present ||
		proxyAnchorFor(owner.target.record, owner.target.raw) != owner.intent.target || !bytes.Equal(owner.target.raw, owner.intent.raw) ||
		!owner.source.missing && (!bytes.Equal(owner.source.raw, owner.target.raw) || !sameProxyRecoveryRecord(owner.source.record, owner.target.record)) {
		return false
	}
	return next.valid() && next.sequence == 1 && next.pending && next.committed.empty() && next.target == owner.intent.target &&
		next.path == store.anchorPath() && bytes.Equal(next.sid, store.record.sid)
}
