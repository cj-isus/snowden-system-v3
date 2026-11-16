//go:build !windows

package filetxn

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Portable compatibility retains original os.File objects and checks identity,
// complete writes, Sync and Close. Path-based create/rename/remove still have
// races; this does not implement Windows relative-handle custody or its ACLs.
type portableBoundedWrite struct {
	selectedContext                                       context.Context
	newOnly                                               bool
	path, temporary                                       string
	limit                                                 int64
	file, directory                                       *os.File
	identity, directoryID                                 os.FileInfo
	attempted, acquiring, ioPending, published, uncertain bool
	closeAttempted                                        bool
	closeErr                                              error
}

func newBoundedWriteFile(path string, limit int64, newOnly bool, _ string, selectedContext context.Context) boundedWriteFile {
	return &portableBoundedWrite{path: path, limit: limit, newOnly: newOnly, selectedContext: selectedContext}
}

func (writer *portableBoundedWrite) publication() WritePublication {
	return WritePublication{Published: writer.published, MayHavePublished: writer.published || writer.uncertain}
}

func (writer *portableBoundedWrite) contextError() error {
	return selectedOutputContextError(writer.selectedContext)
}

func (writer *portableBoundedWrite) validateDirectory() error {
	current, err := os.Lstat(filepath.Dir(writer.path))
	if err != nil || current == nil || !current.IsDir() || writer.directoryID == nil || !os.SameFile(current, writer.directoryID) {
		return errors.Join(ErrBoundedWrite, err)
	}
	return nil
}

func (writer *portableBoundedWrite) validateFile(path string, size int64) error {
	current, err := os.Lstat(path)
	if err != nil || current == nil || !current.Mode().IsRegular() || writer.identity == nil || !os.SameFile(current, writer.identity) || size >= 0 && current.Size() != size {
		return errors.Join(ErrBoundedWrite, err)
	}
	return nil
}

func (writer *portableBoundedWrite) Publish(raw []byte) error {
	if writer.attempted || writer.closeAttempted || writer.path == "" || int64(len(raw)) > writer.limit {
		return ErrBoundedWrite
	}
	writer.attempted = true
	if err := writer.contextError(); err != nil {
		return err
	}
	path, err := filepath.Abs(writer.path)
	if err != nil {
		return err
	}
	writer.path = path
	directory := filepath.Dir(path)
	if writer.selectedContext == nil {
		if err := os.MkdirAll(directory, 0700); err != nil {
			return err
		}
	}
	before, err := os.Lstat(directory)
	if err != nil || !before.IsDir() {
		return errors.Join(ErrBoundedWrite, err)
	}
	writer.acquiring = true
	writer.directory, err = os.Open(directory)
	writer.acquiring = false
	if err != nil {
		return err
	}
	writer.directoryID, err = writer.directory.Stat()
	if err != nil || !os.SameFile(before, writer.directoryID) {
		return errors.Join(ErrBoundedWrite, err)
	}
	writer.acquiring = true
	writer.file, err = os.CreateTemp(directory, ".stored-write-")
	writer.acquiring = false
	if writer.file != nil {
		writer.temporary = writer.file.Name()
	}
	if err != nil {
		return err
	}
	writer.identity, err = writer.file.Stat()
	if err != nil || !writer.identity.Mode().IsRegular() || writer.identity.Size() != 0 {
		return errors.Join(ErrBoundedWrite, err)
	}
	for offset := 0; offset < len(raw); {
		if err := writer.contextError(); err != nil {
			return err
		}
		end := min(offset+32*1024, len(raw))
		writer.ioPending = true
		done, err := writer.file.Write(raw[offset:end])
		writer.ioPending = false
		if done != end-offset {
			return errors.Join(ErrBoundedWrite, io.ErrShortWrite, err)
		}
		if err != nil {
			return errors.Join(ErrBoundedWrite, err)
		}
		offset = end
	}
	if err := writer.contextError(); err != nil {
		return err
	}
	writer.ioPending = true
	err = writer.file.Sync()
	writer.ioPending = false
	if err != nil {
		return err
	}
	if err := writer.validateFile(writer.temporary, int64(len(raw))); err != nil {
		return err
	}
	if err := writer.validateDirectory(); err != nil {
		return err
	}
	if err := writer.contextError(); err != nil {
		return err
	}
	writer.uncertain = true
	if writer.newOnly {
		err = os.Link(writer.temporary, writer.path) // atomic new-name operation; no overwrite fallback
	} else {
		err = os.Rename(writer.temporary, writer.path)
	}
	if err != nil {
		// Link keeps the private source even after publication. Its presence
		// alone cannot prove a failed new-name call left the target absent.
		unchanged := !writer.newOnly || errors.Is(err, os.ErrExist)
		if writer.newOnly && !unchanged {
			_, targetErr := os.Lstat(writer.path)
			unchanged = errors.Is(targetErr, os.ErrNotExist)
		}
		if unchanged && writer.validateFile(writer.temporary, int64(len(raw))) == nil && writer.validateDirectory() == nil {
			writer.uncertain = false
		}
		return err
	}
	writer.published = true
	if err := writer.validateFile(writer.path, int64(len(raw))); err != nil {
		return err
	}
	if err := writer.validateDirectory(); err != nil {
		return err
	}
	if writer.newOnly {
		if err := writer.validateFile(writer.temporary, int64(len(raw))); err != nil {
			return err
		}
		if err := os.Remove(writer.temporary); err != nil {
			return err
		}
	}
	writer.ioPending = true
	err = errors.Join(writer.file.Sync(), writer.directory.Sync())
	writer.ioPending = false
	if err != nil {
		return err
	}
	writer.uncertain = false
	return nil
}

func (writer *portableBoundedWrite) Close() (finalErr error) {
	if writer.closeAttempted {
		return writer.closeErr
	}
	writer.closeAttempted, writer.closeErr = true, ErrWriteRetirement
	defer func() {
		if recover() != nil {
			finalErr = ErrWriteRetirement
		}
		writer.closeErr = finalErr
	}()
	if writer.acquiring || writer.ioPending || writer.uncertain {
		return ErrWriteRetirement
	}
	if writer.file != nil && !writer.published {
		if err := writer.validateFile(writer.temporary, -1); err != nil {
			return err
		}
		if err := writer.validateDirectory(); err != nil {
			return err
		}
		writer.uncertain = true
		if err := os.Remove(writer.temporary); err != nil {
			return err
		}
		writer.uncertain = false
	}
	if writer.file != nil {
		if err := writer.file.Close(); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
	}
	if writer.directory != nil {
		if err := writer.directory.Close(); err != nil {
			return errors.Join(ErrWriteRetirement, err)
		}
	}
	return nil
}
