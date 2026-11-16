//go:build windows

package core

import (
	"context"
	"regexp"
	"strconv"
)

// The encrypted original-write journal alone does not authorize crash recovery:
// owner-death/freshness and legacy interoperability are not implemented. Neither
// a matching loopback port nor a dead listener authorizes restoring another
// process's HKCU settings. Current-session cleanup belongs to ProxyManager.
// Keep the legacy API explicitly unavailable until crash recovery has a shared
// original owner/receipt protocol, including the external UserMode script.
func HealStaleSystemProxy() (bool, error) {
	return false, ErrProxyReceiptUnavailable
}

// LoopbackProxyPort recognizes a compatible address format, never its owner.
// Foreign VPN/proxy applications may use the very same port and bytes.
func LoopbackProxyPort(server string) (int, bool) {
	re := regexp.MustCompile(`^(?:socks=)?127\.0\.0\.1:(\d+)$`)
	m := re.FindStringSubmatch(server)
	if m == nil {
		return 0, false
	}
	port, err := strconv.Atoi(m[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	if port != 1080 {
		return 0, false
	}
	return port, true
}

// Legacy standalone bool seam. App observers pass their actual lifetime to
// ObserveLoopback instead; no App path falls back to this Background scope.
// A true result requires this bounded original observation's successful cleanup.
func LoopbackAlive(port int) bool {
	facts := ObserveLoopback(context.Background(), port)
	return len(facts) == 1 && facts[0].Outcome == LoopbackReachable
}
