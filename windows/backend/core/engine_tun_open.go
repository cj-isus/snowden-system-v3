package core

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

// Caller retains the protected helper lane throughout this operation. This
// original-process slot is only negative ownership, never a native grant.
type engineTUNScopeOpen struct {
	origin    engineProcessOrigin
	helper    *engineProcess
	image     nativeImageLease
	capture   *nativeTUNCapture
	request   netprobe.TUNRequest
	port      uint16
	scope     *engineTUNScope // published before any target-process duplication
	requestID uint64
	requested bool
}

func (e *Engine) beginTUNScope(ctx context.Context, request netprobe.TUNRequest, port uint16) (*engineTUNScope, error) {
	return e.beginTUNScopeOwned(ctx, request, port, nil) // legacy caller retains any returned partial scope
}

func (e *Engine) beginTUNScopeOwned(ctx context.Context, request netprobe.TUNRequest, port uint16, capture *nativeTUNCapture) (result *engineTUNScope, finalErr error) {
	attempt, err := e.reserveTUNScopeOpen(request, port, capture)
	if err != nil {
		return nil, err
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
			if attempt.origin.process.tunOpening == attempt {
				attempt.origin.process.tunOpening = nil // own metadata only, including abnormal original retirement
			}
		}()
		if finalErr != nil {
			result = attempt.scope // actual partial target custody survives failure/Goexit
			finalErr = e.rejectTUNScopeOpenLocked(attempt, finalErr)
		}
	}()
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return nil, err
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, 2*workerControlTimeout)
	if err != nil {
		return nil, err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true
			cancel()
		}
	}
	defer cancelRPC()
	if err := e.openTUNScopePrepared(rpc, attempt); err != nil {
		return nil, err
	}
	cancelRPC() // ordinary return before accepting the scope, outside Engine.mu
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(operation); err != nil {
		return nil, err
	}
	if !time.Now().Before(rpc.deadline) {
		return nil, context.DeadlineExceeded
	}
	if !e.tunScopeOpenCurrentLocked(attempt) || attempt.scope == nil || attempt.scope.closed || attempt.scope.retired || attempt.scope.retirementAttempted ||
		!attempt.origin.process.ready || attempt.origin.process.unusable || attempt.origin.process.exited() || attempt.helper.exited() {
		return nil, ErrNativeScopeUnavailable
	}
	attempt.origin.process.tunOpening = nil // no extra metadata lock/callback after final acceptance
	completed = true
	return attempt.scope, nil
}

