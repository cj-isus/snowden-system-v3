//go:build windows

package filetxn

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// WindowsRemovalSlot keeps the original parent/file handles across one explicit
// caller cleanup and deletes that same object only after normal cleanup success.
// Keep this slot for the storage domain's lifetime, including unknown callbacks,
// native outputs and Close. It supplies no authorization to release protection.
type WindowsRemovalSlot struct {
	mu       sync.Mutex
	maxBytes int64
	owner    *nativeJournalTargets
	failure  error
}

// Bookkeeping only; no file, directory or native handle is acquired here.
func NewWindowsRemovalSlot(maxBytes int64) *WindowsRemovalSlot {
	return &WindowsRemovalSlot{maxBytes: maxBytes}
}

// RemoveAfter requires caller-owned cooperative writer/lifecycle serialization.
// A present file is inspected with the trusted-store ACL/regular/single-link
// policy and held without write/delete sharing while cleanup runs. No pathname
// reopen, digest, PID or later snapshot authorizes its deletion. Confirmed initial
// absence still runs cleanup, then checks absence through the same held parent.
func (slot *WindowsRemovalSlot) RemoveAfter(path string, cleanup func() error) (finalErr error) {
	if slot == nil || slot.maxBytes < 1 || slot.maxBytes > maxFileSize || cleanup == nil {
		return ErrBoundedWrite
	}
	if !slot.mu.TryLock() {
		return ErrWriteBusy
	}
	defer slot.mu.Unlock()
	if slot.owner != nil || slot.failure != nil {
		return errors.Join(ErrWriteRetirement, slot.failure)
	}
	owner, ok := newJournalTargets().(*nativeJournalTargets)
	if !ok || owner == nil {
		return ErrWriteRetirement
	}
	owner.parents.limit = slot.maxBytes
	slot.owner = owner // before the first original ancestor/file acquisition
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			slot.failure = ErrWriteRetirement
			finalErr = errors.Join(finalErr, slot.failure)
			return // an unreturned callback/native call keeps its exact original owner
		}
		if owner.deleting != nil {
			if err := owner.deleting.close(); err != nil {
				slot.failure = errors.Join(ErrWriteRetirement, err)
				finalErr = errors.Join(finalErr, slot.failure)
				return
			}
			owner.deleting = nil
		}
		if err := owner.Close(); err != nil {
			slot.failure = errors.Join(ErrWriteRetirement, err)
			finalErr = errors.Join(finalErr, slot.failure)
			return
		}
		slot.owner = nil // only checked original retirement makes the slot reusable
	}()
	finalErr = slot.removeAfter(owner, path, cleanup)
	returned = true
	return finalErr
}

func (slot *WindowsRemovalSlot) removeAfter(owner *nativeJournalTargets, path string, cleanup func() error) error {
	if err := owner.Acquire([]File{{Path: path}}); err != nil {
		return err
	}
	if err := owner.Check(path); err != nil {
		return err
	}
	binding := owner.bindings[owner.target(path)]
	parent := &owner.parents.directories[binding.parent]
	name := filepath.Base(binding.path)
	owner.deleting = &nativeFileDeletion{}
	deletion := owner.deleting
	err := deletion.prepare(&owner.parents, parent, name, nil)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if err := cleanup(); err != nil {
		return err // the original marker was never given a delete disposition
	}
	if missing {
		return deletion.observeAbsence(&owner.parents, parent, name)
	}
	return deletion.commit(&owner.parents, parent, name)
}
