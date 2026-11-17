package filetxn

import (
	"errors"
	"path/filepath"
)

var ErrWriterBusy = errors.New("filetxn: journal writer is busy")
var ErrReadScopeUnavailable = errors.New("filetxn: coherent read requires an initialized profile lock")

type journalWriterLock interface {
	Acquire() error
	Validate() error
	Close() error
}

func journalWriterPath(journal string) (string, error) {
	if journal == "" {
		return "", ErrRecoveryRequired
	}
	path, err := filepath.Abs(journal)
	if err != nil || filepath.Dir(path) == path {
		return "", errors.Join(ErrRecoveryRequired, err)
	}
	return path + ".writer.lock", nil
}

// WithWriter serializes one complete read/derive/write operation with Store.
// It is not reentrant: callbacks may load data but must not call WithWriter or
// Store.Commit/Recover. Hold caller Serialize/vault locks before entering here.
// No implicit same-process/path "already locked" bypass grants another writer.
func WithWriter(journal string, write func() error) (finalErr error) {
	return withJournalScope(journal, write, false)
}

// WithRead captures public source values against cooperative writers. Windows
// opens only the existing lock/parents and acquires a shared byte-range lock;
// it never initializes storage. Missing lock is an explicit error, not missing
// profile data. The callback must not mutate the coordinated profile files,
// acquire this scope recursively, or return borrowed storage. It may decode/copy
// bounded public input or guard a separate installation-domain mutation.
func WithRead(journal string, read func() error) error {
	return withJournalScope(journal, read, true)
}

func withJournalScope(journal string, use func() error, readOnly bool) (finalErr error) {
	if use == nil {
		return ErrRecoveryRequired
	}
	if _, err := journalWriterPath(journal); err != nil {
		return err
	}
	domain := journalFor(journal)
	release, err := domain.acquireScope(readOnly)
	if err != nil {
		return err
	}
	callbackReturned := true
	defer func() {
		if recover() != nil || !callbackReturned {
			// Goexit also unwinds without returning an ownership outcome.
			// Poison before release, which must retain the original writer.
			finalErr = domain.fail(ErrWriteRetirement)
		}
		if errors.Is(finalErr, ErrWriteRetirement) || errors.Is(finalErr, ErrReadClose) {
			finalErr = domain.fail(finalErr)
		}
		if finalErr == nil {
			finalErr = domain.checkWriter()
		}
		if finalErr == nil && readOnly {
			finalErr = Check(journal)
		}
		finalErr = errors.Join(finalErr, release())
	}()
	if readOnly {
		if err := Check(journal); err != nil {
			return err
		}
	}
	callbackReturned = false
	finalErr = use()
	callbackReturned = true
	return finalErr
}

func (domain *journalDomain) acquireWriter() (func() error, error) {
	return domain.acquireScope(false)
}

func (domain *journalDomain) acquireScope(readOnly bool) (func() error, error) {
	path, err := journalWriterPath(domain.dir)
	if err != nil {
		return nil, err
	}
	if !domain.writerGate.TryLock() {
		return nil, ErrWriterBusy
	}
	if err := domain.fault(); err != nil {
		domain.writerGate.Unlock()
		return nil, err
	}
	if domain.writerOwner != nil {
		domain.writerGate.Unlock()
		return nil, domain.fail(ErrWriteRetirement)
	}
	if readOnly {
		domain.writerOwner = newJournalReaderLock(path)
	} else {
		domain.writerOwner = newJournalWriterLock(path)
	} // retained before any native acquisition
	released := false
	var releaseErr error
	release := func() error {
		if released {
			return releaseErr
		}
		released = true
		defer domain.writerGate.Unlock()
		if err := domain.fault(); err != nil {
			releaseErr = err // retain the lock across unresolved original I/O
			return releaseErr
		}
		if domain.writerOwner != nil {
			if err := invoke(domain.writerOwner.Close); err != nil {
				releaseErr = domain.fail(err)
				return releaseErr
			}
			domain.writerOwner = nil
		}
		return nil
	}
	if domain.writerOwner != nil {
		if err := invoke(domain.writerOwner.Acquire); err != nil {
			return nil, errors.Join(err, release())
		}
	}
	return release, nil // portable adapter supplies process-local serialization only
}

func (domain *journalDomain) checkWriter() error {
	if err := domain.fault(); err != nil {
		return err
	}
	if domain.writerOwner != nil {
		if err := invoke(domain.writerOwner.Validate); err != nil {
			return domain.fail(err)
		}
	}
	return nil
}

// A completed journal transaction remains completed after lock retirement
// fails, but readers/new mutations see the same retained recovery fault.
func (domain *journalDomain) finishWriter(release func() error, finalErr *error, completed bool) {
	if release == nil {
		return
	}
	if err := release(); err != nil {
		domain.cleanupStatus.Store(&journalFailure{err: errors.Join(ErrStagingCleanup, err)})
		if !completed {
			*finalErr = errors.Join(*finalErr, err)
		}
	}
}
