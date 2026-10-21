package render

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
)

func profileEnvelope(t *testing.T) *metadata.Envelope {
	t.Helper()
	now := time.Now().UTC()
	return &metadata.Envelope{SchemaVersion: metadata.SchemaVersion, MetadataVersion: 1,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
		Channels: []metadata.Channel{{ID: "synthetic-profile", Protocol: "vless", Transport: "ws",
			Hostname: "primary.example", Port: 443, OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10",
			CredentialRefs: []string{"vless-uuid"}, ValidationStatus: "configured", Enabled: true}}}
}

func tunedProfile(t *testing.T) *metadata.Envelope {
	t.Helper()
	e := profileEnvelope(t)
	ch := &e.Channels[0]
	ch.ProbeTargets = []string{"https://primary.example/ip", "https://secondary.example/ip"}
	ch.ProbePrimaryHost = "primary.example"
	ch.DiagnosticURL = "https://primary.example/bytes"
	ch.FD = metadata.FailureDomain{TransportFamily: "vless+ws", Carrier: "synthetic-cdn", Origin: ch.OriginServer, ASN: "AS64500", PortFamily: "443", Layer: "tcp", DonorSNI: "donor.example"}
	value, stamp := false, "2026-10-21"
	ch.Multiplex, ch.ValidatedAt = &value, &stamp
	ch.SecretIDs = map[string]string{"vless-uuid": "synthetic-uuid"}
	hy2 := ch.Clone()
	hy2.ID, hy2.Protocol, hy2.Transport = "synthetic-hy2", "hysteria2", "quic"
	hy2.CredentialRefs = []string{"hy2-password", "hy2-obfs-password"}
	hy2.SecretIDs = map[string]string{"hy2-password": "synthetic-hy2-password", "hy2-obfs-password": "synthetic-hy2-obfs"}
	hy2.Multiplex = nil
	hy2.ServerPorts, hy2.HopInterval, hy2.UpMbps, hy2.DownMbps = []string{"20000:20010"}, "30s", 10, 20
	hy2.FD.TransportFamily, hy2.FD.Layer = "hysteria2+quic", "udp"
	e.Channels = append(e.Channels, hy2)
	return e
}

