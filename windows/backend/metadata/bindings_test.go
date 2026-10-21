package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

type bindingFixture struct {
	source *Envelope
	plan   *BindingMigration
	keys   map[string]ed25519.PublicKey
	priv   ed25519.PrivateKey
	now    time.Time
}

func newBindingFixture(t *testing.T) bindingFixture {
	t.Helper()
	now := time.Now().UTC()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	value, stamp := false, "2026-10-21"
	ch := Channel{ID: "legacy-one", Protocol: "vless", Transport: "ws", Hostname: "test.example", Port: 443, OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10", CredentialRefs: []string{"vless-uuid"}, ValidationStatus: "configured", Enabled: true, ValidatedAt: &stamp, Multiplex: &value, ProbeTargets: []string{"https://test.example/ip", "https://secondary.example/ip"}, ProbePrimaryHost: "test.example", DiagnosticURL: "https://test.example/bytes", FD: FailureDomain{ASN: "AS64500", Layer: "tcp"}}
	second := ch.Clone()
	second.ID = "legacy-two"
	bound := ch.Clone()
	bound.ID = "already-bound"
	bound.SecretIDs = map[string]string{"vless-uuid": "existing-id"}
	revoked := ch.Clone()
	revoked.ID = "revoked"
	revoked.Protocol = "retired-unknown"
	e := &Envelope{SchemaVersion: 1, MetadataVersion: 7, IssuedAt: now.Add(-48 * time.Hour), ExpiresAt: now.Add(-24 * time.Hour), KeyID: KeyIDFor(pub), Channels: []Channel{ch, second, bound, revoked}, Revocations: []string{"revoked"}, SplitDirect: []string{"example.exe"}}
	if err := e.Validate(e.IssuedAt, 0); err != nil {
		t.Fatal("invalid fixture:", err)
	}
	if err := e.Sign(priv); err != nil {
		t.Fatal(err)
	}
	p := &BindingMigration{Schema: 1, MetadataVersion: 8, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), Channels: []ChannelBindings{{ChannelID: ch.ID, Secrets: []CredentialBinding{{Ref: "vless-uuid", SecretID: "explicit-one"}}}, {ChannelID: second.ID, Secrets: []CredentialBinding{{Ref: "vless-uuid", SecretID: "explicit-two"}}}}}
	return bindingFixture{e, p, map[string]ed25519.PublicKey{e.KeyID: pub}, priv, now}
}

func bindingJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBindingMigrationExplicitAndImmutable(t *testing.T) {
	f := newBindingFixture(t)
	before := bindingJSON(t, f.source)
	planBefore := bindingJSON(t, f.plan)
	if err := f.source.Validate(f.now, 0); err == nil {
		t.Fatal("source fixture must be expired")
	}
	out, err := MigrateLegacyBindings(f.source, f.plan, f.keys, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if out.Signature != "" {
		t.Fatal("migration retained obsolete signature")
	}
	if out.MetadataVersion != 8 || out.Channels[0].SecretIDs["vless-uuid"] != "explicit-one" || out.Channels[1].SecretIDs["vless-uuid"] != "explicit-two" {
		t.Fatal("explicit IDs/version lost")
	}
	if out.Channels[2].SecretIDs["vless-uuid"] != "existing-id" || out.Channels[3].SecretIDs != nil {
		t.Fatal("existing or revoked record changed")
	}
	if err := out.Verify(f.keys); err == nil {
		t.Fatal("unsigned migrated output verified")
	}
	if err := out.Sign(f.priv); err != nil {
		t.Fatal(err)
	}
	if err := out.Verify(f.keys); err != nil {
		t.Fatal(err)
	}
	if err := out.Validate(f.now, f.source.MetadataVersion); err != nil {
		t.Fatal(err)
	}
	for i := range out.Channels {
		want := f.source.Channels[i].Clone()
		want.SecretIDs = out.Channels[i].SecretIDs
		if !reflect.DeepEqual(out.Channels[i], want) {
			t.Fatal("non-binding channel field changed")
		}
	}
	out.Channels[0].ProbeTargets[0] = "changed"
	*out.Channels[0].Multiplex = true
	*out.Channels[0].ValidatedAt = "changed"
	out.Channels[0].CredentialRefs[0] = "changed"
	out.Channels[2].SecretIDs["vless-uuid"] = "changed"
	out.Revocations[0] = "changed"
	out.SplitDirect[0] = "changed"
	if !bytes.Equal(before, bindingJSON(t, f.source)) || !bytes.Equal(planBefore, bindingJSON(t, f.plan)) {
		t.Fatal("migration aliases or mutates inputs")
	}
	if err := f.source.Verify(f.keys); err != nil {
		t.Fatal("source signature changed")
	}
}

func TestBindingMigrationTransportRoles(t *testing.T) {
	for _, transport := range []string{"ws", "quic", "tcp-reality", "shadowtls"} {
		t.Run(transport, func(t *testing.T) {
			f := newBindingFixture(t)
			ch := &f.source.Channels[0]
			ch.Transport = transport
			ch.Multiplex = nil
			switch transport {
			case "quic":
				ch.Protocol = "hysteria2"
				ch.CredentialRefs = []string{"hy2-password", "hy2-obfs-password"}
			case "tcp-reality":
				ch.UUIDRef = "vless-uuid-c"
				ch.RealityPublicKeyRef = "reality-public-key-c"
				ch.RealityShortIDRef = "reality-short-id-c"
				ch.CredentialRefs = []string{ch.UUIDRef, ch.RealityPublicKeyRef, ch.RealityShortIDRef}
			case "shadowtls":
				ch.UUIDRef = "vless-uuid-d"
				ch.ShadowTLSPasswordRef = "shadowtls-password-d"
				ch.CredentialRefs = []string{ch.UUIDRef, ch.ShadowTLSPasswordRef}
			}
			if err := f.source.Validate(f.source.IssuedAt, 0); err != nil {
				t.Fatal("transport fixture:", err)
			}
			if err := f.source.Sign(f.priv); err != nil {
				t.Fatal(err)
			}
			f.plan.Channels[0].Secrets = nil
			for i, ref := range ch.CredentialRefs {
				f.plan.Channels[0].Secrets = append(f.plan.Channels[0].Secrets, CredentialBinding{Ref: ref, SecretID: "explicit-role-" + string(rune('a'+i))})
			}
			out, err := MigrateLegacyBindings(f.source, f.plan, f.keys, f.now)
			if err != nil {
				t.Fatal(err)
			}
			for _, assignment := range f.plan.Channels[0].Secrets {
				if out.Channels[0].SecretIDs[assignment.Ref] != assignment.SecretID {
					t.Fatal("transport role lost")
				}
			}
			before := ch.Clone()
			before.SecretIDs = out.Channels[0].SecretIDs
			if !reflect.DeepEqual(before, out.Channels[0]) {
				t.Fatal("non-binding transport fields changed")
			}
		})
	}
}

func TestBindingMigrationRejects(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(*bindingFixture)
	}{
		{"nil-source", "missing source", func(f *bindingFixture) { f.source = nil }},
		{"nil-plan", "missing source", func(f *bindingFixture) { f.plan = nil }},
		{"wrong-signature", "source signature", func(f *bindingFixture) { f.source.Signature = "invalid" }},
		{"unknown-key", "source signature", func(f *bindingFixture) { f.keys = nil }},
		{"schema", "plan schema", func(f *bindingFixture) { f.plan.Schema = 2 }},
		{"same-version", "version must increase", func(f *bindingFixture) { f.plan.MetadataVersion = 7 }},
		{"downgrade", "version must increase", func(f *bindingFixture) { f.plan.MetadataVersion = 6 }},
		{"max-source-version", "version must increase", func(f *bindingFixture) { f.source.MetadataVersion = ^uint64(0); _ = f.source.Sign(f.priv) }},
		{"empty-plan", "channel count", func(f *bindingFixture) { f.plan.Channels = nil }},
		{"too-many-channels", "channel count", func(f *bindingFixture) {
			f.plan.Channels = append(f.plan.Channels, f.plan.Channels...)
			f.plan.Channels = append(f.plan.Channels, f.plan.Channels[0])
		}},
		{"unknown-channel", "unknown channel", func(f *bindingFixture) { f.plan.Channels[0].ChannelID = "unknown" }},
		{"revoked-channel", "revoked channel", func(f *bindingFixture) { f.plan.Channels[0].ChannelID = "revoked" }},
		{"existing-bindings", "immutable", func(f *bindingFixture) { f.plan.Channels[0].ChannelID = "already-bound" }},
		{"duplicate-channel", "duplicate channel", func(f *bindingFixture) { f.plan.Channels[1].ChannelID = f.plan.Channels[0].ChannelID }},
		{"missing-channel", "missing legacy channel", func(f *bindingFixture) { f.plan.Channels = f.plan.Channels[:1] }},
		{"empty-roles", "incomplete role", func(f *bindingFixture) { f.plan.Channels[0].Secrets = nil }},
		{"unknown-role", "role assignments", func(f *bindingFixture) { f.plan.Channels[0].Secrets[0].Ref = "other-ref" }},
		{"invalid-id", "role assignments", func(f *bindingFixture) { f.plan.Channels[0].Secrets[0].SecretID = "../unsafe" }},
		{"empty-id", "role assignments", func(f *bindingFixture) { f.plan.Channels[0].Secrets[0].SecretID = "" }},
		{"expired-target", "new expiry", func(f *bindingFixture) { f.plan.ExpiresAt = f.now.Add(-time.Minute) }},
		{"missing-issued", "new validity", func(f *bindingFixture) { f.plan.IssuedAt = time.Time{} }},
		{"future-issued", "new validity", func(f *bindingFixture) { f.plan.IssuedAt = f.now.Add(MaxClockSkew + time.Second) }},
		{"reversed-validity", "new validity", func(f *bindingFixture) { f.plan.IssuedAt = f.plan.ExpiresAt.Add(time.Minute) }},
		{"invalid-source-version", "source version", func(f *bindingFixture) { f.source.MetadataVersion = 0; _ = f.source.Sign(f.priv) }},
		{"invalid-source-shape", "source shape", func(f *bindingFixture) { f.source.Channels[0].ExpectedEgress = "invalid"; _ = f.source.Sign(f.priv) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBindingFixture(t)
			tc.mutate(&f)
			sourceBefore := bindingJSON(t, f.source)
			planBefore := bindingJSON(t, f.plan)
			out, err := MigrateLegacyBindings(f.source, f.plan, f.keys, f.now)
			if out != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s, got %v", tc.want, err)
			}
			if !bytes.Equal(sourceBefore, bindingJSON(t, f.source)) || !bytes.Equal(planBefore, bindingJSON(t, f.plan)) {
				t.Fatal("rejection mutated source/plan")
			}
		})
	}
	t.Run("duplicate-role", func(t *testing.T) {
		f := newBindingFixture(t)
		c := &f.source.Channels[0]
		c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
		c.CredentialRefs = []string{"hy2-password", "hy2-obfs-password"}
		if err := f.source.Sign(f.priv); err != nil {
			t.Fatal(err)
		}
		f.plan.Channels[0].Secrets = []CredentialBinding{{Ref: "hy2-password", SecretID: "one"}, {Ref: "hy2-password", SecretID: "two"}}
		if out, err := MigrateLegacyBindings(f.source, f.plan, f.keys, f.now); out != nil || err == nil || !strings.Contains(err.Error(), "duplicate role") {
			t.Fatalf("duplicate role target: %v", err)
		}
	})
}

