package main

// onboarding_dialogs.go — файловые диалоги для onboarding (V2-049/F8).
// Вынесено в отдельный файл, чтобы импорт wails/runtime жил в одном месте.

import (
	"fmt"
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
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("onboarding: чтение файла: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
