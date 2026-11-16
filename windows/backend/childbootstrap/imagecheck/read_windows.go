//go:build windows

package imagecheck

import (
	"context"
	"errors"
	"io"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// ReadAt borrows a caller-owned original synchronous file handle. Caller must
// serialize its shared file pointer and Close, and independently validate the
// image/path/identity/ACL. This function neither acquires nor closes a handle.
// The caller supplies its existing per-purpose ceiling and original source hook.
func ReadAt(ctx context.Context, handle uintptr, size, ceiling int64, target []byte, offset int64, scope func(context.Context) error) (int, error) {
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return 0, err
	}
	if ceiling < 1 || ceiling > MaxBytes || !nativelease.ValidHandleValue(windows.Handle(handle)) {
		return 0, ErrImage
	}
	if offset < 0 || size < 0 || size > ceiling || offset > size {
		return 0, io.EOF
	}
	if len(target) == 0 {
		return 0, nil
	}
	if offset == size {
		return 0, io.EOF
	}
	if position, err := windows.Seek(windows.Handle(handle), offset, io.SeekStart); err != nil || position != offset {
		return 0, ErrImage
	}
	limit := min(int64(len(target)), size-offset)
	read := 0
	for int64(read) < limit {
		if err := childbootstrap.CheckScope(ctx, scope); err != nil {
			return read, err
		}
		end := min(limit, int64(read)+(32<<10))
		var count uint32
		err := windows.ReadFile(windows.Handle(handle), target[read:end], &count, nil)
		if int64(count) > end-int64(read) {
			return read, ErrImage
		}
		read += int(count)
		if errors.Is(err, windows.ERROR_HANDLE_EOF) || count == 0 && err == nil {
			return read, io.EOF
		}
		if err != nil {
			return read, ErrImage
		}
	}
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return read, err
	}
	if read != len(target) {
		return read, io.EOF
	}
	return read, nil
}
