package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

// appDataDir — корень данных приложения в профиле пользователя.
//
// Урок V2-017 (P0): хранилище секретов и логи НЕ ДОЛЖНЫ жить рядом с exe:
// wails build очищает build/bin, и каждый ребилд молча уничтожал secrets/.
// Данные — в %AppData%\snowden-system; exe остаётся одноразовым артефактом.
func appDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "snowden-system")
	}
	// Фолбэк (UserConfigDir практически всегда доступен в Windows):
	// профиль пользователя, а НЕ каталог exe.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "AppData", "Roaming", "snowden-system")
	}
	return filepath.Join(".", "snowden-system-data")
}

// vaultDir — каталог хранилища секретов (DPAPI, права только у владельца).
func vaultDir() string { return filepath.Join(appDataDir(), "secrets") }

// logsDir — каталог журналов.
func logsDir() string { return filepath.Join(appDataDir(), "logs") }

// migrateLegacyVault — одноразовый перенос хранилища из старого места
// (build/bin/secrets, где его стирал ребилд) в AppData. Перенос ТОЛЬКО если
// в новом месте файла ещё нет, а в старом он есть и непуст — данные
// пользователя не перезаписываются никогда.
func migrateLegacyVault() {
	newPath := filepath.Join(vaultDir(), "vault.v1.json")
	if fileExists(newPath) {
		return
	}
	exe, err := osExecutable()
	if err != nil {
		return
	}
	oldPath := filepath.Join(exeDir(exe), "secrets", "vault.v1.json")
	data, err := os.ReadFile(oldPath)
	if err != nil || len(data) == 0 {
		return
	}
	if err := os.MkdirAll(vaultDir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(newPath, data, 0o600)
}

// execExplorer открывает папку в Проводнике.
func execExplorer(dir string) error {
	_ = os.MkdirAll(dir, 0o755)
	return exec.Command("explorer.exe", dir).Start()
}

// osExecutable/exeDir — обёртки для corebridge (без дублирования логики).
func osExecutable() (string, error) {
	return os.Executable()
}

func exeDir(path string) string {
	return filepath.Dir(path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
