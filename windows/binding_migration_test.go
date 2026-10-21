package main

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

func TestBindingMigrationAmbiguousVaultImport(t *testing.T) {
	for _, name := range []string{"explicit-success", "missing-id", "wrong-kind"} {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t)
			one := f.payload.Secrets[0].Value
			two := "00000000-0000-4000-8000-000000000002"
			if err := f.app.vault.ImportSlots([]secretvault.Slot{{ID: "uuid-one", Kind: "vless-uuid", Value: one}, {ID: "uuid-two", Kind: "vless-uuid", Value: two}}); err != nil {
				t.Fatal(err)
			}
			f.env.Channels[0].SecretIDs = nil
			second := f.env.Channels[0].Clone()
			second.ID = "synthetic-ws-two"
			f.env.Channels = append(f.env.Channels, second)
			legacy := f.seal(t)
			before := snapshotImport(t, f.paths)
			if _, err := f.app.PreviewOnboardingString(legacy, "synthetic-passphrase"); !errors.Is(err, secretvault.ErrAmbiguousKind) {
				t.Fatalf("fixture did not reach legacy ambiguity: %v", err)
			}
			if _, err := f.app.ApplyOnboardingString(legacy, "synthetic-passphrase", true); !errors.Is(err, secretvault.ErrAmbiguousKind) {
				t.Fatalf("legacy Apply ambiguity: %v", err)
			}
			assertImportUnchanged(t, f.paths, before)
			plan := &metadata.BindingMigration{Schema: 1, MetadataVersion: f.env.MetadataVersion + 1, IssuedAt: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour), Channels: []metadata.ChannelBindings{
				{ChannelID: f.env.Channels[0].ID, Secrets: []metadata.CredentialBinding{{Ref: "vless-uuid", SecretID: "uuid-one"}}},
				{ChannelID: second.ID, Secrets: []metadata.CredentialBinding{{Ref: "vless-uuid", SecretID: "uuid-two"}}},
			}}
			if name == "missing-id" {
				plan.Channels[1].Secrets[0].SecretID = "nonexistent-id"
			}
			if name == "wrong-kind" {
				plan.Channels[1].Secrets[0].SecretID = "old-slot"
			}
			migrated, err := metadata.MigrateLegacyBindings(f.env, plan, map[string]ed25519.PublicKey{f.env.KeyID: f.priv.Public().(ed25519.PublicKey)}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			f.env = migrated
			transport := f.seal(t)
			preview, previewErr := f.app.PreviewOnboardingString(transport, "synthetic-passphrase")
			assertImportUnchanged(t, f.paths, before)
			if name != "explicit-success" {
				_, applyErr := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true)
				for _, err := range []error{previewErr, applyErr} {
					if err == nil {
						t.Fatal("invalid exact binding accepted")
					}
					if name == "missing-id" && !errors.Is(err, secretvault.ErrNotFound) {
						t.Fatalf("wrong missing-ID rejection: %v", err)
					}
					if name == "wrong-kind" && !strings.Contains(err.Error(), "kind mismatch") {
						t.Fatalf("wrong type rejection: %v", err)
					}
				}
				assertImportUnchanged(t, f.paths, before)
				return
			}
			if previewErr != nil || preview.Channels != 2 {
				t.Fatalf("migration preview: %v", previewErr)
			}
			if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err != nil {
				t.Fatal(err)
			}
			channels, err := render.DescriptorsFromEnvelope(migrated)
			if err != nil {
				t.Fatal(err)
			}
			cc, err := render.RenderFrom(channels, &vaultSecrets{vault: f.app.vault}, render.WithDefaultChannel(channels[0].ID))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(cc.Raw)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{channels[0].ID: one, channels[1].ID: two}
			seen := 0
			for _, ob := range cfg.Outbounds {
				if value, ok := want[ob.Tag]; ok {
					seen++
					if ob.UUID != value {
						t.Fatal("exact IDs did not select distinct values")
					}
				}
			}
			if seen != 2 {
				t.Fatal("missing rendered channels")
			}
			for id, value := range map[string]string{"uuid-one": one, "uuid-two": two, "old-slot": "synthetic-original-password"} {
				got, err := f.app.vault.Value(id)
				if err != nil || got != value {
					t.Fatal("migration lost an existing slot")
				}
			}
			stored, present, err := f.store.LoadEnvelope()
			if err != nil || !present {
				t.Fatal(err)
			}
			if stored.MetadataVersion != plan.MetadataVersion {
				t.Fatal("migrated version not committed")
			}
			keys, _, err := f.store.LoadKeys()
			if err != nil {
				t.Fatal(err)
			}
			if err := stored.Verify(keys); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(f.paths[1])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(raw, f.payload.Envelope) {
				t.Fatal("checked migrated bytes changed")
			}
		})
	}
}
