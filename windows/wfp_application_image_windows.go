//go:build windows

package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

// Only a verified original negative batch may omit old positive source values
// inside the reusable image inspector. The caller's actual cancellation and
// deadline are unchanged. This context never enters WFP Add or policy routing.
type wfpNegativeImageContext struct{ context.Context }

func (wfpNegativeImageContext) Value(any) any { return nil }

func (owner *nativeWFPSessionOwner) applicationAppID(ctx context.Context, expectedPath string) (appID string, finalErr error) {
	defer func() {
		finalErr = errors.Join(finalErr, owner.queryScope(ctx))
		if finalErr != nil {
			appID = ""
		}
	}()
	if err := owner.queryScope(ctx); err != nil {
		return "", err
	}
	imageContext := ctx
	if batch := owner.queryBatch; batch != nil {
		switch batch.action {
		case wfpPolicyRevoke, wfpPolicyRestrict, wfpPolicyRelease:
			imageContext = wfpNegativeImageContext{ctx}
		}
	}
	if owner.application == nil {
		owner.application = core.NewApplicationImageLease()
		if err := owner.application.Prepare(imageContext); err != nil {
			return "", err // exact partial image remains in the original session
		}
	}
	path, err := owner.application.Path(imageContext)
	if err != nil || expectedPath != "" && path != expectedPath {
		return "", errors.Join(core.ErrNativeImageUnavailable, err)
	}
	if err := owner.queryScope(ctx); err != nil {
		return "", err
	}
	appID, finalErr = owner.imageAppID(ctx, path)
	current, imageErr := owner.application.Path(imageContext)
	finalErr = errors.Join(finalErr, imageErr)
	if current != path {
		finalErr = errors.Join(finalErr, core.ErrNativeImageUnavailable)
	}
	return appID, finalErr
}

// Independent negative retirement. An unresolved WFP query/write/transaction
// keeps its exact image too. Writer callers invoke this after their transaction
// finalizer, so an image fault cannot prevent negative WFP mutations.
func (owner *nativeWFPSessionOwner) retireApplicationImage() error {
	if owner == nil || !owner.callsReturned() || owner.transaction != nil && owner.transaction.phase != wfpTransactionFinished {
		return core.ErrHelperCleanup
	}
	if owner.application == nil {
		return nil
	}
	if err := owner.application.Close(); err != nil {
		return errors.Join(core.ErrHelperCleanup, err)
	}
	owner.application = nil // only the original normally confirmed Close
	return nil
}