func TestProfileSchemaSharedValidation(t *testing.T) {
	wantErrors := map[string]string{
		"one-probe": "probe_targets", "four-probes": "probe_targets", "http-probe": "probe target",
		"primary-mismatch": "first probe target", "ports-on-ws": "server_ports", "ports-no-range": "server_ports",
		"ports-reversed": "server_ports", "ports-overflow": "server_ports", "interval-on-ws": "hop_interval",
		"interval-short": "hop_interval", "interval-long": "hop_interval", "bandwidth-on-ws": "bandwidth",
		"negative-bandwidth": "up_mbps", "half-bandwidth": "up_mbps", "large-bandwidth": "up_mbps",
		"mux-on-hy2": "multiplex", "mux-on-shadowtls": "multiplex", "http-diagnostic": "diagnostic_url",
		"fd-asn": "fd.asn", "fd-port-family": "fd.port_family", "fd-layer": "fd.layer",
		"unknown-protocol": "unsupported protocol", "invalid-egress": "expected_egress", "empty-endpoint": "incomplete endpoint",
	}
	for _, tc := range []struct {
		name   string
		mutate func(*metadata.Channel)
	}{
		{"one-probe", func(c *metadata.Channel) { c.ProbeTargets = c.ProbeTargets[:1] }},
		{"four-probes", func(c *metadata.Channel) { c.ProbeTargets = append(c.ProbeTargets, c.ProbeTargets...) }},
		{"http-probe", func(c *metadata.Channel) { c.ProbeTargets[0] = "http://primary.example/ip" }},
		{"primary-mismatch", func(c *metadata.Channel) { c.ProbePrimaryHost = "other.example" }},
		{"ports-on-ws", func(c *metadata.Channel) { c.ServerPorts = []string{"20000:20010"} }},
		{"ports-no-range", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.ServerPorts = []string{"20000"}
		}},
		{"ports-reversed", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.ServerPorts = []string{"20:10"}
		}},
		{"ports-overflow", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.ServerPorts = []string{"1:65536"}
		}},
		{"interval-on-ws", func(c *metadata.Channel) { c.HopInterval = "30s" }},
		{"interval-short", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.HopInterval = "1s"
		}},
		{"interval-long", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.HopInterval = "11m"
		}},
		{"bandwidth-on-ws", func(c *metadata.Channel) { c.UpMbps, c.DownMbps = 10, 20 }},
		{"negative-bandwidth", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.UpMbps, c.DownMbps = -1, -1
		}},
		{"half-bandwidth", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.UpMbps = 10
		}},
		{"large-bandwidth", func(c *metadata.Channel) {
			c.Protocol, c.Transport, c.Multiplex = "hysteria2", "quic", nil
			c.UpMbps, c.DownMbps = 10001, 10001
		}},
		{"mux-on-hy2", func(c *metadata.Channel) { c.Protocol, c.Transport = "hysteria2", "quic" }},
		{"mux-on-shadowtls", func(c *metadata.Channel) {
			c.Transport = "shadowtls"
			c.UUIDRef = "vless-uuid"
			c.ShadowTLSPasswordRef = "shadowtls-password-d"
			c.CredentialRefs = append(c.CredentialRefs, c.ShadowTLSPasswordRef)
			c.SecretIDs[c.ShadowTLSPasswordRef] = "synthetic-st"
		}},
		{"http-diagnostic", func(c *metadata.Channel) { c.DiagnosticURL = "http://primary.example/bytes" }},
		{"fd-asn", func(c *metadata.Channel) { c.FD.ASN = "64500" }},
		{"fd-port-family", func(c *metadata.Channel) { c.FD.PortFamily = "https" }},
		{"fd-layer", func(c *metadata.Channel) { c.FD.Layer = "quic" }},
		{"unknown-protocol", func(c *metadata.Channel) { c.Protocol = "unknown" }},
		{"invalid-egress", func(c *metadata.Channel) { c.ExpectedEgress = "invalid" }},
		{"empty-endpoint", func(c *metadata.Channel) { c.Hostname = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := tunedProfile(t)
			if err := e.Validate(time.Now(), 0); err != nil {
				t.Fatal("invalid baseline fixture:", err)
			}
			tc.mutate(&e.Channels[0])
			want := wantErrors[tc.name]
			if want == "" {
				t.Fatal("negative case needs expected reason")
			}
			if err := e.Validate(time.Now(), 0); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("signed validator did not reach %s: %v", want, err)
			}
			if _, err := parseSet([]ChannelDescriptor{ChannelDescriptor(e.Channels[0])}); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("embedded validator did not reach %s: %v", want, err)
			}
		})
	}
}

func TestProfileSchemaEveryExtensionIsSigned(t *testing.T) {
	fields := map[string]any{"probe_targets": []any{"https://other.example/ip", "https://secondary.example/ip"}, "probe_primary_host": "other.example", "server_ports": []any{"30000:30010"}, "hop_interval": "31s", "up_mbps": float64(11), "down_mbps": float64(21), "multiplex": true, "diagnostic_url": "https://primary.example/changed"}
	for _, name := range []string{"transport_family", "carrier", "origin", "asn", "port_family", "layer", "donor_sni"} {
		fields["fd."+name] = "changed"
	}
	for name, value := range fields {
		t.Run(name, func(t *testing.T) {
			e := tunedProfile(t)
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			e.KeyID = metadata.KeyIDFor(pub)
			if err := e.Validate(time.Now(), 0); err != nil {
				t.Fatal(err)
			}
			if err := e.Sign(priv); err != nil {
				t.Fatal(err)
			}
			keys := map[string]ed25519.PublicKey{e.KeyID: pub}
			if err := e.Verify(keys); err != nil {
				t.Fatal(err)
			}
			obj := profileObject(t, e)
			index := 0
			if name == "server_ports" || name == "hop_interval" || name == "up_mbps" || name == "down_mbps" {
				index = 1
			}
			channel := obj["channels"].([]any)[index].(map[string]any)
			if len(name) > 3 && name[:3] == "fd." {
				channel["fd"].(map[string]any)[name[3:]] = value
			} else {
				channel[name] = value
			}
			raw, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := metadata.DecodeStrict(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := changed.Verify(keys); err == nil {
				t.Fatal("extension tamper retained valid signature")
			}
		})
	}
}

func TestProfileSchemaCloneIsolation(t *testing.T) {
	e := tunedProfile(t)
	// Populate every mutable field, even though mux+hopping cannot coexist at runtime.
	c := e.Channels[0]
	c.ServerPorts = []string{"20000:20010"}
	before := profileObject(t, c)
	cloned := c.Clone()
	v := reflect.ValueOf(&cloned).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		name := v.Type().Field(i).Name
		switch field.Kind() {
		case reflect.Slice:
			if field.Len() == 0 {
				t.Fatalf("clone fixture missing mutable field %s", name)
			}
			field.Index(0).SetString("changed")
		case reflect.Map:
			if field.Len() == 0 {
				t.Fatalf("clone fixture missing mutable field %s", name)
			}
			for _, key := range field.MapKeys() {
				field.SetMapIndex(key, reflect.ValueOf("changed"))
			}
		case reflect.Pointer:
			if field.IsNil() {
				t.Fatalf("clone fixture missing mutable field %s", name)
			}
			switch field.Elem().Kind() {
			case reflect.String:
				field.Elem().SetString("changed")
			case reflect.Bool:
				field.Elem().SetBool(!field.Elem().Bool())
			default:
				t.Fatalf("clone test needs new pointer type: %s", name)
			}
		}
	}
	if !reflect.DeepEqual(before, profileObject(t, c)) {
		t.Fatal("Clone aliases signed channel")
	}
	beforeRaw := bytes.Clone(e.Canonical())
	channels, err := DescriptorsFromEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	channels[0].ProbeTargets[0] = "changed"
	channels[0].CredentialRefs[0] = "changed"
	channels[0].SecretIDs["vless-uuid"] = "changed"
	*channels[0].Multiplex = true
	*channels[0].ValidatedAt = "changed"
	channels[1].ServerPorts[0] = "changed"
	if !bytes.Equal(beforeRaw, e.Canonical()) {
		t.Fatal("runtime conversion aliases signed input")
	}
}

