package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/snowden-system/windows/backend/filetxn"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/secretvault"
)

type importFixture struct {
	app     *App
	store   *metadata.Store
	paths   []string
	payload *onboarding.BundlePayload
	env     *metadata.Envelope
	priv    ed25519.PrivateKey
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	vaultPath := filepath.Join(t.TempDir(), "vault.json")
	f := &importFixture{app: &App{vault: secretvault.NewManager(vaultPath)}, store: &metadata.Store{Dir: dir}}
	f.paths = []string{f.store.KeysPath(), f.store.EnvelopePath(), filepath.Join(dir, "version.txt"), vaultPath}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.priv = priv
	f.env = &metadata.Envelope{SchemaVersion: 1, MetadataVersion: 2, IssuedAt: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour), KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{{
		ID: "synthetic-ws", Protocol: "vless", Transport: "ws", Hostname: "test.example", Port: 443, OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10", CredentialRefs: []string{"vless-uuid"}, SecretIDs: map[string]string{"vless-uuid": "uuid-one"}, ValidationStatus: "configured", Enabled: true,
	}}}
	f.payload = &onboarding.BundlePayload{Schema: 1, CreatedAt: time.Now().UTC(), TrustedKeys: []onboarding.TrustedKey{{KeyID: f.env.KeyID, Public: pubHex(pub)}}, Secrets: []onboarding.SecretSlot{{ID: "uuid-one", Kind: "vless-uuid", Value: "00000000-0000-4000-8000-000000000001"}}}
	oldPub, oldPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old := *f.env
	old.MetadataVersion = 1
	old.KeyID = metadata.KeyIDFor(oldPub)
	if err := old.Sign(oldPriv); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SaveKeys(map[string]ed25519.PublicKey{old.KeyID: oldPub}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SaveEnvelope(&old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.paths[2], []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.app.vault.ImportSlots([]secretvault.Slot{{ID: "old-slot", Kind: "hy2-password", Value: "synthetic-original-password"}}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *importFixture) seal(t *testing.T) string {
	t.Helper()
	if err := f.env.Sign(f.priv); err != nil {
		t.Fatal(err)
	}
	f.payload.Envelope = mustJSON(t, f.env)
	transport, err := onboarding.Seal(f.payload, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	return transport
}

func snapshotImport(t *testing.T, paths []string) [][]byte {
	t.Helper()
	var out [][]byte
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

func assertImportUnchanged(t *testing.T, paths []string, before [][]byte) {
	t.Helper()
	after := snapshotImport(t, paths)
	for i := range before {
		if !bytes.Equal(before[i], after[i]) {
			t.Errorf("rejected import changed file %d", i)
		}
	}
}

func TestImportStrictPreviewRejectsInvalidSignature(t *testing.T) {
	f := newImportFixture(t)
	transport := f.seal(t)
	payload, err := onboarding.Open(transport, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	var env metadata.Envelope
	if err := json.Unmarshal(payload.Envelope, &env); err != nil {
		t.Fatal(err)
	}
	env.Signature = "invalid"
	payload.Envelope = mustJSON(t, &env)
	transport, err = onboarding.Seal(payload, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotImport(t, f.paths)
	if preview, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase"); err == nil || preview != nil {
		t.Error("preview accepted an unverified envelope")
	}
	assertImportUnchanged(t, f.paths, before)
}

func TestImportRejectedApplyPreservesAllFiles(t *testing.T) {
	for _, name := range []string{"signature", "secret-format"} {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t)
			transport := f.seal(t)
			if name == "signature" {
				f.env.Signature = "invalid"
				f.payload.Envelope = mustJSON(t, f.env)
			} else {
				f.payload.Secrets[0].Value = "synthetic-invalid-uuid"
			}
			var err error
			transport, err = onboarding.Seal(f.payload, "synthetic-passphrase")
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotImport(t, f.paths)
			if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err == nil {
				t.Fatal("invalid import accepted")
			}
			assertImportUnchanged(t, f.paths, before)
		})
	}
}

func TestImportStrictValidationMatrix(t *testing.T) {
	for _, name := range []string{"key-id", "duplicate-key", "bad-key", "schema", "expired", "zero-version", "downgrade", "corrupt-floor", "protocol", "missing-ref", "missing-secret", "wrong-kind", "duplicate-slot", "partial-binding", "unknown-envelope-field"} {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t)
			switch name {
			case "key-id":
				f.payload.TrustedKeys[0].KeyID = "wrong"
			case "duplicate-key":
				f.payload.TrustedKeys = append(f.payload.TrustedKeys, f.payload.TrustedKeys[0])
			case "bad-key":
				f.payload.TrustedKeys[0].Public = "invalid"
			case "schema":
				f.env.SchemaVersion = 99
			case "expired":
				f.env.IssuedAt = time.Now().Add(-2 * time.Hour)
				f.env.ExpiresAt = time.Now().Add(-time.Hour)
			case "zero-version":
				f.env.MetadataVersion = 0
			case "downgrade":
				if err := os.WriteFile(f.paths[2], []byte("3"), 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt-floor":
				if err := os.WriteFile(f.paths[2], []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			case "protocol":
				f.env.Channels[0].Protocol = "unknown"
			case "missing-ref":
				f.env.Channels[0].CredentialRefs = []string{"unknown"}
				f.env.Channels[0].SecretIDs = nil
			case "missing-secret":
				f.payload.Secrets = nil
			case "wrong-kind":
				f.payload.Secrets[0].Kind = "hy2-password"
				f.payload.Secrets[0].Value = "synthetic-password"
			case "duplicate-slot":
				f.payload.Secrets = append(f.payload.Secrets, f.payload.Secrets[0])
			case "partial-binding":
				f.env.Channels[0].SecretIDs["extra"] = "extra-slot"
			}
			transport := f.seal(t)
			if name == "unknown-envelope-field" {
				var raw map[string]any
				if err := json.Unmarshal(f.payload.Envelope, &raw); err != nil {
					t.Fatal(err)
				}
				raw["unknown"] = true
				f.payload.Envelope = mustJSON(t, raw)
				var err error
				transport, err = onboarding.Seal(f.payload, "synthetic-passphrase")
				if err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotImport(t, f.paths)
			if p, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase"); err == nil || p != nil {
				t.Fatal("invalid preview accepted")
			}
			if p, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err == nil || p != nil {
				t.Fatal("invalid Apply accepted")
			}
			assertImportUnchanged(t, f.paths, before)
		})
	}
}

func TestImportCommitAndCheckedBytes(t *testing.T) {
	f := newImportFixture(t)
	f.env.SplitDirect = []string{"BANK.EXE"} // validation must not change signed bytes
	transport := f.seal(t)
	checked := bytes.Clone(f.payload.Envelope)
	before := snapshotImport(t, f.paths)
	preview, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if preview.CurrentVer != 1 || len(preview.Keys) != 1 {
		t.Fatal("preview facts missing")
	}
	assertImportUnchanged(t, f.paths, before)
	result, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentVer != 2 || result.BundleVer != 2 {
		t.Fatal("commit version not published")
	}
	stored, err := os.ReadFile(f.paths[1])
	if err != nil || !bytes.Equal(stored, checked) {
		t.Fatal("stored envelope differs from checked bytes")
	}
	applied, err := f.store.Apply(time.Now(), 2)
	if err != nil {
		t.Fatal("stored signature broken:", err)
	}
	keys, _, err := f.store.LoadKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := applied.Verify(keys); err != nil {
		t.Fatal("validation mutated the signed envelope:", err)
	}
	if value, err := f.app.vault.ValueFor("uuid-one", "vless-uuid"); err != nil || value != f.payload.Secrets[0].Value {
		t.Fatal("vault not committed")
	}
	if _, err := f.app.vault.ValueFor("old-slot", "hy2-password"); err != nil {
		t.Fatal("unrelated slot lost")
	}
	if err := filetxn.Check(f.store.ImportJournal()); err != nil {
		t.Fatal(err)
	}
}

func TestImportCommitFailurePreservesAllFiles(t *testing.T) {
	f := newImportFixture(t)
	transport := f.seal(t)
	before := snapshotImport(t, f.paths)
	f.app.onboardingCommit = func(_ *filetxn.Store, files []filetxn.File) error {
		if len(files) != 4 || bytes.Contains(files[3].Data, []byte(f.payload.Secrets[0].Value)) {
			t.Fatal("staging leaked plaintext or lost files")
		}
		return errors.New("synthetic commit rejected")
	}
	if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err == nil {
		t.Fatal("commit failure accepted")
	}
	assertImportUnchanged(t, f.paths, before)
}

func TestImportPendingJournalBlocksReads(t *testing.T) {
	f := newImportFixture(t)
	transport := f.seal(t)
	before := snapshotImport(t, f.paths)
	if err := os.Mkdir(f.store.ImportJournal(), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase"); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("preview ignored pending journal:", err)
	}
	if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("apply ignored pending journal:", err)
	}
	if _, _, err := f.store.LoadKeys(); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("keys ignored journal")
	}
	if _, _, err := f.store.LoadEnvelope(); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("envelope ignored journal")
	}
	if _, err := f.app.vault.Value("old-slot"); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("vault ignored journal")
	}
	if err := f.app.recoverOnboardingImport(); !errors.Is(err, filetxn.ErrRecoveryRequired) {
		t.Fatal("corrupt journal accepted")
	}
	assertImportUnchanged(t, f.paths, before)
}

