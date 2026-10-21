package main

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

func TestProfileSchemaAppImport(t *testing.T) {
	f := newImportFixture(t)
	ch := &f.env.Channels[0]
	ch.ProbeTargets = []string{"https://test.example/ip", "https://secondary.example/ip"}
	ch.ProbePrimaryHost = "test.example"
	ch.DiagnosticURL = "https://test.example/bytes"
	ch.FD = metadata.FailureDomain{TransportFamily: "vless+ws", Carrier: "synthetic-cdn", ASN: "AS64500", Origin: ch.OriginServer, Layer: "tcp", PortFamily: "443"}
	value := false
	ch.Multiplex = &value
	transport := f.seal(t)
	before := snapshotImport(t, f.paths)
	if _, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase"); err != nil {
		t.Fatal(err)
	}
	assertImportUnchanged(t, f.paths, before)
	if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(f.paths[1])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, f.payload.Envelope) {
		t.Fatal("checked signed bytes not preserved")
	}
	e, err := metadata.DecodeStrict(stored)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := f.store.LoadKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Verify(keys); err != nil {
		t.Fatal(err)
	}
	channels, err := render.DescriptorsFromEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || !reflect.DeepEqual(metadata.Channel(channels[0]), *ch) {
		t.Fatal("profile field lost after App import")
	}
	cc, err := render.RenderFrom(channels, &vaultSecrets{vault: f.app.vault}, render.WithDefaultChannel(ch.ID))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cc.ProbeTargets, ch.ProbeTargets) || cc.DiagnosticURL != ch.DiagnosticURL {
		t.Fatal("imported execution targets changed")
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatal(err)
	}
}

func TestProfileSchemaAppRejectsBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*metadata.Channel)
	}{
		{"probe-http", func(c *metadata.Channel) {
			c.ProbeTargets = []string{"http://test.example/ip", "https://secondary.example/ip"}
		}},
		{"probe-host", func(c *metadata.Channel) {
			c.ProbeTargets = []string{"https://other.example/ip", "https://secondary.example/ip"}
		}},
		{"fd", func(c *metadata.Channel) { c.FD.ASN = "bad-asn" }},
		{"hopping", func(c *metadata.Channel) { c.ServerPorts = []string{"20000:20010"} }},
		{"bandwidth", func(c *metadata.Channel) { c.UpMbps, c.DownMbps = 10, 20 }},
		{"diagnostic", func(c *metadata.Channel) { c.DiagnosticURL = "http://test.example/bytes" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newImportFixture(t)
			tc.mutate(&f.env.Channels[0])
			transport := f.seal(t)
			before := snapshotImport(t, f.paths)
			if _, err := f.app.PreviewOnboardingString(transport, "synthetic-passphrase"); err == nil {
				t.Fatal("invalid preview accepted")
			}
			assertImportUnchanged(t, f.paths, before)
			if _, err := f.app.ApplyOnboardingString(transport, "synthetic-passphrase", true); err == nil {
				t.Fatal("invalid Apply accepted")
			}
			assertImportUnchanged(t, f.paths, before)
		})
	}
}
