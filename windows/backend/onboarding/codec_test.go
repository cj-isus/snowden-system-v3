package onboarding

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	mathrand "math/rand"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/scrypt"
)

func testHeaderTransport(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	h := sealedFormat{Magic: magic, Version: 1, N: scryptN, R: scryptR, P: scryptP, SaltB64: base64.StdEncoding.EncodeToString(make([]byte, 32)), NonceB64: base64.StdEncoding.EncodeToString(make([]byte, 12)), DataB64: base64.StdEncoding.EncodeToString(make([]byte, 16))}
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		mutate(m)
		raw, err = json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	return magic + "1." + base64.RawURLEncoding.EncodeToString(raw)
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Synthetic constant-key frames exercise bounded parsing/decompression only.
// Production uses the pinned scrypt implementation, checked below separately.
func testEncryptedFrame(t *testing.T, gz []byte) string {
	t.Helper()
	salt, nonce := make([]byte, 32), make([]byte, 12)
	block, err := aes.NewCipher(make([]byte, keyLen))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, nonce, gz, aadFor(1, scryptN, scryptR, scryptP, salt))
	h := sealedFormat{Magic: magic, Version: 1, N: scryptN, R: scryptR, P: scryptP, SaltB64: base64.StdEncoding.EncodeToString(salt), NonceB64: base64.StdEncoding.EncodeToString(nonce), DataB64: base64.StdEncoding.EncodeToString(ct)}
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	return magic + "1." + base64.RawURLEncoding.EncodeToString(raw)
}

func TestKDFContainerRejectsBeforeCompute(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"magic", func(m map[string]any) { m["magic"] = "wrong" }},
		{"version", func(m map[string]any) { m["version"] = 2 }},
		{"unknown-field", func(m map[string]any) { m["unknown"] = true }},
		{"bad-n-type", func(m map[string]any) { m["n"] = "32768" }},
		{"cost-memory", func(m map[string]any) { m["n"] = 1 << 21 }},
		{"cost-work", func(m map[string]any) { m["p"] = 1024 }},
		{"salt-base64", func(m map[string]any) { m["salt"] = "!" }},
		{"salt-short", func(m map[string]any) { m["salt"] = base64.StdEncoding.EncodeToString(make([]byte, 15)) }},
		{"salt-long", func(m map[string]any) { m["salt"] = base64.StdEncoding.EncodeToString(make([]byte, 65)) }},
		{"nonce-base64", func(m map[string]any) { m["nonce"] = "!" }},
		{"nonce-short", func(m map[string]any) { m["nonce"] = base64.StdEncoding.EncodeToString(make([]byte, 11)) }},
		{"nonce-long", func(m map[string]any) { m["nonce"] = base64.StdEncoding.EncodeToString(make([]byte, 13)) }},
		{"cipher-base64", func(m map[string]any) { m["data"] = "!" }},
		{"cipher-short", func(m map[string]any) { m["data"] = base64.StdEncoding.EncodeToString(make([]byte, 15)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := bundleCodec{kdf: fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) { calls++; return make([]byte, keyLen), nil })}
			if p, err := c.open(testHeaderTransport(t, tc.mutate), "synthetic-passphrase"); err == nil || p != nil {
				t.Fatal("invalid header accepted")
			}
			if calls != 0 {
				t.Fatal("invalid header reached KDF")
			}
		})
	}
	for _, name := range []string{"transport-size", "passphrase-size", "bad-prefix", "bad-base64", "trailing-header"} {
		t.Run(name, func(t *testing.T) {
			transport, pass := testHeaderTransport(t, nil), "synthetic-passphrase"
			switch name {
			case "transport-size":
				transport = strings.Repeat(" ", MaxBundleSize+1) + transport
			case "passphrase-size":
				pass = strings.Repeat("x", MaxPassphraseSize+1)
			case "bad-prefix":
				transport = "wrong."
			case "bad-base64":
				transport = magic + "1.!"
			case "trailing-header":
				raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(transport, magic+"1."))
				if err != nil {
					t.Fatal(err)
				}
				transport = magic + "1." + base64.RawURLEncoding.EncodeToString(append(raw, []byte(" {}")...))
			}
			calls := 0
			c := bundleCodec{kdf: fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) { calls++; return make([]byte, keyLen), nil })}
			if p, err := c.open(transport, pass); err == nil || p != nil {
				t.Fatal("invalid input accepted")
			}
			if calls != 0 {
				t.Fatal("invalid input reached KDF")
			}
		})
	}
}

