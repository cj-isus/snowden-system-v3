package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// origin is immutable after registration. Other fields require Engine.mu.
// replaced burns the no-mutation outcome before the original close is called;
// current retains the actual candidate/rollback child, including partial spawn.
type engineReloadOperation struct {
	origin   engineProcessOrigin
	replaced bool
	current  *engineStartOperation
}

// Reload validates the candidate before replacing the original worker. A
// restored old configuration still needs Manager's fresh protected-path gate.
func (e *Engine) Reload(ctx context.Context, rawJSON []byte) error {
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return err
	}
	e.mu.Lock()
	isolated := e.isolated
	e.mu.Unlock()
	if !isolated {
		return e.reloadEmbedded(ctx, rawJSON)
	}
	if len(rawJSON) == 0 || len(rawJSON) > maxWorkerConfigBytes {
		return errors.Join(ErrReloadRejected, errors.New("reload: config exceeds worker bounds"))
	}
	attempt, err := e.beginIsolatedReload(operation)
	if err != nil {
		return err
	}
	accepted := false
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		defer func() {
			if e.reloadPending == attempt {
				e.reloadPending = nil
			}
			if attempt.current != nil && e.startPending == attempt.current {
				e.startPending = nil // discard only this reload's child slot
			}
		}()
		if !accepted {
			_ = e.rejectIsolatedReloadLocked(attempt, ErrEngineWorkerUnavailable)
		}
	}()
	limit := 2*(workerHelloTimeout+workerStartTimeout) + workerCloseTimeout + workerReapTimeout
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, limit)
	if err != nil {
		return e.reloadPreflightError(attempt, err)
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true
			cancel()
		}
	}
	defer cancelRPC()
	normalized, err := validatedRuntimeOptions(rpc, rawJSON) // full candidate validation before old close
	if err != nil {
		return e.reloadPreflightError(attempt, err)
	}
	replaced, candidateErr := e.replaceReloadPrepared(rpc, attempt, normalized)
	restored := false
	if candidateErr != nil {
		if !replaced {
			return candidateErr
		}
		if err := e.reloadRollbackEligible(rpc, attempt); err != nil {
			return errors.Join(candidateErr, err)
		}
		previous, err := protectedRuntimeConfig(rpc, attempt.origin.raw) // retained old bytes, never new descriptors
		if err != nil {
			return errors.Join(candidateErr, err)
		}
		if err := e.restartReloadPrepared(rpc, attempt, previous); err != nil {
			return errors.Join(candidateErr, err)
		}
		restored = true
	}
	cancelRPC() // budget cleanup outside Engine.mu, before either accepted outcome
	e.mu.Lock()
	defer e.mu.Unlock()
	cause := contextError(operation)
	if cause == nil {
		switch {
		case !time.Now().Before(rpc.deadline):
			cause = context.DeadlineExceeded
		case !e.isolatedReloadChildCurrentLocked(attempt) || e.cleanupErr != nil || e.box != nil ||
			!attempt.current.process.ready || attempt.current.process.unusable || attempt.current.process.exited() || attempt.current.process.instance == 0:
			cause = ErrEngineWorkerUnavailable
		}
	}
	if cause != nil {
		return errors.Join(candidateErr, e.rejectIsolatedReloadLocked(attempt, cause))
	}
	accepted = true
	if restored {
		return errors.Join(errEngineReloadRolledBack, fmt.Errorf("reload: %w", candidateErr))
	}
	return nil
}

func (e *Engine) beginIsolatedReload(operation *nativeTUNOperation) (*engineReloadOperation, error) {
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
	if e.reloadPending != nil || e.startPending != nil {
		return nil, ErrEngineWorkerUnavailable // another preparation is not proof of an unchanged live graph
	}
	if e.process == nil || e.box != nil {
		return nil, ErrEngineNotRunning
	}
	if !e.process.ready || e.process.unusable || e.process.exited() || e.process.instance == 0 {
		return nil, ErrEngineWorkerUnavailable
	}
	attempt := &engineReloadOperation{origin: engineProcessOrigin{process: e.process, instance: e.process.instance, raw: e.rawJSON}}
	e.reloadPending = attempt
	return attempt, nil
}

func (e *Engine) reloadOriginCurrentLocked(attempt *engineReloadOperation) bool {
	return attempt != nil && e.reloadPending == attempt && !attempt.replaced && e.startPending == nil && e.cleanupErr == nil && e.box == nil &&
		attempt.origin.process != nil && attempt.origin.currentLocked(e) && attempt.origin.process.ready &&
		!attempt.origin.process.unusable && !attempt.origin.process.exited()
}

func (e *Engine) reloadPreflightError(attempt *engineReloadOperation, cause error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reloadPreflightErrorLocked(attempt, cause)
}

