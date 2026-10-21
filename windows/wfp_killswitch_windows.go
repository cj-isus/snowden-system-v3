//go:build windows

package main

// WFP kill switch for production TUN mode.
//
// The policy follows the proven WireGuard/Tailscale layout: a dedicated
// high-priority sublayer contains specific permit filters above a catch-all
// block. Objects are persistent with no automatic expiry. Crash, GUI close and
// retries retain the barrier; explicit Disconnect removes it after the engine
// stops. Local shape checks do not replace OS/live acceptance (PLAN OFF-05).

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

const (
	wfpProviderName = "Snowden System TUN kill switch"
	wfpSublayerName = "Snowden System protected TUN"
	wfpTunName      = "snowden0"
)

var (
	wfpProviderID = wf.ProviderID(wfpStableGUID(1))
	wfpSublayerID = wf.SublayerID(wfpStableGUID(2))

	modIPHlpAPI                     = windows.NewLazySystemDLL("iphlpapi.dll")
	procConvertInterfaceIndexToLUID = modIPHlpAPI.NewProc("ConvertInterfaceIndexToLuid")
)

// wfpStableGUID gives every owned object a deterministic identity. Stable IDs
// make crash cleanup exact: no name matching and no deletion of foreign rules.
func wfpStableGUID(object byte) windows.GUID {
	return windows.GUID{
		Data1: 0x8a6cb7d4,
		Data2: 0x0e71,
		Data3: 0x4e4c,
		Data4: [8]byte{0x9b, 0x9e, 0x75, 0x35, 0x42, 0x2a, 0x90, object},
	}
}

type wfpRuleSpec struct {
	id         byte
	name       string
	layer      wf.LayerID
	weight     uint64
	action     wf.Action
	conditions []*wf.Match
}

func wfpBaseRuleSpecs(appID string) []wfpRuleSpec {
	const (
		// Keep the same compact UINT64 ordering used by WireGuard's proven
		// Windows firewall policy. Only relative order matters inside our
		// dedicated highest-priority sublayer.
		weightApp      = 15
		weightLoopback = 13
		weightDHCP     = 12
		weightBlock    = 0
	)
	return []wfpRuleSpec{
		{10, "permit engine IPv4", wf.LayerALEAuthConnectV4, weightApp, wf.ActionPermit, []*wf.Match{{Field: wf.FieldALEAppID, Op: wf.MatchTypeEqual, Value: appID}}},
		{11, "permit engine IPv6", wf.LayerALEAuthConnectV6, weightApp, wf.ActionPermit, []*wf.Match{{Field: wf.FieldALEAppID, Op: wf.MatchTypeEqual, Value: appID}}},
		{12, "permit loopback IPv4", wf.LayerALEAuthConnectV4, weightLoopback, wf.ActionPermit, []*wf.Match{{Field: wf.FieldFlags, Op: wf.MatchTypeFlagsAllSet, Value: wf.ConditionFlagIsLoopback}}},
		{13, "permit loopback IPv6", wf.LayerALEAuthConnectV6, weightLoopback, wf.ActionPermit, []*wf.Match{{Field: wf.FieldFlags, Op: wf.MatchTypeFlagsAllSet, Value: wf.ConditionFlagIsLoopback}}},
		{14, "permit DHCP IPv4", wf.LayerALEAuthConnectV4, weightDHCP, wf.ActionPermit, []*wf.Match{
			{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: wf.IPProtoUDP},
			{Field: wf.FieldIPLocalPort, Op: wf.MatchTypeEqual, Value: uint16(68)},
			{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeEqual, Value: uint16(67)},
		}},
		{15, "permit DHCP IPv6", wf.LayerALEAuthConnectV6, weightDHCP, wf.ActionPermit, []*wf.Match{
			{Field: wf.FieldIPProtocol, Op: wf.MatchTypeEqual, Value: wf.IPProtoUDP},
			{Field: wf.FieldIPLocalPort, Op: wf.MatchTypeEqual, Value: uint16(546)},
			{Field: wf.FieldIPRemotePort, Op: wf.MatchTypeEqual, Value: uint16(547)},
		}},
		// Catch-all blocks are intentionally last in the returned list. If any
		// prerequisite permit cannot be installed, no blocking rule is added and
		// the caller refuses to start TUN.
		{18, "block direct IPv4", wf.LayerALEAuthConnectV4, weightBlock, wf.ActionBlock, nil},
		{19, "block direct IPv6", wf.LayerALEAuthConnectV6, weightBlock, wf.ActionBlock, nil},
	}
}

func wfpTunRuleSpecs(luid uint64) []wfpRuleSpec {
	const weightTUN = 12
	conditions := func() []*wf.Match {
		return []*wf.Match{{Field: wf.FieldIPLocalInterface, Op: wf.MatchTypeEqual, Value: luid}}
	}
	return []wfpRuleSpec{
		{20, "permit snowden0 IPv4", wf.LayerALEAuthConnectV4, weightTUN, wf.ActionPermit, conditions()},
		{21, "permit snowden0 IPv6", wf.LayerALEAuthConnectV6, weightTUN, wf.ActionPermit, conditions()},
	}
}