func TestImportConcurrentVersions(t *testing.T) {
	f := newImportFixture(t)
	first := f.seal(t)
	f.env.MetadataVersion = 3
	f.payload.Secrets[0].Value = "00000000-0000-4000-8000-000000000002"
	second := f.seal(t)
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	f.app.onboardingCommit = func(tx *filetxn.Store, files []filetxn.File) error {
		calls++
		if calls == 1 {
			close(entered)
			<-release
		}
		return tx.Commit(files)
	}
	results := make(chan error, 2)
	go func() { _, err := f.app.ApplyOnboardingString(first, "synthetic-passphrase", true); results <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first commit not reached")
	}
	go func() { _, err := f.app.ApplyOnboardingString(second, "synthetic-passphrase", true); results <- err }()
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent import hung")
		}
	}
	if calls != 2 {
		t.Fatal("commit count")
	}
	floor, err := readImportFloor(f.store.Dir)
	if err != nil || floor != 3 {
		t.Fatal("wrong final floor")
	}
	value, err := f.app.vault.ValueFor("uuid-one", "vless-uuid")
	if err != nil || value != f.payload.Secrets[0].Value {
		t.Fatal("old import overwrote new credentials")
	}
	before := snapshotImport(t, f.paths)
	if _, err := f.app.ApplyOnboardingString(first, "synthetic-passphrase", true); err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatal(fmt.Sprint("older queued profile accepted: ", err))
	}
	assertImportUnchanged(t, f.paths, before)
}

func TestImportFreshRecoveryRestoresAllFiles(t *testing.T) {
	f := newImportFixture(t)
	before := snapshotImport(t, f.paths)
	journal := f.store.ImportJournal()
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	entries := []map[string]any{}
	for i, name := range []string{"keys", "envelope", "version", "vault"} {
		h := sha256.Sum256(before[i])
		entries = append(entries, map[string]any{"name": name, "existed": true, "sha256": hex.EncodeToString(h[:])})
		if err := os.WriteFile(filepath.Join(journal, name+".old"), before[i], 0600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(map[string]any{"schema": 1, "entries": entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journal, "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.paths {
		if err := os.WriteFile(p, []byte("interrupted synthetic import"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fresh := &App{vault: secretvault.NewManager(f.paths[3])}
	if err := fresh.recoverOnboardingImport(); err != nil {
		t.Fatal(err)
	}
	assertImportUnchanged(t, f.paths, before)
	if _, err := fresh.vault.Value("old-slot"); err != nil {
		t.Fatal(err)
	}
	if err := fresh.recoverOnboardingImport(); err != nil {
		t.Fatal(err)
	}
}
