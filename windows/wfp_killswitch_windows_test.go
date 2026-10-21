//go:build windows

package main

import (
	"reflect"
	"testing"

	"github.com/tailscale/wf"
)

func TestWFPVerifierRejectsPolicyDrift(t *testing.T) {
	spec := wfpBaseRuleSpecs(`synthetic-app-id`)[0]
	base := &wf.Rule{
		Provider: wfpProviderID, Sublayer: wfpSublayerID, Layer: spec.layer,
		Action: spec.action, Weight: spec.weight, Persistent: true,
		Conditions: spec.conditions,
	}
	if !wfpRuleMatches(base, spec) {
		t.Fatal("canonical rule rejected")
	}
	changes := map[string]func(*wf.Rule){
		"weight":             func(r *wf.Rule) { r.Weight++ },
		"disabled":           func(r *wf.Rule) { r.Disabled = true },
		"hard action":        func(r *wf.Rule) { r.HardAction = true },
		"missing conditions": func(r *wf.Rule) { r.Conditions = nil },
		"wrong app": func(r *wf.Rule) {
			r.Conditions = []*wf.Match{{Field: wf.FieldALEAppID, Op: wf.MatchTypeEqual, Value: "another-app"}}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := *base
			change(&r)
			if wfpRuleMatches(&r, spec) {
				t.Fatal("drift accepted")
			}
		})
	}
	for _, s := range wfpTunRuleSpecs(42) {
		r := &wf.Rule{Provider: wfpProviderID, Sublayer: wfpSublayerID, Layer: s.layer,
			Action: s.action, Weight: s.weight, Persistent: true,
			Conditions: wfpTunRuleSpecs(43)[0].conditions}
		if reflect.DeepEqual(r.Conditions, s.conditions) || wfpRuleMatches(r, s) {
			t.Fatal("wrong interface LUID accepted")
		}
	}
}

func TestWFPPolicyShape(t *testing.T) {
	base := wfpBaseRuleSpecs(`c:\snowden-system.exe`)
	if len(base) != 8 {
		t.Fatalf("base rules=%d, want 8", len(base))
	}
	seen := map[byte]bool{}
	blocks := map[wf.LayerID]bool{}
	for i, r := range base {
		if seen[r.id] {
			t.Fatalf("duplicate stable rule id %d", r.id)
		}
		seen[r.id] = true
		if r.action == wf.ActionBlock {
			blocks[r.layer] = true
			if r.weight != 0 || i < len(base)-2 {
				t.Fatalf("catch-all block must be last and lowest weight: %+v", r)
			}
		} else if r.weight == 0 {
			t.Fatalf("permit %q must outrank catch-all", r.name)
		}
	}
	if !blocks[wf.LayerALEAuthConnectV4] || !blocks[wf.LayerALEAuthConnectV6] {
		t.Fatalf("both address families must be blocked: %v", blocks)
	}

	tun := wfpTunRuleSpecs(42)
	if len(tun) != 2 {
		t.Fatalf("tun permits=%d, want 2", len(tun))
	}
	for _, r := range tun {
		if r.action != wf.ActionPermit || r.weight != 12 || len(r.conditions) != 1 || r.conditions[0].Field != wf.FieldIPLocalInterface {
			t.Fatalf("invalid TUN permit: %+v", r)
		}
		if seen[r.id] {
			t.Fatalf("TUN stable id collides: %d", r.id)
		}
		seen[r.id] = true
	}
}

func TestWFPStableObjectIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := byte(1); i <= 21; i++ {
		id := wfpStableGUID(i).String()
		if seen[id] {
			t.Fatalf("duplicate GUID for object %d", i)
		}
		seen[id] = true
	}
}
