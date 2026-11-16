//go:build windows

package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/sys/windows/registry"
)

type proxyRecoveryPause uint8

const (
	proxyRecoveryPauseNone proxyRecoveryPause = iota
	proxyRecoveryPauseWriteUnchanged
	proxyRecoveryPauseNotification
	proxyRecoveryPauseBeforeOpen
	proxyRecoveryPauseBeforeWrite
	proxyRecoveryPauseBeforeNotify
)

func knownUnchangedProxyWrite(attempt *proxyWriteAttempt) bool {
	return attempt != nil && attempt.index >= 0 && attempt.index < len(proxyValueNames) &&
		attempt.callStarted && attempt.callReturned && attempt.readStarted && attempt.readReturned && attempt.outcome == proxyWriteUnchanged &&
		attempt.writeErr != nil && !errors.Is(attempt.writeErr, ErrProxyStateUncertain) && attempt.readErr == nil &&
		attempt.observed.equal(attempt.before) && !attempt.target.equal(attempt.before)
}

func knownProxyNotificationFailure(attempt *proxyNotificationAttempt) bool {
	if attempt == nil || !attempt.returned || !attempt.findStarted || !attempt.findReturned || attempt.result == nil || errors.Is(attempt.result, ErrProxyStateUncertain) {
		return false
	}
	for index, started := range attempt.callStarted {
		if started && !attempt.callReturned[index] {
			return false
		}
	}
	return true
}

// Pure original-slot checks only, also usable on ordinary unwind. No path read,
// source recapture, registry call, speculative Close or allocator work occurs.
func (owner *proxyCrashRecovery) pausePointKnown() bool {
	if owner == nil || owner.store == nil || owner.native == nil || owner.completed || owner.journalNext != nil || owner.current.missing ||
		owner.current.record.phase != proxyReceiptActive && owner.current.record.phase != proxyReceiptPrepared ||
		owner.store.subjectConflict || owner.store.anchorConflict || !owner.store.crypto.retired() || !owner.reader.Retired() ||
		owner.writer.Check() != nil || !owner.anchor.idle() || !owner.stage.idle() || !owner.phasesKnown() ||
		owner.native.closeStarted || owner.native.closed || owner.native.openUnknown || owner.failure == nil {
		return false
	}
	switch owner.pause {
	case proxyRecoveryPauseWriteUnchanged:
		if owner.pauseIndex < 0 || owner.pauseIndex >= len(owner.writes) {
			return false
		}
		attempt := owner.writes[owner.pauseIndex]
		return owner.native.ready() && owner.current.record.phase == proxyReceiptActive && knownUnchangedProxyWrite(attempt) &&
			attempt.index == owner.pauseIndex && attempt.before.equal(owner.current.record.current)
	case proxyRecoveryPauseNotification:
		return owner.native.ready() && owner.current.record.phase == proxyReceiptActive &&
			owner.current.record.current.equal(owner.current.record.original) && knownProxyNotificationFailure(owner.notification)
	case proxyRecoveryPauseBeforeOpen:
		return recoveryContextCancelled(owner.failure) && !owner.native.openStarted && !owner.native.openReturned && owner.native.handle == 0 && owner.native.read == nil
	case proxyRecoveryPauseBeforeWrite:
		return recoveryContextCancelled(owner.failure) && owner.native.ready() && owner.current.record.phase == proxyReceiptPrepared &&
			owner.pauseIndex >= 0 && owner.pauseIndex < len(owner.writes) && owner.current.record.index == byte(owner.pauseIndex) && owner.writes[owner.pauseIndex] == nil
	case proxyRecoveryPauseBeforeNotify:
		return recoveryContextCancelled(owner.failure) && owner.native.ready() && owner.notification == nil &&
			owner.current.record.phase == proxyReceiptActive && owner.current.record.current.equal(owner.current.record.original)
	default:
		return false
	}
}

func recoveryContextCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (owner *proxyCrashRecovery) pauseContext(err error, point proxyRecoveryPause, index int) {
	if owner.manual != nil && owner.admissionValid() && recoveryContextCancelled(err) {
		owner.pause, owner.pauseIndex = point, index
	}
}

func (owner *proxyCrashRecovery) finishRecoveryAttempt(returned bool, result error) {
	owner.returned, owner.failure = returned, result
	if !owner.completed {
		owner.paused = returned && owner.pausePointKnown()
		if !owner.paused {
			owner.store.subjectConflict = true // never turn unknown native/source failure into a resumable pause
		}
	}
}

func recoveryPauseReason(point proxyRecoveryPause) string {
	switch point {
	case proxyRecoveryPauseWriteUnchanged:
		return "unchanged_write"
	case proxyRecoveryPauseNotification:
		return "notification_failed"
	case proxyRecoveryPauseBeforeOpen, proxyRecoveryPauseBeforeWrite, proxyRecoveryPauseBeforeNotify:
		return "cancelled_before_intent"
	default:
		return ""
	}
}

