package secretvault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSecretIDVaultImportPreservesInstances(t *testing.T) {
	m := newTestManager(t)
	slots := []Slot{{"hy2-one", "hy2-password", "synthetic-password-one"}, {"hy2-two", "hy2-password", "synthetic-password-two"}}
	if err := m.ImportSlots(slots); err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots {
		value, err := m.ValueFor(slot.ID, slot.Kind)
		if err != nil || value != slot.Value {
			t.Fatalf("identity %s: %v", slot.ID, err)
		}
	}
	if _, err := m.ValueByKind("hy2-password"); !errors.Is(err, ErrAmbiguousKind) {
		t.Fatalf("legacy ambiguity: %v", err)
	}
	before, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddSave("hy2-password", "new title", "synthetic-password-other", ""); !errors.Is(err, ErrAmbiguousKind) {
		t.Fatalf("ambiguous AddSave: %v", err)
	}
	after, err := os.ReadFile(m.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("ambiguous AddSave changed vault")
	}
	if err := m.ImportSlots([]Slot{{"hy2-one", "hy2-password", "synthetic-password-updated"}}); err != nil {
		t.Fatal(err)
	}
	if value, err := m.ValueFor("hy2-two", "hy2-password"); err != nil || value != slots[1].Value {
		t.Fatal("update touched unrelated instance")
	}
	metas, err := m.List()
	if err != nil || len(metas) != 2 {
		t.Fatalf("slots duplicated: %v", err)
	}
	for _, meta := range metas {
		if meta.Reveals != 0 {
			t.Fatal("internal lookup counted as UI reveal")
		}
	}
	data, err := os.ReadFile(m.path)
	if err != nil || bytes.Contains(data, []byte("synthetic-password")) {
		t.Fatal("plaintext credential in vault file")
	}
}

func TestSecretIDVaultInvalidBatchIsUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		slots []Slot
	}{
		{"empty-id", []Slot{{"", "hy2-password", "synthetic-password"}}},
		{"path-id", []Slot{{"../slot", "hy2-password", "synthetic-password"}}},
		{"control-id", []Slot{{"slot\n", "hy2-password", "synthetic-password"}}},
		{"long-id", []Slot{{strings.Repeat("x", 129), "hy2-password", "synthetic-password"}}},
		{"unknown-kind", []Slot{{"slot", "unknown", "synthetic-password"}}},
		{"empty-value", []Slot{{"slot", "hy2-password", " "}}},
		{"invalid-format", []Slot{{"slot", "vless-uuid", "synthetic-invalid-uuid"}}},
		{"duplicate-id", []Slot{{"slot", "hy2-password", "synthetic-password"}, {"slot", "hy2-password", "synthetic-password-two"}}},
		{"kind-collision", []Slot{{"old", "hy2-obfs-password", "synthetic-obfs"}}},
		{"valid-then-invalid", []Slot{{"new", "hy2-password", "synthetic-password"}, {"bad", "vless-uuid", "synthetic-invalid-uuid"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			if err := m.ImportSlots([]Slot{{"old", "hy2-password", "synthetic-original"}}); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			err = m.ImportSlots(tc.slots)
			if err == nil || strings.Contains(err.Error(), "synthetic-") {
				t.Fatalf("invalid import or credential in error: %v", err)
			}
			after, err := os.ReadFile(m.path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected batch changed vault bytes")
			}
		})
	}
}

func TestSecretIDVaultSaveFailureIsUnchanged(t *testing.T) {
	m := newTestManager(t)
	if err := m.ImportSlots([]Slot{{"old", "hy2-password", "synthetic-original"}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(m.path+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.ImportSlots([]Slot{{"new", "hy2-password", "synthetic-new"}}); err == nil {
		t.Fatal("save fault accepted")
	}
	after, err := os.ReadFile(m.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("save fault changed committed bytes")
	}
	if _, err := m.Value("new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed import visible: %v", err)
	}
}

func TestSecretIDVaultLookup(t *testing.T) {
	cases := []struct {
		name, kind, want string
		wantErr          error
	}{
		{"unique", "hy2-password", "synthetic-password", nil},
		{"missing", "hy2-obfs-password", "", ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			if err := m.ImportSlots([]Slot{{"slot", "hy2-password", "synthetic-password"}}); err != nil {
				t.Fatal(err)
			}
			value, err := m.ValueByKind(tc.kind)
			if !errors.Is(err, tc.wantErr) || value != tc.want {
				t.Fatalf("lookup: %v", err)
			}
		})
	}
	t.Run("missing-id-no-kind-fallback", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.ImportSlots([]Slot{{"slot", "hy2-password", "synthetic-password"}}); err != nil {
			t.Fatal(err)
		}
		if value, err := m.ValueFor("hy2-password", "hy2-password"); !errors.Is(err, ErrNotFound) || value != "" {
			t.Fatalf("ID fallback: %v", err)
		}
	})
	t.Run("kind-mismatch", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.ImportSlots([]Slot{{"slot", "hy2-password", "synthetic-password"}}); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"hy2-obfs-password", ""} {
			if value, err := m.ValueFor("slot", kind); err == nil || value != "" {
				t.Fatal("foreign kind accepted")
			}
		}
	})
	t.Run("legacy-uuid-alias", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.ImportSlots([]Slot{{"slot", "vless-uuid-c", "00000000-0000-4000-8000-000000000001"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := m.ValueFor("slot", "vless-uuid"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unset", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.Seed(); err != nil {
			t.Fatal(err)
		}
		if value, err := m.ValueByKind("hy2-password"); !errors.Is(err, ErrNotSet) || value != "" {
			t.Fatalf("unset: %v", err)
		}
	})
}

func TestSecretIDVaultConcurrentImports(t *testing.T) {
	m := newTestManager(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := m.ImportSlots([]Slot{{fmt.Sprintf("slot-%d", i), "hy2-password", fmt.Sprintf("synthetic-password-%d", i)}}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 12; i++ {
		value, err := m.ValueFor(fmt.Sprintf("slot-%d", i), "hy2-password")
		if err != nil || value != fmt.Sprintf("synthetic-password-%d", i) {
			t.Fatalf("lost concurrent slot %d: %v", i, err)
		}
	}
}
