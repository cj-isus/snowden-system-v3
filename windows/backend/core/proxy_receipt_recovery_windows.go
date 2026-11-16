//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows/registry"
)

const maxProxyRecoveryTransitions = 8 // initial resolution + three prepare/confirm pairs + finish

// Private component transaction. Automatic admission needs original-exit and
// authenticated source together; manual takeover is a separate one-shot grant.
// PID absence, a free lease or matching bytes alone cannot construct this owner.
// The fresh normal session is never adopted into recovery.
type proxyCrashRecovery struct {
	store                     *proxyReceiptStore
	proxy                     *ProxyManager
	lease                     *filetxn.WindowsWriterLease
	reader                    *filetxn.BoundedReadSlot
	writer                    *filetxn.BoundedWriteSlot
	anchor                    *proxyReceiptAnchor
	stage                     *proxyReceiptStage
	subject                   *proxyReceiptSubject
	previous                  *proxyDurableReceipt
	observer                  *proxyReceiptProcess
	manual                    *proxyManualRecovery // explicit source-bound takeover; never an owner-death claim
	native                    *proxyRegistryKeyOwner
	freshNative               *proxyRegistryKeyOwner // retained never-acquired Snapshot reservation
	fresh                     proxyDurableReceipt
	source, current, output   proxyJournalInput
	anchorState               proxyAnchorState
	anchorDigest, stageDigest [sha256.Size]byte
	stageInfo                 os.FileInfo
	stagePresent              bool
	journalNext               *proxyDurableReceipt
	writes                    [3]*proxyWriteAttempt
	retiredWrites             [3]*proxyWriteAttempt // only known unchanged callbacks; bounded previous slots
	notification              *proxyNotificationAttempt
	retiredNotification       *proxyNotificationAttempt
	retiredManual             *proxyManualRecovery
	steps, stepLimit          uint64
	resumeSerial, grantBase   uint64
	pause                     proxyRecoveryPause
	pauseIndex                int
	paused                    bool
	outcome                   string
	attempted, returned       bool
	completed                 bool
	failure                   error
}

func newProxyCrashRecovery(store *proxyReceiptStore, input proxyJournalInput) *proxyCrashRecovery {
	if store == nil || store.priorProcess == nil || store.priorProcess.outcome != proxyPriorProcessExited ||
		!errors.Is(store.priorProcess.result, ErrProxyRecoveryRequired) {
		return nil
	}
	return newProxyRecoveryOwner(store, input, nil)
}

func newProxyRecoveryOwner(store *proxyReceiptStore, input proxyJournalInput, manual *proxyManualRecovery) *proxyCrashRecovery {
	if store == nil || store.subject == nil || store.subject.failure != nil || store.previous == nil || store.priorProcess == nil ||
		!store.priorProcess.retired() || input.missing || !input.record.valid() ||
		input.record.phase != proxyReceiptActive && input.record.phase != proxyReceiptPrepared ||
		input.record.generation > ^uint64(0)-maxProxyRecoveryTransitions || !store.anchor.loaded || store.anchor.state.pending ||
		store.anchor.state.sequence > ^uint64(0)-2*maxProxyRecoveryTransitions {
		return nil
	}
	if manual != nil && !manual.constructionAllowed(store, input) {
		return nil
	}
	if manual == nil && (store.priorProcess.outcome != proxyPriorProcessExited || !errors.Is(store.priorProcess.result, ErrProxyRecoveryRequired)) {
		return nil
	}
	return &proxyCrashRecovery{store: store, lease: store.lease, reader: store.reader, writer: store.writer,
		anchor: store.anchor, stage: store.stage, subject: store.subject, previous: store.previous, observer: store.priorProcess, manual: manual,
		fresh: store.record, source: input, current: input, anchorState: store.anchor.state, stepLimit: maxProxyRecoveryTransitions,
		anchorDigest: sha256.Sum256(store.anchor.current), stageDigest: store.stage.digest, stageInfo: store.stage.info, stagePresent: store.stage.present}
}