func (e *Engine) reloadPreflightErrorLocked(attempt *engineReloadOperation, cause error) error {
	if e.reloadOriginCurrentLocked(attempt) {
		return errors.Join(ErrReloadRejected, cause) // no close was attempted on this unchanged original
	}
	return errors.Join(ErrEngineWorkerUnavailable, cause)
}

func (e *Engine) replaceReloadPrepared(ctx *workerRPCContext, attempt *engineReloadOperation, target []byte) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || attempt == nil {
		return false, ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return false, e.reloadPreflightErrorLocked(attempt, err)
	}
	if !e.reloadOriginCurrentLocked(attempt) {
		return false, ErrEngineWorkerUnavailable
	}
	if len(target) == 0 || len(target) > maxWorkerConfigBytes {
		return false, e.reloadPreflightErrorLocked(attempt, ErrEngineNotConfigured)
	}
	attempt.replaced = true // before fallible original close; no later no-mutation claim
	if err := e.closeBoxResourcesLocked(); err != nil {
		return true, fmt.Errorf("reload: close old box: %w", err)
	}
	return true, e.startReloadChildLocked(ctx, attempt, target)
}

func (e *Engine) startReloadChildLocked(ctx *workerRPCContext, attempt *engineReloadOperation, target []byte) error {
	if ctx == nil {
		return ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if attempt == nil || e.reloadPending != attempt || !attempt.replaced || !e.isolated || e.cleanupErr != nil ||
		e.process != nil || e.box != nil || e.startPending != nil || len(target) == 0 || len(target) > maxWorkerConfigBytes {
		return ErrEngineWorkerUnavailable
	}
	child := &engineStartOperation{raw: target, target: target}
	attempt.current = child
	e.startPending = child
	defer func() {
		if e.startPending == child {
			e.startPending = nil
		}
	}()
	e.rawJSON = target
	return e.startIsolatedLocked(ctx, child)
}

func (e *Engine) reloadRollbackCurrentLocked(attempt *engineReloadOperation) bool {
	return attempt != nil && e.reloadPending == attempt && attempt.replaced && attempt.current != nil && e.isolated &&
		e.process == nil && e.box == nil && e.cleanupErr == nil && e.startPending == nil && len(attempt.current.target) != 0 &&
		len(e.rawJSON) == len(attempt.current.target) && &e.rawJSON[0] == &attempt.current.target[0]
}

func (e *Engine) reloadRollbackEligible(ctx *workerRPCContext, attempt *engineReloadOperation) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil {
		return ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if !e.reloadRollbackCurrentLocked(attempt) {
		return ErrEngineWorkerUnavailable
	}
	return nil
}

func (e *Engine) restartReloadPrepared(ctx *workerRPCContext, attempt *engineReloadOperation, previous []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || !e.reloadRollbackCurrentLocked(attempt) {
		return ErrEngineWorkerUnavailable
	}
	return e.startReloadChildLocked(ctx, attempt, previous)
}

func (e *Engine) isolatedReloadChildCurrentLocked(attempt *engineReloadOperation) bool {
	return attempt != nil && e.reloadPending == attempt && attempt.replaced && e.isolated && attempt.current != nil &&
		attempt.current.process != nil && e.process == attempt.current.process && len(attempt.current.target) != 0 &&
		len(e.rawJSON) == len(attempt.current.target) && &e.rawJSON[0] == &attempt.current.target[0]
}

func (e *Engine) rejectIsolatedReloadLocked(attempt *engineReloadOperation, cause error) error {
	if e.isolatedReloadChildCurrentLocked(attempt) && !attempt.current.retireAttempted {
		attempt.current.retireAttempted = true
		return e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, cause)
	}
	return errors.Join(ErrEngineWorkerUnavailable, cause) // no retry of unknown cleanup or successor adoption
}

// Preserve the existing embedded SDK validation/lifetime/rollback path.
func (e *Engine) reloadEmbedded(ctx context.Context, rawJSON []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(rawJSON) == 0 {
		return errors.Join(ErrReloadRejected, errors.New("reload: empty config"))
	}
	normalized, err := validatedRuntimeOptions(ctx, rawJSON)
	if err != nil {
		return errors.Join(ErrReloadRejected, err)
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
	if e.box == nil && e.process == nil {
		return ErrEngineNotRunning
	}
	oldJSON := append([]byte(nil), e.rawJSON...)
	if err := e.closeBoxLocked(); err != nil {
		return fmt.Errorf("reload: close old box: %w", err)
	}
	e.rawJSON = normalized
	if err := e.startForOperation(ctx); err != nil {
		if contextError(ctx) != nil || e.cleanupErr != nil {
			return err
		}
		e.rawJSON = oldJSON
		if restartErr := e.startForOperation(ctx); restartErr != nil {
			return errors.Join(err, restartErr)
		}
		return errors.Join(errEngineReloadRolledBack, fmt.Errorf("reload: %w", err))
	}
	return nil
}