func TestKDFDecodedPayloadBounds(t *testing.T) {
	for _, name := range []string{"over-limit", "truncated-json-whitespace", "exact-limit", "unknown-field", "trailing-json", "wrong-schema", "bad-gzip", "bad-checksum"} {
		t.Run(name, func(t *testing.T) {
			payload := []byte(`{"schema":1}`)
			var gz []byte
			want := ErrBadFormat
			switch name {
			case "over-limit":
				payload = []byte(strings.Repeat("x", MaxBundleSize+1))
				want = ErrTooLarge
			case "truncated-json-whitespace":
				payload = append(payload, bytes.Repeat([]byte(" "), MaxBundleSize)...)
				want = ErrTooLarge
			case "exact-limit":
				payload = append(payload, bytes.Repeat([]byte(" "), MaxBundleSize-len(payload))...)
				want = nil
			case "unknown-field":
				payload = []byte(`{"schema":1,"unknown":true}`)
			case "trailing-json":
				payload = append(payload, []byte(" {}")...)
			case "wrong-schema":
				payload = []byte(`{"schema":99}`)
			}
			gz = gzipBytes(t, payload)
			if name == "bad-gzip" {
				gz = []byte("invalid gzip")
			}
			if name == "bad-checksum" {
				gz[len(gz)-8] ^= 1
			}
			p, err := (bundleCodec{kdf: fakeExecutor(nil)}).open(testEncryptedFrame(t, gz), "synthetic-passphrase")
			if !errors.Is(err, want) || (p != nil) != (want == nil) {
				t.Fatalf("payload bound result: %v", err)
			}
		})
	}
}

func TestKDFSealRejectsBeforeCompute(t *testing.T) {
	for _, name := range []string{"oversized-name", "oversized-envelope", "oversized-secret", "too-many-slots", "escaped-size", "encoded-output-size", "passphrase-size", "schema", "nil"} {
		t.Run(name, func(t *testing.T) {
			p := &BundlePayload{Schema: 1}
			pass := "synthetic-passphrase"
			switch name {
			case "oversized-name":
				p.DeviceName = strings.Repeat("x", MaxBundleSize+1)
			case "oversized-envelope":
				p.Envelope = json.RawMessage(`"` + strings.Repeat("x", MaxBundleSize+1) + `"`)
			case "oversized-secret":
				p.Secrets = []SecretSlot{{Value: strings.Repeat("x", MaxBundleSize+1)}}
			case "too-many-slots":
				p.Secrets = make([]SecretSlot, MaxBundleSize/16+1)
			case "escaped-size":
				p.DeviceName = strings.Repeat("\x00", MaxBundleSize/2)
			case "encoded-output-size":
				data := make([]byte, 160<<10)
				if _, err := mathrand.New(mathrand.NewSource(1)).Read(data); err != nil {
					t.Fatal(err)
				}
				p.DeviceName = base64.StdEncoding.EncodeToString(data)
			case "passphrase-size":
				pass = strings.Repeat("x", MaxPassphraseSize+1)
			case "schema":
				p.Schema = 99
			case "nil":
				p = nil
			}
			calls := 0
			c := bundleCodec{kdf: fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) { calls++; return make([]byte, keyLen), nil })}
			if out, err := c.seal(p, pass); err == nil || out != "" {
				t.Fatal("invalid export accepted")
			}
			if calls != 0 {
				t.Fatal("invalid export reached KDF")
			}
		})
	}
}

