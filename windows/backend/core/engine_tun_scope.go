package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/netprobe"
)

type engineTUNScope struct {
	instance uint64
	process  *engineProcess
	request  netprobe.TUNRequest
	transfer nativeTUNTransfer
	retired  bool // only a successful retirement of this original engine sets it
	closed   bool // exact scope-close ACK, separately from original Job retirement
}

func validTUNScopeReply(reply workerReply, request netprobe.TUNRequest) bool {
	return validWorkerScopedStateReply(reply, request.Subject, request.Selector, request.Epoch, false, request.Nonce)
}

// Caller holds the existing protectedHelper lane from image/process creation
// through scope revocation and full original retirement. This private method
// cannot adopt an optional lane, PID, user report or arbitrary public handle.
func (e *Engine) beginTUNScope(ctx context.Context, request netprobe.TUNRequest, port uint16) (result *engineTUNScope, finalErr error) {
	return e.beginTUNScopeOwned(ctx, request, port, nil) // legacy private caller owns the returned partial scope
}

func (e *Engine) beginTUNScopeOwned(ctx context.Context, request netprobe.TUNRequest, port uint16, capture *nativeTUNCapture) (result *engineTUNScope, finalErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if capture != nil && (protectedHelper.tun != capture || capture.engine != e || capture.scope != nil || capture.filterStarted) {
		return nil, ErrNativeScopeUnavailable
	}
	if contextError(ctx) != nil || request.Validate() != nil || port == 0 || !e.isolated || e.process == nil || e.cleanupErr != nil || e.tunScope != nil ||
		protectedHelper.owner == nil || protectedHelper.image == nil || protectedHelper.owner.exited() {
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
	if err := protectedHelper.image.ValidateProcess(ctx, protectedHelper.owner.host); err != nil {
		return nil, err
	}
	reply, err := e.isolatedCommandLocked(ctx, workerRequest{Action: "current", Selector: "proxy"})
	if err != nil {
		return nil, err
	}
	if reply.Admitted || reply.Previous != request.Selector || reply.AdmissionEpoch != request.Epoch {
		return nil, ErrNativeScopeUnavailable
	}
	if err := contextError(ctx); err != nil {
		return nil, errors.Join(ErrNativeScopeUnavailable, err) // do not reserve a scope after the preceding IPC wait
	}
	scope := &engineTUNScope{instance: e.process.instance, process: e.process, request: request}
	e.tunScope = scope // retain BEFORE the first duplication
	if capture != nil {
		capture.scope = scope // publish into the original helper owner before duplication/RPC, even on Goexit
	}
	e.clearForwardingPermitsLocked()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed && finalErr == nil {
			finalErr = errors.Join(finalErr, ErrNativeScopeUnavailable)
		}
		if finalErr != nil {
			retirement := &lifecycleCleanup{}
			lifecycleCleanupStep("retire scoped engine", retirement, e.retireIsolatedLocked)
			scope.retired = retirement.result == nil // normal return required, including nil panic payloads
			result = scope                           // partial custody survives even an unsuccessful scope-open
			finalErr = errors.Join(finalErr, retirement.result, scope.process.captureFailure())
		}
	}()
	if err := duplicateTUNActor(ctx, protectedHelper.owner.host, scope.process.host, &scope.transfer); err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err // partial target-handle custody stays in the original retained scope
	}
	wire := &workerTUNScopeRequest{Nonce: request.Nonce, SourcePort: port, TimeoutMillis: request.TimeoutMillis}
	scope.transfer.apply(wire)
	reply, err = scope.process.request(ctx, workerRequest{Action: "tun_scope_open", Selector: "proxy", Tag: request.Selector, Subject: request.Subject, AdmissionEpoch: request.Epoch, TUNScope: wire}, workerControlTimeout)
	if err != nil || contextError(ctx) != nil || e.process != scope.process || e.process.instance != scope.instance || e.process.exited() || !validTUNScopeReply(reply, request) {
		return nil, errors.Join(ErrNativeScopeUnavailable, err, contextError(ctx))
	}
	completed = true
	return scope, nil
}

func (e *Engine) endTUNScope(ctx context.Context, scope *engineTUNScope) (finalErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scope == nil || e.tunScope != scope || e.process != scope.process || e.process == nil || e.process.instance != scope.instance {
		return ErrNativeScopeUnavailable
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed && finalErr == nil {
			finalErr = errors.Join(finalErr, ErrNativeScopeUnavailable)
		}
		if finalErr != nil {
			retirement := &lifecycleCleanup{}
			lifecycleCleanupStep("retire scoped engine", retirement, e.retireIsolatedLocked)
			scope.retired = retirement.result == nil // cleanup panic is never an original retirement receipt
			finalErr = errors.Join(finalErr, retirement.result, scope.process.captureFailure())
		}
	}()
	if err := contextError(ctx); err != nil {
		return err
	}
	reply, err := e.process.request(ctx, workerRequest{Action: "tun_scope_close", Selector: "proxy", Tag: scope.request.Selector, Subject: scope.request.Subject, AdmissionEpoch: scope.request.Epoch, ScopeNonce: scope.request.Nonce}, workerControlTimeout)
	if err != nil || contextError(ctx) != nil || e.process.exited() || !validTUNScopeReply(reply, scope.request) {
		return errors.Join(ErrNativeScopeUnavailable, err, contextError(ctx))
	}
	e.tunScope = nil
	e.clearForwardingPermitsLocked()
	scope.closed = true
	completed = true
	return nil
}

// A cleanup retry may consume only the retained scope's original retirement
// receipt. A later engine or a matching PID/config is not a replacement owner.
func (e *Engine) retireCaptureTUNScope(ctx context.Context, scope *engineTUNScope) error {
	e.mu.Lock()
	retired := scope != nil && (scope.retired || scope.closed)
	e.mu.Unlock()
	if retired {
		return nil
	}
	err := e.endTUNScope(ctx, scope)
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil && scope != nil && (scope.retired || scope.closed) {
		return nil // only this original scope's close/Job receipt can resolve cleanup
	}
	return err
}
