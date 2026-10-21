package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// appDataDir — корень данных приложения в профиле пользователя.
//
// Урок V2-017 (P0): хранилище секретов и логи НЕ ДОЛЖНЫ жить рядом с exe:
// wails build очищает build/bin, и каждый ребилд молча уничтожал secrets/.
// Данные — в %AppData%\snowden-system; exe остаётся одноразовым артефактом.
//
// V2-081 (аудит гигиены): в go test appDataDir резолвится в реальный %AppData%
// пользователя → тесты (NewApp, hardening, planv3) пишут в ПРОДАКШН-журнал
// %AppData%\snowden-system\logs\app.log — загрязнение прод-логов и рост до ГБ.
// Исправление: подменяемая точка (переменная уровня пакета) — тесты подменяют
// через t.TempDir(). Продуктовый путь не меняется.
var appDataDirOverride = func() string { return "" }

// testDataDir — единый изолированный каталог данных на процесс `go test`:
// создаётся лениво, живёт до конца процесса (ОС уберёт %TEMP% при очистке).
var (
	testDataDirOnce sync.Once
	testDataDirPath string
)

// runningUnderGoTest — true, когда процесс запущен как тестовый бинарник
// (`go test` ставит флаг test.v и имя *.test[.exe]). Проверка нужна ТОЛЬКО
// для изоляции данных (V2-081) и не влияет на продуктовое поведение.
func runningUnderGoTest() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	exe := strings.ToLower(os.Args[0])
	return strings.HasSuffix(exe, ".test") || strings.HasSuffix(exe, ".test.exe")
}

// testIsolatedDataDir — temp-каталог для тестового процесса ("" = не удался).
func testIsolatedDataDir() string {
	testDataDirOnce.Do(func() {
		dir, err := os.MkdirTemp("", "snowden-test-data-")
		if err == nil {
			testDataDirPath = dir
		}
	})
	return testDataDirPath
}

func appDataDir() string {
	if override := appDataDirOverride(); override != "" {
		return override
	}
	// V2-081: в тестах прод-каталог пользователя НЕ используется — иначе
	// NewApp()/стойкие тесты писали в реальный app.log (загрязнение прод-журнала
	// и рост файла). Явный override (t.TempDir) сильнее авто-изоляции.
	// Live tests may explicitly opt into the real profile. The opt-in is
	// deliberately verbose and never inferred from build tags: ordinary tests
	// must remain isolated even when a developer accidentally compiles `live`.
	if runningUnderGoTest() && os.Getenv("SNOWDEN_LIVE_USE_PROFILE") != "1" {
		if dir := testIsolatedDataDir(); dir != "" {
			return dir
		}
	}
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
