package secretvault

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

// Slot is an internal import value. Never return it to UI or logs.
type Slot struct {
	ID, Kind, Value string
}

var ErrAmbiguousKind = errors.New("secretvault: ambiguous kind; bind an exact SecretID")

// ImportSlots preserves source identities. Validate and encrypt the entire
// batch before one atomic vault save; existing unrelated slots remain intact.
// This is a vault transaction, not a transaction of metadata/keys/version files.
func (m *Manager) ImportSlots(slots []Slot) error {
	if len(slots) == 0 {
		return nil
	}
	return m.WithPreparedImport(slots, func(p *PreparedImport) error { return m.save(p.file) })
}

// PreparedImport owns private staged state. WithPreparedImport callbacks run
// under the vault lock; PrepareImportSnapshot releases it. Readers never mutate
// the snapshot or call a lock-taking Manager method. Bytes is encrypted data.
type PreparedImport struct {
	owner   *Manager
	file    *fileFormat
	changed bool
	source  vaultSource // private exact encrypted source identity, never UI/log data
}

func (p *PreparedImport) Bytes() ([]byte, error) { return json.MarshalIndent(p.file, "", "  ") }
func (p *PreparedImport) Path() string           { return p.owner.path }
func (p *PreparedImport) Changed() bool          { return p.changed }

// MetaByID reads the same immutable encrypted source as ExportReference.
// No App/lifecycle callbacks or live Manager reads; never return plaintext.
func (p *PreparedImport) MetaByID(id string) (Meta, error) {
	if p == nil || p.owner == nil || p.file == nil || !validSlotID(id) {
		return Meta{}, ErrNotFound
	}
	entry, ok := p.file.Items[id]
	if !ok {
		return Meta{}, ErrNotFound
	}
	return p.owner.metaOf(id, entry, p.file.Audit[id]), nil
}

// ExportSlots is internal plaintext transfer data, never a UI/log result.
func (p *PreparedImport) ExportSlots() ([]Slot, error) {
	ids := make([]string, 0, len(p.file.Items))
	for id := range p.file.Items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var slots []Slot
	for _, id := range ids {
		entry := p.file.Items[id]
		if entry.CipherB64 == "" {
			continue
		}
		value, err := p.value(entry)
		if err != nil {
			return nil, err
		}
		slots = append(slots, Slot{ID: id, Kind: entry.Kind, Value: value})
	}
	return slots, nil
}

// ExportReference resolves exactly the same ID/kind semantics as rendering,
// returning the real stored identity. It never chooses the first of ambiguous
// legacy kind slots or silently falls back from an explicit missing ID.
func (p *PreparedImport) ExportReference(id, kind string) (Slot, error) {
	if p == nil || p.file == nil || kind == "" {
		return Slot{}, errors.New("secretvault: credential snapshot/reference required")
	}
	if id == "" {
		var err error
		id, err = uniqueKind(p.file, Kind(kind))
		if err != nil {
			return Slot{}, err
		}
	} else if !validSlotID(id) {
		return Slot{}, errors.New("secretvault: invalid credential identity")
	}
	entry, exists := p.file.Items[id]
	if !exists {
		return Slot{}, ErrNotFound
	}
	if canonicalKind(entry.Kind) != canonicalKind(kind) {
		return Slot{}, errors.New("secretvault: credential identity/kind mismatch")
	}
	value, err := p.value(entry)
	if err != nil {
		return Slot{}, err
	}
	return Slot{ID: id, Kind: entry.Kind, Value: value}, nil
}
func (p *PreparedImport) Get(ref string) (string, error) {
	id, err := uniqueKind(p.file, Kind(ref))
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", ErrNotFound
	}
	return p.value(p.file.Items[id])
}
func (p *PreparedImport) GetByID(id, kind string) (string, error) {
	entry, ok := p.file.Items[id]
	if !ok {
		return "", ErrNotFound
	}
	if kind == "" || canonicalKind(entry.Kind) != canonicalKind(kind) {
		return "", errors.New("secretvault: SecretID kind mismatch")
	}
	return p.value(entry)
}

