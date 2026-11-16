package metadata

import (
	"errors"
	"os"
	"sync"

	"github.com/snowden-system/windows/backend/filetxn"
)

// Separate fixed domains avoid coupling unrelated policy/checkpoint/envelope
// I/O. Never replace an original reader/writer whose outcome is uncertain.
var (
	storedPolicyFile     = newStoredPublicFile(MaxTrustPolicySize)
	storedCheckpointFile = newStoredPublicFile(maxAcceptedProfileSize)
	storedEnvelopeFile   = newStoredPublicFile(MaxEnvelopeSize)
)

type storedPublicFile struct {
	mu     sync.Mutex
	reader *filetxn.BoundedReadSlot
	writer *filetxn.BoundedWriteSlot
	limit  int64
}

func newStoredPublicFile(limit int64) *storedPublicFile {
	return &storedPublicFile{reader: filetxn.NewBoundedReadSlot(limit), writer: filetxn.NewBoundedWriteSlot(limit), limit: limit}
}

// Callers retain their existing journal/source ownership. This gate never
// acquires the journal, invokes callbacks, parses JSON or calls another loader;
// it is released before any of those steps. It is not a hard I/O deadline.
// present=false is reserved for confirmed absence with no cleanup uncertainty.
// On error, present=true is conservative: presence could not be ruled out.
func (file *storedPublicFile) read(path string) ([]byte, bool, error) {
	file.mu.Lock()
	defer file.mu.Unlock()
	if err := file.writer.Check(); err != nil {
		return nil, true, err // uncertain publication is never confirmed absence
	}
	raw, err := file.reader.Read(path, file.limit)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return raw, true, nil
}

// Decode/validation and the caller's journal serialization remain outside this
// I/O gate. A poisoned reader cannot be bypassed by overwriting its path.
func (file *storedPublicFile) write(path string, raw []byte) error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if !file.reader.Retired() {
		return errors.Join(ErrTrustRole, filetxn.ErrReadClose)
	}
	if err := file.writer.Write(path, raw); err != nil {
		return errors.Join(ErrTrustRole, err)
	}
	return nil
}
