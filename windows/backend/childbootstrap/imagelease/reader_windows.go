//go:build windows

package imagelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"golang.org/x/sys/windows"
)

// Never exported: callers hold Pair.mu through the complete synchronous read.
// The reader starts no goroutine, borrows one original handle and never closes it.
type imageReader struct {
	ctx           context.Context
	handle        windows.Handle
	size, ceiling int64
	scope         func(context.Context) error
}

func (reader *imageReader) ReadAt(target []byte, offset int64) (int, error) {
	return imagecheck.ReadAt(reader.ctx, uintptr(reader.handle), reader.size, reader.ceiling, target, offset, reader.scope)
}

func (pair *Pair) hash(ctx context.Context, file *imageFile, ceiling int64, check func(context.Context) error) ([32]byte, error) {
	var digest [32]byte
	size := uint64(file.identity.FileSizeHigh)<<32 | uint64(file.identity.FileSizeLow)
	if size < 1 || ceiling < 1 || ceiling > MaxApplicationBytes || size > uint64(ceiling) {
		return digest, ErrPair
	}
	reader := &imageReader{ctx: ctx, handle: file.lease.HandleValue(), size: int64(size), ceiling: ceiling,
		scope: func(ctx context.Context) error { return pair.scope(ctx, check) }}
	hash := sha256.New()
	n, err := io.CopyBuffer(hash, io.NewSectionReader(reader, 0, int64(size)), make([]byte, 32<<10))
	if err != nil || n != int64(size) {
		return digest, errors.Join(ErrPair, err)
	}
	copy(digest[:], hash.Sum(nil))
	return digest, pair.scope(ctx, check)
}

// PinApplication consumes an independently authorized digest/size from the
// caller. It hashes held disk bytes; it does not verify signatures or SDK facts.
// Optional inspect follows WithPaths' synchronous, nonreentrant borrowing rules.
func (pair *Pair) PinApplication(ctx context.Context, pin [32]byte, size, helperSize int64, check func(context.Context) error, inspect func(string, string) error) (finalErr error) {
	if pair == nil {
		return ErrPair
	}
	if !pair.mu.TryLock() {
		return errors.Join(ErrPair, ErrBusy)
	}
	completed := false
	defer pair.finish(&completed, &finalErr)
	finalErr = pair.pinApplication(ctx, pin, size, helperSize, check, inspect)
	completed = true
	return
}

func (pair *Pair) pinApplication(ctx context.Context, pin [32]byte, size, helperSize int64, check func(context.Context) error, inspect func(string, string) error) error {
	if !pair.ready || size < 1 || size > MaxApplicationBytes || helperSize < 1 || helperSize > MaxHelperBytes {
		return ErrPair
	}
	if err := pair.validate(ctx, check); err != nil {
		return err
	}
	if err := pair.inspectPaths(ctx, check, inspect); err != nil {
		return err
	}
	app, helper := &pair.files[pair.count-2], &pair.files[pair.count-1]
	if uint64(app.identity.FileSizeHigh)<<32|uint64(app.identity.FileSizeLow) != uint64(size) ||
		uint64(helper.identity.FileSizeHigh)<<32|uint64(helper.identity.FileSizeLow) != uint64(helperSize) {
		return ErrPair
	}
	digest, err := pair.hash(ctx, app, MaxApplicationBytes, check)
	if err != nil {
		return err
	}
	if !bytes.Equal(digest[:], pin[:]) {
		return ErrPair
	}
	if err := pair.validate(ctx, check); err != nil {
		return err
	}
	if err := pair.inspectPaths(ctx, check, inspect); err != nil {
		return err
	}
	return pair.scope(ctx, check)
}

// MatchProcess borrows the caller's original process handle. application fixes
// which of the two held paths is expected; Job/parent identity stay with caller.
func (pair *Pair) MatchProcess(ctx context.Context, process windows.Handle, application bool, check func(context.Context) error) (finalErr error) {
	if pair == nil {
		return ErrPair
	}
	if !pair.mu.TryLock() {
		return errors.Join(ErrPair, ErrBusy)
	}
	completed := false
	defer pair.finish(&completed, &finalErr)
	if !pair.ready {
		completed = true
		return ErrPair
	}
	if finalErr = pair.validate(ctx, check); finalErr == nil {
		expected := pair.helper
		if application {
			expected = pair.application
		}
		finalErr = imagecheck.ProcessPathMatches(ctx, process, expected, check)
	}
	completed = true
	return
}
