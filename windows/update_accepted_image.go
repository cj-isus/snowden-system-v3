package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"
)

// Main-thread-only startup quarantine. Failed preparation has one checked close;
// uncertainty remains reachable until OS exit, never a close/reopen retry proof.
var acceptedStartupImage *core.InstalledPairLease

func checkAcceptedInstalledPair() (finalErr error) {
	if runningVersion == "dev" {
		return nil // explicit nonrelease mode, never an authenticated update target
	}
	if acceptedStartupImage != nil {
		return update.ErrPairRecoveryRequired
	}
	completed, closeAttempted := false, false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, update.ErrPairRecoveryRequired)
		}
		if finalErr != nil && acceptedStartupImage != nil && !closeAttempted {
			closeAttempted = true
			closeErr := acceptedStartupImage.Close()
			finalErr = errors.Join(finalErr, closeErr)
			if closeErr == nil {
				acceptedStartupImage = nil // known bad bytes need not pin repair out of this process
			}
		}
	}()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	receipt, err := update.ReadAcceptedPair(filepath.Dir(exe))
	if err != nil {
		return err
	}
	if receipt == nil {
		completed = true
		return nil // initial install; not a pair-acceptance or release-integrity proof
	}
	current, ce := update.ParseVersion(runningVersion)
	accepted, ae := update.ParseVersion(receipt.Version)
	if ce != nil || ae != nil || current.Compare(accepted) != 0 || nativeProbeHelperSHA256 == "" || receipt.Helper.SHA256 != nativeProbeHelperSHA256 {
		return update.ErrPairRecoveryRequired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // soft cancellation, not a bound on native filesystem latency
	err = core.RetainInstalledPair(ctx, &acceptedStartupImage, receipt.Application.SHA256, receipt.Application.Size, receipt.Helper.SHA256, receipt.Helper.Size)
	if err != nil {
		return err
	}
	if err := acceptedStartupImage.Validate(ctx); err != nil {
		return err
	}
	closeAttempted = true
	if err := acceptedStartupImage.Close(); err != nil {
		return err
	}
	acceptedStartupImage = nil
	completed = true
	return nil
}
