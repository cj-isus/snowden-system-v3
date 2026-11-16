//go:build !windows

package delivery

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Compatibility adapter for the existing portable publisher. Native original
// acquisition guarantees belong to workspace_windows; stdlib I/O here retains
// its own handles and uses bounded inventory/checked Close, not RemoveAll.
type portableWorkspace struct {
	root, metadata, targets string
	dirs                    map[string]os.FileInfo
	file                    *os.File
	closeErr                error
}

func newWorkspacePlatform() workspacePlatform {
	return &portableWorkspace{dirs: make(map[string]os.FileInfo)}
}

func (owner *portableWorkspace) Paths() (string, string) { return owner.metadata, owner.targets }

func (owner *portableWorkspace) Prepare(ctx context.Context, cache map[string][]byte) error {
	if owner.root != "" || len(cache) > 5 || contextError(ctx) != nil {
		return ErrTrust
	}
	var err error
	owner.root, err = os.MkdirTemp("", "snowden-tuf-")
	if err != nil {
		return err
	}
	if err := owner.capture(owner.root); err != nil {
		return err
	}
	owner.metadata, owner.targets = filepath.Join(owner.root, "metadata"), filepath.Join(owner.root, "targets")
	for _, path := range []string{owner.metadata, owner.targets} {
		if err := contextError(ctx); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		if err := owner.capture(path); err != nil {
			return err
		}
	}
	for name, raw := range cache {
		if err := contextError(ctx); err != nil {
			return err
		}
		if !cacheName(name) || len(raw) == 0 || len(raw) > MaxMetadataBytes {
			return ErrTrust
		}
		file, err := os.OpenFile(filepath.Join(owner.metadata, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if file != nil {
			owner.file = file
		}
		if err != nil {
			return err
		}
		if n, err := file.Write(raw); err != nil || n != len(raw) {
			return errors.Join(ErrTrust, io.ErrShortWrite, err)
		}
		if err := file.Sync(); err != nil {
			return err
		}
		if err := owner.closeFile(); err != nil {
			return err
		}
	}
	return nil
}

func (owner *portableWorkspace) capture(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeType != os.ModeDir {
		return ErrTrust
	}
	owner.dirs[path] = info
	return nil
}

func (owner *portableWorkspace) directory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeType != os.ModeDir || !os.SameFile(info, owner.dirs[path]) {
		return ErrTrust
	}
	return nil
}

func (owner *portableWorkspace) closeFile() (finalErr error) {
	if owner.closeErr != nil {
		return owner.closeErr
	}
	if owner.file == nil {
		return nil
	}
	owner.closeErr = ErrTrust
	defer func() {
		if recover() != nil {
			finalErr = ErrTrust
		}
	}()
	if err := owner.file.Close(); err != nil {
		return ErrTrust
	}
	owner.file, owner.closeErr = nil, nil
	return nil
}

func (owner *portableWorkspace) entries(path string) (entries []os.FileInfo, finalErr error) {
	if err := owner.directory(path); err != nil || owner.file != nil {
		return nil, ErrTrust
	}
	file, err := os.Open(path)
	if file != nil {
		owner.file = file
	}
	defer func() {
		if err := owner.closeFile(); err != nil {
			entries, finalErr = nil, err
		}
	}()
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(opened, owner.dirs[path]) {
		return nil, ErrTrust
	}
	entries, err = file.Readdir(maxWorkspaceEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > maxWorkspaceEntries {
		return nil, ErrTrust
	}
	return entries, owner.directory(path)
}

func (owner *portableWorkspace) Close() error {
	if err := owner.closeFile(); err != nil {
		return err
	}
	if owner.root == "" {
		return nil
	}
	rootEntries, err := owner.entries(owner.root)
	if err != nil {
		return err
	}
	for _, entry := range rootEntries {
		if entry.Mode()&os.ModeType != os.ModeDir || entry.Name() != "metadata" && entry.Name() != "targets" {
			return ErrTrust
		}
	}
	for _, dir := range []string{owner.metadata, owner.targets} {
		if owner.dirs[dir] == nil {
			continue // unknown/uncaptured children are not removed; root removal will fail
		}
		entries, err := owner.entries(dir)
		if err != nil || dir == owner.targets && len(entries) != 0 {
			return ErrTrust
		}
		for _, entry := range entries {
			if !entry.Mode().IsRegular() || entry.Size() < 0 || entry.Size() > MaxMetadataBytes || !workspaceMetadataName(entry.Name()) {
				return ErrTrust
			}
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			current, err := os.Lstat(path)
			if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, entry) || owner.directory(dir) != nil || owner.directory(owner.root) != nil {
				return ErrTrust
			}
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		if err := owner.directory(dir); err != nil {
			return err
		}
		if err := os.Remove(dir); err != nil {
			return err
		}
	}
	if err := owner.directory(owner.root); err != nil {
		return err
	}
	return os.Remove(owner.root)
}
