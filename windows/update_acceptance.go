package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

// Each fixed internal mode runs once before Wails. Retain even a failed native
// cleanup owner through terminal dispatcher exit; never replace it on retry.
var updateCodeAcceptanceOwners *App

// Caller owns updateMu + operationMu. Both normal restricted startup and the
// fixed acceptance worker reuse this exact private SDK/native candidate gate.
// Expected digest is target binding, never a replacement for actual authority.
func (a *App) prepareInstalledUpdateCandidate(ctx context.Context, retain bool, expectedSHA string) (root string, verified *verifiedUpdate, finalErr error) {
	root, _, verified, err := a.prepareUpdatePairCandidate(ctx, retain, expectedSHA)
	if err != nil {
		return root, nil, err
	}
	manifest := verified.manifest
	if err := checkInstalledUpdateVersion(manifest); err != nil {
		return root, nil, err
	}
	err = core.RetainInstalledPair(ctx, &a.pairImage, manifest.SHA256, manifest.Size, manifest.NativeHelper.SHA256, manifest.NativeHelper.Size)
	if err != nil {
		return root, nil, err // caller retains partial original image/root owners
	}
	return root, verified, nil
}

// Shared exact candidate/private SDK preparation. The installed-code worker
// still adds its stricter current-version/compiled-pin/native-image gate above;
// forward repair by an older restricted GUI uses the recovery gate separately.
func (a *App) prepareUpdatePairCandidate(ctx context.Context, retain bool, expectedSHA string) (root string, candidate *update.PairCandidate, verified *verifiedUpdate, finalErr error) {
	if a.pairImage != nil || a.pairRoot != nil {
		return "", nil, nil, core.ErrHelperCleanup // no replacement of an uncertain owner
	}
	exe, err := os.Executable()
	if err != nil {
		return "", nil, nil, err
	}
	root = filepath.Dir(exe)
	if err := a.acquirePairRoot(ctx, root); err != nil {
		return root, nil, nil, err
	}
	candidate, err = update.LoadPairCandidate(ctx, root, a.pairRoot)
	if err != nil {
		return root, nil, nil, err
	}
	if expectedSHA != "" {
		sum := sha256.Sum256(candidate.RawManifest)
		if hex.EncodeToString(sum[:]) != expectedSHA {
			return root, nil, nil, update.ErrPairRecoveryRequired
		}
	}
	verified, err = a.prepareVerifiedUpdate(candidate.Directory, retain)
	if err != nil {
		return root, nil, nil, err
	}
	if !bytes.Equal(verified.raw, candidate.RawManifest) || !bytes.Equal(verified.sidecar, candidate.Delivery) {
		return root, nil, nil, update.ErrPairRecoveryRequired
	}
	return root, candidate, verified, nil
}

// Exact original cleanup shared by startup and the worker. An uncertain image
// keeps the writer root; clearing a pointer requires confirmed native Close.
func (a *App) closeInstalledUpdateOwners() (result error) {
	if err := a.closePairRecoveryCheck(); err != nil {
		return err // do not release image/root under an unconfirmed companion process/image owner
	}
	if a.pairImage != nil {
		result = a.pairImage.Close()
		if result == nil {
			a.pairImage = nil
		}
	}
	if a.pairImage == nil {
		result = errors.Join(result, a.closePairRoot())
	}
	return result
}

// Invoked only by the fixed internal dispatcher, before Wails/startup effects.
// NewApp itself is a value constructor. No startup, vault recovery/seed, logs,
// proxy/WFP/TUN/network producers, prefix commit, floor or pending removal here.
func verifyUpdateCodeAcceptance(ctx context.Context, expectedSHA string) (finalErr error) {
	if updateCodeAcceptanceOwners != nil {
		return core.ErrHelperCleanup
	}
	app := NewApp()
	updateCodeAcceptanceOwners = app
	app.installedImageChanged = true // restricted before any original context callback
	app.updateMu.Lock()
	defer app.updateMu.Unlock()
	app.operationMu.Lock()
	defer app.operationMu.Unlock()
	defer func() {
		finalErr = errors.Join(finalErr, app.closeInstalledUpdateOwners())
		if app.pairImage == nil && app.pairRoot == nil {
			updateCodeAcceptanceOwners = nil
		}
	}()
	if err := app.bindHeadlessLifetime(ctx); err != nil {
		return core.ErrHelperCleanup
	}
	root, verified, err := app.prepareInstalledUpdateCandidate(ctx, false, expectedSHA)
	if err != nil {
		return err
	}
	if err := app.validatePairRoot(ctx, root); err != nil {
		return err
	}
	if err := app.pairImage.Validate(ctx); err != nil {
		return err
	}
	return app.withVerifiedUpdatePurposeLocked(verified, true, func() error { return ctx.Err() })
}
