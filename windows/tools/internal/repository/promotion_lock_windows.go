//go:build windows

package repository

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func publisherPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Local drive paths only; no UNC/device namespace or alternate streams.
	volume := filepath.VolumeName(abs)
	if len(volume) != 2 || volume[1] != ':' || strings.Contains(abs[len(volume):], ":") {
		return errors.New("repository: local drive path required")
	}
	for current := abs; ; current = filepath.Dir(current) {
		name, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attr, err := windows.GetFileAttributes(name)
		if err == nil && attr&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("repository: publisher reparse path forbidden")
		}
		if err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

// The stable file is never removed or reclaimed by PID/age. The OS releases
// the byte-range lease on process exit; a competing writer fails immediately.
func publisherWriter(path string, create bool) (func() error, error) {
	if err := publisherPath(path); err != nil {
		return nil, err
	}
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, errors.New("repository: regular writer lock required")
	}
	overlap := &windows.Overlapped{}
	if err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_FAIL_IMMEDIATELY|windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlap); err != nil {
		_ = f.Close()
		return nil, errors.Join(errors.New("repository: publisher writer unavailable"), err)
	}
	return func() error {
		err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlap)
		return errors.Join(err, f.Close())
	}, nil
}
