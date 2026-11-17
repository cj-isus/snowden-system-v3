package core

import (
	"context"

	"github.com/snowden-system/windows/backend/netprobe"
)

type engineTUNScope struct {
	instance            uint64
	process             *engineProcess
	request             netprobe.TUNRequest
	transfer            nativeTUNTransfer
	retired             bool // only a successful retirement of this original engine sets it
	closed              bool // exact scope-close ACK, separately from original Job retirement
	retirementAttempted bool // actual original reaper began; no automatic scope-close retry after uncertainty
}

func validTUNScopeReply(reply workerReply, request netprobe.TUNRequest) bool {
	return validWorkerScopedStateReply(reply, request.Subject, request.Selector, request.Epoch, false, request.Nonce)
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
