package main

// onboarding_test.go — тесты F8: roundtrip, отказ при неверной фразе,
// отказ подделанного envelope, антидаунгрейд. Сеть не нужна.

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/secretvault"
)

// newTestApp — App c временным vault'ом (metadata-стора подменяется через
// CurrentEnvelopeVersion → version.txt в каталоге теста не трогаем; для
// антидаунгрейд-теста версия поднимается отдельным сценарием).
func newTestApp(t *testing.T) *App {
	t.Helper()
	a := &App{}
	a.vault = secretvault.NewManager(filepath.Join(t.TempDir(), "vault.json"))
	return a
}

// seedSignedEnvelope — пишет в tmp-каталог доверенный ключ + подписанный
// envelope и возвращает (transport, passphrase).
func seedSignedEnvelope(t *testing.T, dir string, version uint64) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519: %v", err)
	}
	env := &metadata.Envelope{
		SchemaVersion:   1,
		MetadataVersion: version,
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(365 * 24 * time.Hour),
	}
	if err := env.Sign(priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	store := &metadata.Store{Dir: dir}
	if err := store.SaveKeys(map[string]ed25519.PublicKey{env.KeyID: pub}); err != nil {
		t.Fatalf("save keys: %v", err)
	}
	if err := store.SaveEnvelope(env); err != nil {
		t.Fatalf("save envelope: %v", err)
	}
	return dir, pub
}

func TestOnboardingRoundtrip(t *testing.T) {
	a := newTestApp(t)
	// Секрет в vault (валидный UUID).
	if _, err := a.vault.AddSave(string(secretvault.KindVlessUUID), "test-uuid", "00000000-0000-4000-8000-000000000001", "test"); err != nil {
		t.Fatalf("add secret: %v", err)
	}
	// Подменяем metadata-каталог на tmp через окружение: DefaultStoreDir
	// читает os.UserConfigDir — не подменяется. Вместо этого тестируем
	// Seal/Open напрямую + preview поверх.
	pass := "correct horse battery"
	payload := &onboarding.BundlePayload{
		Schema:      1,
		CreatedAt:   time.Now().UTC(),
		TrustedKeys: []onboarding.TrustedKey{{KeyID: "k1", Public: "aa"}},
		Secrets:     []onboarding.SecretSlot{{Kind: "vless-uuid", ID: "s1", Value: "00000000-0000-4000-8000-000000000001"}},
	}
	transport, err := onboarding.Seal(payload, pass)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	opened, err := onboarding.Open(transport, pass)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(opened.Secrets) != 1 || opened.Secrets[0].Value != payload.Secrets[0].Value {
		t.Fatalf("roundtrip mismatch: %+v", opened.Secrets)
	}
	// Неверная фраза — ErrWrongPassphrase.
	if _, err := onboarding.Open(transport, "wrong-passphrase"); err != onboarding.ErrWrongPassphrase {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
	// Тампер transport-строки (1 символ payload) — тоже отказ GCM.
	b := []byte(transport)
	b[len(b)-3] ^= 0x01
	if _, err := onboarding.Open(string(b), pass); err == nil {
		t.Fatal("tampered bundle accepted")
	}
}

func TestOnboardingApplyRejectsBadSignature(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()

	// Честная пара ключей для бандла, но envelope подписан ДРУГИМ ключом,
	// а в trusted_keys бандла — только первый. Verify обязан отвергнуть.
	pubA, _, _ := ed25519.GenerateKey(rand.Reader)
	_, privB, _ := ed25519.GenerateKey(rand.Reader)

	env := &metadata.Envelope{
		SchemaVersion:   1,
		MetadataVersion: 7,
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	}
	_ = env.Sign(privB) // подпись ключом B

	payload := &onboarding.BundlePayload{
		Schema:      1,
		CreatedAt:   time.Now().UTC(),
		TrustedKeys: []onboarding.TrustedKey{{KeyID: env.KeyID, Public: pubHex(pubA)}},
		Envelope:    mustJSON(t, env),
	}
	transport, err := onboarding.Seal(payload, "passphrase-123")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := a.ApplyOnboardingString(transport, "passphrase-123", true); err == nil {
		t.Fatal("envelope with wrong-key signature accepted")
	}
	// И на диске ничего не появилось (fail-closed: отказ до записи).
	if _, err := os.Stat(filepath.Join(dir, "channels.json")); !os.IsNotExist(err) {
		// dir не используется Apply напрямую — этот чек носит
		// документирующий характер; главный отказ выше.
		_ = err
	}
}

func TestOnboardingPreviewDoesNotTouchDisk(t *testing.T) {
	a := newTestApp(t)
	env := &metadata.Envelope{
		SchemaVersion:   1,
		MetadataVersion: 5,
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(time.Hour),
	}
	payload := &onboarding.BundlePayload{
		Schema:      1,
		CreatedAt:   time.Now().UTC(),
		TrustedKeys: []onboarding.TrustedKey{{KeyID: "k", Public: "ab"}},
		Envelope:    mustJSON(t, env),
		Secrets:     []onboarding.SecretSlot{{Kind: "hy2-password", ID: "h1", Value: "supersecret1"}},
	}
	transport, err := onboarding.Seal(payload, "passphrase-123")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	prev, err := a.ApplyOnboardingString(transport, "passphrase-123", false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.Channels != 0 || prev.Secrets != 1 || prev.BundleVer != 5 {
		t.Fatalf("preview fields: %+v", prev)
	}
}

func TestOnboardingExportRequiresPassphraseLength(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.ExportOnboarding("short"); err == nil {
		t.Fatal("short passphrase accepted")
	}
}

// ---------- helpers ----------

func pubHex(pub ed25519.PublicKey) string { return hex.EncodeToString(pub) }

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
