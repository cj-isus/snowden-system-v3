//go:build !windows

package filetxn

import (
	"errors"
	"os"
)

// Portable compatibility: original os.File and checked Close, with a symlink/
// identity check around Open. This does not claim Windows native acquisition,
// deny-write sharing or a hard deadline for raced special files/filesystems.
func (slot *BoundedReadSlot) open(path string, limit int64, directory bool) error {
	before, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if directory && !before.IsDir() || !directory && (!before.Mode().IsRegular() || before.Size() < 0) {
		return ErrBoundedRead
	}
	if !directory && before.Size() > limit {
		return errors.Join(ErrBoundedRead, ErrReadTooLarge)
	}
	file, err := os.Open(path)
	if file != nil {
		slot.file = file
	}
	if err != nil {
		return err
	}
	opened, err := file.Stat()
	if err != nil || opened.IsDir() != directory || !directory && !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return ErrBoundedRead
	}
	return nil
}

func sameBoundedReadFile(a, b os.FileInfo) bool { return os.SameFile(a, b) }
