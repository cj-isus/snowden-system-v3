package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/update"
)

var (
	ErrInstallationRootUnavailable = errors.New("installation root custody unavailable")
	ErrInstallationRootBusy        = errors.New("installation root is owned by another transaction")
)

type nativeInstallationRoot interface {
	Validate(context.Context, string) error
	PinDirectory(context.Context, string) error
	MoveDirectory(context.Context, string, string) (bool, error)
	ReleaseDirectory(string) error
	PinPayload(context.Context, string, []update.Payload) (update.PayloadFile, error)
	ReleasePayloads() error
	CreateDirectory(context.Context, string) (bool, error)
	CreateUniqueDirectory(context.Context, string) (string, bool, error)
	CreateStagingFile(context.Context, string) (update.StagingFile, error)
	ReleaseStagingFiles() error
	Close() error
}

type nativeInstallationPreparation interface {
	nativeInstallationRoot
	open(context.Context, string) error
}

// Owns original ancestor/root handles, a cooperative writer lock and bounded
// transaction directories. It is not SDK authority, a child-process owner, a
// loaded-code attestation or a durable installer commit. Transaction directory
// custody is limited to the fixed stage/pending/archive namespace.
// App/updateMu serializes use; do not call Validate/Close concurrently.
type InstallationRootLease struct {
	native nativeInstallationRoot
	ready  bool
}

func OpenInstallationRoot(ctx context.Context, root string) (owner *InstallationRootLease, err error) {
	err = RetainInstallationRoot(ctx, &owner, root)
	return owner, err
}

func RetainInstallationRoot(ctx context.Context, slot **InstallationRootLease, root string) (err error) {
	if slot == nil || *slot != nil {
		return ErrInstallationRootBusy
	}
	owner := &InstallationRootLease{}
	*slot = owner
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		completed = true
		return ErrInstallationRootUnavailable
	}
	native := newInstallationRoot()
	owner.native = native // original pure native owner before any root/lock acquisition
	if native == nil {
		completed = true
		return ErrInstallationRootUnavailable
	}
	err = native.open(ctx, root)
	owner.ready = err == nil
	completed = true
	return err
}

func (owner *InstallationRootLease) Validate(ctx context.Context, root string) (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return ErrInstallationRootUnavailable
	}
	err = owner.native.Validate(ctx, root)
	completed = true
	return err
}

func (owner *InstallationRootLease) Close() (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrHelperCleanup)
		}
	}()
	if owner != nil && owner.native != nil {
		owner.ready = false // partial close can never be reused for installation
		err = owner.native.Close()
		if err == nil {
			owner.native = nil
		}
	}
	completed = true
	return err
}

func (owner *InstallationRootLease) PinDirectory(ctx context.Context, path string) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrInstallationRootUnavailable
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return ErrInstallationRootUnavailable
	}
	return owner.native.PinDirectory(ctx, path)
}

func (owner *InstallationRootLease) MoveDirectory(ctx context.Context, from, to string) (moved bool, err error) {
	defer func() {
		if recover() != nil {
			err = ErrInstallationRootUnavailable
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return false, ErrInstallationRootUnavailable
	}
	return owner.native.MoveDirectory(ctx, from, to)
}

func (owner *InstallationRootLease) ReleaseDirectory(path string) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrHelperCleanup
		}
	}()
	if owner == nil || owner.native == nil {
		return ErrHelperCleanup
	}
	return owner.native.ReleaseDirectory(path)
}

func (owner *InstallationRootLease) PinPayload(ctx context.Context, path string, allowed []update.Payload) (file update.PayloadFile, err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return nil, ErrInstallationRootUnavailable
	}
	file, err = owner.native.PinPayload(ctx, path, allowed)
	completed = true
	return file, err // native root retains the exact partial owner even if interrupted
}

func (owner *InstallationRootLease) ReleasePayloads() (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrHelperCleanup)
		}
	}()
	if owner == nil || owner.native == nil {
		return ErrHelperCleanup
	}
	err = owner.native.ReleasePayloads()
	completed = true
	return err
}

func (owner *InstallationRootLease) CreateDirectory(ctx context.Context, path string) (created bool, err error) {
	defer func() {
		if recover() != nil {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return false, ErrInstallationRootUnavailable
	}
	return owner.native.CreateDirectory(ctx, path)
}

func (owner *InstallationRootLease) CreateUniqueDirectory(ctx context.Context, prefix string) (path string, created bool, err error) {
	defer func() {
		if recover() != nil {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return "", false, ErrInstallationRootUnavailable
	}
	return owner.native.CreateUniqueDirectory(ctx, prefix)
}

func (owner *InstallationRootLease) CreateStagingFile(ctx context.Context, path string) (file update.StagingFile, err error) {
	defer func() {
		if recover() != nil {
			err = errors.Join(err, ErrInstallationRootUnavailable)
		}
	}()
	if ctx == nil || owner == nil || !owner.ready || owner.native == nil {
		return nil, ErrInstallationRootUnavailable
	}
	return owner.native.CreateStagingFile(ctx, path) // native root retains every partial original output
}

func (owner *InstallationRootLease) ReleaseStagingFiles() (err error) {
	defer func() {
		if recover() != nil {
			err = errors.Join(err, ErrHelperCleanup)
		}
	}()
	if owner == nil || owner.native == nil {
		return ErrHelperCleanup
	}
	return owner.native.ReleaseStagingFiles()
}