// Before run there are no recovery I/O resources. After run only a completed
// transaction with the exact original HKEY closed permits caller retirement.
func (store *proxyReceiptStore) crashReturned() bool {
	if store == nil || store.crashRecovery == nil {
		return store != nil
	}
	owner := store.crashRecovery
	return !owner.attempted && owner.native == nil || owner.returned && owner.completed && owner.phasesKnown() && owner.native != nil &&
		owner.native.closed && owner.native.closeStarted && owner.native.closeReturned && owner.native.closeErr == nil && owner.native.handle == 0 && owner.native.read == nil
}

func (store *proxyReceiptStore) recoveredForeign(proxy *ProxyManager) bool {
	return store != nil && store.crashRecovery != nil && store.crashRecovery.proxy == proxy && store.crashReturned() &&
		store.crashRecovery.completed && store.crashRecovery.outcome == proxyForeignPreserved && proxy != nil && proxy.native == store.crashRecovery.native &&
		store.crashRecovery.current.record.phase == proxyReceiptForeign && !store.pending && !store.opened && !store.active
}

func (store *proxyReceiptStore) recoverPrior(proxy *ProxyManager) (result error) {
	owner := store.crashRecovery
	if owner == nil || owner.attempted || store.pending || store.opened || store.failure != nil {
		return ErrProxyStateUncertain
	}
	store.pending = true
	result = owner.run(proxy)
	return store.finishRecoveryCall(result)
}

// Both ordinary entry and resumed entry use the same original completion path.
// A paused owner keeps its lease/HKEY and pending hold; it is not retired here.
func (store *proxyReceiptStore) finishRecoveryCall(result error) error {
	owner, proxy := store.crashRecovery, store.crashRecovery.proxy
	if owner.completed && owner.outcome == proxyForeignPreserved {
		if proxy == nil || proxy.native != owner.freshNative || owner.freshNative == nil || owner.freshNative.openStarted || owner.freshNative.handle != 0 || owner.freshNative.read != nil {
			owner.completed, store.subjectConflict = false, true
			result = errors.Join(result, ErrProxyStateUncertain)
		} else {
			// Cleanup must carry the actual recovery HKEY's checked Close witness.
			// The unused Snapshot reservation remains retained in this owner.
			proxy.native = owner.native
		}
	}
	if store.crashReturned() {
		store.pending = false
	} else {
		result = errors.Join(result, ErrProxyStateUncertain)
	}
	store.failure = result
	return result
}

func (owner *proxyCrashRecovery) phasesKnown() bool {
	for _, attempt := range owner.writes {
		if attempt != nil && (!attempt.callReturned || !attempt.readReturned || attempt.outcome == proxyWritePending || attempt.outcome == proxyWriteUnknown) {
			return false
		}
	}
	for _, attempt := range owner.retiredWrites {
		if attempt != nil && !knownUnchangedProxyWrite(attempt) {
			return false
		}
	}
	return (owner.notification == nil || owner.notification.returned) &&
		(owner.retiredNotification == nil || knownProxyNotificationFailure(owner.retiredNotification))
}

func (owner *proxyCrashRecovery) matchesOwner() bool {
	return owner.matchesRecoveryMemory(false)
}

func (owner *proxyCrashRecovery) matchesPausedOwner() bool {
	return owner.matchesRecoveryMemory(true)
}

