package core

import (
	"context"
	"errors"
	"time"
)

// Exact parent consume reservation, not another token or native permission.
// The original process owns the slot; a successor never inherits it.
type engineNativeTUNConsume struct {
	origin              engineProcessOrigin
	permit              *engineNativeTUNPermit
	requestID           uint64
	requested           bool
	retirementAttempted bool
}

// Called only after the compiled platform's exact WFP-ready phase. Both
// parent lanes are burned before any caller context method or outcome.
func (engine *Engine) CommitPreparedNative(ctx context.Context, observation NativeTUNObservation) (NativeTUNAdmission, error) {
	if engine == nil {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable
	}
	attempt := engine.beginNativeTUNConsume()
	accepted := false
	defer func() {
		if attempt.origin.process == nil {
			return
		}
		engine.mu.Lock()
		defer engine.mu.Unlock()
		defer func() {
			if attempt.origin.process.nativeConsume == attempt {
				attempt.origin.process.nativeConsume = nil // only this original slot, including abnormal reap
			}
		}()
		if !accepted {
			_ = engine.rejectNativeTUNConsumeLocked(attempt, ErrForwardingNotAdmitted)
		}
	}()
	if !NativeTUNForwardingAvailable() || attempt.permit == nil || observation.source == nil || attempt.permit.source != observation.source {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable // gate stays closed without context callbacks
	}
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return NativeTUNAdmission{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	commit, _ := ctx.Value(nativeForwardingCommitKey{}).(*nativeForwardingCommit)
	if !commit.current(ctx, engine) {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return NativeTUNAdmission{}, err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true // never retry an unknown parent unlink
			cancel()
		}
	}
	defer cancelRPC()
	if !commit.current(ctx, engine) {
		return NativeTUNAdmission{}, ErrNativeScopeUnavailable // context preparation may have outlived Manager ownership
	}
	if err := engine.consumeNativeTUNPrepared(rpc, operation, observation.source, attempt); err != nil {
		return NativeTUNAdmission{}, err
	}
	cancelRPC()                                   // ordinary return before admission, outside Engine.mu
	managerCurrent := commit.current(ctx, engine) // no Manager/caller callback under Engine.mu
	engine.mu.Lock()
	defer engine.mu.Unlock()
	cause := contextError(operation)
	if cause == nil {
		switch {
		case !time.Now().Before(rpc.deadline):
			cause = context.DeadlineExceeded
		case !managerCurrent || !engine.nativeTUNConsumeCurrentLocked(attempt) ||
			!attempt.origin.process.ready || attempt.origin.process.unusable || attempt.origin.process.exited() ||
			!engine.nativePermitPreparationLocked(operation, observation.source, attempt.permit):
			cause = ErrForwardingNotAdmitted
		}
	}
	if cause != nil {
		return NativeTUNAdmission{}, engine.rejectNativeTUNConsumeLocked(attempt, cause)
	}
	accepted = true
	return NativeTUNAdmission{engine: engine, permit: attempt.permit, commit: commit}, nil
}

// Pure original capture/burn. Context callbacks cannot substitute a later
// permit, process or graph while the RPC scope is being prepared.
func (engine *Engine) beginNativeTUNConsume() *engineNativeTUNConsume {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	permit := engine.nativePermit
	origin := engineProcessOrigin{process: engine.process, raw: engine.rawJSON}
	if origin.process != nil {
		origin.instance = origin.process.instance
	}
	engine.clearForwardingPermitsLocked()
	attempt := &engineNativeTUNConsume{origin: origin, permit: permit}
	if origin.process != nil {
		origin.process.nativeConsume = attempt
	}
	return attempt
}

// Negative ownership only. Ready/unusable/exit and permit expiry are separate
// positive checks, so an uncertain original can still follow its actual reaper.
func (engine *Engine) nativeTUNConsumeCurrentLocked(attempt *engineNativeTUNConsume) bool {
	return attempt != nil && attempt.permit != nil && attempt.origin.process != nil && attempt.origin.currentLocked(engine) &&
		attempt.origin.process.nativeConsume == attempt && attempt.permit.process == attempt.origin.process &&
		attempt.permit.instance == attempt.origin.instance && engine.cleanupErr == nil &&
		engine.startPending == nil && engine.reloadPending == nil && engine.tunScope == nil
}

func (engine *Engine) rejectNativeTUNConsumeLocked(attempt *engineNativeTUNConsume, cause error) error {
	if attempt != nil && attempt.requested && !attempt.retirementAttempted && engine.nativeTUNConsumeCurrentLocked(attempt) {
		attempt.retirementAttempted = true
		return engine.retireIsolatedFailureLocked(ErrForwardingNotAdmitted, cause)
	}
	return errors.Join(ErrForwardingNotAdmitted, cause) // no cleanup of a successor or already-handed-off owner
}

func (engine *Engine) consumeNativeTUNPrepared(ctx *workerRPCContext, operation *nativeTUNOperation, source *nativeTUNObservationSource, attempt *engineNativeTUNConsume) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if !NativeTUNForwardingAvailable() || ctx == nil || !engine.nativeTUNConsumeCurrentLocked(attempt) || !engine.nativePermitPreparationLocked(operation, source, attempt.permit) {
		return ErrNativeScopeUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err // no protocol reservation or native consume was issued
	}
	permit, process := attempt.permit, attempt.origin.process
	command := workerRequest{Action: "admit_native", Selector: "proxy", Subject: permit.subject, Tag: permit.selector, AdmissionEpoch: permit.epoch,
		ScopeNonce: permit.source.scope.request.Nonce, NativeAdmissionToken: permit.token}
	sequence := process.sequence
	attempt.requestID, attempt.requested = sequence+1, true
	reply, err := process.request(ctx, command, workerControlTimeout)
	if process.sourceCanceledBeforeRequest(ctx, sequence, err) {
		attempt.requested = false
		return err // burned parent token does not authorize a retry or poison an untouched worker
	}
	if err != nil || contextError(ctx) != nil || contextError(operation) != nil || !engine.nativeTUNConsumeCurrentLocked(attempt) ||
		process.exited() || !validWorkerControlReply(&command, reply) {
		return engine.rejectNativeTUNConsumeLocked(attempt, errors.Join(err, contextError(ctx), contextError(operation)))
	}
	return nil
}
