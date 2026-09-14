package main

// Тесты продления lease (V2-046, P1): renewKillSwitchLease обязан перезапускать
// helper (kill старого PID + новый detached) и обновлять state-файл.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenewKillSwitchLeaseRestartsHelperAndRefreshesState(t *testing.T) {
	dir := t.TempDir()
	oldPath := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "killswitch.json") }
	defer func() { killSwitchStatePathFn = oldPath }()

	// Предусловие: применённая политика (helper + state-файл).
	exe := "C:/fake/snowden-system.exe"
	// Мок helper'а: реальный exe не нужен — инъекция через fn-переменную.
	origHelperFn := startKillSwitchHelperFn
	helperCalls := 0
	startKillSwitchHelperFn = func(_ string, exp time.Duration) error {
			helperCalls++
			st := killSwitchState{HelperPID: 9000 + helperCalls, AppliedAt: time.Now()}
			data, _ := json.Marshal(st)
			return os.WriteFile(killSwitchStatePathFn(), data, 0o600)
		}
	defer func() { startKillSwitchHelperFn = origHelperFn }()
	if err := applyKillSwitch(&fakeRunner{}, exe, 90*time.Second); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var old killSwitchState
	data, _ := os.ReadFile(killSwitchStatePathFn())
	if err := json.Unmarshal(data, &old); err != nil || old.HelperPID == 0 {
		t.Fatalf("state after apply missing/broken: %v", err)
	}

	if err := renewKillSwitchLease(exe, 90*time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if helperCalls != 2 { // apply + renew
		t.Fatalf("helper must be started twice (apply+renew), got %d", helperCalls)
	}
	var fresh killSwitchState
	data2, _ := os.ReadFile(killSwitchStatePathFn())
	if err := json.Unmarshal(data2, &fresh); err != nil {
		t.Fatalf("state after renew unreadable: %v", err)
	}
	if fresh.HelperPID == 0 || fresh.HelperPID == old.HelperPID {
		t.Fatalf("helper must be restarted with a NEW pid, got old=%d new=%d", old.HelperPID, fresh.HelperPID)
	}
	if !fresh.AppliedAt.After(old.AppliedAt) {
		t.Fatalf("AppliedAt must be refreshed: old=%v new=%v", old.AppliedAt, fresh.AppliedAt)
	}
}

func TestRenewKillSwitchLeaseWithoutStateFailsHonest(t *testing.T) {
	dir := t.TempDir()
	oldPath := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return filepath.Join(dir, "absent.json") }
	defer func() { killSwitchStatePathFn = oldPath }()

	err := renewKillSwitchLease("C:/fake/snowden-system.exe", 90*time.Second)
	if err == nil {
		t.Fatal("renew without prior apply must fail honestly, not silently no-op")
	}
	if !strings.Contains(err.Error(), "lease") {
		t.Fatalf("error should mention lease: %v", err)
	}
}
