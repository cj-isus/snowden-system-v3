package filetxn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var ErrTransactionBusy = errors.New("filetxn: journal operation already active")

type journalFailure struct{ err error }

// The existing Serialize registry retains I/O owners too. A new Store value or
// absent journal pathname cannot erase an uncertain original operation.
type journalDomain struct {
	dir                       string
	serial, operation         sync.Mutex
	writerGate                sync.Mutex
	writerOwner               journalWriterLock // writerGate; never discard uncertain native ownership
	failure                   atomic.Pointer[journalFailure]
	cleanupStatus             atomic.Pointer[journalFailure]
	reader, directory         *BoundedReadSlot
	stageWriter, targetWriter *BoundedWriteSlot
	pending                   *journalCleanup // operation gate only; Check reads atomic failure
	targets                   journalTargets  // same operation gate; retained on uncertain retirement
}

func newJournalWriter() *BoundedWriteSlot {
	writer := NewBoundedWriteSlot(maxFileSize)
	writer.allowEmpty = true // absent backups and Keep.new files are empty, not missing
	return writer
}

func journalFor(dir string) *journalDomain {
	key := journalKey(dir)
	if value, ok := profileLocks.Load(key); ok {
		return value.(*journalDomain)
	}
	domain := &journalDomain{dir: key, reader: NewBoundedReadSlot(maxFileSize), directory: NewBoundedReadSlot(maxFileSize), stageWriter: newJournalWriter(), targetWriter: newJournalWriter()}
	value, _ := profileLocks.LoadOrStore(key, domain)
	return value.(*journalDomain)
}

func journalKey(path string) string {
	key, err := filepath.Abs(path)
	if err != nil {
		key = filepath.Clean(path)
	}
	return strings.ToLower(key) // preserve existing Serialize key semantics
}

func (domain *journalDomain) fault() error {
	if failure := domain.failure.Load(); failure != nil {
		return failure.err
	}
	return nil
}

func (domain *journalDomain) fail(err error) error {
	failure := &journalFailure{err: errors.Join(ErrRecoveryRequired, err)}
	domain.failure.CompareAndSwap(nil, failure)
	return domain.fault()
}

func (domain *journalDomain) enter() error {
	if !domain.operation.TryLock() {
		return errors.Join(ErrRecoveryRequired, ErrTransactionBusy)
	}
	if err := domain.fault(); err != nil {
		domain.operation.Unlock()
		return err
	}
	return nil
}

func (domain *journalDomain) read(path string, limit int64) ([]byte, bool, error) {
	if err := domain.fault(); err != nil {
		return nil, true, err
	}
	raw, err := domain.reader.Read(path, limit)
	if !domain.reader.Retired() {
		return nil, true, domain.fail(err)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return raw, true, nil
}

func (domain *journalDomain) snapshotDirectory(path string) (os.FileInfo, error) {
	if err := domain.fault(); err != nil {
		return nil, err
	}
	info, err := domain.directory.snapshotDirectory(path)
	if !domain.directory.Retired() {
		return nil, domain.fail(err)
	}
	return info, err
}

func (domain *journalDomain) write(path string, raw []byte, newOnly bool) error {
	if err := domain.fault(); err != nil {
		return err
	}
	writer := domain.targetWriter
	if newOnly {
		writer = domain.stageWriter
	}
	var err error
	if newOnly {
		if domain.pending != nil && domain.pending.directory != nil {
			if journalKey(filepath.Dir(path)) != journalKey(domain.pending.path) {
				return domain.fail(ErrStagingCleanup)
			}
			if err := domain.sameDirectory(domain.pending); err != nil {
				return domain.fail(err)
			}
			err = writer.writeInDirectory(path, raw, true, journalKey(domain.pending.path))
		} else {
			err = writer.WriteNew(path, raw)
		}
	} else {
		err = writer.Write(path, raw)
	}
	if retirementErr := writer.Check(); retirementErr != nil {
		return domain.fail(errors.Join(err, retirementErr))
	}
	return err
}

// CleanupError describes retained staging debris or postcommit retirement
// failures. It does not change the commit result. Check independently blocks
// uncertain original resources; debris blocks the next stage until resolved.
func (s *Store) CleanupError() error {
	if failure := journalFor(s.Dir).cleanupStatus.Load(); failure != nil {
		return failure.err
	}
	return nil
}

// QuarantineError observes only the process-local quarantine of original I/O
// resources. It does not read disk, acquire/release a native scope, recover a
// journal or prove clean storage when nil. A completed commit stays completed.
func (s *Store) QuarantineError() error {
	return journalFor(s.Dir).fault()
}
