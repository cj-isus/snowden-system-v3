//go:build windows

package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"time"
)

type proxyManualRecovery struct {
	proxy                     *ProxyManager
	store                     *proxyReceiptStore
	subject                   *proxyReceiptSubject
	source                    proxyJournalInput
	priorRecovery, recovery   *proxyCrashRecovery
	priorFailure              error
	resume                    *proxyRecoveryResumeSubject
	anchorState               proxyAnchorState
	anchorDigest, stageDigest [sha256.Size]byte
	stageInfo                 os.FileInfo
	stagePresent              bool
	token                     [32]byte
	expires                   time.Time
	ctx                       context.Context
	confirmStarted, consumed  bool
	accepted, executing       bool
	returned, finished        bool
	invalidated               bool
}

func manualProxySemanticFailure(err error) bool {
	return err == nil || !errors.Is(err, ErrProxyStateUncertain) && !errors.Is(err, ErrProxyReceiptSubjectChanged) &&
		(errors.Is(err, ErrProxyPriorOwnerUnresolved) || errors.Is(err, ErrProxyPriorOwnerChanged) || errors.Is(err, ErrProxyRecoveryRequired))
}

func (pm *ProxyManager) manualPreparationReady() bool {
	store := pm.receiptStore
	return proxyOwner == pm && !pm.pending && !pm.originalSet && pm.key == 0 && pm.phaseErrorLocked() == nil &&
		pm.native != nil && !pm.native.openStarted && pm.native.handle == 0 && pm.native.read == nil &&
		store != nil && !store.active && !store.opened && !store.closed && !store.pending &&
		!store.subjectConflict && !store.anchorConflict && manualProxySemanticFailure(store.failure) && store.operationsReturned() &&
		store.writer.Check() == nil && store.previous != nil && store.subject != nil && store.subject.failure == nil &&
		store.priorProcess != nil && store.priorProcess.retired() && store.priorProcess.outcome != proxyPriorProcessRunning &&
		!errors.Is(store.priorProcess.result, ErrProxyPriorOwnerRunning) &&
		(store.crashRecovery == nil || !store.crashRecovery.attempted && store.crashRecovery.native == nil)
}

func newProxyManualRecovery(pm *ProxyManager, source proxyJournalInput) *proxyManualRecovery {
	store := pm.receiptStore
	if !pm.manualPreparationReady() || source.missing || !source.record.valid() ||
		!sameProxyRecoveryRecord(source.record, *store.previous) || source.digest != store.subject.plainSHA256 ||
		sha256.Sum256(source.raw) != store.subject.cipherSHA256 || !sameProxyReceiptFile(source.info, store.subject.info) ||
		source.record.phase != proxyReceiptActive && source.record.phase != proxyReceiptPrepared ||
		source.record.generation > ^uint64(0)-maxProxyRecoveryTransitions || store.anchor.state.sequence > ^uint64(0)-2*maxProxyRecoveryTransitions {
		return nil
	}
	return &proxyManualRecovery{proxy: pm, store: store, subject: store.subject, source: source,
		priorRecovery: store.crashRecovery, priorFailure: store.failure, anchorState: store.anchor.state,
		anchorDigest: sha256.Sum256(store.anchor.current), stageDigest: store.stage.digest, stageInfo: store.stage.info, stagePresent: store.stage.present}
}

func (offer *proxyManualRecovery) revalidate() error {
	if offer.resume != nil {
		return offer.resume.revalidate(offer)
	}
	pm, store := offer.proxy, offer.store
	if pm == nil || store == nil || pm.manualRecovery != offer || offer.finished || offer.consumed ||
		pm.receiptStore != store || !pm.manualPreparationReady() || store.subject != offer.subject || store.crashRecovery != offer.priorRecovery ||
		!sameProxyAnchorState(store.anchor.state, offer.anchorState) || sha256.Sum256(store.anchor.current) != offer.anchorDigest ||
		store.stage.present != offer.stagePresent || store.stage.digest != offer.stageDigest ||
		offer.stagePresent && !sameProxyReceiptFile(store.stage.info, offer.stageInfo) {
		return ErrProxyReceiptSubjectChanged
	}
	if err := store.stage.recheck(); err != nil {
		store.subjectConflict = true
		return err
	}
	if err := offer.subject.revalidate(); err != nil {
		return err
	}
	observed, err := store.readJournal()
	if err != nil {
		store.subjectConflict = true
		return err
	}
	if observed.missing || !sameProxyReceiptFile(offer.source.info, observed.info) || !bytes.Equal(offer.source.raw, observed.raw) ||
		observed.digest != offer.source.digest || !sameProxyRecoveryRecord(observed.record, offer.source.record) {
		store.subjectConflict = true
		return ErrProxyReceiptSubjectChanged
	}
	if err := store.stage.recheck(); err != nil {
		store.subjectConflict = true
		return err
	}
	return offer.subject.revalidate()
}

func (offer *proxyManualRecovery) constructionAllowed(store *proxyReceiptStore, input proxyJournalInput) bool {
	return offer != nil && offer.resume == nil && offer.store == store && offer.proxy != nil && offer.proxy.manualRecovery == offer &&
		offer.accepted && offer.consumed && offer.executing && !offer.finished && !offer.invalidated && offer.ctx != nil &&
		store.subject == offer.subject && store.crashRecovery == offer.priorRecovery &&
		(offer.priorRecovery == nil || !offer.priorRecovery.attempted && offer.priorRecovery.native == nil) &&
		!input.missing && sameProxyRecoveryRecord(input.record, offer.source.record) && bytes.Equal(input.raw, offer.source.raw) &&
		sameProxyReceiptFile(input.info, offer.source.info) && input.digest == offer.source.digest
}

func (offer *proxyManualRecovery) authorizes(owner *proxyCrashRecovery) bool {
	if offer != nil && offer.resume != nil {
		return offer.resume.authorizes(offer, owner)
	}
	return offer != nil && owner != nil && offer.recovery == owner && owner.manual == offer &&
		offer.proxy == owner.proxy && offer.proxy != nil && offer.proxy.manualRecovery == offer &&
		offer.store == owner.store && offer.store.crashRecovery == owner && offer.subject == owner.subject &&
		offer.accepted && offer.consumed && offer.executing && !offer.finished && !offer.invalidated && offer.ctx != nil &&
		owner.observer != nil && owner.observer.retired() && owner.observer.outcome != proxyPriorProcessRunning &&
		sameProxyRecoveryRecord(owner.source.record, offer.source.record) && owner.source.digest == offer.source.digest &&
		bytes.Equal(owner.source.raw, offer.source.raw) && sameProxyReceiptFile(owner.source.info, offer.source.info)
}
