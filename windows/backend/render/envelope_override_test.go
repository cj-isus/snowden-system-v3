package render

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

// withEnvelopeStore — подмена AppData-хранилища на временный каталог.
func withEnvelopeStore(t *testing.T) *metadata.Store {
	t.Helper()
	dir := t.TempDir()
	store := &metadata.Store{Dir: dir}
	old := envelopeStore
	envelopeStore = store
	t.Cleanup(func() { envelopeStore = old })
	return store
}

// signEnvelope — подготовить валидно подписанный envelope с одним каналом.
func signEnvelope(t *testing.T, store *metadata.Store, ch metadata.Channel, version uint64, revocations []string) *metadata.Envelope {
	return signEnvelopeEvidence(t, store, ch, version, revocations, true)
}

func signEnvelopeEvidence(t *testing.T, store *metadata.Store, ch metadata.Channel, version uint64, revocations []string, withEvidence bool) *metadata.Envelope {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveKeys(map[string]ed25519.PublicKey{metadata.KeyIDFor(pub): pub}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	va := "2026-09-09"
	e := &metadata.Envelope{
		SchemaVersion:   metadata.SchemaVersion,
		MetadataVersion: version,
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(24 * time.Hour),
		KeyID:           metadata.KeyIDFor(pub),
		Channels:        []metadata.Channel{ch},
		Revocations:     revocations,
	}
	// evidenceTuple — параметр: false = специально без validated_at (для
	// теста статус-гейта); true = честный evidence tuple.
	if withEvidence {
		e.Channels[0].ValidatedAt = &va
	}
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEnvelope(e); err != nil {
		t.Fatal(err)
	}
	return e
}

// envelopeChannel — тестовый дескриптор в формате envelope.
func envelopeChannel() metadata.Channel {
	return metadata.Channel{
		ID: "channel-x-test", Protocol: "vless", Transport: "ws",
		Hostname: "test.example", Port: 443,
		OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10",
		CredentialRefs: []string{"vless-uuid"}, ValidationStatus: "configured",
		Enabled: true,
	}
}

func TestLoadDescriptorsWithoutEnvelopeUsesEmbedded(t *testing.T) {
	withEnvelopeStore(t)
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 3 || channels[0].ID != "channel-a-vless" {
		t.Fatalf("embedded set expected (3 канала, V2-044), got %+v", channels)
	}
}

func TestEnvelopeReplacesEmbeddedSet(t *testing.T) {
	store := withEnvelopeStore(t)
	signEnvelope(t, store, envelopeChannel(), 1, nil)
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != "channel-x-test" {
		t.Fatalf("envelope must replace embedded set, got %+v", channels)
	}
}

func TestTamperedEnvelopeFailsClosed(t *testing.T) {
	store := withEnvelopeStore(t)
	e := signEnvelope(t, store, envelopeChannel(), 1, nil)
	// Подделка payload после подписи.
	e.Channels[0].OriginServer = "203.0.113.66"
	if err := store.SaveEnvelope(e); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDescriptors(); err == nil {
		t.Fatal("tampered envelope must fail closed, not fall back to embedded")
	}
}

func TestEnvelopeWithoutKeysFailsClosed(t *testing.T) {
	store := withEnvelopeStore(t)
	signEnvelope(t, store, envelopeChannel(), 1, nil)
	if err := os.Remove(filepath.Join(store.Dir, "trusted_keys.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDescriptors(); err == nil {
		t.Fatal("envelope without trusted keys must fail closed")
	}
}

func TestEnvelopeDowngradeFailsClosed(t *testing.T) {
	store := withEnvelopeStore(t)
	signEnvelope(t, store, envelopeChannel(), 5, nil)
	if _, err := LoadDescriptors(); err != nil {
		t.Fatalf("v5 must be accepted: %v", err)
	}
	// Откат на более старую версию (даже валидно подписанную) — reject.
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_ = store.SaveKeys(map[string]ed25519.PublicKey{metadata.KeyIDFor(pub): pub})
	now := time.Now().UTC()
	old := &metadata.Envelope{
		SchemaVersion: metadata.SchemaVersion, MetadataVersion: 3,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(24 * time.Hour),
		KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{envelopeChannel()},
	}
	_ = old.Sign(priv)
	if err := store.SaveEnvelope(old); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDescriptors(); err == nil {
		t.Fatal("version downgrade must be rejected (anti-rollback)")
	}
}

func TestEnvelopeRevocationRemovesChannel(t *testing.T) {
	store := withEnvelopeStore(t)
	signEnvelope(t, store, envelopeChannel(), 1, []string{"channel-x-test"})
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatalf("revocation of the only channel must still load (render will refuse later), got: %v", err)
	}
	// Ревокация единственного канала — набор пуст; это валидный load, но
	// Selectable/RenderFrom откажутся. По контракту FR-008 fail-closed стоит
	// на этапе рендера, не загрузки. Проверяем, что канал удалён.
	for _, ch := range channels {
		if ch.ID == "channel-x-test" {
			t.Fatal("revoked channel must be removed")
		}
	}
}

func TestEnvelopeLiveVerifiedWithoutEvidenceIsDowngraded(t *testing.T) {
	store := withEnvelopeStore(t)
	ch := envelopeChannel()
	ch.ValidationStatus = "live-verified"
	signEnvelopeEvidence(t, store, ch, 1, nil, false) // нет validated_at
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if channels[0].ValidationStatus != "configured" {
		t.Fatalf("live-verified without evidence must be downgraded to configured, got %q", channels[0].ValidationStatus)
	}
	// Пониженный до configured канал остаётся selectable (осознанное правило
	// V2-032: иначе его невозможно когда-либо проверить live), но НЕ может
	// стать default и не выбирается авто-failover'ом. Проверяем контракт:
	// Selectable его видит, но ValidationStatus честно "configured".
	got := Selectable(channels)
	if len(got) != 1 || got[0].ValidationStatus != "configured" {
		t.Fatalf("expected configured-only selectable channel, got %+v", got)
	}
}

func TestEnvelopeChannelPassesSameStrictGate(t *testing.T) {
	store := withEnvelopeStore(t)
	ch := envelopeChannel()
	ch.Protocol = "trojan" // неизвестный протокол — должен отбраковаться общим гейтом
	signEnvelope(t, store, ch, 1, nil)
	if _, err := LoadDescriptors(); err == nil {
		t.Fatal("envelope channel must pass the same strict descriptor gate")
	}
}

func TestEnvelopeRenderFailsClosedWhenAllRevoked(t *testing.T) {
	store := withEnvelopeStore(t)
	signEnvelope(t, store, envelopeChannel(), 1, []string{"channel-x-test"})
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderFrom(channels, &failingSecrets{}); err == nil {
		t.Fatal("render with zero channels must fail closed")
	}
}

// failingSecrets — заглушка, без значений (не используется до провала).
type failingSecrets struct{}

func (f *failingSecrets) Get(string) (string, error) { return "", os.ErrNotExist }
func (f *failingSecrets) PinCertPath(string) string  { return "" }

// V2-034: version.txt коммитится только после успешного прохождения строгого
// гейта. Невалидный envelope с ВЫСОКИМ version не должен фиксировать version
// (иначе после исправления envelope'а антидаунгрейд навсегда отвергал бы его).
func TestInvalidHighVersionEnvelopeDoesNotCommitVersion(t *testing.T) {
	store := withEnvelopeStore(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveKeys(map[string]ed25519.PublicKey{metadata.KeyIDFor(pub): pub}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	// Валидная подпись, но канал не пройдёт строгий гейт (неизвестный протокол).
	ch := envelopeChannel()
	ch.Protocol = "trojan"
	e := &metadata.Envelope{
		SchemaVersion: metadata.SchemaVersion, MetadataVersion: 42,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(24 * time.Hour),
		KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{ch},
	}
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEnvelope(e); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDescriptors(); err == nil {
		t.Fatal("invalid envelope must fail")
	}
	if v := readEnvelopeVersion(store.Dir); v != 0 {
		t.Fatalf("version must not be committed on failed gate, got %d", v)
	}
	// Теперь чиним envelope (валидный протокол, тот же version 42) — должен
	// приняться, потому что версия не была зафиксирована отказом.
	ch.Protocol = "vless"
	e2 := &metadata.Envelope{
		SchemaVersion: metadata.SchemaVersion, MetadataVersion: 42,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(24 * time.Hour),
		KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{ch},
	}
	if err := e2.Sign(priv); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEnvelope(e2); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDescriptors(); err != nil {
		t.Fatalf("fixed envelope must be accepted after failed one: %v", err)
	}
	if v := readEnvelopeVersion(store.Dir); v != 42 {
		t.Fatalf("version must be committed after success, got %d", v)
	}
}
