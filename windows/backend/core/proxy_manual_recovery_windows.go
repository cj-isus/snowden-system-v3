//go:build windows

package core

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"
)

const proxyManualConfirmationTTL = 2 * time.Minute

// Value-only graph check for the Manager's exact native component. No engine,
// protection or settings observation is performed and no recovery grant arises.
func (pm *ProxyManager) proxyPreparationRetired() bool {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return proxyOwner != pm && !pm.pending && !pm.originalSet && pm.key == 0 &&
		(pm.native == nil || pm.native.closed && pm.native.handle == 0) &&
		(pm.writerLease == nil || pm.writerLease.Retired()) &&
		(pm.receiptStore == nil || pm.receiptStore.closed && !pm.receiptStore.pending && !pm.receiptStore.active)
}

func (offer *proxyManualRecovery) matchesToken(text string) bool {
	if offer == nil || len(text) != hex.EncodedLen(len(offer.token)) {
		return false
	}
	raw, err := hex.DecodeString(text)
	return err == nil && subtle.ConstantTimeCompare(raw, offer.token[:]) == 1
}

// Core operation only. App/Manager must serialize their own lifecycle and pass
// the real request context; this does not stop an Engine or release protection.
func (pm *ProxyManager) PrepareProxyRecovery(ctx context.Context) (preview *ProxyRecoveryPreview, result error) {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if ctx == nil || contextError(ctx) != nil {
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	if pm.receiptStore != nil && pm.receiptStore.crashRecovery != nil && pm.receiptStore.crashRecovery.paused {
		return pm.prepareProxyRecoveryResumeLocked(ctx)
	}
	created := false
	var prepared *proxyManualRecovery
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			preview, result = nil, ErrProxyStateUncertain
			return // unknown original frame is never cleaned up speculatively
		}
		if result != nil && (created || prepared != nil && pm.manualRecovery == prepared) &&
			!pm.pending && !pm.originalSet && pm.key == 0 && pm.phaseErrorLocked() == nil {
			if _, err := pm.retireKeyLocked(proxyNotWritten); err != nil {
				result = errors.Join(result, ErrProxyStateUncertain, err)
			}
		}
	}()
	if proxyOwner == nil {
		if err := pm.reserveSnapshotLocked(); err != nil {
			returned = true
			return nil, err
		}
		created = true
		// Semantic intake refusal is assessed below using its original objects.
		// Preview never invokes automatic system-settings recovery.
		_ = pm.acquireWriterForLocked(false)
	}
	if proxyOwner != pm || pm.manualRecovery != nil || !pm.manualPreparationReady() {
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	store := pm.receiptStore
	if err := store.subject.revalidate(); err != nil {
		returned = true
		return nil, err
	}
	input, err := store.readJournal()
	if err != nil {
		returned = true
		return nil, err
	}
	offer := newProxyManualRecovery(pm, input)
	if offer == nil {
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	pm.manualRecovery = offer // retain before entropy/further original source I/O
	prepared = offer
	if _, err := rand.Read(offer.token[:]); err != nil || offer.token == [32]byte{} {
		offer.invalidated, returned = true, true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	if err := offer.revalidate(); err != nil {
		offer.invalidated, returned = true, true
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		offer.invalidated, returned = true, true
		return nil, err
	}
	offer.expires = time.Now().Add(proxyManualConfirmationTTL) // monotonic component retained privately
	count := proxyRecoveryRecordedValueCount(input.record)
	prior := "unobserved"
	if store.priorProcess.outcome == proxyPriorProcessExited {
		prior = "observed_exited"
	} else if store.priorProcess.outcome == proxyPriorProcessReusedPID {
		prior = "pid_changed"
	}
	returned = true
	return &ProxyRecoveryPreview{Token: hex.EncodeToString(offer.token[:]), Scope: ProxyRecoveryRecordedValues,
		ExpiresAt: offer.expires.UTC().Format(time.RFC3339Nano), RecordedValues: count, PriorProcess: prior}, nil
}

func (pm *ProxyManager) ConfirmProxyRecovery(ctx context.Context, request ProxyRecoveryConfirmation) (value *ProxyRecoveryResult, result error) {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	offer := pm.manualRecovery
	if !offer.matchesToken(request.Token) || request.Scope != ProxyRecoveryRecordedValues {
		return nil, ErrProxyRecoveryConsentInvalid
	}
	if offer.finished || offer.consumed || offer.confirmStarted || offer.invalidated {
		return nil, ErrProxyRecoveryConsentExpired
	}
	offer.confirmStarted = true // even a source/expiry failure cannot replay confirmation
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			value, result = nil, ErrProxyStateUncertain
		}
		offer.returned, offer.executing = returned, false
		if result != nil {
			offer.invalidated = true
		}
	}()
	if ctx == nil || contextError(ctx) != nil || offer.expires.IsZero() || !time.Now().Before(offer.expires) {
		returned = true
		return nil, ErrProxyRecoveryConsentExpired
	}
	if err := offer.revalidate(); err != nil {
		returned = true
		return nil, err
	}
	if contextError(ctx) != nil || !time.Now().Before(offer.expires) {
		returned = true
		return nil, ErrProxyRecoveryConsentExpired // source I/O and queue time cannot extend the start deadline
	}
	if offer.resume != nil {
		offer.ctx = ctx
		value, result = pm.confirmProxyRecoveryResumeLocked(offer)
		returned = true
		return value, result
	}
	offer.consumed, offer.accepted, offer.executing, offer.ctx = true, true, true, ctx
	store := offer.store
	store.failure = nil // only the retained, explicitly permitted semantic intake refusal; never an unknown call
	owner := newProxyRecoveryOwner(store, offer.source, offer)
	if owner == nil {
		store.failure = offer.priorFailure
		offer.consumed, offer.accepted = false, false // no owner/grant was created; confirmStarted still forbids replay
		returned = true
		return nil, ErrProxyRecoveryConsentUnavailable
	}
	offer.recovery, store.crashRecovery = owner, owner // predecessor was retained and never attempted/acquired
	pm.pending = true                                  // the same original phase fence used by automatic intake
	err := store.recoverPrior(pm)
	if store.operationsReturned() {
		pm.pending = false
	} else {
		err = errors.Join(err, ErrProxyStateUncertain)
	}
	value, result = pm.finishProxyRecoveryLocked(owner, err)
	returned = true
	return value, result
}

