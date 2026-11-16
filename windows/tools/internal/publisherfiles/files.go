// Package publisherfiles provides bounded reads and the existing publisher
// new-output commit boundary. It does not discover files or select key stores.
package publisherfiles

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func Read(path string, limit int64) ([]byte, error) {
	data, _, err := ReadSnapshot(path, limit, false)
	return data, err
}

// ReadSnapshot returns the opened regular-file identity for an explicit later
// path fence. Empty orphan files are allowed only by the maintenance caller;
// the ordinary publisher Read contract still requires nonempty input.
func ReadSnapshot(path string, limit int64, allowEmpty bool) ([]byte, os.FileInfo, error) {
	if limit <= 0 {
		return nil, nil, errors.New("publisher: invalid read bound")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || !allowEmpty && info.Size() == 0 || info.Size() > limit {
		return nil, nil, errors.New("publisher: bounded regular file required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() > limit {
		return nil, nil, errors.New("publisher: file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(opened, after) || !allowEmpty && len(data) == 0 || int64(len(data)) > limit || after.Size() != int64(len(data)) {
		return nil, nil, errors.New("publisher: file changed or outside read bound")
	}
	if err = f.Close(); err != nil {
		return nil, nil, err
	}
	return data, after, nil
}

// PublishNew reuses metadatatool's hard-link commit: publish a fully written
// file only if absent. Parent must exist. Unsupported filesystems fail;
// Windows ACL/reparse/interprocess and directory durability remain gates.
func PublishNew(path string, data []byte) error {
	return PublishNewChecked(path, data, nil)
}

// PublishNewChecked repeats a caller's value-only validity gate after the
// staged bytes have been synced/closed, immediately before new-only linking.
// This is not a Windows directory durability or interprocess CAS guarantee.
func PublishNewChecked(path string, data []byte, check func() error) error {
	return PublishNewStreamChecked(path, func(w io.Writer) error {
		n, err := w.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		return err
	}, check)
}

// Stream large explicit backups through the same synced new-only link boundary.
// The producer bounds its output and keeps authority/freshness in check.
func PublishNewStreamChecked(path string, write func(io.Writer) error, check func() error) error {
	if write == nil {
		return errors.New("publisher: stream producer required")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".publisher-stage-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	defer f.Close()
	if err = write(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}
	return os.Link(temp, path)
}