func (owner *proxyCrashRecovery) matchesRecoveryMemory(paused bool) bool {
	store, proxy := owner.store, owner.proxy
	if store == nil || proxy == nil || store.crashRecovery != owner || !owner.attempted || owner.completed ||
		proxyOwner != proxy || proxy.receiptStore != store || proxy.writerLease != owner.lease || proxy.originalSet || proxy.key != 0 || !proxy.pending ||
		owner.native == nil || owner.native == proxy.native || owner.freshNative == nil || proxy.native != owner.freshNative ||
		owner.freshNative.openStarted || owner.freshNative.handle != 0 || owner.freshNative.read != nil ||
		store.lease != owner.lease || store.reader != owner.reader || store.writer != owner.writer ||
		store.anchor != owner.anchor || store.stage != owner.stage || store.subject != owner.subject || store.previous != owner.previous || store.priorProcess != owner.observer ||
		!store.pending || store.opened || store.active || store.closed || !paused && store.failure != nil || store.subjectConflict || store.anchorConflict ||
		owner.subject.failure != nil || owner.subject.previous != owner.previous || owner.subject.observer != owner.observer ||
		!store.noWriteReturned() || !store.replayReturned() || !store.reconciliationReturned() || !owner.phasesKnown() ||
		!store.crypto.retired() || !owner.reader.Retired() || owner.writer.Check() != nil || !owner.anchor.idle() || !owner.stage.idle() ||
		!sameProxyReceiptFile(store.fileInfo, owner.current.info) {
		return false
	}
	if paused {
		if !owner.returned || !owner.paused || !owner.pausePointKnown() {
			return false
		}
	} else if owner.returned || !owner.admissionValid() {
		return false
	}
	fresh := store.record
	return fresh.generation == 0 && fresh.phase == 0 && fresh.index == 0 && fresh.written == 0 && fresh.nonce == owner.fresh.nonce &&
		fresh.pid == owner.fresh.pid && fresh.creation == owner.fresh.creation && bytes.Equal(fresh.sid, owner.fresh.sid) &&
		fresh.original.equal(proxyRegistryState{}) && fresh.current.equal(proxyRegistryState{}) && fresh.target.equal(proxyRegistryState{})
}

func (owner *proxyCrashRecovery) admissionValid() bool {
	if owner.observer == nil || !owner.observer.retired() {
		return false
	}
	if owner.manual != nil {
		return owner.manual.authorizes(owner)
	}
	return owner.observer.outcome == proxyPriorProcessExited && errors.Is(owner.observer.result, ErrProxyRecoveryRequired)
}

// Cancellation fences the next native intent. Readback and journal finalization
// of an already returned setter remain owned and must not be discarded by it.
func (owner *proxyCrashRecovery) nativeIntentAllowed() error {
	if !owner.admissionValid() {
		return ErrProxyRecoveryConsentUnavailable
	}
	if owner.manual != nil {
		return contextError(owner.manual.ctx)
	}
	return nil
}

func (owner *proxyCrashRecovery) run(proxy *ProxyManager) (result error) {
	if owner == nil || owner.attempted {
		return ErrProxyStateUncertain
	}
	owner.attempted, owner.proxy, owner.native = true, proxy, &proxyRegistryKeyOwner{} // retained before Open
	if proxy != nil {
		owner.freshNative = proxy.native
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		owner.finishRecoveryAttempt(returned, result)
	}()
	if err := owner.subject.revalidate(); err != nil {
		returned = true
		return err
	}
	if err := owner.checkSource(); err != nil {
		returned = true
		return err
	}
	if err := owner.nativeIntentAllowed(); err != nil {
		owner.pauseContext(err, proxyRecoveryPauseBeforeOpen, -1)
		returned = true
		return err
	}
	if err := owner.native.open(registry.QUERY_VALUE | registry.SET_VALUE); err != nil {
		returned = true
		return err
	}
	result = owner.finishRecoveryValues()
	returned = true
	return result
}

// Reused by a fresh explicit grant on the same retained owner/native key.
func (owner *proxyCrashRecovery) finishRecoveryValues() error {
	if err := owner.restoreValues(); err != nil {
		return err
	}
	if owner.current.record.phase != proxyReceiptClosed && owner.current.record.phase != proxyReceiptForeign ||
		owner.current.record.phase == proxyReceiptForeign && owner.outcome != proxyForeignPreserved ||
		owner.current.record.phase == proxyReceiptClosed && owner.outcome != proxyRestored && owner.outcome != proxyNotWritten {
		return ErrProxyStateUncertain
	}
	if err := owner.checkSource(); err != nil {
		return err
	}
	if err := owner.native.close(); err != nil || !owner.native.closed || owner.native.handle != 0 {
		return errors.Join(ErrProxyStateUncertain, err)
	}
	owner.completed = true
	if owner.outcome == proxyForeignPreserved {
		return ErrProxyOwnershipConflict
	}
	owner.store.previous = nil // original record/subject remain retained in this completed component owner
	owner.store.opened = true  // subsequent ordinary Snapshot captures current OS settings afresh
	return nil
}
