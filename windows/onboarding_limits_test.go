package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/onboarding"
)

func TestKDFOnboardingFileBounds(t *testing.T) {
	for _, name := range []string{"valid", "boundary", "oversized", "oversized-whitespace", "empty", "directory", "missing"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "synthetic.snob")
			data := " \nSNOB1.synthetic\r\n"
			var want error
			switch name {
			case "boundary":
				data = "SNOB1." + strings.Repeat("A", onboarding.MaxBundleSize-6)
			case "oversized":
				data = strings.Repeat("A", onboarding.MaxBundleSize+1)
				want = onboarding.ErrTooLarge
			case "oversized-whitespace":
				data = strings.Repeat(" ", onboarding.MaxBundleSize+1)
				want = onboarding.ErrTooLarge
			case "empty":
				data = ""
				want = onboarding.ErrBadFormat
			case "directory":
				path = t.TempDir()
			case "missing":
				want = os.ErrNotExist
			}
			if name != "directory" && name != "missing" {
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := readOnboardingTransport(path)
			if name == "directory" {
				if err == nil {
					t.Fatal("directory accepted")
				}
				return
			}
			if !errors.Is(err, want) {
				t.Fatalf("file bound error: %v", err)
			}
			if want == nil && got != strings.TrimSpace(data) {
				t.Fatal("file contents changed")
			}
		})
	}
}

func TestKDFExportPasswordLimitBeforeVault(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if out, err := (&App{}).ExportOnboarding(strings.Repeat("x", onboarding.MaxPassphraseSize+1)); !errors.Is(err, onboarding.ErrTooLarge) || out != "" {
		t.Fatalf("export accessed profile before password limit: %v", err)
	}
}
