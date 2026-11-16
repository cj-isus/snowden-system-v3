package secretvault

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/snowden-system/windows/backend/filetxn"
)

const maxVaultFileSize = 16 << 20 // same bound as the fixed import-journal target

var (
	ErrSourceChanged   = errors.New("secretvault: stored source changed; reload before retry")
	ErrJournalRequired = errors.New("secretvault: bind the profile import journal before writing")
	vaultFiles         sync.Map
)

// Original I/O owners outlive a Manager value. A second Manager cannot discard
// an uncertain read/write by creating fresh slots for the same canonical path.
type vaultFile struct {
	path, journal string
	mu            sync.Mutex // I/O/binding only: no parsing, Manager or journal callback
	reader        *filetxn.BoundedReadSlot
	writer        *filetxn.BoundedWriteSlot
}

// Exact encrypted bytes and absence are distinct. This receipt is private and
// never serialized, returned to UI or used as credential/import authority.
type vaultSource struct {
	file    *vaultFile
	present bool
	digest  [32]byte
}

func vaultFileFor(path string) (*vaultFile, error) {
	if path == "" {
		return nil, errors.New("secretvault: empty storage path")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	key := strings.ToLower(path)
	if value, ok := vaultFiles.Load(key); ok {
		return value.(*vaultFile), nil
	}
	file := &vaultFile{path: path, reader: filetxn.NewBoundedReadSlot(maxVaultFileSize), writer: filetxn.NewBoundedWriteSlot(maxVaultFileSize)}
	value, _ := vaultFiles.LoadOrStore(key, file)
	return value.(*vaultFile), nil
}

func (file *vaultFile) bind(journal string) error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.journal != "" && !strings.EqualFold(file.journal, journal) {
		return errors.New("secretvault: storage is already bound to another journal")
	}
	file.journal = journal
	return nil
}

func (file *vaultFile) read() ([]byte, vaultSource, error) {
	file.mu.Lock()
	defer file.mu.Unlock()
	if err := filetxn.Check(file.journal); err != nil {
		return nil, vaultSource{}, err // a fresh/unbound Manager cannot bypass known binding
	}
	if err := file.writer.Check(); err != nil {
		return nil, vaultSource{}, err
	}
	raw, err := file.reader.Read(file.path, maxVaultFileSize)
	if errors.Is(err, os.ErrNotExist) {
		return nil, vaultSource{file: file}, nil
	}
	if err != nil {
		return nil, vaultSource{}, err
	}
	return raw, vaultSource{file: file, present: true, digest: sha256.Sum256(raw)}, nil
}

func (file *vaultFile) ready() error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if !file.reader.Retired() {
		return filetxn.ErrReadClose
	}
	return file.writer.Check()
}

func (file *vaultFile) write(raw []byte, newOnly bool) error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if !file.reader.Retired() {
		return filetxn.ErrReadClose
	}
	if newOnly {
		return file.writer.WriteNew(file.path, raw)
	}
	return file.writer.Write(file.path, raw)
}

func (m *Manager) readSource() ([]byte, vaultSource, error) {
	if err := filetxn.Check(m.importJournal); err != nil {
		return nil, vaultSource{}, err
	}
	file, err := vaultFileFor(m.path)
	if err != nil {
		return nil, vaultSource{}, err
	}
	return file.read()
}

// Caller holds m.mu, and a shared writer scope when this is a final fence.
func (m *Manager) checkSource(source vaultSource) error {
	_, current, err := m.readSource()
	if err != nil {
		return err
	}
	if source.file == nil || current != source {
		return ErrSourceChanged
	}
	return nil
}

// Lock order: optional caller Serialize -> Manager.mu -> journal writer -> I/O.
// No App/lifecycle callback or nested Store/WithWriter is allowed in write.
func (m *Manager) withWriter(write func() error) error {
	if m.importJournal == "" {
		return ErrJournalRequired
	}
	return filetxn.WithWriter(m.importJournal, func() error {
		if err := filetxn.Check(m.importJournal); err != nil {
			return err
		}
		return write()
	})
}

// MigrateLegacy copies encrypted bytes only into a confirmed absent target.
// The original legacy file remains untouched. Existing/corrupt target data is
// never replaced, and an occupied name at publication fails the new-only write.
func (m *Manager) MigrateLegacy(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.withWriter(func() error {
		_, target, err := m.readSource()
		if err != nil || target.present {
			return err
		}
		legacy, err := vaultFileFor(path)
		if err != nil {
			return err
		}
		raw, source, err := legacy.read()
		if err != nil || !source.present {
			return err
		}
		if _, err := decodeVault(raw, source); err != nil {
			return err
		}
		return target.file.write(raw, true)
	})
}