func (p *PreparedImport) value(entry storedEntry) (string, error) {
	value, err := p.owner.entryValue(entry)
	if err != nil {
		return "", err
	}
	if err := validateKind(Kind(entry.Kind), value); err != nil {
		return "", errors.New("secretvault: stored credential format is invalid")
	}
	return value, nil
}
func (p *PreparedImport) PinCertPath(string) string { return "" }

// PrepareImportSnapshot releases the vault before lifecycle/App callbacks.
// Its owned file is immutable to consumers; staged credentials are not saved.
func (m *Manager) PrepareImportSnapshot(slots []Slot) (*PreparedImport, error) {
	var prepared *PreparedImport
	err := m.WithPreparedImport(slots, func(p *PreparedImport) error { prepared = p; return nil })
	return prepared, err
}

// WithPreparedSnapshot checks an export/read-only source under the journal
// writer scope. No Store/WithWriter, Manager or App/lifecycle calls in inspect.
// Imports use WithPreparedTransaction for a final comparison inside Store.
func (m *Manager) WithPreparedSnapshot(p *PreparedImport, inspect func() error) error {
	return m.withPreparedSnapshot(p, inspect, false, "")
}

// WithPreparedReadSnapshot uses the existing initialized shared journal lock.
// journal must match this manager's immutable binding. Preview never creates
// lock/parents or saves staged credentials/history. The same callback
// restrictions as WithPreparedSnapshot apply.
func (m *Manager) WithPreparedReadSnapshot(p *PreparedImport, journal string, inspect func() error) error {
	return m.withPreparedSnapshot(p, inspect, true, journal)
}

