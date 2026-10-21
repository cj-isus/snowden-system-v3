package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

type bindingCLIFixture struct {
	source, plan, key, output string
	pub                       ed25519.PublicKey
	now                       time.Time
}

func bindingInputSnapshot(t *testing.T, f bindingCLIFixture) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, path := range []string{f.source, f.plan, f.key} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[path] = raw
	}
	return out
}

func TestBindingCLIFailureIsolation(t *testing.T) {
	for _, name := range []string{"args", "large-source", "large-plan", "large-key", "large-output", "unknown-plan", "source-tamper", "wrong-key", "inconsistent-key", "inline-key", "bad-key", "existing-output", "source-alias", "plan-alias", "key-alias", "missing-parent"} {
		t.Run(name, func(t *testing.T) {
			f := newBindingCLIFixture(t)
			args := []string{f.source, f.plan, "@" + f.key, f.output}
			want := ""
			var wantErr error
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch name {
			case "args":
				args = nil
				want = "usage"
			case "large-source":
				write(f.source, bytes.Repeat([]byte(" "), metadata.MaxBindingMigrationSize+1))
				want = "source: file too large"
			case "large-plan":
				write(f.plan, bytes.Repeat([]byte(" "), metadata.MaxBindingMigrationSize+1))
				want = "plan: file too large"
			case "large-key":
				write(f.key, bytes.Repeat([]byte("x"), 1025))
				want = "private key file: file too large"
			case "large-output":
				raw, err := os.ReadFile(f.source)
				if err != nil {
					t.Fatal(err)
				}
				e, err := metadata.DecodeStrict(raw)
				if err != nil {
					t.Fatal(err)
				}
				e.Channels[0].FD.Carrier = "x"
				raw, err = json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				e.Channels[0].FD.Carrier = strings.Repeat("x", metadata.MaxBindingMigrationSize-16-len(raw)+1)
				keyHex, err := os.ReadFile(f.key)
				if err != nil {
					t.Fatal(err)
				}
				key, err := hex.DecodeString(string(keyHex))
				if err != nil {
					t.Fatal(err)
				}
				if err := e.Sign(ed25519.PrivateKey(key)); err != nil {
					t.Fatal(err)
				}
				raw, err = json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) != metadata.MaxBindingMigrationSize-16 {
					t.Fatal("large output fixture is not bounded on input")
				}
				write(f.source, raw)
				want = "output too large"
			case "unknown-plan":
				write(f.plan, []byte(`{"schema":1,"unknown":true}`))
				want = "unknown field"
			case "source-tamper":
				raw, err := os.ReadFile(f.source)
				if err != nil {
					t.Fatal(err)
				}
				write(f.source, bytes.Replace(raw, []byte("synthetic-ws"), []byte("tampered-ws"), 1))
				want = "source signature"
			case "wrong-key":
				_, priv, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				write(f.key, []byte(hex.EncodeToString(priv)))
				want = "source signature"
			case "inconsistent-key":
				raw, err := os.ReadFile(f.key)
				if err != nil {
					t.Fatal(err)
				}
				key, err := hex.DecodeString(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				key[63] ^= 1
				write(f.key, []byte(hex.EncodeToString(key)))
				want = "inconsistent public half"
			case "inline-key":
				args[2] = "not-a-key-file"
				want = "@file"
			case "bad-key":
				write(f.key, []byte("not-hex"))
				want = "invalid hex or size"
			case "existing-output":
				write(f.output, []byte("synthetic-existing-output"))
				wantErr = os.ErrExist
			case "source-alias":
				args[3] = f.source
				wantErr = os.ErrExist
			case "plan-alias":
				args[3] = f.plan
				wantErr = os.ErrExist
			case "key-alias":
				args[3] = f.key
				wantErr = os.ErrExist
			case "missing-parent":
				args[3] = filepath.Join(filepath.Dir(f.output), "missing", "output.json")
				wantErr = os.ErrNotExist
			}
			before := bindingInputSnapshot(t, f)
			err := runBind(args, f.now)
			if err == nil || (want != "" && !strings.Contains(err.Error(), want)) || (wantErr != nil && !errors.Is(err, wantErr)) {
				t.Fatalf("expected rejection %s/%v, got %v", want, wantErr, err)
			}
			if strings.Contains(err.Error(), string(before[f.key])) {
				t.Fatal("private key contents leaked")
			}
			for path, raw := range before {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(raw, after) {
					t.Fatal("input modified on rejection")
				}
			}
			if name == "existing-output" {
				after, err := os.ReadFile(f.output)
				if err != nil || string(after) != "synthetic-existing-output" {
					t.Fatal("existing output replaced")
				}
			} else {
				if _, err := os.Stat(f.output); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("rejection published output")
				}
			}
			temps, err := filepath.Glob(filepath.Join(filepath.Dir(f.output), ".binding-*"))
			if err != nil || len(temps) != 0 {
				t.Fatal("publisher left a stage file")
			}
		})
	}
}

