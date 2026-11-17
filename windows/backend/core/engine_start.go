package core

import (
	"context"
	"errors"
	"time"
)

// raw is immutable after registration; other fields are guarded by Engine.mu.
// The actual process is retained here before any native acquisition. A failed
// cleanup attempt cannot be retried implicitly by this operation's finalizer.
type engineStartOperation struct {
	raw, target     []byte
	process         *engineProcess
	retireAttempted bool
}

// Start reserves the original cold-start operation before preparing its RPC
// context/config, then accepts only the same owned process after budget cleanup.
func (e *Engine) Start(ctx context.Context) error {
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return err
	}
	e.mu.Lock()
	isolated := e.isolated
	e.mu.Unlock()
	if !isolated {
		return e.startEmbedded(ctx)
	}
	attempt, err := e.beginIsolatedStart(operation)
	if err != nil {
		return err
	}
	accepted := false
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		defer func() {
			if e.startPending == attempt {
				e.startPending = nil // never discard a successor's start reservation
			}
		}()
		if !accepted && !attempt.retireAttempted {
			_ = e.rejectIsolatedStartLocked(attempt, ErrEngineWorkerUnavailable)
		}
	}()
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerHelloTimeout+workerStartTimeout)
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
	normalized, err := protectedRuntimeConfig(rpc, attempt.raw) // legacy file validation outside Engine.mu
	if err != nil {
		return err
	}
	if err := e.startPrepared(rpc, attempt, normalized); err != nil {
		return err
	}
	cancelRPC() // only this RPC budget ends; the worker has its independent lifetime
	e.mu.Lock()
	defer e.mu.Unlock()
	cause := contextError(operation)
	if cause == nil {
		switch {
		case !time.Now().Before(rpc.deadline):
			cause = context.DeadlineExceeded
		case !e.isolatedStartCurrentLocked(attempt) || e.cleanupErr != nil || e.box != nil || !attempt.process.ready ||
			attempt.process.unusable || attempt.process.exited() || attempt.process.instance == 0:
			cause = ErrEngineWorkerUnavailable
		}
	}
	if cause != nil {
		return e.rejectIsolatedStartLocked(attempt, cause)
	}
	accepted = true
	return nil
}

func (e *Engine) beginIsolatedStart(operation *nativeTUNOperation) (*engineStartOperation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(operation); err != nil {
		return nil, err
	}
	if !e.isolated {
		return nil, ErrEngineWorkerUnavailable
	}
	if e.cleanupErr != nil {
		return nil, e.cleanupErr
	}
	if e.box != nil || e.process != nil || e.startPending != nil || e.reloadPending != nil {
		return nil, ErrEngineAlreadyRunning // a reservation is busy, not a health receipt
	}
	if len(e.rawJSON) == 0 {
		return nil, ErrEngineNotConfigured
	}
	attempt := &engineStartOperation{raw: e.rawJSON}
	e.startPending = attempt
	return attempt, nil
}

func (e *Engine) startPrepared(ctx *workerRPCContext, attempt *engineStartOperation, normalized []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || attempt == nil {
		return ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	if !e.isolated || e.startPending != attempt || e.process != nil || e.box != nil || len(attempt.raw) == 0 ||
		len(e.rawJSON) != len(attempt.raw) || &e.rawJSON[0] != &attempt.raw[0] {
		return ErrEngineWorkerUnavailable // Stop or a replacement revoked this exact attempt
	}
	if len(normalized) == 0 {
		return ErrEngineNotConfigured
	}
	attempt.target = normalized
	e.rawJSON = normalized // these owned bytes passed the existing full config validator
	return e.startIsolatedLocked(ctx, attempt)
}

func (e *Engine) isolatedStartCurrentLocked(attempt *engineStartOperation) bool {
	return attempt != nil && e.startPending == attempt && e.isolated && attempt.process != nil && e.process == attempt.process &&
		len(attempt.target) != 0 && len(e.rawJSON) == len(attempt.target) && &e.rawJSON[0] == &attempt.target[0]
}

func (e *Engine) rejectIsolatedStartLocked(attempt *engineStartOperation, cause error) error {
	if attempt != nil && !attempt.retireAttempted && e.isolatedStartCurrentLocked(attempt) {
		attempt.retireAttempted = true // before fallible native cleanup; no implicit retry on Goexit
		return e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, cause)
	}
	return errors.Join(ErrEngineWorkerUnavailable, cause) // no ownership of a successor
}

// Preserve the embedded worker's independent background lifetime and original
// cancellation handoff. Application composition has no embedded fallback.
func (e *Engine) startEmbedded(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if e.isolated {
		return ErrEngineWorkerUnavailable
	}
	if e.cleanupErr != nil {
		return e.cleanupErr
	}
	if e.box != nil || e.process != nil || e.startPending != nil || e.reloadPending != nil {
		return ErrEngineAlreadyRunning
	}
	if len(e.rawJSON) == 0 {
		return ErrEngineNotConfigured
	}
	return e.startForOperation(ctx)
}