func TestBindingMigrationStrictPlan(t *testing.T) {
	f := newBindingFixture(t)
	valid := bindingJSON(t, f.plan)
	for _, name := range []string{"valid", "size-boundary", "too-large", "unknown-root", "unknown-role", "duplicate-property", "nested-duplicate", "casefold-alias", "unicode-alias", "deep", "trailing", "null", "bad-json"} {
		t.Run(name, func(t *testing.T) {
			raw := bytes.Clone(valid)
			want := ""
			switch name {
			case "size-boundary":
				raw = append(raw, bytes.Repeat([]byte(" "), MaxBindingMigrationSize-len(raw))...)
			case "too-large":
				raw = bytes.Repeat([]byte(" "), MaxBindingMigrationSize+1)
				want = "too large"
			case "unknown-root":
				raw = bytes.Replace(raw, []byte(`"schema":1`), []byte(`"schema":1,"unknown":true`), 1)
				want = "unknown field"
			case "unknown-role":
				raw = bytes.Replace(raw, []byte(`"ref":`), []byte(`"unknown":1,"ref":`), 1)
				want = "unknown field"
			case "duplicate-property":
				raw = bytes.Replace(raw, []byte(`"schema":1`), []byte(`"schema":1,"schema":1`), 1)
				want = "duplicate JSON"
			case "nested-duplicate":
				raw = bytes.Replace(raw, []byte(`"secret_id":"explicit-one"`), []byte(`"secret_id":"explicit-one","secret_id":"explicit-two"`), 1)
				want = "duplicate JSON"
			case "casefold-alias":
				raw = bytes.Replace(raw, []byte(`"secret_id":"explicit-one"`), []byte(`"secret_id":"explicit-one","SECRET_ID":"explicit-two"`), 1)
				want = "property"
			case "unicode-alias":
				raw = bytes.Replace(raw, []byte(`"secret_id":"explicit-one"`), []byte(`"secret_id":"explicit-one","\u017Fecret_id":"explicit-two"`), 1)
				want = "property"
			case "deep":
				raw = []byte(`{"unknown":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}")
				want = "nesting"
			case "trailing":
				raw = append(raw, []byte(" {}")...)
				want = "trailing"
			case "null":
				raw = []byte("null")
				want = "object"
			case "bad-json":
				raw = []byte(`{"schema":`)
				want = "JSON"
			}
			p, err := DecodeBindingMigration(raw)
			if want == "" {
				if err != nil || p.MetadataVersion != 8 {
					t.Fatalf("valid plan rejected: %v", err)
				}
				return
			}
			if p != nil || err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %s: %v", want, err)
			}
		})
	}
}