func TestBindingCLIInputBoundsAndPublication(t *testing.T) {
	for _, name := range []string{"boundary", "directory", "missing"} {
		t.Run(name, func(t *testing.T) {
			f := newBindingCLIFixture(t)
			if name == "boundary" {
				for _, item := range []struct {
					path  string
					limit int
				}{{f.source, metadata.MaxBindingMigrationSize}, {f.plan, metadata.MaxBindingMigrationSize}, {f.key, 1024}} {
					raw, err := os.ReadFile(item.path)
					if err != nil {
						t.Fatal(err)
					}
					raw = append(raw, bytes.Repeat([]byte(" "), item.limit-len(raw))...)
					if err := os.WriteFile(item.path, raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
				before := bindingInputSnapshot(t, f)
				if err := runBind([]string{f.source, f.plan, "@" + f.key, f.output}, f.now); err != nil {
					t.Fatal(err)
				}
				for path, want := range before {
					raw, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(raw, want) {
						t.Fatal("success modified source input")
					}
				}
				out, err := os.ReadFile(f.output)
				if err != nil {
					t.Fatal(err)
				}
				e, err := metadata.DecodeStrict(out)
				if err != nil {
					t.Fatal(err)
				}
				if err := e.Verify(map[string]ed25519.PublicKey{e.KeyID: f.pub}); err != nil {
					t.Fatal(err)
				}
				if e.MetadataVersion != 2 {
					t.Fatal("wrong version")
				}
				return
			}
			path := filepath.Dir(f.source)
			want := "regular file"
			if name == "missing" {
				path = filepath.Join(path, "not-present")
				want = ""
			}
			_, err := readBindingFile(path, 1024)
			if err == nil || (want != "" && !strings.Contains(err.Error(), want)) {
				t.Fatalf("bounded file: %v", err)
			}
		})
	}
}

func newBindingCLIFixture(t *testing.T) bindingCLIFixture {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e := &metadata.Envelope{SchemaVersion: 1, MetadataVersion: 1, IssuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), KeyID: metadata.KeyIDFor(pub), Channels: []metadata.Channel{{ID: "synthetic-ws", Protocol: "vless", Transport: "ws", Hostname: "test.example", Port: 443, OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10", CredentialRefs: []string{"vless-uuid"}, ValidationStatus: "configured", Enabled: true}}}
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{"schema": 1, "metadata_version": 2, "issued_at": now.Add(-time.Minute), "expires_at": now.Add(time.Hour), "channels": []any{map[string]any{"channel_id": "synthetic-ws", "secrets": []any{map[string]string{"ref": "vless-uuid", "secret_id": "synthetic-uuid"}}}}}
	f := bindingCLIFixture{source: filepath.Join(dir, "source.json"), plan: filepath.Join(dir, "bindings.json"), key: filepath.Join(dir, "synthetic.key"), output: filepath.Join(dir, "migrated.json"), pub: pub, now: now}
	for path, value := range map[string]any{f.source: e, f.plan: plan} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f.key, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

// The subprocess runs this test binary's main entrypoint with synthetic inputs.
// It is not the Wails EXE and never starts a VPN or privileged helper.
func TestBindingCLIEntry(t *testing.T) {
	if os.Getenv("SNOWDEN_TEST_BINDING_HELPER") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"metadatatool"}, os.Args[i+1:]...)
				main()
				return
			}
		}
		os.Exit(2)
	}
	f := newBindingCLIFixture(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestBindingCLIEntry$", "--", "bind", f.source, f.plan, "@"+f.key, f.output)
	cmd.Env = append(os.Environ(), "SNOWDEN_TEST_BINDING_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("explicit binding publisher unavailable: %v; %s", err, output)
	}
	raw, err := os.ReadFile(f.output)
	if err != nil {
		t.Fatal(err)
	}
	e, err := metadata.DecodeStrict(raw)
	if err != nil {
		t.Fatal(err)
	}
	if e.MetadataVersion != 2 || e.Channels[0].SecretIDs["vless-uuid"] != "synthetic-uuid" {
		t.Fatal("explicit bindings lost")
	}
	if err := e.Verify(map[string]ed25519.PublicKey{e.KeyID: f.pub}); err != nil {
		t.Fatal(err)
	}
}
