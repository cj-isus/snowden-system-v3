package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/update"
)

const maxApplicationImageBytes int64 = update.MaxPayloadSize

// Same original native image owner as the helper: ancestor/parent/helper
// handles stay held through candidate acceptance. This does not authenticate
// code against journal hashes; the App must first provide its actual SDK proof.
type InstalledPairLease struct {
	image nativeImageLease
	ready bool
}

func OpenInstalledPair(ctx context.Context, appSHA string, appSize int64, helperSHA string, helperSize int64) (owner *InstalledPairLease, err error) {
	err = RetainInstalledPair(ctx, &owner, appSHA, appSize, helperSHA, helperSize)
	return owner, err
}

// RetainInstalledPair publishes the original receipt in the caller's retained
// slot before native preparation. A nonnil slot is never replaced, even on error.
func RetainInstalledPair(ctx context.Context, slot **InstalledPairLease, appSHA string, appSize int64, helperSHA string, helperSize int64) (err error) {
	if slot == nil || *slot != nil {
		return ErrHelperCleanup
	}
	owner := &InstalledPairLease{}
	*slot = owner
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = ErrNativeImageUnavailable
		}
	}()
	err = owner.prepare(ctx, appSHA, appSize, helperSHA, helperSize)
	completed = true
	return err
}

func (owner *InstalledPairLease) prepare(ctx context.Context, appSHA string, appSize int64, helperSHA string, helperSize int64) error {
	if owner == nil || owner.image != nil {
		return ErrHelperCleanup
	}
	image := newNativeImage()
	owner.image = image // pure owner is retained before any native call
	if image == nil {
		return ErrNativeImageUnavailable
	}
	if err := image.prepareInstalled(ctx, helperSHA); err != nil {
		return err
	}
	if err := pinApplicationImage(ctx, owner.image, appSHA, appSize, helperSize); err != nil {
		return err
	}
	owner.ready = true
	return nil
}

func (owner *InstalledPairLease) Validate(ctx context.Context) error {
	if owner == nil || !owner.ready || owner.image == nil {
		return ErrNativeImageUnavailable
	}
	return owner.image.Validate(ctx)
}

func (owner *InstalledPairLease) Close() (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrHelperCleanup)
		}
	}()
	if owner != nil && owner.image != nil {
		err = owner.image.Close()
		if err == nil {
			owner.image, owner.ready = nil, false
		}
	}
	completed = true
	return err
}