func addWFPRule(s *wf.Session, spec wfpRuleSpec) error {
	rule := &wf.Rule{
		ID:          wf.RuleID(wfpStableGUID(spec.id)),
		Name:        spec.name,
		Description: "Snowden System fail-closed TUN policy",
		Layer:       spec.layer,
		Sublayer:    wfpSublayerID,
		Weight:      spec.weight,
		Conditions:  spec.conditions,
		Action:      spec.action,
		// A soft catch-all block is required here. ALE may classify a flow
		// before and again after Windows routes it onto the TUN; making the
		// first block final prevents the later, more specific LUID permit from
		// taking effect. The block still wins over all lower-weight rules in
		// this top-weight owned sublayer.
		HardAction: false,
		Persistent: true,
		Provider:   wfpProviderID,
	}
	if err := s.AddRule(rule); err != nil {
		return fmt.Errorf("WFP add %q: %w", spec.name, err)
	}
	return nil
}

// applyWFPBase installs the lockdown before starting the TUN engine. The
// application itself remains able to establish protected transport sockets;
// all other new outbound connections are blocked until the TUN LUID permit is
// attached.
func applyWFPBase(exePath string) error {
	appID, err := wf.AppID(exePath)
	if err != nil {
		return fmt.Errorf("WFP app id: %w", err)
	}
	s, err := wf.New(&wf.Options{Name: wfpProviderName, Description: "persistent strict TUN kill switch"})
	if err != nil {
		return fmt.Errorf("WFP open: %w", err)
	}
	defer s.Close()
	rules, err := s.Rules()
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.Provider == wfpProviderID {
			// Never tear down a live barrier just to retry/reconnect. A damaged
			// owned policy requires explicit Disconnect before a fresh install.
			if err := verifyWFPPolicy(false); err != nil {
				return err
			}
			return revokeWFPTUN()
		}
	}
	if err := s.AddProvider(&wf.Provider{
		ID:          wfpProviderID,
		Name:        wfpProviderName,
		Description: "Owned only by Snowden System",
		Persistent:  true,
	}); err != nil {
		return fmt.Errorf("WFP provider: %w", err)
	}
	if err := s.AddSublayer(&wf.Sublayer{
		ID:          wfpSublayerID,
		Name:        wfpSublayerName,
		Description: "Specific permits followed by direct-traffic catch-all block",
		Persistent:  true,
		Provider:    wfpProviderID,
		Weight:      ^uint16(0),
	}); err != nil {
		return fmt.Errorf("WFP sublayer: %w", err)
	}
	for _, spec := range wfpBaseRuleSpecs(appID) {
		if err := addWFPRule(s, spec); err != nil {
			return err
		}
	}
	if err := verifyWFPPolicy(false); err != nil {
		return err
	}
	return nil
}

// permitWFPTUN waits for the adapter created by sing-box, resolves its LUID,
// adds the only general egress permit and verifies the effective owned set.
func permitWFPTUN(name string, timeout time.Duration) error {
	luid, err := waitInterfaceLUID(name, timeout)
	if err != nil {
		return err
	}
	s, err := wf.New(&wf.Options{Name: wfpProviderName})
	if err != nil {
		return fmt.Errorf("WFP open for TUN permit: %w", err)
	}
	defer s.Close()
	for _, spec := range wfpTunRuleSpecs(luid) {
		// A prior partial attempt may have installed one family.
		_ = s.DeleteRule(wf.RuleID(wfpStableGUID(spec.id)))
		if err := addWFPRule(s, spec); err != nil {
			return err
		}
	}
	return verifyWFPPolicy(true)
}

