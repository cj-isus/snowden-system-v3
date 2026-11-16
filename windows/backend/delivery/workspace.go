package delivery

import (
	"context"
	"errors"
	"strings"
	"sync"
)

const maxWorkspaceEntries = 64

type workspacePlatform interface {
	Prepare(context.Context, map[string][]byte) error
	Paths() (metadata, targets string)
	Close() error
}

var workspaceMu sync.Mutex
var retainedWorkspace *verificationWorkspace

type verificationWorkspace struct {
	platform workspacePlatform // legacy original-resource receipt only; no new native workspace
	storage  *sdkMetadataStorage
	released bool
	result   error
}

// Reserve one verifier owner before seeding owned public bytes. The caller must
// close a non-nil result even when seeding failed. Retained legacy original I/O
// and cache-reader quarantine remain process-wide admission barriers.
func openWorkspace(ctx context.Context, cache map[string][]byte) (owner *verificationWorkspace, finalErr error) {
	if !workspaceMu.TryLock() {
		return nil, ErrTrust
	}
	if retainedWorkspace != nil || !cacheReaderRetired() {
		workspaceMu.Unlock()
		return nil, ErrTrust
	}
	owner = &verificationWorkspace{}
	retainedWorkspace = owner
	defer func() {
		if recover() != nil {
			finalErr = ErrTrust
		}
	}()
	owner.storage = newSDKMetadataStorage(ctx) // retained before bounded public byte cloning
	if err := owner.storage.Seed(cache); err != nil {
		return owner, err
	}
	return owner, nil
}

func (owner *verificationWorkspace) Paths() (metadata, targets string) {
	return sdkMetadataNamespace, sdkTargetNamespace
}

func (owner *verificationWorkspace) Close() (finalErr error) {
	if owner.released {
		return owner.result
	}
	owner.released, owner.result = true, ErrTrust
	defer workspaceMu.Unlock()
	defer func() {
		if recover() != nil {
			finalErr = ErrTrust
		}
		owner.result = finalErr
	}()
	if retainedWorkspace != owner || !cacheReaderRetired() {
		return ErrTrust // no directory deletion under an uncertain original reader
	}
	if owner.platform != nil {
		if err := owner.platform.Close(); err != nil {
			return errors.Join(ErrTrust, err)
		}
	}
	owner.storage.Close() // memory only; no native resource or cleanup failure to hide
	retainedWorkspace = nil
	return nil
}

func cacheReaderRetired() bool {
	return metadataCacheReader.Retired()
}

// Cleanup is limited to the exact pinned SDK's metadata namespace. Unexpected
// entries are retained, never recursively traversed or treated as authority.
func workspaceMetadataName(name string) bool {
	if cacheName(name) || name == "root.json" {
		return true
	}
	suffix, ok := strings.CutPrefix(name, "tuf_tmp")
	if !ok || len(suffix) == 0 || len(suffix) > 10 {
		return false
	}
	for _, digit := range suffix {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
