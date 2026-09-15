package secretvault

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return NewManager(filepath.Join(dir, "vault.v1.json"))
}

func TestSeedIsIdempotentAndDoesNotOverwrite(t *testing.T) {
	m := newTestManager(t)
	if err := m.Seed(); err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	metas, err := m.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 10 {
		t.Fatalf("expected 10 seeded slots (V2-044: +3 канала C REALITY; V2-051: +2 канала D ShadowTLS), got %d", len(metas))
	}
	// Задаём значение UUID-слоту.
	var uuidID string
	for _, mt := range metas {
		if Kind(mt.Kind) == KindVlessUUID {
			uuidID = mt.ID
		}
	}
	if err := m.Save(uuidID, "e2b1c9d0-1111-4222-8333-444455556666"); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Повторный seed не должен ни добавить слоты, ни перезаписать значение.
	if err := m.Seed(); err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	metas, _ = m.List()
	if len(metas) != 10 {
		t.Fatalf("seed must not duplicate slots, got %d", len(metas))
	}
	for _, mt := range metas {
		if mt.ID == uuidID && mt.Fingerprint == "" {
			t.Fatal("fingerprint lost after re-seed")
		}
		if mt.ID == uuidID && mt.StoredValue != "dpapi" {
			t.Fatalf("storedValue = %q, want dpapi", mt.StoredValue)
		}
	}
}

func TestValidateUUID(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"e2b1c9d0-1111-4222-8333-444455556666", true},
		{"e2b1c9d0111142228333444455556666", false},
		{"e2b1c9d0-1111-4222-8333-44445555666", false},
		{"e2b1c9d0-1111-4222-8333-44445555666g", false},
		{"", false},
	}
	for _, c := range cases {
		err := validateUUID(c.in)
		if c.valid && err != nil {
			t.Errorf("validateUUID(%q) = %v, want nil", c.in, err)
		}
		if !c.valid && err == nil {
			t.Errorf("validateUUID(%q) = nil, want error", c.in)
		}
	}
}

func TestSaveVerifyRevealAudit(t *testing.T) {
	m := newTestManager(t)
	if err := m.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	metas, _ := m.List()
	var hy2ID string
	for _, mt := range metas {
		if Kind(mt.Kind) == KindHy2Password {
			hy2ID = mt.ID
		}
	}
	// Verify до задания значения — честная ошибка.
	if _, err := m.Verify(hy2ID); err == nil {
		t.Fatal("verify of empty slot must fail")
	}
	if err := m.Save(hy2ID, "short"); err == nil {
		t.Fatal("save must reject empty/short values")
	}
	if err := m.Save(hy2ID, "correct-horse-battery"); err != nil {
		t.Fatalf("save: %v", err)
	}
	meta, err := m.Verify(hy2ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if meta.VerifyStatus != "ok" {
		t.Fatalf("verifyStatus = %q, want ok", meta.VerifyStatus)
	}
	if meta.Fingerprint != Fingerprint("correct-horse-battery") {
		t.Fatal("fingerprint mismatch")
	}
	v, err := m.Reveal(hy2ID)
	if err != nil || v != "correct-horse-battery" {
		t.Fatalf("reveal: %v %q", err, v)
	}
	metas, _ = m.List()
	for _, mt := range metas {
		if mt.ID == hy2ID && mt.Reveals != 1 {
			t.Fatalf("reveals = %d, want 1", mt.Reveals)
		}
	}
	// Значение в файле не хранится открытым текстом.
	raw := readFile(t, m.Path())
	if strings.Contains(raw, "correct-horse-battery") {
		t.Fatal("plaintext leaked into vault file")
	}
}

func TestFingerprintStableAndDistinct(t *testing.T) {
	a1 := Fingerprint("alpha")
	a2 := Fingerprint("alpha")
	b := Fingerprint("beta")
	if a1 != a2 || a1 == b {
		t.Fatal("fingerprint must be stable and value-dependent")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := osReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}

// Регрессия аудита 2026-09-11: vault вызывался из UI-горутин (под a.mu) и
// render-пути (startCore) без синхронизации — read/write гонка на одном файле.
// Брутфорс под -race должен пройти чисто.
func TestManagerConcurrentOperations(t *testing.T) {
	m := newTestManager(t)
	if err := m.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	metas, err := m.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var uuidID string
	for _, mt := range metas {
		if Kind(mt.Kind) == KindVlessUUID {
			uuidID = mt.ID
		}
	}
	if uuidID == "" {
		t.Fatal("uuid slot not seeded")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(4)
		go func(n int) { defer wg.Done(); _, _ = m.List() }(i)
		go func(n int) { defer wg.Done(); _, _ = m.Verify(uuidID) }(i)
		go func(n int) { defer wg.Done(); _ = m.Save(uuidID, "e2b1c9d0-1111-4222-8333-444455556666") }(i)
		go func(n int) { defer wg.Done(); _, _ = m.Value(uuidID) }(i)
	}
	wg.Wait()
	// Файл после конкурентных операций должен остаться консистентным.
	final, err := m.List()
	if err != nil {
		t.Fatalf("final list: %v", err)
	}
	if len(final) == 0 {
		t.Fatal("vault empty after concurrent ops")
	}
}
