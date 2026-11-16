package filetxn

import (
	"errors"
	"io"
	"os"
	"sync"
)

var (
	ErrBoundedRead  = errors.New("filetxn: bounded public file read failed")
	ErrReadBusy     = errors.New("filetxn: public file reader busy")
	ErrReadClose    = errors.New("filetxn: public file reader retirement unconfirmed")
	ErrReadTooLarge = errors.New("filetxn: public file exceeds read bound")
)

type boundedReadFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

// BoundedReadSlot owns at most one original reader, including failed acquisition
// and uncertain Close. Keep the slot for its domain's lifetime; do not replace a
// poisoned slot to bypass retirement. It carries no metadata or trust authority.
// Separate consumers can use separate slots without sharing their failure state.
type BoundedReadSlot struct {
	mu            sync.Mutex
	maxBytes      int64
	file          boundedReadFile
	acquiring     bool
	closeErr      error
	selectedInput bool
}

// NewBoundedReadSlot allocates bookkeeping only. Read enforces a positive limit
// no larger than this ceiling or filetxn's existing 16 MiB in-memory file bound.
func NewBoundedReadSlot(maxBytes int64) *BoundedReadSlot {
	return &BoundedReadSlot{maxBytes: maxBytes}
}

// NewSelectedInputSlot owns untrusted regular-file input selected by the user.
// On Windows it checks the original handle/path/identity and denies write and
// delete sharing, without requiring trusted-store ownership, DACL or a unique
// hardlink. Bytes still require the consumer's independent authentication. Keep
// this slot for the input domain's lifetime, including after uncertain Close;
// never use it for metadata, vault, journal, SDK cache or installed code reads.
func NewSelectedInputSlot(maxBytes int64) *BoundedReadSlot {
	return &BoundedReadSlot{maxBytes: maxBytes, selectedInput: true}
}

func (slot *BoundedReadSlot) Read(path string, limit int64) ([]byte, error) {
	raw, _, err := slot.ReadSnapshot(path, limit)
	return raw, err
}

// ReadSnapshot pairs bounded bytes with immutable metadata from the same
// original reader, after its checked Close. On Windows SameWindowsFile compares
// these snapshots without reopening a path. A snapshot holds no native custody
// and supplies neither trust nor atomic compare-and-swap/rollback authority.
func (slot *BoundedReadSlot) ReadSnapshot(path string, limit int64) (raw []byte, info os.FileInfo, finalErr error) {
	if slot == nil || slot.maxBytes < 1 || slot.maxBytes > maxFileSize || limit < 1 || limit > slot.maxBytes {
		return nil, nil, ErrBoundedRead
	}
	if !slot.mu.TryLock() {
		return nil, nil, ErrReadBusy
	}
	defer slot.mu.Unlock()
	if slot.file != nil || slot.closeErr != nil || slot.acquiring {
		return nil, nil, ErrReadClose // never retry Close or replace a retained resource
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			finalErr = ErrBoundedRead
		}
		if err := slot.close(); err != nil {
			// Cleanup uncertainty must not remain classifiable as not-exist,
			// even if Open originally returned a missing-path error.
			finalErr = err
		}
		if finalErr != nil {
			raw, info = nil, nil
		}
	}()
	raw, info, finalErr = slot.readSnapshotLocked(path, limit)
	returned = true // ordinary missing/errors remain distinct from abnormal unwind
	return raw, info, finalErr
}

// Only ReadSnapshot calls this while holding the original slot mutex. That
// caller owns checked retirement for every ordinary or abnormal return.
func (slot *BoundedReadSlot) readSnapshotLocked(path string, limit int64) ([]byte, os.FileInfo, error) {
	slot.acquiring = true
	err := slot.open(path, limit+1, false)
	slot.acquiring = false
	if err != nil {
		return nil, nil, err
	}
	before, err := slot.file.Stat()
	if errors.Is(err, ErrReadTooLarge) {
		return nil, nil, errors.Join(ErrBoundedRead, ErrReadTooLarge)
	}
	if err != nil || before == nil || !before.Mode().IsRegular() || before.Size() < 0 {
		return nil, nil, ErrBoundedRead
	}
	if before.Size() > limit {
		return nil, nil, errors.Join(ErrBoundedRead, ErrReadTooLarge)
	}
	raw, err := io.ReadAll(io.LimitReader(slot.file, limit+1))
	if err != nil {
		return nil, nil, ErrBoundedRead
	}
	if int64(len(raw)) > limit {
		return nil, nil, errors.Join(ErrBoundedRead, ErrReadTooLarge)
	}
	if int64(len(raw)) != before.Size() {
		return nil, nil, ErrBoundedRead
	}
	after, err := slot.file.Stat()
	if err != nil || after == nil || !after.Mode().IsRegular() || !sameBoundedReadFile(before, after) ||
		after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, nil, ErrBoundedRead
	}
	return raw, after, nil // values escape only after the caller's original checked Close
}

// Retired reports resource ownership only, not metadata validity or durability.
// A busy reader is not retired. This is used before deleting its SDK workspace.
func (slot *BoundedReadSlot) Retired() bool {
	if slot == nil || !slot.mu.TryLock() {
		return false
	}
	defer slot.mu.Unlock()
	return slot.file == nil && slot.closeErr == nil && !slot.acquiring
}

func (slot *BoundedReadSlot) close() (finalErr error) {
	if slot.closeErr != nil {
		return slot.closeErr
	}
	if slot.file == nil {
		if slot.acquiring {
			slot.closeErr = ErrReadClose // a panic did not return an original owner
			return slot.closeErr
		}
		return nil
	}
	slot.closeErr = ErrReadClose
	defer func() {
		if recover() != nil {
			finalErr = ErrReadClose
		}
	}()
	if err := slot.file.Close(); err != nil {
		return ErrReadClose
	}
	slot.file, slot.closeErr, slot.acquiring = nil, nil, false
	return nil
}
