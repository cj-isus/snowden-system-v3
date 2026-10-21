package main

// onboarding_dialogs.go — файловые диалоги для onboarding (V2-049/F8).
// Вынесено в отдельный файл, чтобы импорт wails/runtime жил в одном месте.

import (
	"fmt"
	"github.com/snowden-system/windows/backend/onboarding"
	"io"
	"os"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveOnboardingFile — диалог сохранения transport-строки в файл (.snob).
func (a *App) SaveOnboardingFile(passphrase string) (string, error) {
	transport, err := a.ExportOnboarding(passphrase)
	if err != nil {
		return "", err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: "snowden-onboarding.snob",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Onboarding-бандл (*.snob)", Pattern: "*.snob"},
		},
	})
	if err != nil || path == "" {
		return "", nil // отмена — не ошибка
	}
	if err := os.WriteFile(path, []byte(transport), 0o600); err != nil {
		return "", fmt.Errorf("onboarding: запись файла: %w", err)
	}
	a.appendLog("info", "onboarding: бандл сохранён в файл")
	return path, nil
}

// LoadOnboardingFile — диалог выбора .snob-файла, возвращает transport-строку.
func (a *App) LoadOnboardingFile() (string, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{
			{DisplayName: "Onboarding-бандл (*.snob)", Pattern: "*.snob"},
			{DisplayName: "Все файлы", Pattern: "*"},
		},
	})
	if err != nil || path == "" {
		return "", nil
	}
	return readOnboardingTransport(path)
}

// Bound the file read before loading it into memory or invoking the codec.
func readOnboardingTransport(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("onboarding: чтение файла: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", onboarding.ErrBadFormat
	}
	if st.Size() > onboarding.MaxBundleSize {
		return "", onboarding.ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, onboarding.MaxBundleSize+1))
	if err != nil {
		return "", fmt.Errorf("onboarding: чтение файла: %w", err)
	}
	if len(data) > onboarding.MaxBundleSize {
		return "", onboarding.ErrTooLarge
	}
	transport := strings.TrimSpace(string(data))
	if transport == "" {
		return "", onboarding.ErrBadFormat
	}
	return transport, nil
}
