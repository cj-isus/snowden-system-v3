package core

import (
	"context"
	"errors"
)

// Fixed sibling recovery executable, authenticated against actual SDK-owned
// application bytes. No dependency on a possibly missing main/helper image.
// This is image custody, not role/trust authority or a process owner.
type RecoveryImageLease struct {
	image nativeImageLease
	ready bool
}

type recoveryImagePreparation interface {
	nativeImageLease
	open(context.Context, string, int64) error
}

func OpenRecoveryImage(ctx context.Context, digest string, size int64) (owner *RecoveryImageLease, finalErr error) {
	finalErr = RetainRecoveryImage(ctx, &owner, digest, size)
	return owner, finalErr
}

// The caller's original slot is published before acquisition, including errors.
// Never pass a temporary replacement for an already-owned partial receipt.
func RetainRecoveryImage(ctx context.Context, slot **RecoveryImageLease, digest string, size int64) (finalErr error) {
	if slot == nil || *slot != nil {
		return ErrHelperCleanup
	}
	owner := &RecoveryImageLease{}
	*slot = owner
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrNativeImageUnavailable)
		}
	}()
	finalErr = owner.open(ctx, digest, size)
	completed = true
	return finalErr
}

func (owner *RecoveryImageLease) open(ctx context.Context, digest string, size int64) (err error) {
	if owner == nil || owner.image != nil {
		return ErrHelperCleanup
	}
	image := newRecoveryNativeImage()
	owner.image = image
	if image == nil {
		return ErrNativeImageUnavailable
	}
	err = image.open(ctx, digest, size)
	owner.ready = err == nil
	return err
}

func (owner *RecoveryImageLease) Executable() string {
	if owner == nil || !owner.ready || owner.image == nil {
		return ""
	}
	return owner.image.Executable()
}

func (owner *RecoveryImageLease) Validate(ctx context.Context) error {
	if owner == nil || !owner.ready || owner.image == nil {
		return ErrNativeImageUnavailable
	}
	return owner.image.Validate(ctx)
}

func (owner *RecoveryImageLease) ValidateProcess(ctx context.Context, host processHost) error {
	if err := owner.Validate(ctx); err != nil {
		return err
	}
	return owner.image.ValidateProcess(ctx, host)
}

func (owner *RecoveryImageLease) Close() (finalErr error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrHelperCleanup)
		}
	}()
	if owner != nil && owner.image != nil {
		owner.ready = false
		if err := owner.image.Close(); err != nil {
			return err
		}
		owner.image = nil
	}
	completed = true
	return nil
}