func waitInterfaceLUID(name string, timeout time.Duration) (uint64, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		interfaces, err := net.Interfaces()
		if err != nil {
			last = err.Error()
		} else {
			for _, iface := range interfaces {
				if strings.EqualFold(iface.Name, name) {
					var luid uint64
					ret, _, _ := procConvertInterfaceIndexToLUID.Call(uintptr(uint32(iface.Index)), uintptr(unsafe.Pointer(&luid)))
					if ret == 0 && luid != 0 {
						return luid, nil
					}
					last = fmt.Sprintf("ConvertInterfaceIndexToLuid=%d", ret)
				}
			}
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("WFP: интерфейс %s не появился за %s (%s)", name, timeout, last)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func verifyWFPPolicy(requireTUN bool) error {
	s, err := wf.New(&wf.Options{Name: wfpProviderName})
	if err != nil {
		return fmt.Errorf("WFP verify open: %w", err)
	}
	defer s.Close()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	sublayers, err := s.Sublayers(wfpProviderID)
	if err != nil {
		return fmt.Errorf("WFP verify sublayer: %w", err)
	}
	validSublayer := false
	for _, sl := range sublayers {
		if sl.ID == wfpSublayerID {
			validSublayer = sl.Provider == wfpProviderID && sl.Persistent && sl.Weight == ^uint16(0)
		}
	}
	if !validSublayer {
		return fmt.Errorf("WFP verify: sublayer ownership/persistence/weight mismatch")
	}
	appID, err := wf.AppID(exe)
	if err != nil {
		return err
	}
	rules, err := s.Rules()
	if err != nil {
		return fmt.Errorf("WFP enumerate filters: %w", err)
	}
	want := map[wf.RuleID]wfpRuleSpec{}
	for _, spec := range wfpBaseRuleSpecs(appID) {
		want[wf.RuleID(wfpStableGUID(spec.id))] = spec
	}
	if requireTUN {
		luid, err := waitInterfaceLUID(wfpTunName, 250*time.Millisecond)
		if err != nil {
			return err
		}
		for _, spec := range wfpTunRuleSpecs(luid) {
			want[wf.RuleID(wfpStableGUID(spec.id))] = spec
		}
	}
	found := map[wf.RuleID]bool{}
	for _, rule := range rules {
		spec, owned := want[rule.ID]
		if !owned {
			if rule.Provider == wfpProviderID && rule.ID != wf.RuleID(wfpStableGUID(20)) && rule.ID != wf.RuleID(wfpStableGUID(21)) {
				return fmt.Errorf("WFP verify: unexpected owned filter %q", rule.Name)
			}
			continue
		}
		if !wfpRuleMatches(rule, spec) {
			return fmt.Errorf("WFP verify: filter %q has unexpected ownership/shape", rule.Name)
		}
		found[rule.ID] = true
	}
	for id, spec := range want {
		if !found[id] {
			return fmt.Errorf("WFP verify: обязательный filter %q отсутствует", spec.name)
		}
	}
	return nil
}

// Compare security-relevant shape, including AppID/LUID and conditions on a
// catch-all block. Condition enumeration order has no policy meaning.
func wfpRuleMatches(rule *wf.Rule, spec wfpRuleSpec) bool {
	if rule.Provider != wfpProviderID || rule.Sublayer != wfpSublayerID ||
		rule.Layer != spec.layer || rule.Action != spec.action || !rule.Persistent ||
		rule.Disabled || rule.BootTime || rule.HardAction || rule.Weight != spec.weight ||
		len(rule.Conditions) != len(spec.conditions) {
		return false
	}
	used := make([]bool, len(spec.conditions))
	for _, got := range rule.Conditions {
		found := false
		for i, want := range spec.conditions {
			if !used[i] && reflect.DeepEqual(got, want) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Remove only TUN permits, retaining the base lockdown throughout teardown.
func revokeWFPTUN() error {
	s, err := wf.New(&wf.Options{Name: wfpProviderName})
	if err != nil {
		return err
	}
	defer s.Close()
	rules, err := s.Rules()
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.Provider == wfpProviderID && (rule.ID == wf.RuleID(wfpStableGUID(20)) || rule.ID == wf.RuleID(wfpStableGUID(21))) {
			if err := s.DeleteRule(rule.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeWFPPolicy removes only objects carrying our stable provider ID.
func removeWFPPolicy() error {
	s, err := wf.New(&wf.Options{Name: wfpProviderName})
	if err != nil {
		return fmt.Errorf("WFP cleanup open: %w", err)
	}
	defer s.Close()
	var errs []string
	rules, enumErr := s.Rules()
	if enumErr != nil {
		return fmt.Errorf("WFP cleanup enumerate: %w", enumErr)
	}
	for _, rule := range rules {
		if rule.Provider == wfpProviderID {
			if err := s.DeleteRule(rule.ID); err != nil {
				errs = append(errs, fmt.Sprintf("delete filter %s: %v", rule.Name, err))
			}
		}
	}
	sublayers, err := s.Sublayers(wfpProviderID)
	if err != nil {
		errs = append(errs, "enumerate sublayers: "+err.Error())
	}
	for _, sl := range sublayers {
		if sl.ID == wfpSublayerID {
			if err := s.DeleteSublayer(sl.ID); err != nil {
				errs = append(errs, "delete sublayer: "+err.Error())
			}
		}
	}
	providers, err := s.Providers()
	if err != nil {
		errs = append(errs, "enumerate providers: "+err.Error())
	}
	for _, provider := range providers {
		if provider.ID == wfpProviderID {
			if err := s.DeleteProvider(provider.ID); err != nil {
				errs = append(errs, "delete provider: "+err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("WFP cleanup incomplete: %s", strings.Join(errs, "; "))
	}
	return nil
}

// externalTUNProbe proves that a process not covered by the engine AppID can
// reach the expected egress only through the permitted TUN interface. The
// target is an IP literal so Windows DNS Client (a separate svchost AppID)
// cannot turn a successful LUID policy into a false failure.
func externalTUNProbe(expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		cmd := exec.Command("curl.exe", "--silent", "--show-error", "--fail", "--noproxy", "*", "--max-time", "3", "https://1.1.1.1/cdn-cgi/trace")
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}
		got := ""
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "ip=") {
				got = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "ip="))
				break
			}
		}
		if net.ParseIP(got) == nil {
			lastErr = fmt.Errorf("trace response has no valid egress")
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if got != strings.TrimSpace(expected) {
			return fmt.Errorf("WFP external protected probe: egress mismatch")
		}
		return nil
	}
	return fmt.Errorf("WFP external protected probe: route not ready before timeout: %w", lastErr)
}
