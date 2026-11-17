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
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

const (
	wfpProviderName = "Snowden System TUN kill switch"
	wfpSublayerName = "Snowden System protected TUN"
	wfpTunName      = render.TUNInterfaceName
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

func waitInterfaceLUID(name string, timeout time.Duration) (uint64, error) {
	return waitInterfaceLUIDContext(context.Background(), name, timeout)
}

func waitInterfaceLUIDContext(parent context.Context, name string, timeout time.Duration) (uint64, error) {
	if parent == nil || timeout <= 0 || timeout > 30*time.Second || name != wfpTunName {
		return 0, fmt.Errorf("invalid TUN interface wait configuration")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	var last string
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		interfaces, err := net.Interfaces()
		if err != nil {
			last = err.Error()
		} else {
			var found uint64
			for _, iface := range interfaces {
				if strings.EqualFold(iface.Name, name) && iface.Flags&net.FlagUp != 0 {
					var luid uint64
					ret, _, _ := procConvertInterfaceIndexToLUID.Call(uintptr(uint32(iface.Index)), uintptr(unsafe.Pointer(&luid)))
					if ret == 0 && luid != 0 {
						if found != 0 {
							return 0, fmt.Errorf("ambiguous TUN interface identity")
						}
						found = luid
					}
					last = fmt.Sprintf("ConvertInterfaceIndexToLuid=%d", ret)
				}
			}
			if found != 0 && ctx.Err() == nil {
				return found, nil
			}
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("WFP: TUN interface unavailable (%s): %w", last, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func verifyWFPPolicy(requireTUN bool) error {
	return verifyWFPPolicyContext(context.Background(), requireTUN, 0)
}

func verifyWFPPolicyContext(ctx context.Context, requireTUN bool, expectedLUID uint64) error {
	return verifyWFPScopeContext(ctx, requireTUN, expectedLUID, nil, false, nil)
}

// The legacy base observation tolerates existing general rules 20/21. A closed
// admission proof must instead reject them and any diagnostic/unknown filter,
// before helper creation and again after original diagnostic retirement.
func verifyWFPClosedContext(ctx context.Context) error {
	return verifyWFPScopeContext(ctx, false, 0, nil, true, nil)
}

// Diagnostic verification never tolerates the general LUID permits. Only the
// current exact six-condition, nonpersistent filter may accompany the base.
// nil requires its absence, including during pre-acquisition and retirement.
func verifyWFPDiagnosticContext(ctx context.Context, owner *nativeWFPSessionOwner, diagnostic *wfpRuleSpec) error {
	if owner == nil {
		return fmt.Errorf("WFP diagnostic original session unavailable")
	}
	return verifyWFPScopeContext(ctx, false, 0, diagnostic, true, owner)
}

func verifyWFPScopeContext(ctx context.Context, requireTUN bool, expectedLUID uint64, diagnostic *wfpRuleSpec, strictDiagnostic bool, owner *nativeWFPSessionOwner) error {
	if ctx == nil {
		return fmt.Errorf("WFP verification context unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if owner == nil {
		_, err := withOwnedWFPReader(ctx, func(current *nativeWFPSessionOwner) (struct{}, error) {
			if err := current.openSessionContext(ctx); err != nil {
				return struct{}{}, err
			}
			return struct{}{}, verifyWFPScopeContext(ctx, requireTUN, expectedLUID, diagnostic, strictDiagnostic, current)
		})
		return err
	}
	appID, err := owner.applicationAppID(ctx, "")
	if err != nil {
		return err
	}
	rules, validSublayer, err := owner.readPolicy(ctx, true)
	if err != nil {
		return fmt.Errorf("WFP verify owned policy: %w", err)
	}
	if !validSublayer {
		return fmt.Errorf("WFP verify: sublayer ownership/persistence/weight mismatch")
	}
	want := map[wf.RuleID]wfpRuleSpec{}
	for _, spec := range wfpBaseRuleSpecs(appID) {
		want[wf.RuleID(wfpStableGUID(spec.id))] = spec
	}
	if diagnostic != nil {
		if !strictDiagnostic || requireTUN || diagnostic.id != 30 {
			return fmt.Errorf("WFP diagnostic verification scope mismatch")
		}
		want[wf.RuleID(wfpStableGUID(diagnostic.id))] = *diagnostic
	}
	if requireTUN {
		luid, err := waitInterfaceLUIDContext(ctx, wfpTunName, 250*time.Millisecond)
		if err != nil {
			return err
		}
		if expectedLUID != 0 && luid != expectedLUID {
			return fmt.Errorf("WFP: TUN interface changed during lifecycle gate")
		}
		for _, spec := range wfpTunRuleSpecs(luid) {
			want[wf.RuleID(wfpStableGUID(spec.id))] = spec
		}
	}
	found := map[wf.RuleID]bool{}
	for _, rule := range rules {
		spec, owned := want[rule.ID]
		if !owned {
			if rule.Provider == wfpProviderID && (strictDiagnostic || rule.ID != wf.RuleID(wfpStableGUID(20)) && rule.ID != wf.RuleID(wfpStableGUID(21))) {
				return fmt.Errorf("WFP verify: unexpected owned filter %q", rule.Name)
			}
			continue
		}
		matches := wfpRuleMatches(rule, spec)
		if diagnostic != nil && spec.id == diagnostic.id {
			matches = wfpRuleMatchesPersistence(rule, spec, false) && !rule.PermitIfMissing && rule.Callout == (wf.CalloutID{}) && len(rule.ProviderData) == 0
		}
		if !matches {
			return fmt.Errorf("WFP verify: filter %q has unexpected ownership/shape", rule.Name)
		}
		found[rule.ID] = true
	}
	for id, spec := range want {
		if !found[id] {
			return fmt.Errorf("WFP verify: обязательный filter %q отсутствует", spec.name)
		}
	}
	return owner.queryScope(ctx)
}

// Compare security-relevant shape, including AppID/LUID and conditions on a
// catch-all block. Condition enumeration order has no policy meaning.
func wfpRuleMatches(rule *wf.Rule, spec wfpRuleSpec) bool {
	return wfpRuleMatchesPersistence(rule, spec, true)
}

func wfpRuleMatchesPersistence(rule *wf.Rule, spec wfpRuleSpec, persistent bool) bool {
	if rule.Provider != wfpProviderID || rule.Sublayer != wfpSublayerID ||
		rule.Layer != spec.layer || rule.Action != spec.action || rule.Persistent != persistent ||
		rule.Disabled || rule.BootTime || rule.HardAction || rule.PermitIfMissing || rule.Callout != (wf.CalloutID{}) || rule.Weight != spec.weight ||
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

// externalTUNProbe observes IPv4 egress from another executable alongside the
// verified LUID policy. It is not native DNS/UDP/IPv6 or kernel-owner proof.
func externalTUNProbe(expected string, timeout time.Duration) error {
	return externalTUNProbeContext(context.Background(), expected, timeout)
}

func externalTUNProbeContext(parent context.Context, expected string, timeout time.Duration) error {
	expectedIP := net.ParseIP(strings.TrimSpace(expected))
	if expectedIP == nil || expectedIP.To4() == nil || timeout <= 0 || timeout > 30*time.Second {
		return fmt.Errorf("external TUN IPv4 expectation is unavailable")
	}
	ctx, release, err := core.AdmitDiagnostic(parent, core.DiagnosticProtected, 3, 3*(4096+1), timeout)
	if err != nil {
		return err
	}
	defer release()
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return fmt.Errorf("external TUN system helper unavailable")
	}
	helperPath := filepath.Join(systemDirectory, "curl.exe")
	parentPath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("external TUN bypass identity unavailable")
	}
	parentID, err := readOwnedWFPAppID(ctx, parentPath)
	if err != nil || parentID == "" {
		return fmt.Errorf("external TUN bypass AppID unavailable")
	}
	helperID, err := readOwnedWFPAppID(ctx, helperPath)
	if err != nil || helperID == "" || strings.EqualFold(helperID, parentID) {
		return fmt.Errorf("external TUN helper must not share the bypass AppID")
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		output, err := core.CaptureProtectedHelper(attemptCtx, helperPath, []string{
			"-q", "--silent", "--fail", "--noproxy", "*", "--proxy", "",
			"--max-time", "3", "--connect-timeout", "2", "--max-filesize", "4096",
			"--proto", "=https", "--ipv4", "--write-out", "\\nSNOWDEN_HTTP_STATUS:%{http_code}\\n", render.ExternalTUNProbeURL,
		}, 4096)
		cancel()
		if errors.Is(err, core.ErrProtectedHelperCleanup) || errors.Is(err, core.ErrProtectedHelperBusy) {
			return err // never replace or work around an uncertain helper owner
		}
		if err == nil && ctx.Err() == nil {
			const statusMarker = "\nSNOWDEN_HTTP_STATUS:"
			body := string(output)
			marker := strings.LastIndex(body, statusMarker)
			if marker < 0 || strings.TrimSpace(body[marker+len(statusMarker):]) != "200" {
				return fmt.Errorf("external TUN trace status not confirmed")
			}
			var got net.IP
			fields := 0
			for _, line := range strings.Split(body[:marker], "\n") {
				if value, present := strings.CutPrefix(strings.TrimSpace(line), "ip="); present {
					fields++
					got = net.ParseIP(strings.TrimSpace(value))
				}
			}
			if fields == 1 && got != nil {
				if !got.Equal(expectedIP) {
					return fmt.Errorf("external TUN IPv4 egress mismatch")
				}
				return nil
			}
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
	return fmt.Errorf("external TUN IPv4 flow not confirmed within budget")
}
