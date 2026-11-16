package filetxn

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrBoundedWrite    = errors.New("filetxn: bounded public file write failed")
	ErrWriteRetirement = errors.New("filetxn: public file writer outcome or retirement unconfirmed")
	ErrWriteBusy       = errors.New("filetxn: selected output writer busy")
)

// WritePublication describes name publication only, never trust, directory
// durability or successful retirement. Even Published with an error requires
// explicit reporting: the file may exist despite a later flush/Close/cancel error.
type WritePublication struct {
	Published        bool
	MayHavePublished bool
}

type selectedOutputGuardKey struct{}

// WithSelectedOutputGuard adds a negative publication fence, never filesystem
// authority. The callback must read values only: no App/journal locks, I/O or
// cleanup. An existing guard cannot be replaced to revive a stale request.
func WithSelectedOutputGuard(ctx context.Context, check func() error) context.Context {
	if ctx == nil {
		return nil
	}
	if ctx.Value(selectedOutputGuardKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, selectedOutputGuardKey{}, check)
}

func selectedOutputContextError(ctx context.Context) (finalErr error) {
	if ctx == nil {
		return nil
	} // ordinary trusted-store writers have no selected context
	defer func() {
		if recover() != nil {
			finalErr = ErrBoundedWrite
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if value := ctx.Value(selectedOutputGuardKey{}); value != nil {
		check, ok := value.(func() error)
		if !ok || check == nil {
			return ErrBoundedWrite
		}
		return check()
	}
	return nil
}

type boundedWriteFile interface {
	Publish([]byte) error
	Close() error
	publication() WritePublication
}

// BoundedWriteSlot retains one original operation, including partial acquisition
// and uncertain publication/Close. Keep it for the storage domain's lifetime;
// replacing a poisoned slot would discard its original resource receipt.
// Callers still own journal/source serialization and the input bytes until Write
// returns. This is neither a journal nor a trust verifier/interprocess lock.
type BoundedWriteSlot struct {
	allowEmpty     bool // journal backups/new files may legitimately be empty
	mu             sync.Mutex
	maxBytes       int64
	file           boundedWriteFile
	retirementErr  error
	selectedOutput bool
}

func NewBoundedWriteSlot(maxBytes int64) *BoundedWriteSlot {
	return &BoundedWriteSlot{maxBytes: maxBytes}
}

// NewSelectedOutputSlot is for already encrypted user-selected output only.
// It retains the same original writer and checked publication/Close, without
// adopting trusted-store ACL requirements or creating missing parent directories.
// Keep it for the output domain's lifetime; never use it for trusted local state.
func NewSelectedOutputSlot(maxBytes int64) *BoundedWriteSlot {
	return &BoundedWriteSlot{maxBytes: maxBytes, selectedOutput: true}
}

// WriteSelected never blocks behind another selected-output operation. Context
// gates acquisition/work/publication and the returned success, not synchronous
// OS cancellation. After publication, finish flush/retirement before returning
// cancellation, preserving confirmed/possible publication alongside the error.
func (slot *BoundedWriteSlot) WriteSelected(ctx context.Context, path string, raw []byte) (WritePublication, error) {
	if ctx == nil || slot == nil || !slot.selectedOutput {
		return WritePublication{}, ErrBoundedWrite
	}
	return slot.writeOwned(path, raw, false, "", ctx)
}

// Check waits for an ordinary active writer and rejects retained uncertainty.
// It acquires no journal/caller lock and invokes no callbacks. Waiting I/O has
// no hard deadline; callers must not hold this slot recursively.
func (slot *BoundedWriteSlot) Check() error {
	if slot == nil || slot.maxBytes < 1 || slot.maxBytes > maxFileSize {
		return ErrBoundedWrite
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.check()
}

func (slot *BoundedWriteSlot) check() error {
	if slot.file != nil || slot.retirementErr != nil {
		return errors.Join(ErrWriteRetirement, slot.retirementErr)
	}
	return nil
}

// Write reports success only after the full publication and original Close.
// A failed call can have changed the canonical name; it grants no rollback,
// initial-state fallback or permission to bypass a retained writer.
func (slot *BoundedWriteSlot) Write(path string, raw []byte) (finalErr error) {
	return slot.write(path, raw, false)
}

// WriteNew uses the same original owner but never overwrites an occupied name.
func (slot *BoundedWriteSlot) WriteNew(path string, raw []byte) error {
	return slot.write(path, raw, true)
}

func (slot *BoundedWriteSlot) write(path string, raw []byte, newOnly bool) (finalErr error) {
	return slot.writeInDirectory(path, raw, newOnly, "")
}

// directory is private to the journal's original directory owner. It permits
// the temporary writer to share that owner's DELETE access without releasing
// its deny-delete handle. Ordinary metadata writers pass no exception.
func (slot *BoundedWriteSlot) writeInDirectory(path string, raw []byte, newOnly bool, directory string) (finalErr error) {
	_, err := slot.writeOwned(path, raw, newOnly, directory, nil)
	return err
}

func (slot *BoundedWriteSlot) writeOwned(path string, raw []byte, newOnly bool, directory string, selectedContext context.Context) (publication WritePublication, finalErr error) {
	if slot == nil || slot.maxBytes < 1 || slot.maxBytes > maxFileSize || len(raw) == 0 && !slot.allowEmpty || int64(len(raw)) > slot.maxBytes {
		return publication, ErrBoundedWrite
	}
	if slot.selectedOutput != (selectedContext != nil) || slot.selectedOutput && (directory != "" || newOnly) {
		return publication, ErrBoundedWrite // explicit mode, no journal exception/fallback
	}
	if selectedContext != nil {
		if err := selectedOutputContextError(selectedContext); err != nil {
			return publication, err
		}
		if !slot.mu.TryLock() {
			return publication, ErrWriteBusy
		}
	} else {
		slot.mu.Lock()
	}
	defer slot.mu.Unlock()
	if err := slot.check(); err != nil {
		return publication, err
	}
	defer func() {
		if recover() != nil {
			finalErr = errors.Join(finalErr, ErrBoundedWrite)
		}
		if slot.file != nil {
			publication = slot.file.publication() // pure facts before cleanup can change its own outcome
		}
		if selectedContext != nil && finalErr == nil && !publication.Published {
			finalErr = ErrBoundedWrite
		}
		if err := slot.close(); err != nil {
			finalErr = errors.Join(finalErr, err)
		}
		if selectedContext != nil {
			finalErr = errors.Join(finalErr, selectedOutputContextError(selectedContext))
		}
	}()
	// Construction allocates bookkeeping only. Reserve it before acquisition.
	slot.file = newBoundedWriteFile(path, slot.maxBytes, newOnly, directory, selectedContext)
	return publication, slot.file.Publish(raw)
}

func (slot *BoundedWriteSlot) close() (finalErr error) {
	if slot.file == nil {
		return nil
	}
	slot.retirementErr = ErrWriteRetirement // sticky if Close panics or cannot confirm
	defer func() {
		if recover() != nil {
			finalErr = slot.retirementErr
		}
	}()
	if err := slot.file.Close(); err != nil {
		slot.retirementErr = errors.Join(ErrWriteRetirement, err)
		return slot.retirementErr
	}
	slot.file, slot.retirementErr = nil, nil
	return nil
}