func (pm *ProxyManager) finishProxyRecoveryLocked(owner *proxyCrashRecovery, err error) (*ProxyRecoveryResult, error) {
	if owner == nil || owner.manual == nil || pm.manualRecovery != owner.manual {
		return nil, ErrProxyStateUncertain
	}
	store := owner.store
	if !owner.completed || !store.crashReturned() || err != nil && !store.recoveredForeign(pm) {
		return nil, errors.Join(ErrProxyStateUncertain, err)
	}
	outcome := owner.outcome
	if outcome == proxyForeignPreserved {
		pm.conflictObserved = true
		_, err = pm.retireKeyLocked(proxyForeignPreserved)
	} else {
		_, err = pm.retireKeyLocked(proxyNotWritten) // fresh normal-session reservation never opened Internet Settings
	}
	if err != nil || !store.closed || pm.writerLease == nil || !pm.writerLease.Retired() || proxyOwner == pm {
		return nil, errors.Join(ErrProxyStateUncertain, err)
	}
	count := 0
	for _, attempt := range owner.writes {
		if attempt != nil && attempt.outcome == proxyWriteConfirmed && attempt.callReturned && attempt.readReturned {
			count++
		}
	}
	owner.manual.finished = true
	return &ProxyRecoveryResult{Outcome: outcome, RestoredValues: count}, nil
}

func proxyRecoveryRecordedValueCount(record proxyDurableReceipt) int {
	mask, count := record.written, 0
	if record.phase == proxyReceiptPrepared {
		mask |= 1 << record.index
	}
	for index := range proxyValueNames {
		if mask&(1<<index) != 0 {
			count++
		}
	}
	return count
}

// Cancellation may release an unused preparation even after expiry. It never
// aborts/retries an accepted recovery or discards an unknown original call.
func (pm *ProxyManager) CancelProxyRecovery(token string) (result error) {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	pm.mu.Lock()
	defer pm.mu.Unlock()
	offer := pm.manualRecovery
	if !offer.matchesToken(token) {
		return ErrProxyRecoveryConsentInvalid
	}
	if offer.finished {
		return nil
	}
	if offer.resume != nil && !offer.consumed && !offer.executing {
		// Retire only this unused grant. The original paused HKEY/lease and
		// pending transaction remain owned, including a newly unprovable source.
		offer.invalidated, offer.finished = true, true
		return nil
	}
	if offer.consumed || offer.executing || pm.pending || pm.originalSet || pm.key != 0 || pm.phaseErrorLocked() != nil {
		return ErrProxyStateUncertain
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
	}()
	offer.invalidated = true
	if _, err := pm.retireKeyLocked(proxyNotWritten); err != nil {
		returned = true
		return errors.Join(ErrProxyStateUncertain, err)
	}
	offer.finished, returned = true, true
	return nil
}
