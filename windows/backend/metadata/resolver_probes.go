package metadata

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const MaxDNSProbeTargets = 3
const PrimaryDNSProbeTarget = "https://1.1.1.1/dns-query"

// Compiled measurement endpoints, not claims about independent physical paths
// or application DNS configuration. Return owned values, never a mutable global.
func DefaultDNSProbeTargets() []string {
	return []string{PrimaryDNSProbeTarget, "https://dns.google/dns-query"}
}

// Empty preserves compiled defaults. An explicit list cannot silently fall
// back after validation failure. Distinct hosts are redundancy, not operator/
// ASN/account independence. These URLs carry no credentials or DNS query text.
func ValidateDNSProbeTargets(targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) < 2 || len(targets) > MaxDNSProbeTargets {
		return errors.New("resolver probe requires 2..3 HTTPS origins")
	}
	seen := map[string]bool{}
	for _, target := range targets {
		if len(target) > 2048 || strings.TrimSpace(target) != target || strings.ContainsAny(target, "?#") {
			return errors.New("resolver probe URL length/query/fragment is invalid")
		}
		u, err := url.Parse(target)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.Opaque != "" || strings.HasSuffix(u.Host, ":") {
			return errors.New("resolver probe requires an unambiguous HTTPS URL without credentials/query/fragment")
		}
		if port := u.Port(); port != "" {
			value, err := strconv.Atoi(port)
			if err != nil || value < 1 || value > 65535 {
				return errors.New("resolver probe port is invalid")
			}
		}
		host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
		if host == "" || strings.ContainsAny(host, "%\\ \t\r\n") {
			return errors.New("resolver probe hostname is invalid")
		}
		if ip := net.ParseIP(host); ip != nil {
			host = ip.String()
		} else if !dnsProbeHostname(host) {
			return errors.New("resolver probe requires an ASCII DNS name or exact IP literal")
		}
		if seen[host] {
			return errors.New("resolver probe origins must use distinct hosts")
		}
		seen[host] = true // alternate ports/paths on one host are not another source
	}
	return nil
}

func dnsProbeHostname(host string) bool {
	if len(host) > 253 || strings.Trim(host, "0123456789.") == "" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