func TestKDFSealDoesNotMutateAndPredictsSize(t *testing.T) {
	p := &BundlePayload{DeviceName: "synthetic"}
	c := bundleCodec{kdf: fakeExecutor(nil)}
	transport, err := c.seal(p, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if p.Schema != 0 {
		t.Fatal("Seal mutated payload")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(transport, magic+"1."))
	if err != nil {
		t.Fatal(err)
	}
	var h sealedFormat
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	ct, err := base64.StdEncoding.DecodeString(h.DataB64)
	if err != nil {
		t.Fatal(err)
	}
	if len(transport) != encodedSize(len(ct)) {
		t.Fatal("encoded-size estimate drifted")
	}
	if got, err := c.open(transport, "synthetic-passphrase"); err != nil || got.Schema != 1 {
		t.Fatal(err)
	}
}

func TestKDFLegacyFixtureDecrypts(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-v1.txt")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(string(data), "synthetic-legacy-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if p.DeviceName != "synthetic-legacy-v1" || len(p.Secrets) != 1 || p.Secrets[0].Value != "synthetic-not-a-real-secret" {
		t.Fatal("legacy fixture changed")
	}
}

func TestKDFHeaderAADTamper(t *testing.T) {
	for _, field := range []string{"n", "r", "p", "salt"} {
		t.Run(field, func(t *testing.T) {
			c := bundleCodec{kdf: fakeExecutor(nil)}
			transport, err := c.seal(&BundlePayload{Schema: 1}, "synthetic-passphrase")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(transport, magic+"1."))
			if err != nil {
				t.Fatal(err)
			}
			var h sealedFormat
			if err := json.Unmarshal(raw, &h); err != nil {
				t.Fatal(err)
			}
			switch field {
			case "n":
				h.N /= 2
			case "r":
				h.R *= 2
			case "p":
				h.P *= 2
			case "salt":
				salt, err := base64.StdEncoding.DecodeString(h.SaltB64)
				if err != nil {
					t.Fatal(err)
				}
				salt[0] ^= 1
				h.SaltB64 = base64.StdEncoding.EncodeToString(salt)
			}
			raw, err = json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.open(magic+"1."+base64.RawURLEncoding.EncodeToString(raw), "synthetic-passphrase"); !errors.Is(err, ErrWrongPassphrase) {
				t.Fatalf("AAD tamper accepted: %v", err)
			}
		})
	}
}

func TestKDFRealRoundtripAndLegacyDecoder(t *testing.T) {
	p := &BundlePayload{Schema: 1, DeviceName: "synthetic-current"}
	transport, err := Seal(p, "synthetic-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	// Independently follow the pre-fix decoder with trusted export defaults;
	// never feed attacker-selected expensive parameters to this reference.
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(transport, magic+"1."))
	if err != nil {
		t.Fatal(err)
	}
	var h sealedFormat
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	if h.N != scryptN || h.R != scryptR || h.P != scryptP {
		t.Fatal("export defaults changed")
	}
	salt, _ := base64.StdEncoding.DecodeString(h.SaltB64)
	nonce, _ := base64.StdEncoding.DecodeString(h.NonceB64)
	ct, _ := base64.StdEncoding.DecodeString(h.DataB64)
	key, err := scrypt.Key([]byte("synthetic-passphrase"), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, nonce, ct, aadFor(1, scryptN, scryptR, scryptP, salt))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	payloadJSON, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var got BundlePayload
	if err := json.Unmarshal(payloadJSON, &got); err != nil || got.DeviceName != p.DeviceName {
		t.Fatal("legacy decoder incompatible")
	}
	if _, err := Open(transport, "wrong-passphrase"); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatal("wrong passphrase accepted")
	}
	ct[0] ^= 1
	h.DataB64 = base64.StdEncoding.EncodeToString(ct)
	raw, err = json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(magic+"1."+base64.RawURLEncoding.EncodeToString(raw), "synthetic-passphrase"); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatal("tampered ciphertext accepted")
	}
}
