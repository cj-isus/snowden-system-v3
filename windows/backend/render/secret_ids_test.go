package render

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/secretvault"
)

type exactVaultSource struct {
	vault       *secretvault.Manager
	pins        map[string]string
	legacyReads int
}

func (s *exactVaultSource) Get(string) (string, error) {
	s.legacyReads++
	return "", fmt.Errorf("unexpected legacy lookup")
}
func (s *exactVaultSource) GetByID(id, kind string) (string, error) {
	return s.vault.ValueFor(id, kind)
}
func (s *exactVaultSource) PinCertPath(id string) string { return s.pins[id] }

func TestSecretIDSignedTransportRoles(t *testing.T) {
	testSignedTransportRoles(t, false)
}

func TestProfileSchemaSignedTransportRoles(t *testing.T) {
	testSignedTransportRoles(t, true)
}

func testSignedTransportRoles(t *testing.T, tuned bool) {
	t.Helper()
	store := withEnvelopeStore(t)
	channels, err := parseDescriptors(descriptorsRaw)
	if err != nil {
		t.Fatal(err)
	}
	// A second HY2 instance uses the same roles/kinds, distinct IDs and values.
	for _, ch := range channels {
		if ch.Protocol == "hysteria2" {
			ch.ID = "synthetic-hy2-second"
			channels = append(channels, ch)
			break
		}
	}
	base := testSecrets(false)
	key, err := rsaGen()
	if err != nil {
		t.Fatal(err)
	}
	pin := filepath.Join(t.TempDir(), "pin.pem")
	if err := os.WriteFile(pin, []byte(pemOf(key)), 0600); err != nil {
		t.Fatal(err)
	}
	src := &exactVaultSource{vault: secretvault.NewManager(filepath.Join(t.TempDir(), "vault.json")), pins: map[string]string{}}
	var slots []secretvault.Slot
	var signedChannels []metadata.Channel
	wantValues := map[string]map[string]string{}
	for i := range channels {
		ch := &channels[i]
		ch.ValidationStatus = "configured"
		if tuned {
			ch.ProbeTargets = []string{"https://primary.example/ip", "https://secondary.example/ip"}
			ch.ProbePrimaryHost = "primary.example"
			ch.DiagnosticURL = "https://primary.example/bytes"
			ch.FD = FailureDomain{TransportFamily: ch.Protocol + "+" + ch.Transport, Carrier: "synthetic-cdn", Origin: ch.OriginServer, ASN: "AS64500", PortFamily: "443", Layer: "tcp", DonorSNI: "donor.example"}
			if ch.Protocol == "hysteria2" {
				ch.ServerPorts, ch.HopInterval = []string{"20000:20010"}, "30s"
				ch.UpMbps, ch.DownMbps = 10, 20
				ch.FD.Layer = "udp"
			}
			if ch.Transport == "ws" || ch.Transport == "tcp-reality" {
				value := ch.Transport == "tcp-reality"
				ch.Multiplex = &value
			}
		}
		ch.SecretIDs = map[string]string{}
		wantValues[ch.ID] = map[string]string{}
		for j, ref := range ch.CredentialRefs {
			id := fmt.Sprintf("synthetic-slot-%d-%d", i, j)
			kind, err := credentialKind(*ch, ref)
			if err != nil {
				t.Fatal(err)
			}
			value := base.values[ref]
			if ch.Protocol == "hysteria2" {
				value = fmt.Sprintf("synthetic-hy2-value-%d-%d", i, j)
				src.pins[ch.ID] = pin
			}
			ch.SecretIDs[ref] = id
			wantValues[ch.ID][ref] = value
			slots = append(slots, secretvault.Slot{ID: id, Kind: kind, Value: value})
		}
		signedChannels = append(signedChannels, metadata.Channel(*ch).Clone())
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	env := &metadata.Envelope{SchemaVersion: metadata.SchemaVersion, MetadataVersion: 1, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), KeyID: metadata.KeyIDFor(pub), Channels: signedChannels}
	if err := env.Sign(priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	payload := &onboarding.BundlePayload{Schema: 1, CreatedAt: now, Envelope: raw, TrustedKeys: []onboarding.TrustedKey{{KeyID: env.KeyID, Public: hex.EncodeToString(pub)}}}
	for _, slot := range slots {
		payload.Secrets = append(payload.Secrets, onboarding.SecretSlot{ID: slot.ID, Kind: slot.Kind, Value: slot.Value})
	}
	transport, err := onboarding.Seal(payload, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := onboarding.Open(transport, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := metadata.DecodeStrict(opened.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.Verify(map[string]ed25519.PublicKey{env.KeyID: pub}); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(now, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveKeys(map[string]ed25519.PublicKey{env.KeyID: pub}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEnvelope(decoded); err != nil {
		t.Fatal(err)
	}
	imported := make([]secretvault.Slot, len(opened.Secrets))
	for i, slot := range opened.Secrets {
		imported[i] = secretvault.Slot{ID: slot.ID, Kind: slot.Kind, Value: slot.Value}
	}
	if err := src.vault.ImportSlots(imported); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 5 {
		t.Fatalf("transport instances: %d", len(loaded))
	}
	for _, ch := range loaded {
		t.Run(ch.ID, func(t *testing.T) {
			for _, want := range signedChannels {
				if want.ID == ch.ID && !reflect.DeepEqual(metadata.Channel(ch), want) {
					t.Fatal("signed channel field lost")
				}
			}
			cc, err := RenderFrom([]ChannelDescriptor{ch}, src, WithDefaultChannel(ch.ID))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := config.Parse(cc.Raw); err != nil {
				t.Fatal(err)
			}
			if tuned {
				assertProfileRuntime(t, ch, cc)
			}
			var cfg struct {
				Outbounds []config.Outbound `json:"outbounds"`
			}
			if err := json.Unmarshal(cc.Raw, &cfg); err != nil {
				t.Fatal(err)
			}
			for _, ob := range cfg.Outbounds {
				switch {
				case ob.Tag == ch.ID && ch.Protocol == "hysteria2":
					if ob.Password != wantValues[ch.ID]["hy2-password"] || ob.Obfs["password"] != wantValues[ch.ID]["hy2-obfs-password"] {
						t.Fatal("HY2 instances shared credentials")
					}
				case ob.Tag == ch.ID && ch.Transport == "ws":
					if ob.UUID != wantValues[ch.ID]["vless-uuid"] {
						t.Fatal("WS UUID lost")
					}
				case ob.Tag == ch.ID && ch.Transport == "tcp-reality":
					if ob.UUID != wantValues[ch.ID][ch.UUIDRef] {
						t.Fatal("REALITY UUID lost")
					}
					reality, ok := ob.TLS["reality"].(map[string]any)
					if !ok || reality["public_key"] != wantValues[ch.ID][ch.RealityPublicKeyRef] || reality["short_id"] != wantValues[ch.ID][ch.RealityShortIDRef] {
						t.Fatal("REALITY key/short-ID lost")
					}
				case ob.Tag == ch.ID && ch.Transport == "shadowtls":
					if ob.UUID != wantValues[ch.ID][ch.UUIDRef] {
						t.Fatal("ShadowTLS inner UUID lost")
					}
				case ob.Tag == ch.ID+"-st":
					if ob.Password != wantValues[ch.ID][ch.ShadowTLSPasswordRef] {
						t.Fatal("ShadowTLS password lost")
					}
				}
			}
		})
	}
	if src.legacyReads != 0 {
		t.Fatal("explicit IDs fell back to legacy lookup")
	}
}

func TestSecretIDRenderFailsClosed(t *testing.T) {
	for _, name := range []string{"missing-id", "kind-mismatch", "source-without-id-support", "partial-map", "undeclared-map"} {
		t.Run(name, func(t *testing.T) {
			ch := ChannelDescriptor{ID: "synthetic-ws", Protocol: "vless", Transport: "ws", Hostname: "test.example", Port: 443, OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10", CredentialRefs: []string{"vless-uuid"}, SecretIDs: map[string]string{"vless-uuid": "slot"}, ValidationStatus: "configured", Enabled: true}
			src := &exactVaultSource{vault: secretvault.NewManager(filepath.Join(t.TempDir(), "vault.json"))}
			var source SecretsSource = src
			switch name {
			case "kind-mismatch":
				if err := src.vault.ImportSlots([]secretvault.Slot{{ID: "slot", Kind: "hy2-password", Value: "synthetic-password"}}); err != nil {
					t.Fatal(err)
				}
			case "source-without-id-support":
				source = testSecrets(false)
			case "partial-map":
				ch.CredentialRefs = append(ch.CredentialRefs, "another-role")
			case "undeclared-map":
				ch.SecretIDs = map[string]string{"another-role": "slot"}
			}
			if cc, err := RenderFrom([]ChannelDescriptor{ch}, source, WithDefaultChannel(ch.ID)); err == nil || cc != nil {
				t.Fatal("invalid binding rendered a configuration")
			}
			if src.legacyReads != 0 {
				t.Fatal("failed exact lookup fell back to legacy")
			}
		})
	}
}

func TestSecretIDBindingValidation(t *testing.T) {
	for _, name := range []string{"missing", "empty", "path", "control", "long", "duplicate-ref"} {
		t.Run(name, func(t *testing.T) {
			ch := envelopeChannel()
			ch.SecretIDs = map[string]string{"vless-uuid": "slot"}
			switch name {
			case "missing":
				ch.SecretIDs = map[string]string{"wrong": "slot"}
			case "empty":
				ch.SecretIDs["vless-uuid"] = ""
			case "path":
				ch.SecretIDs["vless-uuid"] = "../slot"
			case "control":
				ch.SecretIDs["vless-uuid"] = "slot\n"
			case "long":
				ch.SecretIDs["vless-uuid"] = strings.Repeat("x", 129)
			case "duplicate-ref":
				ch.CredentialRefs = []string{"vless-uuid", "vless-uuid"}
				ch.SecretIDs["extra"] = "extra"
			}
			store := withEnvelopeStore(t)
			signEnvelope(t, store, ch, 1, nil)
			if _, err := LoadDescriptors(); err == nil {
				t.Fatal("invalid signed binding accepted")
			}
			if readEnvelopeVersion(store.Dir) != 0 {
				t.Fatal("invalid binding advanced version floor")
			}
		})
	}
}
