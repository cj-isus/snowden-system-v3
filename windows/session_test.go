package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/snowden-system/windows/backend/core"
)

func TestSeamlessRejectsStructuralChanges(t *testing.T) {
	base := `{"outbounds":[{"type":"selector","tag":"proxy","default":"a","outbounds":["a","b"]},{"type":"vless","tag":"a","server":"old.example"}],"route":{"final":"proxy"}}`
	for _, tt := range []struct {
		name, raw string
		want      bool
	}{
		{"selector", `{"outbounds":[{"type":"selector","tag":"proxy","default":"b","outbounds":["a","b"]},{"type":"vless","tag":"a","server":"old.example"}],"route":{"final":"proxy"}}`, true},
		{"server", `{"outbounds":[{"type":"selector","tag":"proxy","default":"b","outbounds":["a","b"]},{"type":"vless","tag":"a","server":"new.example"}],"route":{"final":"proxy"}}`, false},
		{"invalid", `{`, false},
		{"no selector", `{"outbounds":[]}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectorOnlyChange([]byte(base), []byte(tt.raw)); got != tt.want {
				t.Fatalf("compatible=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestMetricsSessionMatchesAuthenticatedEngine(t *testing.T) {
	if !clashAPIBuilt {
		t.Skip("production with_clash_api tag required")
	}
	m := newMetricsSession()
	if m == nil {
		t.Fatal("no metrics port")
	}
	cfg := map[string]any{
		"log":          map[string]any{"disabled": true},
		"outbounds":    []any{map[string]any{"type": "direct", "tag": "direct"}},
		"route":        map[string]any{"final": "direct"},
		"experimental": map[string]any{"clash_api": map[string]any{"external_controller": m.port, "secret": m.secret}},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := core.NewEngine(raw)
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	applied := metricsForConfig(raw)
	if applied == nil || applied.port != m.port || applied.secret != m.secret {
		t.Fatal("polling a different metrics session")
	}
	if view, err := applied.poll(); err != nil || !view.Available {
		t.Fatalf("authenticated poll failed: %v", err)
	}
	applied.secret = "synthetic-wrong-secret"
	if _, err := applied.poll(); err == nil {
		t.Fatal("controller accepted wrong secret")
	}
}
