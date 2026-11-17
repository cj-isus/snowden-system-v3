package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The process may still exist after an unaccepted change. Manager must not
// infer that the old selector is unchanged merely from Status() == running.
var errSwitchStateUnconfirmed = errors.New("selector state requires reconciliation")

func (e *Engine) SwitchDefaultContext(ctx context.Context, selectorTag, tag string) (string, error) {
	e.mu.Lock()
	isolated := e.isolated
	origin := engineProcessOrigin{process: e.process, raw: e.rawJSON}
	if origin.process != nil {
		origin.instance = origin.process.instance
	}
	e.mu.Unlock()
	if !isolated {
		return e.switchDefaultEmbedded(ctx, selectorTag, tag)
	}
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return "", err
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return "", err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true
			cancel()
		}
	}
	defer cancelRPC()
	previous, scope, err := e.switchDefaultPrepared(rpc, origin, selectorTag, tag)
	if err != nil {
		return "", err
	}
	resolved := false
	defer func() {
		if !resolved {
			e.mu.Lock()
			defer e.mu.Unlock()
			_ = e.rejectIsolatedSwitchLocked(scope, ErrEngineWorkerUnavailable)
		}
	}()
	cancelRPC() // an interrupted parent unlink cannot return an accepted mutation
	e.mu.Lock()
	defer e.mu.Unlock()
	cause := contextError(operation)
	if cause == nil {
		switch {
		case !time.Now().Before(rpc.deadline):
			cause = context.DeadlineExceeded
		case !e.isolatedSwitchCurrentLocked(scope) || e.cleanupErr != nil || !scope.process.ready || scope.process.unusable || scope.process.exited() ||
			scope.process.instance != origin.instance || scope.process.sequence < scope.id:
			cause = ErrEngineWorkerUnavailable
		}
	}
	resolved = true // a fallible original reap must not be retried by the abnormal-exit defer
	if cause != nil {
		return "", e.rejectIsolatedSwitchLocked(scope, cause)
	}
	return previous, nil
}

// Exact original published graph, not only equal config contents or a saved
// numeric instance. Read-only status requests need not invalidate a switch.
func (e *Engine) isolatedSwitchCurrentLocked(scope *workerStateReplyScope) bool {
	return scope != nil && scope.process != nil && e.isolated && e.process == scope.process &&
		len(scope.targetRaw) != 0 && len(e.rawJSON) == len(scope.targetRaw) && &e.rawJSON[0] == &scope.targetRaw[0]
}

func (e *Engine) rejectIsolatedSwitchLocked(scope *workerStateReplyScope, cause error) error {
	if e.isolatedSwitchCurrentLocked(scope) {
		return errors.Join(errSwitchStateUnconfirmed, ErrEngineWorkerUnavailable, e.retireIsolatedFailureLocked(cause))
	}
	return errors.Join(errSwitchStateUnconfirmed, ErrEngineWorkerUnavailable, cause) // never reap a successor
}

func (e *Engine) switchDefaultPrepared(ctx *workerRPCContext, origin engineProcessOrigin, selectorTag, tag string) (string, *workerStateReplyScope, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil {
		return "", nil, ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return "", nil, err
	}
	if e.cleanupErr != nil {
		return "", nil, e.cleanupErr
	}
	if e.startPending != nil || e.reloadPending != nil || e.tunScope != nil || e.process != nil && e.process.tunOpening != nil {
		return "", nil, ErrEngineWorkerUnavailable // no selector mutation over an unfinished lifecycle owner
	}
	if !origin.currentLocked(e) {
		return "", nil, errors.Join(errSwitchStateUnconfirmed, ErrEngineWorkerUnavailable)
	}
	if selectorTag != "proxy" || !validWorkerTag(tag) {
		return "", nil, ErrSelectorSwitchUnavailable
	}
	e.clearForwardingPermitsLocked()
	command := workerRequest{Action: "switch", Selector: selectorTag, Tag: tag}
	scope, err := prepareWorkerStateReply(e.process, e.rawJSON, command)
	if err != nil {
		return "", nil, e.retireIsolatedFailureLocked(err)
	}
	command.replyState = scope
	reply, err := e.isolatedCommandLocked(ctx, command)
	if err != nil {
		return "", nil, err
	}
	if len(scope.targetRaw) == 0 {
		return "", nil, e.retireIsolatedFailureLocked(errWorkerProtocol)
	}
	if err := contextError(ctx); err != nil {
		return "", nil, e.retireIsolatedFailureLocked(err) // worker may already have changed
	}
	if err := e.publishSelectorConfiguration(scope.targetRaw); err != nil {
		return "", nil, e.retireIsolatedFailureLocked(errors.New("engine selector snapshot sync failed"))
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || reply.Subject != subject {
		return "", nil, e.retireIsolatedFailureLocked(errWorkerProtocol)
	}
	if err := contextError(ctx); err != nil || e.process != scope.process || scope.process.exited() {
		return "", nil, e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, err)
	}
	return reply.Previous, scope, nil
}

// The embedded worker/local selector keeps its existing SDK/rollback behavior.
// Application composition uses only the isolated path; there is no fallback.
func (e *Engine) switchDefaultEmbedded(ctx context.Context, selectorTag, tag string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if e.cleanupErr != nil {
		return "", e.cleanupErr
	}
	if e.isolated {
		return "", ErrSelectorSwitchUnavailable
	}
	if e.box == nil {
		return "", ErrEngineNotRunning
	}
	out, ok := e.box.Outbound().Outbound(selectorTag)
	if !ok {
		return "", fmt.Errorf("%w: outbound %q not found", ErrSelectorSwitchUnavailable, selectorTag)
	}
	sel, isSel := out.(selectorControl)
	if !isSel {
		return "", fmt.Errorf("%w: %q is not a selector", ErrSelectorSwitchUnavailable, selectorTag)
	}
	previous := sel.Now()
	if previous == tag {
		return previous, nil
	}
	if !sel.SelectOutbound(tag) {
		return previous, fmt.Errorf("%w: outbound %q is not a selector candidate", ErrSelectorSwitchUnavailable, tag)
	}
	if err := e.rewriteSelectorDefault(selectorTag, tag); err != nil {
		sel.SelectOutbound(previous)
		return previous, fmt.Errorf("sync raw config after switch: %w", err)
	}
	return previous, nil
}
