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

type proxyRecoveryResumeSubject struct {
	owner                     *proxyCrashRecovery
	native, freshNative       *proxyRegistryKeyOwner
	lease                     *filetxn.WindowsWriterLease
	reader                    *filetxn.BoundedReadSlot
	writer                    *filetxn.BoundedWriteSlot
	nativeHandle              windows.Handle
	current                   proxyJournalInput
	anchorState               proxyAnchorState
	anchorDigest, stageDigest [sha256.Size]byte
	stageInfo                 os.FileInfo
	stagePresent              bool
	steps, serial             uint64
	pause                     proxyRecoveryPause
	index                     int
	writes, retiredWrites     [3]*proxyWriteAttempt
	notification              *proxyNotificationAttempt
	retiredNotification       *proxyNotificationAttempt
	manualDigest              [sha256.Size]byte
}

func recoveryManualDigest(offer *proxyManualRecovery) [sha256.Size]byte {
	if offer == nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(offer.token[:])
}

func newProxyRecoveryResumeSubject(owner *proxyCrashRecovery) *proxyRecoveryResumeSubject {
	if !owner.matchesPausedOwner() || owner.steps > ^uint64(0)-maxProxyRecoveryTransitions || owner.resumeSerial == ^uint64(0) ||
		owner.current.record.generation > ^uint64(0)-maxProxyRecoveryTransitions || owner.anchorState.sequence > ^uint64(0)-2*maxProxyRecoveryTransitions {
		return nil
	}
	return &proxyRecoveryResumeSubject{owner: owner, native: owner.native, freshNative: owner.freshNative, lease: owner.lease,
		reader: owner.reader, writer: owner.writer, nativeHandle: owner.native.handle, current: owner.current,
		anchorState: owner.anchorState, anchorDigest: owner.anchorDigest, stageDigest: owner.stageDigest, stageInfo: owner.stageInfo,
		stagePresent: owner.stagePresent, steps: owner.steps, serial: owner.resumeSerial, pause: owner.pause, index: owner.pauseIndex,
		writes: owner.writes, retiredWrites: owner.retiredWrites, notification: owner.notification, retiredNotification: owner.retiredNotification,
		manualDigest: recoveryManualDigest(owner.manual)}
}

// Repeated readonly proof. It neither temporarily clears a failure nor grants
// publication/native authority; the active publisher still requires matchesOwner.
func (subject *proxyRecoveryResumeSubject) revalidate(offer *proxyManualRecovery) error {
	owner := subject.owner
	if !owner.matchesPausedOwner() || owner.native != subject.native || owner.freshNative != subject.freshNative || owner.lease != subject.lease ||
		owner.reader != subject.reader || owner.writer != subject.writer || owner.native.handle != subject.nativeHandle ||
		owner.steps != subject.steps || owner.resumeSerial != subject.serial || owner.pause != subject.pause || owner.pauseIndex != subject.index ||
		owner.writes != subject.writes || owner.retiredWrites != subject.retiredWrites || owner.notification != subject.notification || owner.retiredNotification != subject.retiredNotification ||
		recoveryManualDigest(owner.manual) != subject.manualDigest || !sameProxyRecoveryRecord(owner.current.record, subject.current.record) ||
		!bytes.Equal(owner.current.raw, subject.current.raw) || owner.current.digest != subject.current.digest || !sameProxyReceiptFile(owner.current.info, subject.current.info) ||
		!sameProxyAnchorState(owner.anchorState, subject.anchorState) || owner.anchorDigest != subject.anchorDigest || owner.stageDigest != subject.stageDigest ||
		owner.stagePresent != subject.stagePresent || subject.stagePresent && !sameProxyReceiptFile(owner.stageInfo, subject.stageInfo) {
		owner.store.subjectConflict = true
		return ErrProxyReceiptSubjectChanged
	}
	if offer != nil && (offer.resume != subject || offer.proxy != owner.proxy || owner.proxy.manualRecovery != offer || offer.store != owner.store ||
		offer.finished || offer.consumed || offer.invalidated || !sameProxyRecoveryRecord(offer.source.record, subject.current.record) ||
		offer.source.digest != subject.current.digest || !bytes.Equal(offer.source.raw, subject.current.raw) || !sameProxyReceiptFile(offer.source.info, subject.current.info)) {
		return ErrProxyRecoveryConsentInvalid
	}
	err := owner.checkSource()
	if err != nil && (errors.Is(err, ErrProxyReceiptSubjectChanged) || errors.Is(err, ErrProxyAnchorUnavailable) || errors.Is(err, ErrProxyStageConflict) || !owner.pausePointKnown()) {
		owner.store.subjectConflict = true
	}
	return err // a known retired readonly error keeps the same subject; it grants no permission
}

// After confirmation, bind the evolving chain to this exact paused source and
// its newly consumed grant. No previous token or mutable display DTO can retag it.
func (subject *proxyRecoveryResumeSubject) authorizes(offer *proxyManualRecovery, owner *proxyCrashRecovery) bool {
	if owner != subject.owner || owner.manual != offer || offer.recovery != owner || owner.proxy.manualRecovery != offer || offer.resume != subject ||
		offer.proxy != owner.proxy || offer.store != owner.store || offer.subject != owner.subject || offer.priorRecovery != owner ||
		!offer.accepted || !offer.consumed || !offer.executing || offer.finished || offer.invalidated || offer.ctx == nil ||
		owner.native != subject.native || owner.freshNative != subject.freshNative || owner.lease != subject.lease || owner.reader != subject.reader || owner.writer != subject.writer ||
		subject.nativeHandle != 0 && owner.native.handle != subject.nativeHandle || recoveryManualDigest(owner.retiredManual) != subject.manualDigest ||
		owner.resumeSerial != subject.serial+1 || owner.grantBase != subject.steps || owner.stepLimit != subject.steps+maxProxyRecoveryTransitions ||
		owner.steps < subject.steps || owner.steps > owner.stepLimit || owner.current.record.generation < subject.current.record.generation {
		return false
	}
	delta := owner.steps - subject.steps
	return owner.current.record.generation-subject.current.record.generation == delta &&
		owner.anchorState.sequence >= subject.anchorState.sequence && (owner.anchorState.sequence-subject.anchorState.sequence)/2 == delta &&
		(owner.anchorState.sequence-subject.anchorState.sequence)%2 == 0
}
