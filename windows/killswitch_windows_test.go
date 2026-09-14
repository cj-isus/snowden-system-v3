package main

// Тесты kill switch (V2-038) — чистые функции + fake runner: никакого
// реального netsh в тестах. Проверяются инварианты безопасности порядка
// (allow до block), идемпотентность снятия, dead-man контракт и файл состояния.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRunner — записывает команды вместо исполнения.
type fakeRunner struct {
	commands []string
	failOn   string // подстрока имени правила, на котором падать ("" = не падать)
}

func (f *fakeRunner) Run(name string, args ...string) error {
	joined := name + " " + strings.Join(args, " ")
	if f.failOn != "" && strings.Contains(joined, f.failOn) {
		return fmt.Errorf("fake failure on %s", f.failOn)
	}
	f.commands = append(f.commands, joined)
	return nil
}

func (f *fakeRunner) countByName(suffix string) int {
	n := 0
	for _, c := range f.commands {
		if strings.Contains(c, "name="+killSwitchPolicyName+suffix) {
			n++
		}
	}
	return n
}

func (f *fakeRunner) hasDelete(rule string) bool {
	for _, c := range f.commands {
		if strings.Contains(c, "delete rule") && strings.Contains(c, "name="+rule) {
			return true
		}
	}
	return false
}

func (f *fakeRunner) hasAdd(rule string) bool {
	for _, c := range f.commands {
		if strings.Contains(c, "add rule") && strings.Contains(c, "name="+rule) {
			return true
		}
	}
	return false
}

// TestKillSwitchPolicyShape — состав политики: exe-allow, loopback, ICMP, LAN,
// два блок-правила (v4+v6). Инвариант: правило нашего exe существует — иначе
// политика душит собственный движок (in-process sing-box).
func TestKillSwitchPolicyShape(t *testing.T) {
	rules := killSwitchAllowRules(`C:\Programs\snowden-system\snowden-system.exe`)
	if len(rules) < 5 {
		t.Fatalf("too few allow rules: %d", len(rules))
	}
	joined := ""
	for _, r := range rules {
		joined += strings.Join(r, " ") + "\n"
	}
	for _, want := range []string{
		"program=C:\\Programs\\snowden-system\\snowden-system.exe",
		"remoteip=127.0.0.0/8,::1",
		"icmp:4", "icmp:6", "remoteip=LocalSubnet",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("allow policy missing %q\n%s", want, joined)
		}
	}
	blocks := killSwitchBlockRules()
	if len(blocks) != 2 {
		t.Fatalf("want exactly 2 block rules (v4+v6), got %d", len(blocks))
	}
	if !strings.Contains(strings.Join(blocks[0], " "), "0.0.0.0/0") ||
		!strings.Contains(strings.Join(blocks[1], " "), "::/0") {
		t.Fatalf("block rules must cover v4 and v6: %v", blocks)
	}
}

// TestKillSwitchRuleNamesMatchPolicy — список имён для снятия покрывает ВСЕ
// правила политики (иначе частичное снятие оставит блок навсегда).
func TestKillSwitchRuleNamesMatchPolicy(t *testing.T) {
	want := map[string]bool{}
	for _, r := range append(killSwitchAllowRules("EXE"), killSwitchBlockRules()...) {
		for _, part := range r {
			if strings.HasPrefix(part, "name=") {
				want[strings.TrimPrefix(part, "name=")] = true
			}
		}
	}
	got := killSwitchAllRuleNames()
	if len(got) != len(want) {
		t.Fatalf("rule names %v vs policy %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Fatalf("extra name in removal list: %s", n)
		}
	}
}

// TestApplyKillSwitchOrderAndContent — порядок критичен: allow ДО block
// (окно «allow без block» безопасно, «block без allow exe» = самоубийство
// политики), block последними.
func TestApplyKillSwitchOrderAndContent(t *testing.T) {
	dir := t.TempDir()
	orig := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "ks.json") }
	defer func() { killSwitchStatePathFn = orig }()

	r := &fakeRunner{}
	exe := `C:\x\snowden-system.exe`
	helperStarted := false
	origHelper := startKillSwitchHelperFn
	startKillSwitchHelperFn = func(string, time.Duration) error { helperStarted = true; return nil }
	defer func() { startKillSwitchHelperFn = origHelper }()
	if err := applyKillSwitch(r, exe, killSwitchExpireDefault); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !helperStarted {
		t.Fatal("dead-man helper must start before any firewall change")
	}
	lastAdd, firstBlock := -1, -1
	for i, c := range r.commands {
		if strings.Contains(c, "add rule") {
			lastAdd = i
			if strings.Contains(c, "-block-") && firstBlock == -1 {
				firstBlock = i
			}
		}
	}
	if lastAdd == -1 || firstBlock == -1 {
		t.Fatalf("missing add rules: %v", r.commands)
	}
	appRuleIdx := -1
	for i, c := range r.commands {
		if strings.Contains(c, "name="+killSwitchPolicyName+"-app ") || strings.HasSuffix(c, "name="+killSwitchPolicyName+"-app") ||
			strings.Contains(c, "program="+exe) {
			appRuleIdx = i
			break
		}
	}
	if appRuleIdx == -1 {
		t.Fatalf("app-allow rule missing: %v", r.commands)
	}
	if appRuleIdx > firstBlock {
		t.Fatalf("app allow must precede block rules: app=%d firstBlock=%d", appRuleIdx, firstBlock)
	}
	if !r.hasAdd(killSwitchPolicyName + "-loopback") {
		t.Fatalf("loopback allow missing")
	}
}