func TestProfileSchemaLegacyCanonicalAndMuxStates(t *testing.T) {
	const legacy = `{"schema_version":1,"metadata_version":1,"issued_at":"2026-09-11T00:00:00Z","expires_at":"2026-09-12T00:00:00Z","key_id":"legacy-key","channels":[{"id":"legacy","protocol":"vless","transport":"ws","hostname":"primary.example","port":443,"origin_server":"203.0.113.10","expected_egress":"203.0.113.10","credential_refs":["vless-uuid"],"validation_status":"configured","enabled":true}]}`
	e, err := metadata.DecodeStrict([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, []byte(legacy)))
	if !bytes.Equal(e.Canonical(), []byte(legacy)) {
		t.Fatal("old canonical bytes changed")
	}
	if err := e.Verify(map[string]ed25519.PublicKey{"legacy-key": pub}); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"omitted", "false", "true"} {
		t.Run(state, func(t *testing.T) {
			ch := profileEnvelope(t).Channels[0]
			if state != "omitted" {
				value := state == "true"
				ch.Multiplex = &value
			}
			raw, err := json.Marshal(ch)
			if err != nil {
				t.Fatal(err)
			}
			var decoded metadata.Channel
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ch, decoded) {
				t.Fatal("optional-state roundtrip changed")
			}
			if bytes.Contains(raw, []byte(`"multiplex"`)) != (state != "omitted") {
				t.Fatal("explicit false collapsed into omission")
			}
			cc, err := RenderFrom([]ChannelDescriptor{ChannelDescriptor(decoded)}, testSecrets(false), WithDefaultChannel(ch.ID))
			if err != nil {
				t.Fatal(err)
			}
			var cfg config.Config
			if err := json.Unmarshal(cc.Raw, &cfg); err != nil {
				t.Fatal(err)
			}
			for _, ob := range cfg.Outbounds {
				if ob.Tag == ch.ID && (ob.Multiplex != nil) != (state == "true") {
					t.Fatal("mux does not follow signed state")
				}
			}
		})
	}
}

