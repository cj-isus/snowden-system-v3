package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func withKillSwitchMarker(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "killswitch.json")
	previous := killSwitchStatePathFn
	killSwitchStatePathFn = func() string { return path }
	t.Cleanup(func() { killSwitchStatePathFn = previous })
	if raw != "<absent>" {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func assertMarkerUnchanged(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != want {
		t.Fatalf("recovery marker changed: got %q, %v; want %q", raw, err, want)
	}
}

func TestStartupRetainsEveryProtectionMarker(t *testing.T) {
	for _, raw := range []string{
		`{"policy":"strict-wfp","helper_pid":0}`,
		`{"helper_pid":4242,"applied_at":"2020-01-01T00:00:00Z"}`,
		`{"policy":"future-policy"}`, `{"broken":`, `{}`, `null`, "",
	} {
		t.Run(raw, func(t *testing.T) {
			path := withKillSwitchMarker(t, raw)
			logs := 0
			reportRetainedKillSwitch(func(level, text string) {
				logs++
				if level != "warn" || text == "" {
					t.Fatal("missing actionable retained-policy warning")
				}
			})
			if logs != 1 || !strictProtectionRetained() {
				t.Fatal("startup forgot protection intent")
			}
			assertMarkerUnchanged(t, path, raw)
		})
	}
	t.Run("no marker", func(t *testing.T) {
		withKillSwitchMarker(t, "<absent>")
		reportRetainedKillSwitch(func(string, string) { t.Fatal("no marker must not produce a recovery warning") })
		if strictProtectionRetained() {
			t.Fatal("absent marker treated as retained intent")
		}
	})
	t.Run("unreadable directory", func(t *testing.T) {
		path := withKillSwitchMarker(t, "<absent>")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		calls := 0
		reportRetainedKillSwitch(func(string, string) { calls++ })
		if calls != 1 || !strictProtectionRetained() {
			t.Fatal("unreadable marker must require recovery")
		}
	})
}

func TestArmStrictRejectsLegacyOrUnknownState(t *testing.T) {
	for _, raw := range []string{
		`{"helper_pid":4242}`, `{}`, `null`, "", `{"broken":`,
		`{"policy":"future-policy"}`,
		`{"policy":"strict-wfp","helper_pid":4242}`,
		`{"policy":"strict-wfp","helper_pid":-1}`,
	} {
		t.Run(raw, func(t *testing.T) {
			path := withKillSwitchMarker(t, raw)
			err := armStrictKillSwitch(path, "synthetic.exe", func(string) error {
				t.Fatal("unknown/legacy marker reached WFP mutation")
				return nil
			})
			if !errors.Is(err, errProtectionRecoveryRequired) {
				t.Fatalf("want explicit recovery error, got %v", err)
			}
			assertMarkerUnchanged(t, path, raw)
		})
	}
	t.Run("unreadable", func(t *testing.T) {
		path := withKillSwitchMarker(t, "<absent>")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		err := armStrictKillSwitch(path, "synthetic.exe", func(string) error {
			t.Fatal("unreadable marker reached WFP mutation")
			return nil
		})
		if !errors.Is(err, errProtectionRecoveryRequired) {
			t.Fatalf("read failure lost recovery requirement: %v", err)
		}
	})
}

func TestArmStrictWritesIntentBeforePolicyAndRetainsItOnFailure(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			name := map[bool]string{false: "new", true: "retained"}[existing] + "/" + map[bool]string{false: "success", true: "apply fails"}[fail]
			t.Run(name, func(t *testing.T) {
				raw := `{"policy":"strict-wfp","helper_pid":0,"applied_at":"2020-01-01T00:00:00Z"}`
				initial := "<absent>"
				if existing {
					initial = raw
				}
				path := withKillSwitchMarker(t, initial)
				cause := errors.New("synthetic WFP failure")
				calls := 0
				err := armStrictKillSwitch(path, "synthetic.exe", func(exe string) error {
					calls++
					if exe != "synthetic.exe" {
						t.Fatalf("wrong executable: %q", exe)
					}
					data, readErr := os.ReadFile(path)
					var state killSwitchState
					if readErr != nil || json.Unmarshal(data, &state) != nil || state.Policy != "strict-wfp" || state.HelperPID != 0 {
						t.Fatal("WFP apply ran without a strict intent marker")
					}
					if fail {
						return cause
					}
					return nil
				})
				if calls != 1 || (fail && !errors.Is(err, cause)) || (!fail && err != nil) || !strictProtectionRetained() {
					t.Fatalf("apply outcome/intent mismatch: calls=%d err=%v", calls, err)
				}
				if existing {
					assertMarkerUnchanged(t, path, raw)
				}
			})
		}
	}
	t.Run("cannot persist intent", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "not-directory")
		if err := os.WriteFile(parent, []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := armStrictKillSwitch(filepath.Join(parent, "marker"), "synthetic.exe", func(string) error {
			t.Fatal("WFP apply ran without persisted intent")
			return nil
		})
		if err == nil {
			t.Fatal("marker persistence failure was swallowed")
		}
	})
}

