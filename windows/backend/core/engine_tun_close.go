package core

import (
	"context"
	"errors"
	"time"
)

// A negative parent reservation for this original scope only. An actual ACK
// remains a separate cleanup fact when the caller later becomes stale.
type engineTUNScopeClose struct {
	origin       engineProcessOrigin
	scope        *engineTUNScope
	requestID    uint64
	requested    bool
	acknowledged bool
}

func (e *Engine) endTUNScope(ctx context.Context, scope *engineTUNScope) (finalErr error) {
	attempt, err := e.reserveTUNScopeClose(scope)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed && finalErr == nil {
			finalErr = errors.Join(finalErr, ErrNativeScopeUnavailable)
		}
		if completed && finalErr == nil {
			return // own slot was discarded under the final acceptance lock
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		defer func() {
			if attempt.origin.process.tunClosing == attempt {
				attempt.origin.process.tunClosing = nil // independent of an abnormal original reap
			}
		}()
		if finalErr != nil {
			finalErr = e.rejectTUNScopeCloseLocked(attempt, finalErr)
		}
	}()
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return err
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true
			cancel()
		}
	}
	defer cancelRPC()
	if err := e.closeTUNScopePrepared(rpc, attempt); err != nil {
		return err
	}
	cancelRPC() // no context callback after final acceptance, outside Engine.mu
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(operation); err != nil {
		return err
	}
	if !time.Now().Before(rpc.deadline) {
		return context.DeadlineExceeded
	}
	if !e.tunScopeCloseCurrentLocked(attempt) || !attempt.acknowledged || !scope.closed ||
		!scope.process.ready || scope.process.unusable || scope.process.exited() {
		return ErrNativeScopeUnavailable
	}
	scope.process.tunClosing = nil
	completed = true
	return nil
}

// Capture the actual active scope before caller context getters. Invalidating
// old permits precedes this reservation; an ACK does not require another clear.
func (e *Engine) reserveTUNScopeClose(scope *engineTUNScope) (*engineTUNScopeClose, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scope == nil || !e.isolated || e.process == nil || e.process != scope.process || e.process.instance != scope.instance || scope.instance == 0 ||
		e.tunScope != scope || scope.closed || scope.retired || scope.retirementAttempted || e.cleanupErr != nil ||
		e.startPending != nil || e.reloadPending != nil || scope.process.tunClosing != nil {
		return nil, ErrNativeScopeUnavailable
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || subject != scope.request.Subject {
		return nil, ErrNativeScopeUnavailable
	}
	attempt := &engineTUNScopeClose{origin: engineProcessOrigin{process: scope.process, instance: scope.instance, raw: e.rawJSON}, scope: scope}
	e.clearForwardingPermitsLocked()
	scope.process.tunClosing = attempt
	return attempt, nil
}

func (e *Engine) tunScopeCloseCurrentLocked(attempt *engineTUNScopeClose) bool {
	if attempt == nil || attempt.scope == nil || attempt.origin.process == nil || !attempt.origin.currentLocked(e) ||
		attempt.origin.process.tunClosing != attempt || attempt.scope.process != attempt.origin.process || attempt.scope.instance != attempt.origin.instance ||
		attempt.scope.retired || attempt.scope.retirementAttempted || e.cleanupErr != nil || e.startPending != nil || e.reloadPending != nil {
		return false
	}
	if attempt.acknowledged {
		return attempt.scope.closed && e.tunScope == nil
	}
	return !attempt.scope.closed && e.tunScope == attempt.scope
}

func (e *Engine) closeTUNScopePrepared(ctx *workerRPCContext, attempt *engineTUNScopeClose) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || !e.tunScopeCloseCurrentLocked(attempt) || attempt.acknowledged {
		return ErrNativeScopeUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	scope, process := attempt.scope, attempt.origin.process
	command := workerRequest{Action: "tun_scope_close", Selector: "proxy", Tag: scope.request.Selector, Subject: scope.request.Subject,
		AdmissionEpoch: scope.request.Epoch, ScopeNonce: scope.request.Nonce}
	attempt.requestID, attempt.requested = process.sequence+1, true
	reply, err := process.request(ctx, command, workerControlTimeout)
	if err != nil || !e.tunScopeCloseCurrentLocked(attempt) || !validTUNScopeReply(reply, scope.request) {
		return errors.Join(ErrNativeScopeUnavailable, err)
	}
	// The original validated reply confirms actual scope/flow retirement. Keep
	// this negative cleanup fact even if later outer cancellation fails; it does
	// not prove Job retirement, a fresh operation or forwarding admission.
	scope.closed, attempt.acknowledged = true, true
	e.tunScope = nil
	if err := contextError(ctx); err != nil {
		return err
	}
	if process.unusable || process.exited() {
		return ErrNativeScopeUnavailable
	}
	return nil
}

func (e *Engine) rejectTUNScopeCloseLocked(attempt *engineTUNScopeClose, cause error) error {
	scope := attempt.scope
	if !scope.closed && !scope.retired && !scope.retirementAttempted && e.tunScope == scope && attempt.origin.currentLocked(e) &&
		e.cleanupErr == nil && e.startPending == nil && e.reloadPending == nil {
		retirement := &lifecycleCleanup{}
		lifecycleCleanupStep("retire scoped engine", retirement, e.retireIsolatedLocked)
		cause = errors.Join(cause, retirement.result)
	}
	return errors.Join(ErrNativeScopeUnavailable, cause, scope.process.captureFailure())
}