// TestApplyKillSwitchFailureRollsBack — падение на block-правиле = попытка
// снять уже применённые allow (fail-safe: не оставлять половинную политику).
func TestApplyKillSwitchFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	orig := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "ks.json") }
	defer func() { killSwitchStatePathFn = orig }()

	r := &fakeRunner{failOn: "-block-v4"}
	origHelper := startKillSwitchHelperFn
	startKillSwitchHelperFn = func(string, time.Duration) error { return nil }
	defer func() { startKillSwitchHelperFn = origHelper }()
	err := applyKillSwitch(r, `C:\x\app.exe`, killSwitchExpireDefault)
	if err == nil || !strings.Contains(err.Error(), "block-правило не применено") {
		t.Fatalf("want block failure error, got %v", err)
	}
	if !r.hasDelete(killSwitchPolicyName+"-app") || !r.hasDelete(killSwitchPolicyName+"-loopback") {
		t.Fatalf("rollback must delete applied allow rules: %v", r.commands)
	}
}

// TestRemoveKillSwitchIdempotent — повторное снятие не падает: delete
// несуществующих правил игнорируется.
func TestRemoveKillSwitchIdempotent(t *testing.T) {
	dir := t.TempDir()
	orig := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "ks.json") }
	defer func() { killSwitchStatePathFn = orig }()

	r := &fakeRunner{}
	if err := removeKillSwitch(r); err != nil {
		t.Fatalf("first remove: %v", err)
	}
	if err := removeKillSwitch(r); err != nil {
		t.Fatalf("second remove must tolerate absent rules: %v", err)
	}
	for _, name := range killSwitchAllRuleNames() {
		if !r.hasDelete(name) {
			t.Fatalf("removal must attempt every rule: missing %s", name)
		}
	}
}

// TestRemoveKillSwitchReportsRealFailure — реальная ошибка netsh (не «нет
// правила») должна вернуться вызывающему: частично снятая политика хуже
// честной ошибки.
func TestRemoveKillSwitchReportsRealFailure(t *testing.T) {
	dir := t.TempDir()
	orig := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "ks.json") }
	defer func() { killSwitchStatePathFn = orig }()

	r := &fakeRunner{failOn: "-block-v6"}
	if err := removeKillSwitch(r); err == nil {
		t.Fatal("real netsh failure must surface")
	}
}

// TestKillSwitchExpireRequestedArgs — парсинг аргументов helper-режима:
// ровно --killswitch-expire + валидные секунды; второй флаг без первого,
// мусор, переплата (>1ч) — всё не helper.
func TestKillSwitchExpireRequestedArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
		secs int
	}{
		{"valid", []string{"--killswitch-expire", "--killswitch-seconds=90"}, true, 90},
		{"flag only", []string{"--killswitch-expire"}, false, 0},
		{"seconds only", []string{"--killswitch-seconds=90"}, false, 0},
		{"zero", []string{"--killswitch-expire", "--killswitch-seconds=0"}, false, 0},
		{"negative", []string{"--killswitch-expire", "--killswitch-seconds=-5"}, false, 0},
		{"over hour", []string{"--killswitch-expire", "--killswitch-seconds=7200"}, false, 0},
		{"garbage", []string{"--killswitch-expire", "--killswitch-seconds=abc"}, false, 0},
	}
	for _, tc := range cases {
		oldArgs := os.Args
		os.Args = append([]string{"prog"}, tc.args...)
		got, ok := killSwitchExpireRequested()
		os.Args = oldArgs
		if ok != tc.want || (ok && got != tc.secs) {
			t.Fatalf("%s: got (%d,%v), want (%d,%v)", tc.name, got, ok, tc.secs, tc.want)
		}
	}
}

// TestKillSwitchStateFileRoundTrip — state-файл пишется и читается
// (PID helper'а для зачистки).
func TestKillSwitchStateFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "ks.json") }
	defer func() { killSwitchStatePathFn = orig }()

	st := killSwitchState{HelperPID: 4242, AppliedAt: time.Now()}
	data, err := jsonMarshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(killSwitchStatePathFn(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(killSwitchStatePathFn())
	if err != nil {
		t.Fatal(err)
	}
	var back killSwitchState
	if err := jsonUnmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.HelperPID != 4242 {
		t.Fatalf("pid: %d", back.HelperPID)
	}
}
