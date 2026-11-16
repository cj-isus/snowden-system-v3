//go:build windows

package filetxn

import (
	"errors"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"
)

// WindowsWriterLease extends the existing journal writer's original native
// lock across a caller-owned lifecycle. It is not a journal, authenticated
// receipt, arbitrary-HKCU lock or proof that legacy writers cooperate. Keep the
// exact object after any unknown acquisition/validation/Close; never copy it.
type WindowsWriterLease struct {
	mu                                   sync.Mutex
	path                                 string
	domain                               *journalDomain
	release                              func() error
	attempted, returned, active, pending bool
	closeStarted, closed                 bool
	failure, closeErr                    error
}

// Bookkeeping only. The caller must retain this object before Acquire.
func NewWindowsWriterLease(path string) *WindowsWriterLease {
	return &WindowsWriterLease{path: path}
}

func (lease *WindowsWriterLease) Acquire() (result error) {
	if lease == nil {
		return ErrWriteRetirement
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.attempted || lease.closeStarted {
		return ErrWriteRetirement
	}
	lease.attempted = true
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			lease.failure, result = ErrWriteRetirement, ErrWriteRetirement
		}
		lease.returned = returned
	}()
	if !filepath.IsAbs(lease.path) || len(lease.path) > 32760 {
		returned = true
		return ErrBoundedWrite
	}
	lease.domain = journalFor(lease.path)
	lease.pending = true
	var err error
	lease.release, err = lease.domain.acquireWriter() // same native original parent/file/LockFileEx owner
	lease.pending = false
	returned = true
	if err != nil {
		if lease.domain.fault() == nil && (errors.Is(err, ErrWriterBusy) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)) {
			return ErrWriterBusy
		}
		return err
	}
	if lease.release == nil {
		lease.failure = ErrWriteRetirement
		return lease.failure
	}
	lease.active = true
	return nil
}

// Native validation is outside caller publication locks. This method can do
// filesystem/ACL I/O; Retired below is the separate value-only receipt check.
func (lease *WindowsWriterLease) Validate() (result error) {
	if lease == nil {
		return ErrWriteRetirement
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if !lease.returned || !lease.active || lease.pending || lease.closeStarted || lease.failure != nil || lease.domain == nil {
		return errors.Join(ErrWriteRetirement, lease.failure)
	}
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			lease.failure, result = ErrWriteRetirement, ErrWriteRetirement
		}
	}()
	lease.pending = true
	result = lease.domain.checkWriter()
	lease.pending = false
	if result != nil {
		lease.failure = errors.Join(ErrWriteRetirement, result)
	}
	returned = true
	return result
}

func (lease *WindowsWriterLease) Close() (result error) {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closeStarted {
		return lease.closeErr
	}
	if lease.pending || lease.failure != nil || lease.attempted && !lease.returned {
		return errors.Join(ErrWriteRetirement, lease.failure)
	}
	lease.closeStarted, lease.closeErr = true, ErrWriteRetirement
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			lease.closeErr, result = ErrWriteRetirement, ErrWriteRetirement
		}
	}()
	if lease.domain != nil && lease.domain.fault() != nil {
		returned = true
		lease.closeErr = errors.Join(ErrWriteRetirement, lease.domain.fault())
		return lease.closeErr
	}
	if lease.release != nil {
		lease.pending = true
		err := lease.release()
		lease.pending = false
		if err != nil {
			returned = true
			lease.closeErr = errors.Join(ErrWriteRetirement, err)
			return lease.closeErr
		}
	}
	lease.active, lease.closed, lease.closeErr = false, true, nil
	returned = true
	return nil
}

// No native work, wait or retry. A failed/ongoing scope cannot mint a positive
// retirement receipt while another native call owns this object's mutex.
func (lease *WindowsWriterLease) Retired() bool {
	if lease == nil || !lease.mu.TryLock() {
		return false
	}
	defer lease.mu.Unlock()
	return lease.closed && lease.closeStarted && lease.closeErr == nil && lease.failure == nil && !lease.active && !lease.pending
}
