package main

// Strict WFP orchestration and explicit migration from retired netsh builds.
// No lease, timer or marker PID authorizes cleanup. Only Disconnect does.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const killSwitchPolicyName = "snowden-system-tun-killswitch"

// KillSwitchRunner isolates legacy rule deletion from the operating system.
type KillSwitchRunner interface {
	Run(name string, args ...string) error
}

type osRunner struct{}

func (osRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// HelperPID is decoded solely to reject mixed/legacy state. It is never used to
// inspect or terminate a process. New markers do not contain a PID.
type killSwitchState struct {
	HelperPID int       `json:"helper_pid,omitempty"`
	AppliedAt time.Time `json:"applied_at"`
	Policy    string    `json:"policy"`
}

var killSwitchStatePathFn = func() string {
	return filepath.Join(appDataDir(), "killswitch.json")
}

var errProtectionRecoveryRequired = errors.New("сохранено устаревшее или повреждённое состояние защиты; сначала явно нажмите Отключить с правами администратора")

func strictProtectionRetained() bool {
	_, err := os.Stat(killSwitchStatePathFn())
	return err == nil || !os.IsNotExist(err)
}

// Accept every spelling of retired helper flags, including incomplete/invalid
// values. Such an invocation must return before GUI/update/SingleInstance.
func legacyKillSwitchHelperRequested(args []string) bool {
	for _, arg := range args {
		for _, flag := range []string{"--killswitch-expire", "--killswitch-seconds"} {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}

func reportRetainedKillSwitch(logf func(level, text string)) {
	if strictProtectionRetained() {
		logf("warn", "Состояние защиты прошлого запуска сохранено. Устаревшая защита требует явного отключения с правами администратора; автоматическая очистка запрещена.")
	}
}

// These exact seven names are the entire legacy cleanup scope. No wildcard or
// global firewall reset; no ability to install another obsolete netsh policy.
func killSwitchAllRuleNames() []string {
	return []string{
		killSwitchPolicyName + "-app",
		killSwitchPolicyName + "-loopback",
		killSwitchPolicyName + "-icmp",
		killSwitchPolicyName + "-icmp6",
		killSwitchPolicyName + "-lan",
		killSwitchPolicyName + "-block-v4",
		killSwitchPolicyName + "-block-v6",
	}
}

func killSwitchDeleteArgs(rule string) []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + rule}
}

func removeKillSwitch(r KillSwitchRunner) error {
	var errs []error
	for _, name := range killSwitchAllRuleNames() {
		if err := r.Run("netsh", killSwitchDeleteArgs(name)...); err != nil && !isMissingLegacyRuleError(err) {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("kill switch: снятие неполное: %w", err)
	}
	return nil
}

func isMissingLegacyRuleError(err error) bool {
	if err == nil {
		return false
	}
	// Failure to execute netsh is not evidence that its rules are absent.
	var execErr *exec.Error
	var pathErr *os.PathError
	if errors.As(err, &execErr) || errors.As(err, &pathErr) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no rules were specified") ||
		strings.Contains(message, "no rules match") ||
		strings.Contains(message, "ни одно правило не соответствует") ||
		strings.Contains(message, "правило не найдено")
}

func armWFPKillSwitch(exe string) error {
	return armStrictKillSwitch(killSwitchStatePathFn(), exe, applyWFPBase)
}

// No migration is guessed from a PID or damaged file. Retained strict intent
// permits rearm, but does not prove enforcement: apply must still succeed.
func armStrictKillSwitch(path, exe string, apply func(string) error) error {
	data, err := os.ReadFile(path)
	if err == nil {
		var state killSwitchState
		if json.Unmarshal(data, &state) != nil || state.Policy != "strict-wfp" || state.HelperPID != 0 {
			return errProtectionRecoveryRequired
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("%w: %w", errProtectionRecoveryRequired, err)
	} else {
		data, err = json.Marshal(killSwitchState{AppliedAt: time.Now(), Policy: "strict-wfp"})
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("WFP strict marker directory: %w", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("WFP strict marker: %w", err)
		}
	}
	// Intent must survive apply failure/crash. Never remove it here.
	return apply(exe)
}

func disarmKillSwitch() error {
	return releaseKillSwitch(killSwitchStatePathFn(), removeWFPPolicy, osRunner{})
}

// Caller has explicitly requested Disconnect and confirmed engine cleanup.
// Attempt both backends and retain recovery intent on any error.
func releaseKillSwitch(path string, removeWFP func() error, legacy KillSwitchRunner) error {
	wfpErr := removeWFP()
	legacyErr := removeKillSwitch(legacy)
	if err := errors.Join(wfpErr, legacyErr); err != nil {
		return fmt.Errorf("kill switch cleanup: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("kill switch marker cleanup: %w", err)
	}
	return nil
}
