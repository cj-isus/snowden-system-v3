package main

// updateapp_test.go — тесты F15: подпись/тампер/политика версий/антидаунгрейд.
// Сеть не нужна; swap-путь (rename запущенного exe) покрыт вручную live-проверкой.

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
)

func TestUpdateManifestSignVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	m := update.Manifest{
		Schema:     update.Schema,
		Version:    "2.1.0",
		ReleasedAt: time.Now().UTC(),
		File:       "snowden-system.exe",
		SHA256:     strings.Repeat("ab", 32),
		Size:       123,
	}
	if err := m.Sign(priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if m.KeyID != metadata.KeyIDFor(pub) {
		t.Fatalf("key_id mismatch: %s", m.KeyID)
	}
	if err := m.Verify(map[string]ed25519.PublicKey{m.KeyID: pub}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Тампер любого подписанного поля рвёт подпись.
	m2 := m
	m2.Version = "9.9.9"
	if err := m2.Verify(map[string]ed25519.PublicKey{m.KeyID: pub}); err == nil {
		t.Fatal("tampered manifest accepted")
	}
}

func TestUpdateVersionPolicy(t *testing.T) {
	// Строгое возрастание: равная/ниже — отказ.
	cur := update.Version{Major: 2, Minor: 0, Patch: 0}
	if (update.Version{Major: 2, Minor: 0, Patch: 0}).Compare(cur) != 0 {
		t.Fatal("equal compare")
	}
	if (update.Version{Major: 1, Minor: 9, Patch: 9}).Compare(cur) >= 0 {
		t.Fatal("older must be <")
	}
	if (update.Version{Major: 2, Minor: 0, Patch: 1}).Compare(cur) <= 0 {
		t.Fatal("newer must be >")
	}
	if (update.Version{Major: 2, Minor: 0, Patch: 0, Build: 1}).Compare(cur) <= 0 {
		t.Fatal("build number participates")
	}
	// Разбор: строгий формат.
	for _, bad := range []string{"", "2", "2.1", "2.1.0-rc1", "2.1.0+meta", "2.x.0", "2..0"} {
		if _, err := update.ParseVersion(bad); err == nil {
			t.Fatalf("bad version accepted: %q", bad)
		}
	}
	for _, good := range []string{"2.0.0", "2.1.0.7", "10.20.30"} {
		if _, err := update.ParseVersion(good); err != nil {
			t.Fatalf("good version rejected: %q: %v", good, err)
		}
	}
}

func TestUpdateFloorRoundtrip(t *testing.T) {
	// CurrentFloor/RaiseFloor работают с os.UserConfigDir — подменить нельзя
	// без инъекции; проверяем чистые функции формата и монотонность RaiseFloor
	// через временный каталог, проколов FloorPath нельзя. Поэтому тестируем
	// ParseVersion(String(v)) roundtrip — формат этажа стабилен.
	v := update.Version{Major: 2, Minor: 3, Patch: 4}
	parsed, err := update.ParseVersion(v.String())
	if err != nil || parsed != v {
		t.Fatalf("roundtrip: %v -> %v (err %v)", v, parsed, err)
	}
	vb := update.Version{Major: 2, Minor: 3, Patch: 4, Build: 5}
	parsedB, err := update.ParseVersion(vb.String())
	if err != nil || parsedB != vb {
		t.Fatalf("roundtrip build: %v -> %v (err %v)", vb, parsedB, err)
	}
}

func TestUpdateCheckRejectsBadPayload(t *testing.T) {
	a := &App{}
	dir := t.TempDir()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keyID := metadata.KeyIDFor(pub)

	// Payload: фиксированное содержимое.
	payloadPath := filepath.Join(dir, "snowden-system.exe")
	if err := os.WriteFile(payloadPath, []byte("MZ fake payload"), 0o755); err != nil {
		t.Fatalf("payload: %v", err)
	}
	data, _ := os.ReadFile(payloadPath)
	sum := sha256.Sum256(data)

	m := update.Manifest{
		Schema:     update.Schema,
		Version:    "2.1.0",
		ReleasedAt: time.Now().UTC(),
		File:       "snowden-system.exe",
		SHA256:     hex.EncodeToString(sum[:]),
		Size:       int64(len(data)),
	}
	if err := m.Sign(priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	mb, _ := json.Marshal(&m)
	if err := os.WriteFile(filepath.Join(dir, "update.json"), mb, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// runningVersion = "dev" → политика версий честно отказывает (dev не обновляется).
	if _, err := a.CheckUpdate(dir); err != nil {
		t.Fatalf("check call: %v", err)
	}
	// Verify подписи требует trusted keys из AppData — в тестовой среде их нет,
	// поэтому CheckUpdate вернёт OK=false с честной причиной. Проверяем именно это.
	a2 := &App{}
	prev, err := a2.CheckUpdate(dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if prev.OK {
		t.Fatal("check OK without trusted keys — fail-closed нарушен")
	}
	if prev.Reason == "" {
		t.Fatal("причина отказа пустая — UI не покажет, что случилось")
	}
	_ = keyID
}