func TestExplicitProtectionRecoveryRetainsMarkerUntilAllCleanupSucceeds(t *testing.T) {
	for _, tc := range []struct {
		name                string
		raw                 string
		wfpFail, legacyFail bool
	}{
		{"strict", `{"policy":"strict-wfp"}`, false, false},
		{"legacy", `{"helper_pid":4242}`, false, false},
		{"damaged", `{"broken":`, false, false},
		{"absent", "<absent>", false, false},
		{"WFP fails", `{"helper_pid":4242}`, true, false},
		{"netsh fails", `{"policy":"strict-wfp"}`, false, true},
		{"both fail", `{"helper_pid":4242}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := withKillSwitchMarker(t, tc.raw)
			wfpCause, legacyCause := errors.New("synthetic WFP cleanup"), errors.New("synthetic netsh cleanup")
			wfpCalled := false
			assertIntent := func() {
				if tc.raw != "<absent>" {
					assertMarkerUnchanged(t, path, tc.raw)
				}
			}
			r := &fakeRunner{before: func() {
				if !wfpCalled {
					t.Fatal("legacy cleanup skipped WFP cleanup")
				}
				assertIntent()
			}}
			if tc.legacyFail {
				r.failOn, r.err = "-app", legacyCause
			}
			err := releaseKillSwitch(path, func() error {
				wfpCalled = true
				assertIntent()
				if tc.wfpFail {
					return wfpCause
				}
				return nil
			}, r)
			if !wfpCalled || len(r.commands) != 7 {
				t.Fatal("not all backends/rules attempted")
			}
			if tc.wfpFail || tc.legacyFail {
				if err == nil || (tc.wfpFail && !errors.Is(err, wfpCause)) || (tc.legacyFail && !errors.Is(err, legacyCause)) {
					t.Fatalf("cleanup failure lost: %v", err)
				}
				assertIntent()
			} else if err != nil || strictProtectionRetained() {
				t.Fatalf("successful explicit recovery did not retire marker: %v", err)
			}
		})
	}
}

func TestExplicitProtectionRecoveryCanRetryAndReportsMarkerFailure(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		raw := `{"helper_pid":4242}`
		path := withKillSwitchMarker(t, raw)
		r := &fakeRunner{failOn: "-app"}
		if err := releaseKillSwitch(path, func() error { return nil }, r); err == nil {
			t.Fatal("expected first cleanup failure")
		}
		assertMarkerUnchanged(t, path, raw)
		r.failOn = ""
		if err := releaseKillSwitch(path, func() error { return nil }, r); err != nil || strictProtectionRetained() {
			t.Fatalf("explicit recovery could not retry: %v", err)
		}
	})
	t.Run("marker removal fails", func(t *testing.T) {
		path := withKillSwitchMarker(t, "<absent>")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "keep"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := releaseKillSwitch(path, func() error { return nil }, &fakeRunner{}); err == nil || !strictProtectionRetained() {
			t.Fatalf("marker removal failure must be visible: %v", err)
		}
	})
}