func TestProfileSchemaRevocationAndStrictShapes(t *testing.T) {
	t.Run("revoked-inert", func(t *testing.T) {
		e := tunedProfile(t)
		e.Channels[1].Protocol = "retired-unknown"
		e.Channels[1].FD.Layer = "retired-unknown"
		e.Revocations = []string{e.Channels[1].ID}
		if err := e.Validate(time.Now(), 0); err != nil {
			t.Fatal(err)
		}
		out, err := DescriptorsFromEnvelope(e)
		if err != nil || len(out) != 1 {
			t.Fatalf("revoked channel activated or rejected: %v", err)
		}
	})
	for _, name := range []string{"unknown-channel", "unknown-fd", "wrong-mux-type", "wrong-fd-type", "trailing"} {
		t.Run(name, func(t *testing.T) {
			obj := profileObject(t, tunedProfile(t))
			ch := obj["channels"].([]any)[0].(map[string]any)
			switch name {
			case "unknown-channel":
				ch["future_unknown"] = true
			case "unknown-fd":
				ch["fd"].(map[string]any)["future_unknown"] = true
			case "wrong-mux-type":
				ch["multiplex"] = "false"
			case "wrong-fd-type":
				ch["fd"] = "AS64500"
			}
			raw, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			if name == "trailing" {
				raw = append(raw, []byte(" {}")...)
			}
			if _, err := metadata.DecodeStrict(raw); err == nil {
				t.Fatal("invalid signed shape accepted")
			}
		})
	}
}

func TestProfileSchemaEmbeddedStrictDecode(t *testing.T) {
	for _, name := range []string{"valid", "unknown-set", "unknown-channel", "unknown-fd", "trailing"} {
		t.Run(name, func(t *testing.T) {
			obj := map[string]any{"schema": 2, "channels": profileObject(t, tunedProfile(t))["channels"]}
			ch := obj["channels"].([]any)[0].(map[string]any)
			switch name {
			case "unknown-set":
				obj["unknown"] = true
			case "unknown-channel":
				ch["unknown"] = true
			case "unknown-fd":
				ch["fd"].(map[string]any)["unknown"] = true
			}
			raw, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			if name == "trailing" {
				raw = append(raw, []byte(" {}")...)
			}
			_, err = parseDescriptors(raw)
			if (err == nil) != (name == "valid") {
				t.Fatalf("strict embedded decode result: %v", err)
			}
		})
	}
}

func assertProfileRuntime(t *testing.T, ch ChannelDescriptor, cc *ClientConfig) {
	t.Helper()
	if !reflect.DeepEqual(cc.ProbeTargets, ch.ProbeTargets) || cc.DiagnosticURL != ch.DiagnosticURL {
		t.Fatal("probe/diagnostic fields lost before execution")
	}
	var cfg config.Config
	if err := json.Unmarshal(cc.Raw, &cfg); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Tag != ch.ID {
			continue
		}
		found = true
		if ch.Protocol == "hysteria2" && (!reflect.DeepEqual(ob.ServerPorts, ch.ServerPorts) || ob.HopInterval != ch.HopInterval || ob.UpMbps != ch.UpMbps || ob.DownMbps != ch.DownMbps) {
			t.Fatal("HY2 tuning lost in runtime")
		}
		if ch.Transport == "ws" || ch.Transport == "tcp-reality" {
			want := ch.Multiplex != nil && *ch.Multiplex
			if (ob.Multiplex != nil) != want {
				t.Fatal("signed mux lost in runtime")
			}
		}
	}
	if !found {
		t.Fatal("no rendered channel outbound")
	}
	// Parse actual pinned sing-box options; no Box.New/Start or network calls.
	var options option.Options
	if err := options.UnmarshalJSONContext(include.Context(context.Background()), cc.Raw); err != nil {
		t.Fatal("native sing-box options:", err)
	}
}

func profileObject(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestProfileSchemaWireFields(t *testing.T) {
	fields := map[string]any{
		"probe_targets":      []any{"https://primary.example/ip", "https://secondary.example/ip"},
		"probe_primary_host": "primary.example", "server_ports": []any{"20000:20010"},
		"hop_interval": "30s", "up_mbps": float64(10), "down_mbps": float64(20),
		"multiplex": false, "diagnostic_url": "https://primary.example/bytes",
		"fd": map[string]any{"transport_family": "synthetic-family", "carrier": "synthetic-cdn", "origin": "203.0.113.10", "asn": "AS64500", "port_family": "443", "layer": "tcp", "donor_sni": "donor.example"},
	}
	for field, value := range fields {
		t.Run(field, func(t *testing.T) {
			obj := profileObject(t, profileEnvelope(t))
			obj["channels"].([]any)[0].(map[string]any)[field] = value
			raw, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			env, err := metadata.DecodeStrict(raw)
			if err != nil {
				t.Fatalf("supported descriptor field rejected by signed schema: %v", err)
			}
			got := profileObject(t, env)["channels"].([]any)[0].(map[string]any)[field]
			if !reflect.DeepEqual(got, value) {
				t.Fatalf("field %s lost: got %v", field, got)
			}
		})
	}
}
