package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestSecretIDLegacyAmbiguity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	m := secretvault.NewManager(path)
	if _, err := m.AddSave("hy2-password", "synthetic", "synthetic-password-one", ""); err != nil {
		t.Fatal(err)
	}
	// Duplicate only synthetic ciphertext, to reproduce a vault with two slots
	// before the ID-preserving import API exists. No owner data is involved.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f map[string]any
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	items := f["items"].(map[string]any)
	for _, entry := range items {
		items["second-synthetic-slot"] = entry
		break
	}
	data, err = json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if value, err := (&vaultSecrets{vault: m}).Get("hy2-password"); err == nil || value != "" {
		t.Fatal("ambiguous kind selected a value instead of rejecting")
	}
}

func TestSecretIDOnboardingPreservesIDs(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := newTestApp(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	env := &metadata.Envelope{
		SchemaVersion: metadata.SchemaVersion, MetadataVersion: ^uint64(0),
		IssuedAt: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour),
		KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{{
			ID: "synthetic-ws", Protocol: "vless", Transport: "ws", Hostname: "test.example", Port: 443,
			OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10",
			CredentialRefs: []string{"vless-uuid"}, ValidationStatus: "configured", Enabled: true,
		}},
	}
	if err := env.Sign(priv); err != nil {
		t.Fatal(err)
	}
	slots := []onboarding.SecretSlot{
		{ID: "synthetic-uuid", Kind: "vless-uuid", Value: "00000000-0000-4000-8000-000000000001"},
		{ID: "synthetic-hy2-one", Kind: "hy2-password", Value: "synthetic-password-one"},
		{ID: "synthetic-hy2-two", Kind: "hy2-password", Value: "synthetic-password-two"},
	}
	payload := &onboarding.BundlePayload{Schema: 1, CreatedAt: time.Now().UTC(),
		TrustedKeys: []onboarding.TrustedKey{{KeyID: env.KeyID, Public: pubHex(pub)}},
		Envelope:    mustJSON(t, env), Secrets: slots}
	transport, err := onboarding.Seal(payload, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyOnboardingString(transport, "synthetic-passphrase", true); err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots {
		value, err := a.vault.Value(slot.ID)
		if err != nil || value != slot.Value {
			t.Fatalf("import lost slot identity %s: %v", slot.ID, err)
		}
	}
}
