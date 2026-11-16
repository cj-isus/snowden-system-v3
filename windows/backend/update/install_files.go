package update

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func pairDirectory(path string) (os.FileInfo, error) {
	return pairMetadataSnapshot(path, true) // both sides of SameFile now have handle-derived IDs
}

func pairRoot(root string) (os.FileInfo, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || len(root) > 2048 || strings.HasPrefix(root, `\\`) || strings.HasPrefix(root, "//") {
		return nil, ErrPairRecoveryRequired
	}
	return pairDirectory(root)
}

func pairAbsent(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

func pairRead(path string, limit int64) (raw []byte, finalErr error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	lease, err := openPairMetadata(path, false)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, lease.Close()) }()
	opened, err := lease.owner.file.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != info.Size() {
		return nil, ErrPairRecoveryRequired
	}
	raw, err = io.ReadAll(io.LimitReader(lease.owner.file, limit+1))
	if err != nil || int64(len(raw)) != info.Size() || int64(len(raw)) > limit {
		return nil, ErrPairRecoveryRequired
	}
	if err := lease.Close(); err != nil {
		return nil, err
	}
	after, err := pairMetadataSnapshot(path, false)
	if err != nil || !samePairFile(opened, after) || opened.Size() != after.Size() || opened.ModTime() != after.ModTime() {
		return nil, ErrPairRecoveryRequired
	}
	return raw, nil
}

type pairContextWriter struct {
	ctx context.Context
	dst io.Writer
}

func (w pairContextWriter) Write(raw []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.dst.Write(raw)
}

func (t *PairTransaction) writeNew(ctx context.Context, rel string, write func(io.Writer) error) (finalErr error) {
	if ctx == nil || ctx.Err() != nil {
		return ErrPairRecoveryRequired
	}
	if t.writes == nil || len(t.writes) >= maxPairStagingWrites || t.writes[rel] != nil {
		return ErrPairRecoveryRequired
	}
	if _, err := StagingFileLimit(rel); err != nil {
		return err
	}
	path := filepath.Join(t.dir, rel)
	resource := &pairStagingWrite{}
	t.writes[rel] = resource // retain receipt before acquisition, including partial writes
	var err error
	if t.creationCustody == nil {
		return ErrPairRecoveryRequired
	}
	resource.file, err = t.creationCustody.CreateStagingFile(ctx, path)
	if resource.file != nil {
		identity, identityErr := resource.file.Identity()
		if identityErr == nil && identity.Valid() {
			t.files[rel] = identity // original native creation, also retained for known partial failures
		} else if err == nil {
			err = errors.Join(ErrPairRecoveryRequired, identityErr)
		}
	} else if err == nil {
		err = ErrPairRecoveryRequired
	}
	if err != nil {
		return errors.Join(err, resource.Close()) // original native root also keeps the partial writer
	}
	defer func() { finalErr = errors.Join(finalErr, resource.Close()) }()
	if err := write(pairContextWriter{ctx: ctx, dst: resource.file}); err != nil {
		return err
	}
	if err := resource.file.Sync(); err != nil {
		return err
	}
	if err := resource.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	resource.complete = true
	return nil
}

func (t *PairTransaction) bytes(ctx context.Context, rel string, raw []byte) error {
	return t.writeNew(ctx, rel, func(dst io.Writer) error {
		n, err := dst.Write(raw)
		if err == nil && n != len(raw) {
			err = io.ErrShortWrite
		}
		return err
	})
}

func (t *PairTransaction) identity(ctx context.Context) error {
	if t.custody == nil || t.custody.Validate(ctx, t.root) != nil {
		return ErrPairRecoveryRequired
	}
	root, err := pairRoot(t.root)
	if err != nil || !samePairFile(root, t.rootInfo) {
		return ErrPairRecoveryRequired
	}
	dir, err := pairDirectory(t.dir)
	if err != nil || !samePairFile(dir, t.dirInfo) {
		return ErrPairRecoveryRequired
	}
	for name, original := range t.dirs {
		current, err := pairDirectory(filepath.Join(t.dir, name))
		if err != nil || !samePairFile(current, original) {
			return ErrPairRecoveryRequired
		}
	}
	if t.published {
		pending, err := pairDirectory(filepath.Join(t.root, pairPendingName))
		if err != nil || !samePairFile(pending, t.pendingInfo) {
			return ErrPairRecoveryRequired
		}
	}
	return nil
}

// Only unpublished staging owned by this exact constructor can be discarded.
// Unknown/replaced files or extra entries remain; installed names are untouched.
func (t *PairTransaction) Discard() error {
	if !t.mu.TryLock() {
		return ErrPairBusy
	}
	defer t.mu.Unlock()
	if t.dir == "" {
		return nil
	}
	if t.published || t.intentPending || t.custody == nil {
		return ErrPairRecoveryRequired
	}
	t.prepared = false // discard, including a partial one, cannot later apply
	if err := t.retireWrites(); err != nil {
		return err // never unlink or close directory custody under an uncertain writer
	}
	// Close child pins before removing empty directories. Keep the original
	// root writer lock and private creation IDs through all cleanup operations.
	for _, name := range pairChildDirectories() {
		if err := t.custody.ReleaseDirectory(filepath.Join(t.dir, name)); err != nil {
			return err
		}
	}
	if err := t.custody.ReleaseDirectory(t.dir); err != nil {
		return err
	}
	ctx := context.Background()
	if t.identity(ctx) != nil {
		return ErrPairRecoveryRequired
	}
	var failures error
	for rel, original := range t.files {
		path := filepath.Join(t.dir, rel)
		current, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err == nil {
			current, err = pairCreatedSnapshot(path, original)
		}
		if err != nil || !current.Mode().IsRegular() {
			failures = errors.Join(failures, ErrPairRecoveryRequired)
			continue
		}
		failures = errors.Join(failures, os.Remove(path))
	}
	if failures != nil {
		return failures
	}
	for _, name := range pairChildDirectories() {
		if _, exists := t.dirs[name]; exists {
			if err := os.Remove(filepath.Join(t.dir, name)); err != nil {
				return err
			}
			delete(t.dirs, name)
		}
	}
	if err := os.Remove(t.dir); err != nil {
		return err
	}
	t.dir = ""
	return nil
}
