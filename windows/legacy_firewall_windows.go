//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/snowden-system/windows/backend/core"
	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/charmap"
)

const (
	legacyFirewallCommandTimeout = 5 * time.Second // admission/operation policy; original negative reap is separate
	legacyFirewallOutputLimit    = 4096
)

var errLegacyFirewallRuleAbsent = errors.New("legacy firewall rule absent")

// Real runner failures never expose arbitrary command output to UI/logs and
// cannot be waived by the injected legacy runner's text compatibility heuristic.
type legacyFirewallCommandFailure struct {
	rule  string
	code  uint32
	cause error
}

func (failure *legacyFirewallCommandFailure) Error() string {
	if failure.cause != nil {
		return "legacy firewall cleanup did not complete"
	}
	return fmt.Sprintf("legacy firewall rule %q cleanup failed (status %d)", failure.rule, failure.code)
}

func (failure *legacyFirewallCommandFailure) Unwrap() error { return failure.cause }

type osRunner struct{}

// Only the exact seven historical deletes are accepted. Manual Disconnect
// already owns the serialized release phase; no shell/PATH or general runner.
func (osRunner) Run(name string, args ...string) error {
	rule := ""
	var command []string
	if name == "netsh" {
		for _, candidate := range killSwitchAllRuleNames() {
			expected := killSwitchDeleteArgs(candidate)
			if slices.Equal(args, expected) {
				rule, command = candidate, expected // execute owned canonical arguments, never the caller's mutable slice
				break
			}
		}
	}
	if rule == "" {
		return &legacyFirewallCommandFailure{cause: core.ErrNativeScopeUnavailable}
	}
	ctx, cancel := context.WithTimeout(context.Background(), legacyFirewallCommandTimeout)
	defer cancel()
	directory, err := windows.GetSystemDirectory()
	if err != nil || directory == "" {
		return &legacyFirewallCommandFailure{rule: rule, cause: errors.Join(core.ErrNativeScopeUnavailable, err)}
	}
	result, err := core.CaptureProtectedCommand(ctx, filepath.Join(directory, "netsh.exe"), command, legacyFirewallOutputLimit)
	if err != nil {
		return &legacyFirewallCommandFailure{rule: rule, cause: err}
	}
	if result.ExitCode == 0 {
		return nil
	}
	if result.ExitCode == 1 && legacyFirewallMissingOutput(result.Output) {
		return errLegacyFirewallRuleAbsent // exact fact only after original command/Job/I/O retirement
	}
	return &legacyFirewallCommandFailure{rule: rule, code: result.ExitCode}
}

func legacyFirewallMissingOutput(output []byte) bool {
	if len(output) == 0 || len(output) > legacyFirewallOutputLimit || netguardUTF16LE(output) && len(output)%2 != 0 {
		return false
	}
	// Reuse existing pure UTF-8/UTF-16LE/Windows-1251 decoding; no log or
	// NetGuard task is read or invoked. Only exact whole messages are accepted.
	if legacyFirewallMissingMessage(strings.Join(netguardLogLines(output), "\n")) {
		return true
	}
	if decoded, err := charmap.CodePage866.NewDecoder().Bytes(output); err == nil && legacyFirewallMissingMessage(string(decoded)) {
		return true
	}
	// The existing log heuristic recognizes ASCII UTF-16 and BOMs. A Russian
	// UTF-16 message without BOM can still be compared as complete code units.
	if len(output)%2 == 0 {
		units := make([]uint16, len(output)/2)
		for index := range units {
			units[index] = uint16(output[2*index]) | uint16(output[2*index+1])<<8
		}
		return legacyFirewallMissingMessage(string(utf16.Decode(units)))
	}
	return false
}

func legacyFirewallMissingMessage(message string) bool {
	message = strings.ToLower(strings.Join(strings.Fields(strings.TrimPrefix(message, "\ufeff")), " "))
	switch message {
	case "no rules match the specified criteria.", "no rules match the specified criteria",
		"no rules were specified.", "no rules were specified",
		"ни одно правило не соответствует указанным критериям.", "ни одно правило не соответствует указанным критериям",
		"ни одно правило не соответствует указанным условиям.", "ни одно правило не соответствует указанным условиям",
		"правило не найдено.", "правило не найдено":
		return true
	default:
		return false
	}
}
