//go:build windows

package core

import (
	"context"
	"sync"

	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"golang.org/x/sys/windows"
)

// ReadAt over an original synchronous handle. No os.File/finalizer takes
// ownership away from the lane; one mutex preserves ReaderAt's concurrency
// contract around the shared file pointer. Each syscall payload is bounded.
type nativeImageReader struct {
	mu     sync.Mutex
	ctx    context.Context
	handle windows.Handle
	size   int64
	limit  int64 // zero retains the native-helper32MiB ceiling
}

func (reader *nativeImageReader) ReadAt(target []byte, offset int64) (int, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if contextError(reader.ctx) != nil {
		return 0, contextError(reader.ctx)
	}
	ceiling := maxNativeImageBytes
	if reader.limit != 0 {
		if reader.limit < 1 || reader.limit > maxApplicationImageBytes {
			return 0, ErrNativeImageUnavailable
		}
		ceiling = reader.limit
	}
	read, err := imagecheck.ReadAt(reader.ctx, uintptr(reader.handle), reader.size, ceiling, target, offset, contextError)
	if err == imagecheck.ErrImage {
		return read, ErrNativeImageUnavailable
	}
	return read, err
}