func (m *Manager) withPreparedSnapshot(p *PreparedImport, inspect func() error, readOnly bool, requestedJournal string) error {
	if p == nil || p.owner != m || inspect == nil {
		return errors.New("secretvault: invalid staged import owner")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	check := func() error {
		if err := m.checkSource(p.source); err != nil {
			return err
		}
		return inspect()
	}
	if readOnly {
		if m.importJournal == "" {
			return ErrJournalRequired
		}
		journal, err := filepath.Abs(requestedJournal)
		if err != nil || requestedJournal == "" || !strings.EqualFold(journal, m.importJournal) {
			return errors.New("secretvault: prepared snapshot journal mismatch")
		}
		return filetxn.WithRead(m.importJournal, check)
	}
	return m.withWriter(check)
}

// WithPreparedTransaction keeps local vault ownership while Store acquires the
// shared native scope. Its final source comparison runs after all backups/new
// bytes are staged, before the journal is published. The existing public-state
// fence remains part of the same scope. Callers supply a fresh, private Store;
// commit must invoke its Commit synchronously, without App/lifecycle callbacks.
func (m *Manager) WithPreparedTransaction(p *PreparedImport, tx *filetxn.Store, commit func() error) error {
	if p == nil || p.owner != m || tx == nil || tx.Dir == "" || commit == nil {
		return errors.New("secretvault: invalid prepared transaction")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	journal, err := filepath.Abs(tx.Dir)
	if err != nil || m.importJournal == "" || !strings.EqualFold(journal, m.importJournal) {
		return errors.New("secretvault: prepared transaction journal mismatch")
	}
	before := tx.BeforeCommit
	checked := false
	tx.BeforeCommit = func() error {
		if err := m.checkSource(p.source); err != nil {
			return err
		}
		if before != nil {
			if err := before(); err != nil {
				return err
			}
		}
		checked = true
		return nil
	}
	defer func() { tx.BeforeCommit = before }()
	err = commit()
	if err == nil && !checked {
		return errors.New("secretvault: transaction skipped its final source fence")
	}
	return err
}

func (m *Manager) WithPreparedImport(slots []Slot, use func(*PreparedImport) error) error {
	if use == nil {
		return errors.New("secretvault: missing import consumer")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return err
	}
	source := f.source
	seen := make(map[string]bool, len(slots))
	for i, slot := range slots {
		if !validSlotID(slot.ID) || seen[slot.ID] {
			return fmt.Errorf("secretvault: slot %d: invalid or duplicate ID", i)
		}
		seen[slot.ID] = true
		known := false
		for _, k := range AllKinds() {
			if string(k) == slot.Kind {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("secretvault: slot %d: unknown kind", i)
		}
		value := strings.TrimSpace(slot.Value)
		if value == "" {
			return fmt.Errorf("secretvault: slot %d: empty value", i)
		}
		if err := validateKind(Kind(slot.Kind), value); err != nil {
			// Format errors can evolve; do not propagate arbitrary credential text.
			return fmt.Errorf("secretvault: slot %d: invalid value for kind", i)
		}
		if old, ok := f.Items[slot.ID]; ok && old.Kind != slot.Kind {
			return fmt.Errorf("secretvault: slot %d: ID already belongs to another kind", i)
		}
	}
	now := time.Now()
	changed := false
	for _, slot := range slots {
		if old, exists := f.Items[slot.ID]; exists {
			if value, err := m.entryValue(old); err == nil && value == strings.TrimSpace(slot.Value) {
				continue // exact replay preserves ciphertext and verification audit
			}
		}
		cipher, err := dpapiProtect([]byte(strings.TrimSpace(slot.Value)))
		if err != nil {
			return fmt.Errorf("secretvault: encrypt import: %w", err)
		}
		entry, exists := f.Items[slot.ID]
		if !exists {
			entry = storedEntry{Kind: slot.Kind, Title: slot.ID, Hint: "импорт с другого устройства"}
			f.Order = append(f.Order, slot.ID)
		}
		entry.CipherB64 = base64Std.EncodeToString(cipher)
		entry.UpdatedAt = now
		f.Items[slot.ID] = entry
		a := f.Audit[slot.ID]
		if a.CreatedAt.IsZero() {
			a.CreatedAt = now
		}
		a.VerifyStatus, a.VerifyError, a.LastVerifiedAt = "unverified", "", nil
		f.Audit[slot.ID] = a
		changed = true
	}
	return use(&PreparedImport{owner: m, file: f, changed: changed, source: source})
}

// RecoverImport serializes recovery with all vault reads/writes. Keeping the
// guard after a failed recovery prevents accidentally using a mixed profile.
// It also binds the fixed profile journal before the first mutation; callers
// cannot switch this Manager/path to another journal to bypass a held writer.
func (m *Manager) RecoverImport(journal string, run func(vaultPath string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if journal == "" || run == nil {
		return ErrJournalRequired
	}
	journal, err := filepath.Abs(journal)
	if err != nil || filepath.Dir(journal) == journal {
		return errors.Join(ErrJournalRequired, err)
	}
	if m.importJournal != "" && !strings.EqualFold(m.importJournal, journal) {
		return errors.New("secretvault: cannot rebind a Manager to another journal")
	}
	file, err := vaultFileFor(m.path)
	if err != nil {
		return err
	}
	if err := file.bind(journal); err != nil {
		return err
	}
	m.importJournal = journal
	if err := file.ready(); err != nil {
		return err
	}
	return run(m.path)
}

func validSlotID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ValueFor resolves an ID only; a missing ID never falls back to a kind.
// Kind aliases retain compatibility with the original per-channel C/D slots.
func (m *Manager) ValueFor(id, expectedKind string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return "", err
	}
	entry, ok := f.Items[id]
	if !ok {
		return "", ErrNotFound
	}
	if canonicalKind(entry.Kind) != canonicalKind(expectedKind) || expectedKind == "" {
		return "", errors.New("secretvault: SecretID kind mismatch")
	}
	return m.entryValue(entry)
}

func canonicalKind(kind string) string {
	switch Kind(kind) {
	case KindVlessUUIDC, KindVlessUUIDD:
		return string(KindVlessUUID)
	default:
		return kind
	}
}

// ValueByKind is the migration path for profiles without SecretID bindings.
// Even an empty second slot is ambiguous; list order must not choose credentials.
func (m *Manager) ValueByKind(kind string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return "", err
	}
	id, err := uniqueKind(f, Kind(kind))
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", ErrNotFound
	}
	return m.entryValue(f.Items[id])
}

func uniqueKind(f *fileFormat, kind Kind) (string, error) {
	id := ""
	for candidate, entry := range f.Items {
		if entry.Kind != string(kind) {
			continue
		}
		if id != "" {
			return "", ErrAmbiguousKind
		}
		id = candidate
	}
	return id, nil
}

func (m *Manager) entryValue(entry storedEntry) (string, error) {
	if entry.CipherB64 == "" {
		return "", ErrNotSet
	}
	plain, err := m.decryptEntry(entry)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
