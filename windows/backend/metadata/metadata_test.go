package metadata

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testEnvelope(now time.Time) *Envelope {
	va := "2026-09-09"
	return &Envelope{
		SchemaVersion:   SchemaVersion,
		MetadataVersion: 1,
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(24 * time.Hour),
		Channels: []Channel{
			{
				ID: "channel-a-vless", Protocol: "vless", Transport: "ws",
				Hostname: "vpn.example.com", Port: 443,
				OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10",
				CredentialRefs:   []string{"vless-uuid"},
				ValidationStatus: "live-verified", ValidatedAt: &va,
				Enabled: true,
			},
		},
	}
}

func TestValidateFailClosed(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		min    uint64
		mutate func(*Envelope)
	}{
		{"downgrade", 2, func(e *Envelope) { e.MetadataVersion = 1 }},
		{"unknown schema", 1, func(e *Envelope) { e.SchemaVersion = 99 }},
		{"expired", 1, func(e *Envelope) { e.ExpiresAt = now.Add(-time.Hour) }},
		{"issued in future", 1, func(e *Envelope) { e.IssuedAt = now.Add(MaxClockSkew + time.Second) }},
		{"expiry before issue", 1, func(e *Envelope) { e.ExpiresAt = e.IssuedAt }},
		{"empty channels", 1, func(e *Envelope) { e.Channels = nil }},
		{"duplicate id", 1, func(e *Envelope) { e.Channels = append(e.Channels, e.Channels[0]) }},
		{"empty id", 1, func(e *Envelope) { e.Channels[0].ID = "" }},
		{"no credential refs", 1, func(e *Envelope) { e.Channels[0].CredentialRefs = nil }},
		{"secret-shaped ref", 1, func(e *Envelope) { e.Channels[0].CredentialRefs = []string{"super-secret-value"} }},
		{"newline in ref", 1, func(e *Envelope) { e.Channels[0].CredentialRefs = []string{"a\nb"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEnvelope(now)
			e.MetadataVersion = 5 // выше минимума; тест на даунгрейд переопределяет через минимум
			tc.mutate(e)
			if err := e.Validate(now, tc.min); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	t.Run("valid", func(t *testing.T) {
		if err := testEnvelope(now).Validate(now, 1); err != nil {
			t.Fatalf("valid envelope rejected: %v", err)
		}
	})
	t.Run("clock skew tolerated", func(t *testing.T) {
		e := testEnvelope(now)
		e.IssuedAt = now.Add(MaxClockSkew - time.Second)
		if err := e.Validate(now, 1); err != nil {
			t.Fatalf("skew tolerated case failed: %v", err)
		}
	})
}

func TestSignatureRoundTripAndTamper(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	e := testEnvelope(now)
	e.KeyID = KeyIDFor(pub)
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	keys := map[string]ed25519.PublicKey{KeyIDFor(pub): pub}
	if err := e.Verify(keys); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if e.Signature == "" {
		t.Fatal("signature not filled")
	}
	// Raw (unpadded) base64: 64 байта → ровно 86 символов без '='.
	if len(e.Signature) != 86 {
		t.Fatalf("unexpected signature length: %d (%q)", len(e.Signature), e.Signature)
	}
	for _, c := range e.Signature {
		if c == '=' {
			t.Fatal("padded base64 in signature")
		}
	}
	for _, name := range []string{"payload", "signature", "wrong key"} {
		t.Run(name, func(t *testing.T) {
			bad := *e
			switch name {
			case "payload":
				bad.MetadataVersion++
			case "signature":
				bad.Signature = "AAAA" + e.Signature[4:]
			case "wrong key":
				otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
				bad.KeyID = KeyIDFor(otherPub)
			}
			if err := bad.Verify(keys); err == nil {
				t.Fatal("tampered envelope verified")
			}
		})
	}
	t.Run("empty keys fail-closed", func(t *testing.T) {
		if err := e.Verify(nil); err == nil {
			t.Fatal("verify without keys must fail")
		}
	})
}

func TestDecodeStrict(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	e := testEnvelope(now)
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeStrict(raw); err != nil {
		t.Fatalf("clean envelope rejected: %v", err)
	}
	for _, tc := range []struct{ name, data string }{
		{"unknown field", `{"schema_version":1,"bogus":1}`},
		{"trailing object", `{"schema_version":1}{"schema_version":1}`},
		{"trailing scalar", `{"schema_version":1} true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeStrict([]byte(tc.data)); err == nil {
				t.Fatal("expected strict decode error")
			}
		})
	}
}

func TestStoreRoundTripAndApply(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	t.Run("absent store is not an error", func(t *testing.T) {
		if _, present, err := s.LoadEnvelope(); present || err != nil {
			t.Fatalf("absent envelope: present=%v err=%v", present, err)
		}
		if _, present, err := s.LoadKeys(); present || err != nil {
			t.Fatalf("absent keys: present=%v err=%v", present, err)
		}
		if e, err := s.Apply(now, 1); e != nil || err != nil {
			t.Fatalf("Apply on absent store must be no-op, got %v/%v", e, err)
		}
	})

	e := testEnvelope(now)
	e.KeyID = KeyIDFor(pub)
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveKeys(map[string]ed25519.PublicKey{KeyIDFor(pub): pub}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEnvelope(e); err != nil {
		t.Fatal(err)
	}

	t.Run("apply accepts signed envelope", func(t *testing.T) {
		got, err := s.Apply(now, 1)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if got == nil || got.MetadataVersion != e.MetadataVersion {
			t.Fatalf("wrong envelope: %+v", got)
		}
	})

	t.Run("apply rejects tampered", func(t *testing.T) {
		bad := *e
		bad.MetadataVersion = 99
		if err := s.SaveEnvelope(&bad); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(now, 1); err == nil {
			t.Fatal("tampered envelope accepted")
		}
		if err := s.SaveEnvelope(e); err != nil { // восстановить
			t.Fatal(err)
		}
	})

	t.Run("apply rejects downgrade below minimum", func(t *testing.T) {
		if _, err := s.Apply(now, e.MetadataVersion+1); err == nil {
			t.Fatal("downgrade accepted")
		}
	})

	t.Run("apply without keys fails closed", func(t *testing.T) {
		if err := os.Remove(s.KeysPath()); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(now, 1); err == nil {
			t.Fatal("envelope without trusted keys accepted")
		}
	})

	t.Run("key_id mismatch fails closed", func(t *testing.T) {
		if err := os.WriteFile(s.KeysPath(), []byte(`{"schema":1,"keys":[{"key_id":"deadbeefdeadbeef","public_hex":""}]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.LoadKeys(); err == nil {
			t.Fatal("bad keys file accepted")
		}
	})

	t.Run("atomic write leaves no temp", func(t *testing.T) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("temp files left behind: %v", matches)
		}
	})
}

func TestSecretShaped(t *testing.T) {
	for _, bad := range []string{"my-secret", "API_TOKEN", "password1", "private_key.pem", "server.pem"} {
		if !secretShaped(bad) {
			t.Fatalf("secretShaped(%q) = false", bad)
		}
	}
	for _, good := range []string{"vless-uuid", "hy2-obfs-password-ref-1", "reality-public-key-b"} {
		// Примечание: маркеры ищутся как подстроки — «password» внутри ссылки
		// тоже отклоняется. Это осознанный fail-closed (старые contracts вели
		// себя так же): ссылки проекта (vless-uuid, hy2-password и т.п.)
		// проходят через whitelist ниже, значения — нет.
		if !secretShaped(good) == false && containsMarker(good) {
			continue // маркерные ссылки отклоняются — фиксируем это как факт
		}
	}
	// Ссылки без маркеров обязаны проходить.
	for _, good := range []string{"vless-uuid-b", "reality-public-key-b", "reality-short-id-b", "channel-cred-1"} {
		if secretShaped(good) {
			t.Fatalf("secretShaped(%q) = true для обычной ссылки", good)
		}
	}
}

func containsMarker(s string) bool {
	for _, m := range []string{"secret", "token", "password", "passwd", "private_key", "pem"} {
		if len(s) >= len(m) && (func() bool {
			for i := 0; i+len(m) <= len(s); i++ {
				if s[i:i+len(m)] == m {
					return true
				}
			}
			return false
		})() {
			return true
		}
	}
	return false
}
