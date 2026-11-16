package delivery

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
)

// These identifiers are never filesystem paths passed to the OS. The local
// updater requires explicit Storage, so reverting to the upstream constructor
// cannot silently turn these into relative scratch directories.
const (
	sdkMetadataNamespace = "snowden-sdk-metadata"
	sdkTargetNamespace   = "snowden-sdk-targets"
	maxSDKCacheFiles     = 6 // root plus the existing five retained metadata roles
	maxSDKStoreSteps     = 4 * MaxRecords
	maxSDKStoreWorkBytes = 4 * (MaxPackageBytes + MaxStateBytes)
)

// Single verifier owner, serialized by workspaceMu. Bounds apply before cloning;
// this is a cache, not retained trust state or a second TUF policy. SDK reads and
// writes receive independent bytes, never aliases of caller/store/retained data.
type sdkMetadataStorage struct {
	ctx        context.Context
	files      map[string][]byte
	live, work int64
	steps      int
	seeded     bool
	closed     bool
	err        error
}

func newSDKMetadataStorage(ctx context.Context) *sdkMetadataStorage {
	return &sdkMetadataStorage{ctx: ctx, files: make(map[string][]byte, maxSDKCacheFiles)}
}

func (store *sdkMetadataStorage) fail(err error) error {
	if store.err == nil {
		store.err = errors.Join(ErrTrust, err)
	}
	return store.err
}

// Upstream may treat any local read error as a cache miss. Its caller MUST
// observe this sticky fault after every SDK entrypoint and before giving proof.
func (store *sdkMetadataStorage) Check() error {
	if store == nil {
		return ErrTrust
	}
	if store.err != nil {
		return store.err
	}
	if store.closed {
		return ErrTrust
	}
	if err := contextError(store.ctx); err != nil {
		return store.fail(err)
	}
	return nil
}

func (store *sdkMetadataStorage) charge(size int64) error {
	if err := store.Check(); err != nil {
		return err
	}
	if size < 0 || store.steps >= maxSDKStoreSteps || size > maxSDKStoreWorkBytes-store.work {
		return store.fail(ErrBudget)
	}
	store.steps++
	store.work += size
	return nil
}

func (store *sdkMetadataStorage) metadataName(path string) (string, error) {
	if len(path) > 128 || filepath.Clean(path) != path || filepath.Dir(path) != sdkMetadataNamespace {
		return "", store.fail(ErrTrust)
	}
	name := filepath.Base(path)
	if name != "root.json" && !cacheName(name) {
		return "", store.fail(ErrTrust)
	}
	return name, nil
}

func (store *sdkMetadataStorage) Seed(cache map[string][]byte) error {
	if err := store.Check(); err != nil {
		return err
	}
	if store.seeded || len(store.files) != 0 || len(cache) > 5 {
		return store.fail(ErrBudget)
	}
	store.seeded = true
	for name, raw := range cache {
		if !cacheName(name) {
			return store.fail(ErrTrust)
		}
		if err := store.WriteMetadata(filepath.Join(sdkMetadataNamespace, name), raw); err != nil {
			return err
		}
	}
	return nil
}

func (store *sdkMetadataStorage) EnsurePaths(metadataDir, targetsDir string) error {
	if err := store.charge(0); err != nil {
		return err
	}
	if !store.seeded || metadataDir != sdkMetadataNamespace || targetsDir != sdkTargetNamespace {
		return store.fail(ErrTrust)
	}
	return nil // no directories, file handles, permissions or working-directory changes
}

func (store *sdkMetadataStorage) ReadMetadata(path string) ([]byte, error) {
	if err := store.charge(0); err != nil {
		return nil, err
	}
	name, err := store.metadataName(path)
	if err != nil {
		return nil, err
	}
	raw, exists := store.files[name]
	if !exists {
		return nil, fs.ErrNotExist // only a genuine allowed-name cache miss is non-sticky
	}
	if err := store.charge(int64(len(raw))); err != nil {
		return nil, err
	}
	return bytes.Clone(raw), nil
}

func (store *sdkMetadataStorage) WriteMetadata(path string, raw []byte) error {
	if err := store.Check(); err != nil {
		return err
	}
	name, err := store.metadataName(path)
	if err != nil {
		return err
	}
	previous, exists := store.files[name]
	live := store.live - int64(len(previous)) + int64(len(raw))
	if len(raw) == 0 || len(raw) > MaxMetadataBytes || !exists && len(store.files) >= maxSDKCacheFiles || live > maxSDKCacheFiles*MaxMetadataBytes {
		return store.fail(ErrBudget)
	}
	if err := store.charge(int64(len(raw))); err != nil {
		return err
	}
	store.files[name] = bytes.Clone(raw)
	store.live = live
	clear(previous) // all SDK reads received clones, never this owned buffer
	return nil
}

func (store *sdkMetadataStorage) ReadTarget(string) ([]byte, error) {
	if err := store.Check(); err != nil {
		return nil, err
	}
	return nil, store.fail(ErrTrust) // artifacts are verified by the existing bounded batch gate
}

func (store *sdkMetadataStorage) WriteTarget(string, []byte) error {
	if err := store.Check(); err != nil {
		return err
	}
	return store.fail(ErrTrust) // no target-cache/file fallback through this metadata cache
}

func (store *sdkMetadataStorage) Close() {
	if store == nil || store.closed {
		return
	}
	store.closed = true
	for _, raw := range store.files {
		clear(raw) // public metadata, not a claim of secret zeroization or RSS reclamation
	}
	clear(store.files)
	store.files = nil
	store.live = 0
}
