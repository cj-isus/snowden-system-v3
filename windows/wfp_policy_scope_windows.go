//go:build windows

package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/core"
)

// Positive WFP work uses the existing real-deadline check and original
// value-only operation sources. This creates no context, timer or native call.
// Original negative Close/Free/Abort must not depend on this operation remaining
// current: they retain their own handle/call-return requirements instead.
func wfpOperationScope(ctx context.Context) error {
	err := childbootstrap.CheckScope(ctx, func(ctx context.Context) error {
		if err := diagnosticWFPLeaseSourceError(ctx); err != nil {
			return err
		}
		if err := protectionArmSourceError(ctx); err != nil {
			return err
		}
		if !core.OperationSourcesCurrent(ctx) {
			return core.ErrNativeScopeUnavailable
		}
		return nil
	})
	if err != nil {
		return errors.Join(core.ErrNativeScopeUnavailable, err)
	}
	return nil
}

// Revoke/Restrict/Release retain the caller's independent negative-cleanup context.
// A revoked grant's value-only sources must not veto the original revocation.
// This classification is internal policy routing, never a forwarding permit.
func wfpPolicyOperationScope(ctx context.Context, action wfpPolicyAction) error {
	if ctx == nil {
		return core.ErrNativeScopeUnavailable
	}
	switch action {
	case wfpPolicyArm:
		return wfpOperationScope(ctx)
	case wfpPolicyGrant:
		return wfpDiagnosticOperationScope(ctx) // base-barrier intent cannot authorize general TUN permits
	case wfpPolicyRevoke, wfpPolicyRestrict, wfpPolicyRelease:
		return ctx.Err()
	default:
		return core.ErrNativeScopeUnavailable
	}
}

// Query polarity comes from the original bound policy batch, never a caller
// context flag. Read-only snapshots retain that binding even after replacing
// session.transaction. Both caller and original batch remain required; native
// Free/Destroy/Abort/Close do not consult this positive-data admission fence.
func (owner *nativeWFPSessionOwner) queryScope(ctx context.Context) error {
	if owner == nil {
		return core.ErrNativeScopeUnavailable
	}
	batch := owner.queryBatch
	if batch == nil {
		return wfpOperationScope(ctx)
	}
	if !batch.queryActive.Load() || batch.session != owner || batch.transaction == nil || batch.transaction.batch != batch {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpPolicyOperationScope(ctx, batch.action); err != nil {
		return err
	}
	return batch.operationScope()
}

// Base-barrier contexts deliberately survive network cancellation. They must
// never supply diagnostic or general forwarding authority in its place.
func wfpDiagnosticOperationScope(ctx context.Context) error {
	if ctx == nil || ctx.Value(protectionArmContextKey{}) != nil {
		return core.ErrNativeScopeUnavailable
	}
	return wfpOperationScope(ctx)
}

func (batch *nativeWFPBatch) operationScope() error {
	if batch == nil {
		return core.ErrNativeScopeUnavailable
	}
	return wfpPolicyOperationScope(batch.operation, batch.action)
}

func (transaction *nativeWFPTransaction) operationScope(ctx context.Context) error {
	if transaction == nil {
		return core.ErrNativeScopeUnavailable
	}
	batch := transaction.batch
	if batch == nil {
		return wfpOperationScope(ctx) // legacy unbound transaction; actual policy writes use a bound batch
	}
	if batch.transaction != transaction {
		return core.ErrNativeScopeUnavailable
	}
	if err := wfpPolicyOperationScope(ctx, batch.action); err != nil {
		return err // the caller can shorten, never replace the original batch source
	}
	return batch.operationScope()
}