// No caller context getters/native calls before the original slot is reserved.
// The protected helper lane owns the captured image throughout preparation.
func (e *Engine) reserveTUNScopeOpen(request netprobe.TUNRequest, port uint16, capture *nativeTUNCapture) (*engineTUNScopeOpen, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if capture != nil && (protectedHelper.tun != capture || capture.engine != e || capture.scope != nil || capture.filterStarted) {
		return nil, ErrNativeScopeUnavailable
	}
	if request.Validate() != nil || port == 0 || !e.isolated || e.process == nil || e.cleanupErr != nil || e.tunScope != nil ||
		e.startPending != nil || e.reloadPending != nil || e.process.tunOpening != nil || e.process.instance == 0 || !e.process.canReserveStateCommand() ||
		protectedHelper.owner == nil || protectedHelper.image == nil || protectedHelper.owner == e.process || protectedHelper.owner.exited() {
		return nil, ErrNativeScopeUnavailable
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || subject != request.Subject {
		return nil, ErrNativeScopeUnavailable
	}
	cfg, _, err := runtimeConfiguration(e.rawJSON)
	if err != nil {
		return nil, ErrNativeScopeUnavailable
	}
	tunCount := 0
	for _, inbound := range cfg.Inbounds {
		if inbound.Type == "tun" && inbound.Tag != "" {
			tunCount++
		} else if inbound.Type == "tun" {
			return nil, ErrNativeScopeUnavailable
		}
	}
	if tunCount != 1 {
		return nil, ErrNativeScopeUnavailable
	}
	attempt := &engineTUNScopeOpen{origin: engineProcessOrigin{process: e.process, instance: e.process.instance, raw: e.rawJSON},
		helper: protectedHelper.owner, image: protectedHelper.image, capture: capture, request: request, port: port}
	e.clearForwardingPermitsLocked()
	e.process.tunOpening = attempt
	return attempt, nil
}

func (e *Engine) tunScopeOpenCurrentLocked(attempt *engineTUNScopeOpen) bool {
	if attempt == nil || attempt.origin.process == nil || !attempt.origin.currentLocked(e) || attempt.origin.process.tunOpening != attempt ||
		e.cleanupErr != nil || e.startPending != nil || e.reloadPending != nil || e.tunScope != attempt.scope ||
		protectedHelper.owner != attempt.helper || protectedHelper.image == nil {
		return false
	}
	return attempt.capture == nil || protectedHelper.tun == attempt.capture && attempt.capture.engine == e &&
		attempt.capture.scope == attempt.scope && !attempt.capture.filterStarted
}

func (e *Engine) openTUNScopePrepared(ctx *workerRPCContext, attempt *engineTUNScopeOpen) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || !e.tunScopeOpenCurrentLocked(attempt) {
		return ErrNativeScopeUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := attempt.image.ValidateProcess(ctx, attempt.helper.host); err != nil {
		return err
	}
	reply, err := e.isolatedCommandLocked(ctx, workerRequest{Action: "current", Selector: "proxy"})
	if err != nil {
		return err
	}
	if reply.Admitted || reply.Previous != attempt.request.Selector || reply.AdmissionEpoch != attempt.request.Epoch || contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	scope := &engineTUNScope{instance: attempt.origin.instance, process: attempt.origin.process, request: attempt.request}
	attempt.scope, e.tunScope = scope, scope
	if attempt.capture != nil {
		attempt.capture.scope = scope // original helper capture owns partial transfer before native work
	}
	if err := duplicateTUNActor(ctx, attempt.helper.host, scope.process.host, &scope.transfer); err != nil {
		return err
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	wire := &workerTUNScopeRequest{Nonce: attempt.request.Nonce, SourcePort: attempt.port, TimeoutMillis: attempt.request.TimeoutMillis}
	scope.transfer.apply(wire)
	command := workerRequest{Action: "tun_scope_open", Selector: "proxy", Tag: attempt.request.Selector, Subject: attempt.request.Subject, AdmissionEpoch: attempt.request.Epoch, TUNScope: wire}
	attempt.requestID, attempt.requested = scope.process.sequence+1, true
	reply, err = scope.process.request(ctx, command, workerControlTimeout)
	if err != nil || contextError(ctx) != nil || !e.tunScopeOpenCurrentLocked(attempt) || scope.process.exited() || !validTUNScopeReply(reply, attempt.request) {
		return errors.Join(ErrNativeScopeUnavailable, err, contextError(ctx))
	}
	return nil
}

// Actual partial transfer belongs to this original target scope even if its
// pending preparation was revoked. Only the exact original graph may be reaped.
func (e *Engine) rejectTUNScopeOpenLocked(attempt *engineTUNScopeOpen, cause error) error {
	scope := attempt.scope
	if scope == nil {
		return cause
	}
	if !scope.closed && !scope.retired && !scope.retirementAttempted && e.tunScope == scope && attempt.origin.currentLocked(e) &&
		e.cleanupErr == nil && e.startPending == nil && e.reloadPending == nil && scope.process.tunClosing == nil {
		retirement := &lifecycleCleanup{}
		lifecycleCleanupStep("retire scoped engine", retirement, e.retireIsolatedLocked)
		cause = errors.Join(cause, retirement.result)
	}
	return errors.Join(ErrNativeScopeUnavailable, cause, scope.process.captureFailure())
}
