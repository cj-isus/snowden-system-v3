package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

// Caller owns updateMu+operationMu. Backend writers/DELETE payloads are already
// confirmed retired before this synchronous callback; its original transaction
// mutex stays held. Never call PairRecovery.Retire/normal closePairRoot here.
func (a *App) pairReadiness(root string, verified *verifiedUpdate, purpose updateApplyPurpose, current func() bool) update.PairReadiness {
	return func(ctx context.Context, bind func(update.DirectoryCustodian) error) (finalErr error) {
		if ctx == nil || current == nil || bind == nil || a.pairRecoveryCheck != nil || a.pairRoot == nil || ctx.Err() != nil || !current() {
			return update.ErrPairRecoveryRequired
		}
		policyGuard := func(ctx context.Context) error {
			if ctx.Err() != nil || !current() {
				return context.Canceled
			}
			// No root validation after its confirmed release: the child needs the
			// same writer lock. The held companion image pins its original root;
			// backend bind will compare ALL saved private root/directory/file IDs.
			return a.withVerifiedUpdateModeLocked(verified, purpose, func() error { return ctx.Err() })
		}
		if a.recoveryEntrypointImage != nil {
			// A cold companion is itself the actual target NEW build. Repeat its
			// real self-image/SDK gate; no recursive child or root release needed.
			if err := a.recoveryEntrypointImage.Validate(ctx); err != nil {
				return err
			}
			if err := policyGuard(ctx); err != nil {
				return err
			}
			if err := bind(a.pairRoot); err != nil {
				return err
			}
			return policyGuard(ctx)
		}
		complete := false
		defer func() {
			if !complete {
				finalErr = errors.Join(finalErr, a.closePairRecoveryCheck())
			}
		}()
		if err := a.validatePairRoot(ctx, root); err != nil {
			return err
		}
		sum := sha256.Sum256(verified.raw)
		var err error
		a.pairRecoveryCheck, err = core.OpenUpdateRecoveryCheck(ctx, verified.manifest, hex.EncodeToString(sum[:]), policyGuard)
		if err != nil {
			return err
		}
		// Direct original close is deliberate at this backend-supplied safe
		// boundary. closePairRoot would recursively lock the active recovery tx.
		if err := a.pairRoot.Close(); err != nil {
			return err // keep that exact partial owner; no replacement or child
		}
		a.pairRoot = nil
		if err := a.pairRecoveryCheck.Verify(ctx, policyGuard); err != nil {
			return err
		}
		if err := a.pairRecoveryCheck.ValidateRecoveryReady(ctx); err != nil {
			return err
		}
		if err := policyGuard(ctx); err != nil {
			return err
		}
		if err := a.acquirePairRoot(ctx, root); err != nil {
			return err // acquire stores even the new partial original owner in App
		}
		if err := bind(a.pairRoot); err != nil {
			return err // never substitute journal old hashes for the original tx
		}
		if err := a.pairRecoveryCheck.ValidateRecoveryReady(ctx); err != nil {
			return err
		}
		if err := policyGuard(ctx); err != nil {
			return err
		}
		complete = true // retain the read-only companion through Main image acquisition
		return nil
	}
}

func (a *App) validatePairRecoveryReady(ctx context.Context) error {
	if a.recoveryEntrypointImage != nil {
		return a.recoveryEntrypointImage.Validate(ctx)
	}
	if a.pairRecoveryCheck == nil {
		return update.ErrPairRecoveryRequired
	}
	return a.pairRecoveryCheck.ValidateRecoveryReady(ctx)
}

func (a *App) closePairRecoveryCheck() error {
	if a.pairRecoveryCheck == nil {
		return nil
	}
	if err := a.pairRecoveryCheck.Close(); err != nil {
		return err // original Job/image pointer stays reachable, no fresh owner
	}
	a.pairRecoveryCheck = nil
	return nil
}
