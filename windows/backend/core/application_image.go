package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ApplicationImageLease reuses current-application file/ancestor custody. It
// authenticates neither a release signature nor the kernel's loaded sections.
// Allocate and retain this exact owner before Prepare; never copy or replace a
// partial/uncertain owner. Path returns a name, not a transferable grant.
type ApplicationImageLease struct {
	mu                              sync.Mutex // operational serialization; Close never waits for native work
	retiring                        atomic.Bool
	image                           verifierApplicationLease
	attempted, ready, fault, closed bool
}

// Pure allocation: the original native image is present before its first open.
func NewApplicationImageLease() *ApplicationImageLease {
	return &ApplicationImageLease{image: newVerifierApplication()}
}

func (owner *ApplicationImageLease) finish(returned *bool, result *error, positive bool) {
	defer owner.mu.Unlock()
	if recovered := recover(); recovered != nil || !*returned {
		owner.ready, owner.fault = false, true
		*result = errors.Join(*result, ErrNativeImageUnavailable, ErrHelperCleanup)
	}
	if positive && owner.retiring.Load() {
		owner.ready = false
		*result = errors.Join(*result, ErrNativeImageUnavailable)
	}
}

func (owner *ApplicationImageLease) Prepare(ctx context.Context) (finalErr error) {
	if owner == nil || !owner.mu.TryLock() {
		return errors.Join(ErrNativeImageUnavailable, ErrHelperBusy)
	}
	returned := false
	defer owner.finish(&returned, &finalErr, true)
	if owner.retiring.Load() || owner.attempted || owner.fault || owner.closed || owner.image == nil {
		returned = true
		return ErrNativeImageUnavailable
	}
	owner.attempted = true
	finalErr = owner.image.prepare(ctx)
	owner.ready = finalErr == nil
	returned = true
	return finalErr
}

func (owner *ApplicationImageLease) Path(ctx context.Context) (path string, finalErr error) {
	if owner == nil || !owner.mu.TryLock() {
		return "", errors.Join(ErrNativeImageUnavailable, ErrHelperBusy)
	}
	returned := false
	defer func() {
		if finalErr != nil {
			path = ""
		}
	}()
	defer owner.finish(&returned, &finalErr, true)
	if owner.retiring.Load() || !owner.ready || owner.fault || owner.closed || owner.image == nil {
		returned = true
		return "", ErrNativeImageUnavailable
	}
	if finalErr = owner.image.Validate(ctx); finalErr == nil {
		path = owner.image.Executable()
		finalErr = contextError(ctx)
		if path == "" {
			finalErr = errors.Join(finalErr, ErrNativeImageUnavailable)
		}
	}
	returned = true
	return path, finalErr
}

// Close seals positive use before trying the operational lock. Unknown image
// calls remain sticky; no successor path/open or raw handle retry substitutes
// for retirement. File-level Close keeps its existing original-call guards.
func (owner *ApplicationImageLease) Close() (finalErr error) {
	if owner == nil {
		return nil
	}
	owner.retiring.Store(true)
	if !owner.mu.TryLock() {
		return errors.Join(ErrHelperBusy, ErrHelperCleanup)
	}
	returned := false
	defer owner.finish(&returned, &finalErr, false)
	owner.ready = false
	if owner.fault {
		returned = true
		return ErrHelperCleanup
	}
	if !owner.closed && owner.image != nil {
		finalErr = owner.image.Close()
	}
	if finalErr == nil {
		owner.closed = true
	}
	returned = true
	return finalErr
}
