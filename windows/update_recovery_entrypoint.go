package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

// Terminal headless dispatcher owner. Never replace partial native resources;
// os.Exit after DispatchUpdateRecovery releases any unconfirmed originals.
var recoveryEntrypointOwners *App

func verifyRecoveryEntrypoint(ctx context.Context, expectedSHA string) error {
	return runRecoveryEntrypoint(ctx, expectedSHA, false)
}

func repairFromRecoveryEntrypoint(ctx context.Context) error {
	return runRecoveryEntrypoint(ctx, "", true)
}

// No Wails startup, vault seed/recovery, configuration/network services,
// enrollment, protection changes or old-code rollback. Actual root/candidate/
// current-version/compiled-pin/private SDK proof precede repair mutations.
func runRecoveryEntrypoint(ctx context.Context, expectedSHA string, repair bool) (finalErr error) {
	if recoveryEntrypointOwners != nil || ctx == nil || ctx.Err() != nil {
		return core.ErrHelperCleanup
	}
	a := NewApp()
	recoveryEntrypointOwners = a
	a.ctx, a.installedImageChanged = ctx, true
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, update.ErrPairRecoveryRequired)
		}
		if a.pairAcceptance.GUITransferred() {
			return // original process/Job/self image retained until terminal os.Exit
		}
		finalErr = errors.Join(finalErr, a.closeUpdateCodeAcceptance())
		finalErr = errors.Join(finalErr, a.closeInstalledUpdateOwners())
		if a.pairRoot == nil && a.pairAcceptance == nil && a.recoveryEntrypointImage != nil {
			if err := a.recoveryEntrypointImage.Close(); err != nil {
				finalErr = errors.Join(finalErr, err)
			} else {
				a.recoveryEntrypointImage = nil
			}
		}
	}()
	root, candidate, verified, err := a.prepareUpdatePairCandidate(ctx, false, expectedSHA)
	if err != nil {
		return err
	}
	manifest := verified.manifest
	if err := checkInstalledUpdateVersion(manifest); err != nil {
		return err // this companion must be the target NEW build, not an old backup
	}
	err = core.RetainRecoveryImage(ctx, &a.recoveryEntrypointImage, manifest.SHA256, manifest.Size)
	if err != nil {
		return err
	}
	guard := func(ctx context.Context) error {
		if ctx.Err() != nil || a.recoveryEntrypointImage == nil {
			return context.Canceled
		}
		if err := a.recoveryEntrypointImage.Validate(ctx); err != nil {
			return err
		}
		if a.pairRoot != nil {
			if err := a.validatePairRoot(ctx, root); err != nil {
				return err
			}
		}
		return a.withVerifiedUpdatePurposeLocked(verified, true, func() error { return ctx.Err() })
	}
	if err := guard(ctx); err != nil {
		return err
	}
	if !repair {
		if err := manifest.NativeHelper.Payload.Verify(filepath.Join(candidate.Directory, update.RecoveryHelperCopyFile)); err != nil {
			return err // readiness also requires the retained NEW helper, not just the running companion
		}
		if err := guard(ctx); err != nil {
			return err // repeat actual source/time after the possibly stalled retained-copy read
		}
		completed = true
		return nil // actual native cleanup must also succeed before the dispatcher emits a receipt
	}
	a.pairRecovery, err = update.OpenPairRecovery(ctx, root, verified.raw, verified.sidecar, a.pairRoot)
	if err != nil {
		return err
	}
	if err := a.pairRecovery.Apply(ctx, func(commit func() error) error {
		if ctx.Err() != nil || a.recoveryEntrypointImage.Validate(ctx) != nil || a.validatePairRoot(ctx, root) != nil {
			return update.ErrPairRecoveryRequired
		}
		return a.withVerifiedUpdatePurposeLocked(verified, true, func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return commit() // short native/filesystem step; no App callbacks under journal
		})
	}, a.pairReadiness(root, verified, updateCurrentCodePurpose,
		func() bool { return !a.closing.Load() && ctx.Err() == nil })); err != nil {
		return err
	}
	if err := a.pairRecovery.Retire(); err != nil {
		return err
	}
	if err := a.pairRoot.ReleasePayloads(); err != nil {
		return err
	}
	a.pairRecovery = nil
	if err := guard(ctx); err != nil {
		return err
	}
	sum := sha256.Sum256(verified.raw)
	err = core.RetainUpdateCodeAcceptance(ctx, &a.pairAcceptance, manifest, hex.EncodeToString(sum[:]), guard)
	if err != nil {
		return err
	}
	if err := a.closePairRoot(); err != nil {
		return err
	}
	if err := a.pairAcceptance.Verify(ctx, guard); err != nil {
		return err // fixed main sibling must run its own actual SDK/native gate
	}
	if err := a.pairAcceptance.StartGUI(ctx, false, false, guard); err != nil {
		return err // cold repair never resumes VPN intent from an unauthenticated argument
	}
	var authorization core.UpdateGUICommit
	receipt, err := a.pairAcceptance.CommitGUI(ctx, guard, func(commit core.UpdateGUICommit) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !lifecyclePublicationCurrent(ctx) || a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() ||
			!a.startupIntent.currentLocked(a) || a.restartCommit != nil || a.releaseCommit != nil {
			return context.Canceled
		}
		if err := commit.Accept(); err != nil {
			return err
		}
		authorization = commit // cold repair authorizes only its original maintenance lifetime, never VPN resume
		return nil
	})
	if consumeErr := receipt.Consume(authorization); consumeErr != nil {
		return errors.Join(err, consumeErr)
	}
	a.closing.Store(true)
	completed = true
	return err // dispatcher exits this parent; a completed receipt is retained alongside any late error
}
