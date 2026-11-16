package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
)

type guiLeaseMode uint8

const (
	guiLeaseUnused guiLeaseMode = iota
	guiLeaseLegacy
	guiLeaseAncestor
)

// Embedded in the original GUI session before any handle adoption. Do not copy.
type updateGUINativeLease struct {
	native      nativelease.Lease
	ancestor    updateGUIAncestorLease
	mode        guiLeaseMode          // one fixed branch before adoption; never reset after any result
	stdio       [4]*updateGUIExchange // original seed/ARM/OWN/ACK I/O, retained even when bootstrap fails
	stdioNext   uint8
	stdioFailed bool
}

func updateGUILeaseError(err error) error {
	if err == nativelease.ErrLease {
		return ErrUpdateGUIBootstrap
	}
	if errors.Is(err, nativelease.ErrLease) {
		return errors.Join(ErrUpdateGUIBootstrap, err) // keep original cancellation/source cause
	}
	return err // purpose-specific callback errors keep their original identity
}

func (lease *updateGUINativeLease) adopt(handles [2]uint64) error {
	if lease == nil || lease.mode != guiLeaseUnused {
		return ErrUpdateGUIBootstrap
	}
	lease.mode = guiLeaseLegacy
	return updateGUILeaseError(lease.native.Adopt(handles))
}

func (lease *updateGUINativeLease) adoptAncestor(ctx context.Context, handles [3]uint64) error {
	if lease == nil || lease.mode != guiLeaseUnused {
		return ErrUpdateGUIBootstrap
	}
	lease.mode = guiLeaseAncestor
	return lease.ancestor.adopt(ctx, handles)
}

func (lease *updateGUINativeLease) validateJob(ctx context.Context) error {
	if lease == nil {
		return ErrUpdateGUIBootstrap
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if lease.mode == guiLeaseAncestor {
		return lease.ancestor.validate(ctx)
	}
	if lease.mode != guiLeaseLegacy {
		return ErrUpdateGUIBootstrap
	}
	return updateGUILeaseError(lease.native.ValidateJob())
}

func (lease *updateGUINativeLease) awaitParent(ctx context.Context) error {
	if lease == nil {
		return ErrUpdateGUIBootstrap
	}
	if lease.mode == guiLeaseAncestor {
		return lease.ancestor.await(ctx)
	}
	if lease.mode != guiLeaseLegacy {
		return ErrUpdateGUIBootstrap
	}
	return updateGUILeaseError(lease.native.AwaitParent(ctx, contextError))
}

func (lease *updateGUINativeLease) authenticate(ctx context.Context, expected string) error {
	if lease == nil {
		return ErrUpdateGUIBootstrap
	}
	if lease.mode == guiLeaseAncestor {
		return lease.ancestor.authenticate(ctx, expected)
	}
	if lease.mode == guiLeaseLegacy {
		return nil // existing legacy parent policy remains with its original purpose callbacks
	}
	return ErrUpdateGUIBootstrap
}