// Proposal eligibility only. A source-bound fresh confirmation is still needed.
func (pm *ProxyManager) proxyRecoveryResumable() bool {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.receiptStore != nil && pm.receiptStore.crashRecovery != nil && pm.receiptStore.crashRecovery.matchesPausedOwner() &&
		(pm.manualRecovery == nil || pm.manualRecovery.finished || pm.manualRecovery.consumed && pm.manualRecovery.returned && !pm.manualRecovery.executing)
}

func (pm *ProxyManager) prepareProxyRecoveryResumeLocked(ctx context.Context) (preview *ProxyRecoveryPreview, result error) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			preview, result = nil, ErrProxyStateUncertain
		}
	}()
	owner := pm.receiptStore.crashRecovery
	if !owner.matchesPausedOwner() || pm.manualRecovery != nil && !pm.manualRecovery.finished &&
		(!pm.manualRecovery.consumed || !pm.manualRecovery.returned || pm.manualRecovery.executing) {
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	subject := newProxyRecoveryResumeSubject(owner)
	if subject == nil {
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	if err := subject.revalidate(nil); err != nil {
		returned = true
		return nil, err
	}
	offer := &proxyManualRecovery{proxy: pm, store: owner.store, subject: owner.subject, source: owner.current,
		priorRecovery: owner, priorFailure: owner.store.failure, resume: subject}
	if _, err := rand.Read(offer.token[:]); err != nil || offer.token == [32]byte{} {
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	if err := subject.revalidate(nil); err != nil {
		returned = true
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		returned = true
		return nil, err
	}
	offer.expires = time.Now().Add(proxyManualConfirmationTTL)
	pm.manualRecovery = offer // known readonly preparation; original held transaction stays untouched
	returned = true
	return &ProxyRecoveryPreview{Token: hex.EncodeToString(offer.token[:]), Scope: ProxyRecoveryRecordedValues,
		ExpiresAt: offer.expires.UTC().Format(time.RFC3339Nano), RecordedValues: proxyRecoveryRecordedValueCount(owner.current.record),
		PriorProcess: "retained_owner", Resume: true, ResumeReason: recoveryPauseReason(owner.pause)}, nil
}

func (pm *ProxyManager) confirmProxyRecoveryResumeLocked(offer *proxyManualRecovery) (*ProxyRecoveryResult, error) {
	owner := offer.resume.owner
	if err := offer.resume.revalidate(offer); err != nil {
		return nil, err
	}
	if contextError(offer.ctx) != nil || !time.Now().Before(offer.expires) {
		return nil, ErrProxyRecoveryConsentExpired // repeated source I/O cannot extend consent start time
	}
	offer.consumed, offer.accepted, offer.executing = true, true, true
	offer.recovery = owner
	previousPause := owner.pause
	if previousPause == proxyRecoveryPauseWriteUnchanged {
		index := owner.pauseIndex
		owner.retiredWrites[index], owner.writes[index] = owner.writes[index], nil // only checked normally returned unchanged Go frames
	}
	if previousPause == proxyRecoveryPauseNotification {
		owner.retiredNotification, owner.notification = owner.notification, nil
	}
	owner.retiredManual, owner.manual = owner.manual, offer // bounded previous grant; no linked grant history
	owner.resumeSerial++
	owner.grantBase, owner.stepLimit = owner.steps, owner.steps+maxProxyRecoveryTransitions
	owner.returned, owner.paused, owner.pause, owner.failure = false, false, proxyRecoveryPauseNone, nil
	owner.store.failure = nil // only this exact known pause hold; source/unknown flags were never cleared
	result := owner.runResumed(previousPause == proxyRecoveryPauseBeforeWrite)
	result = owner.store.finishRecoveryCall(result)
	if owner.store.operationsReturned() {
		pm.pending = false
	} else {
		result = errors.Join(result, ErrProxyStateUncertain)
	}
	return pm.finishProxyRecoveryLocked(owner, result)
}

func (owner *proxyCrashRecovery) runResumed(cancelPrepared bool) (result error) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		owner.finishRecoveryAttempt(returned, result)
	}()
	if err := owner.checkSource(); err != nil {
		returned = true
		return err
	}
	if !owner.native.openStarted {
		if err := owner.nativeIntentAllowed(); err != nil {
			owner.pauseContext(err, proxyRecoveryPauseBeforeOpen, -1)
			returned = true
			return err
		}
		if err := owner.native.open(registry.QUERY_VALUE | registry.SET_VALUE); err != nil {
			returned = true
			return err
		}
	}
	if !owner.native.ready() {
		returned = true
		return ErrProxyStateUncertain
	}
	if cancelPrepared {
		// The retained live owner proves its setter never started. Do not
		// attribute a subsequently matching target tuple to that absent call.
		if err := owner.resolvePrepared(false); err != nil {
			returned = true
			return err
		}
	}
	result = owner.finishRecoveryValues()
	returned = true
	return result
}
