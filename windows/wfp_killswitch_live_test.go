//go:build live && windows

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/tailscale/wf"
)

// TestLiveWFPBasePolicy is intentionally run only in an elevated, isolated
// test process. applyWFPBase closes its WFP session before the assertions, so
// a blocked external curl proves that the persistent objects survive session
// close. Cleanup is deferred and verified by a second curl.
func TestLiveWFPBasePolicy(t *testing.T) {
	if !isAdmin() {
		t.Skip("requires elevated process")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_ = removeWFPPolicy()
	defer func() {
		if err := removeWFPPolicy(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()

	if !externalCurlEventuallyWorks(20*time.Second, 2) {
		t.Skip("direct network unavailable before WFP test")
	}
	if err := applyWFPBase(exe); err != nil {
		t.Fatalf("apply base: %v", err)
	}
	if err := verifyWFPPolicy(false); err != nil {
		t.Fatalf("verify: %v", err)
	}
	for i := 0; i < 3; i++ {
		if externalCurlWorks(3 * time.Second) {
			t.Fatal("external process escaped catch-all WFP block")
		}
	}

	// The owning executable permit is necessary for in-process transport
	// sockets used by the embedded sing-box engine.
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 8*time.Second)
	if err != nil {
		t.Fatalf("owned executable TCP permit is ineffective: %v", err)
	}
	conn.Close()

	if err := removeWFPPolicy(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !externalCurlEventuallyWorks(20*time.Second, 2) {
		t.Fatal("external connectivity not restored after WFP cleanup")
	}
}

// TestLiveWFPTUNRouteAndCrashBlock validates the layer that does not depend on
// any remote Snowden server: Windows routing into a temporary Wintun adapter,
// the LUID permit, and the fail-closed transition when its engine disappears.
// The test-only outbound is direct, so this proves firewall mechanics rather
// than protected egress; production still requires externalTUNProbe to match
// the configured VPS address before it can report running.
func TestLiveWFPTUNRouteAndCrashBlock(t *testing.T) {
	if !isAdmin() {
		t.Skip("requires elevated process")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_ = removeWFPPolicy()
	var engine *core.Engine
	defer func() {
		if engine != nil {
			_ = engine.Stop()
		}
		if err := removeWFPPolicy(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()

	before, ok := externalLiteralIP(20 * time.Second)
	if !ok {
		t.Skip("direct network unavailable before TUN test")
	}
	const directTUNConfig = `{
  "log":{"level":"error"},
  "dns":{"servers":[{"tag":"tunnel-dns","address":"https://1.1.1.1/dns-query","detour":"direct"}],"rules":[{"server":"tunnel-dns"}],"strategy":"prefer_ipv4"},
  "inbounds":[{"type":"tun","tag":"tun-in","interface_name":"snowden0","address":["10.203.0.1/30","fdfe:dcba:9876::1/126"],"auto_route":true,"strict_route":true,"route_exclude_address":["172.16.0.0/12"],"stack":"gvisor"}],
  "outbounds":[{"type":"direct","tag":"direct"}],
  "route":{"auto_detect_interface":true,"rules":[{"protocol":"dns","action":"hijack-dns"}],"final":"direct"}
}`

	// Prove that this machine can route through the exact same test TUN before
	// attributing a failure to WFP. This is especially important on developer
	// hosts that may already be behind another full-tunnel VPN.
	engine = core.NewEngine([]byte(directTUNConfig))
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("start control TUN without WFP: %v", err)
	}
	control, ok := externalLiteralEventuallyIP(20 * time.Second)
	if stopErr := engine.Stop(); stopErr != nil {
		t.Fatalf("stop control TUN without WFP: %v", stopErr)
	}
	engine = nil
	if !ok {
		t.Skip("test TUN cannot carry traffic without WFP in the current network environment")
	}
	if before != control {
		t.Logf("control TUN selected a different physical egress: before=%s through-tun=%s", before, control)
	}
	if !externalLiteralEventuallyWorks(20*time.Second, 2) {
		t.Fatal("direct connectivity did not recover after control TUN cleanup")
	}

	if err := applyWFPBase(exe); err != nil {
		t.Fatalf("apply base: %v", err)
	}
	engine = core.NewEngine([]byte(directTUNConfig))
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("start test TUN: %v", err)
	}
	if err := permitWFPTUN(wfpTunName, 8*time.Second); err != nil {
		t.Fatalf("permit TUN LUID: %v", err)
	}
	after, ok := externalLiteralEventuallyIP(20 * time.Second)
	if !ok {
		t.Fatalf("external process cannot traverse permitted TUN LUID\n%s", liveWFPDiagnostics())
	}
	if control != after {
		t.Fatalf("WFP-permitted TUN egress=%s, want control egress=%s", after, control)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop test TUN: %v", err)
	}
	engine = nil
	for i := 0; i < 3; i++ {
		if _, ok := externalLiteralIP(3 * time.Second); ok {
			t.Fatal("external process escaped after TUN stopped with WFP active")
		}
	}
	if err := removeWFPPolicy(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !externalLiteralEventuallyWorks(20*time.Second, 2) {
		t.Fatal("external connectivity not restored after TUN/WFP cleanup")
	}
}

func liveWFPDiagnostics() string {
	var b strings.Builder
	luid, luidErr := waitInterfaceLUID(wfpTunName, time.Second)
	fmt.Fprintf(&b, "resolved LUID=%d err=%v\n", luid, luidErr)
	s, err := wf.New(&wf.Options{Name: wfpProviderName})
	if err != nil {
		fmt.Fprintf(&b, "open WFP=%v\n", err)
	} else {
		rules, rulesErr := s.Rules()
		fmt.Fprintf(&b, "enumerate WFP=%v\n", rulesErr)
		for _, rule := range rules {
			if rule.Provider == wfpProviderID {
				fmt.Fprintf(&b, "rule=%q layer=%v weight=%d action=%v hard=%v conditions=%+v\n", rule.Name, rule.Layer, rule.Weight, rule.Action, rule.HardAction, rule.Conditions)
			}
		}
		s.Close()
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", `Get-NetIPInterface -InterfaceAlias 'snowden0' | Format-List InterfaceAlias,InterfaceIndex,InterfaceMetric,ConnectionState,NlMtu,AddressFamily; Get-NetIPAddress -InterfaceAlias 'snowden0' | Format-List IPAddress,PrefixLength,AddressFamily; Get-NetRoute -InterfaceAlias 'snowden0' | Sort-Object AddressFamily,DestinationPrefix | Format-Table -AutoSize DestinationPrefix,NextHop,RouteMetric,ifIndex`)
	if out, routeErr := cmd.CombinedOutput(); routeErr != nil {
		fmt.Fprintf(&b, "route diagnostics error=%v output=%s\n", routeErr, out)
	} else {
		fmt.Fprintf(&b, "routes:\n%s\n", out)
	}
	return b.String()
}

func externalCurlWorks(timeout time.Duration) bool {
	seconds := int(timeout.Seconds())
	cmd := exec.Command("curl.exe", "--silent", "--show-error", "--fail", "--noproxy", "*", "--max-time", strconv.Itoa(seconds), "https://api.ipify.org")
	return cmd.Run() == nil
}

func externalCurlIP(timeout time.Duration) (string, bool) {
	seconds := int(timeout.Seconds())
	cmd := exec.Command("curl.exe", "--silent", "--show-error", "--fail", "--noproxy", "*", "--max-time", strconv.Itoa(seconds), "https://api.ipify.org")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	ip := strings.TrimSpace(string(out))
	return ip, net.ParseIP(ip) != nil
}

func externalLiteralIP(timeout time.Duration) (string, bool) {
	seconds := int(timeout.Seconds())
	cmd := exec.Command("curl.exe", "--silent", "--show-error", "--fail", "--noproxy", "*", "--max-time", strconv.Itoa(seconds), "https://1.1.1.1/cdn-cgi/trace")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ip=") {
			ip := strings.TrimPrefix(line, "ip=")
			return ip, net.ParseIP(ip) != nil
		}
	}
	return "", false
}

func externalLiteralEventuallyWorks(timeout time.Duration, consecutive int) bool {
	deadline := time.Now().Add(timeout)
	seen := 0
	for time.Now().Before(deadline) {
		if _, ok := externalLiteralIP(5 * time.Second); ok {
			seen++
			if seen >= consecutive {
				return true
			}
		} else {
			seen = 0
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func externalLiteralEventuallyIP(timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ip, ok := externalLiteralIP(3 * time.Second); ok {
			return ip, true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", false
}

func externalCurlEventuallyWorks(timeout time.Duration, consecutive int) bool {
	deadline := time.Now().Add(timeout)
	seen := 0
	for time.Now().Before(deadline) {
		if externalCurlWorks(5 * time.Second) {
			seen++
			if seen >= consecutive {
				return true
			}
		} else {
			seen = 0
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
