package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	commands []string
	failOn   string
	err      error
	before   func()
}

func (f *fakeRunner) Run(name string, args ...string) error {
	if f.before != nil {
		f.before()
	}
	joined := name + " " + strings.Join(args, " ")
	f.commands = append(f.commands, joined)
	if f.failOn != "" && strings.Contains(joined, f.failOn) {
		if f.err != nil {
			return f.err
		}
		return fmt.Errorf("fake failure on %s", f.failOn)
	}
	return nil
}

func TestLegacyRemovalIsExactAndIdempotent(t *testing.T) {
	want := []string{"app", "loopback", "icmp", "icmp6", "lan", "block-v4", "block-v6"}
	r := &fakeRunner{}
	for attempt := 0; attempt < 2; attempt++ {
		if err := removeKillSwitch(r); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.commands) != 14 {
		t.Fatalf("want seven deletions twice, got %v", r.commands)
	}
	for i, suffix := range want {
		cmd := "netsh advfirewall firewall delete rule name=" + killSwitchPolicyName + "-" + suffix
		if r.commands[i] != cmd || r.commands[i+7] != cmd {
			t.Fatalf("legacy scope/order drift: %v", r.commands)
		}
	}
}

func TestLegacyRemovalReportsFailureAndContinues(t *testing.T) {
	cause := errors.New("synthetic access denied")
	r := &fakeRunner{failOn: "-app", err: cause}
	if err := removeKillSwitch(r); !errors.Is(err, cause) {
		t.Fatalf("real failure lost: %v", err)
	}
	if len(r.commands) != 7 {
		t.Fatal("one deletion failure prevented remaining cleanup")
	}
}

func TestMissingLegacyRuleErrorIsLocaleAware(t *testing.T) {
	for _, message := range []string{
		"No rules were specified.", "No rules match the specified criteria.",
		"Ни одно правило не соответствует указанным критериям.", "Правило не найдено",
	} {
		t.Run(message, func(t *testing.T) {
			if !isMissingLegacyRuleError(errors.New(message)) {
				t.Fatal("known absent rule should be idempotent")
			}
			if err := removeKillSwitch(&fakeRunner{failOn: "name=", err: errors.New(message)}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"permission", errors.New("Access is denied")},
		{"unknown localization", errors.New("Неизвестная ошибка")},
		{"generic missing", errors.New("Файл не найдено")},
		{"missing executable", &exec.Error{Name: "netsh", Err: errors.New("Правило не найдено")}},
		{"path error", &os.PathError{Op: "exec", Path: "netsh", Err: errors.New("No rules match")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if isMissingLegacyRuleError(tc.err) {
				t.Fatalf("command failure incorrectly accepted as absent rules: %v", tc.err)
			}
		})
	}
}

func TestStrictMarkerRoundTripHasNoHelperPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marker.json")
	if err := armStrictKillSwitch(path, "synthetic.exe", func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), `"policy":"strict-wfp"`) || strings.Contains(string(raw), "helper_pid") {
		t.Fatalf("new marker must be strict without a PID: %s, %v", raw, err)
	}
}
