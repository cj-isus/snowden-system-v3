package metadata

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestSecretIDLegacySignatureCompatibility(t *testing.T) {
	// Exact schema-1 canonical bytes from the pre-binding channel shape. New
	// omitted fields must not invalidate an already signed legacy envelope.
	raw := []byte(`{"schema_version":1,"metadata_version":1,"issued_at":"2026-10-20T10:00:00Z","expires_at":"2026-10-21T10:00:00Z","key_id":"legacy-key","channels":[{"id":"test","protocol":"vless","transport":"ws","hostname":"test.example","port":443,"origin_server":"203.0.113.10","expected_egress":"203.0.113.10","credential_refs":["vless-uuid"],"validation_status":"configured","enabled":true}]}`)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecodeStrict(raw)
	if err != nil {
		t.Fatal(err)
	}
	env.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, raw))
	if err := env.Verify(map[string]ed25519.PublicKey{"legacy-key": pub}); err != nil {
		t.Fatal("legacy canonical bytes changed:", err)
	}
	if string(env.Canonical()) != string(raw) {
		t.Fatal("legacy canonical JSON changed")
	}
}

func TestSecretIDBindingIsSigned(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	env := testEnvelope(time.Now().UTC())
	env.KeyID = KeyIDFor(pub)
	env.Channels[0].SecretIDs = map[string]string{"vless-uuid": "slot-one"}
	if err := env.Sign(priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStrict(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.Verify(map[string]ed25519.PublicKey{env.KeyID: pub}); err != nil {
		t.Fatal(err)
	}
	decoded.Channels[0].SecretIDs["vless-uuid"] = "slot-two"
	if err := decoded.Verify(map[string]ed25519.PublicKey{env.KeyID: pub}); err == nil {
		t.Fatal("binding tamper retained valid signature")
	}
}
