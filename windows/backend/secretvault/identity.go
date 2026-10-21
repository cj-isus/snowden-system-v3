package secretvault

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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

// PreparedImport is private staged state; callbacks run under the vault lock.
// Its methods must not call back into Manager. Bytes contains only DPAPI data.
type PreparedImport struct {
	owner *Manager
	file  *fileFormat
}

func (p *PreparedImport) Bytes() ([]byte, error) { return json.MarshalIndent(p.file, "", "  ") }
func (p *PreparedImport) Path() string           { return p.owner.path }
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
	for _, slot := range slots {
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
	}
	return use(&PreparedImport{owner: m, file: f})
}

// RecoverImport serializes recovery with all vault reads/writes. Keeping the
// guard after a failed recovery prevents accidentally using a mixed profile.
func (m *Manager) RecoverImport(journal string, run func(vaultPath string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.importJournal = journal
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
