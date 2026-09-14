package main

// updateapp.go — применение обновлений приложения (V2-050/F15).
//
// Сценарий: владелец (или скрипт сборки) кладёт рядом с установленным exe
// update.json + payload (или указывает каталог с ними). Приложение:
//
//	1) читает манифест, проверяет подпись по trusted_keys.json (та же таблица
//	   доверия, что для envelope каналов — единственная цепочка издателя);
//	2) проверяет SHA-256 и размер payload;
//	3) требует версию строго больше текущей И выше этажа принятых версий
//	   (антидаунгрейд переживает переустановку старой версии поверх);
//	4) атомарная подмена: payload → temp в каталоге exe → rename поверх
//	   текущего exe (Windows: цель не должна быть запущена → swap выполняется
//	   специальным путём: текущий exe переименовывается в .old, новый
//	   переименовывается на его место, старт нового процесса, выход старого;
//	   .old удаляется новым процессом при старте);
//	5) новый процесс при старте поднимает этаж версий (update.RaiseFloor)
//	   и подчищает .old.
//
// Каждый шаг пишет в журнал; любые отказы оставляют систему в исходном
// состоянии (fail-closed): подпись/хэш/версия проверяются до первого rename.

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// runningVersion — версия текущего бинарника. Задаётся ldflags при сборке;
// для dev-сборок — честная метка "dev" (обновления на dev не применяются:
// нет ни осмысленного сравнения, ни подписанной истории).
var runningVersion = "dev"

// UpdatePreview — что покажет UI до применения.
type UpdatePreview struct {
	CurrentVersion string `json:"current_version"`
	Version        string `json:"version"`
	Notes          string `json:"notes,omitempty"`
	ReleasedAt     string `json:"released_at,omitempty"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
	KeyID          string `json:"key_id"`
	Floor          string `json:"floor"`
	OK             bool   `json:"ok"`
	Reason         string `json:"reason,omitempty"`
}

// trustedKeysForUpdate — та же таблица доверия, что у envelope.
func trustedKeysForUpdate() (map[string]ed25519.PublicKey, error) {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, fmt.Errorf("update: store dir: %w", err)
	}
	keys, present, err := (&metadata.Store{Dir: dir}).LoadKeys()
	if err != nil {
		return nil, fmt.Errorf("update: trusted keys: %w", err)
	}
	if !present || len(keys) == 0 {
		return nil, errors.New("update: доверенных ключей нет (fail-closed) — импортируйте профиль владельца")
	}
	return keys, nil
}

// loadAndVerifyManifest — чтение + структура + подпись (без payload).
func loadAndVerifyManifest(updateDir string) (*update.Manifest, error) {
	mb, err := os.ReadFile(filepath.Join(updateDir, "update.json"))
	if err != nil {
		return nil, fmt.Errorf("update: манифест не читается: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(mb)))
	dec.DisallowUnknownFields()
	var m update.Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("update: манифест: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	keys, err := trustedKeysForUpdate()
	if err != nil {
		return nil, err
	}
	if err := m.Verify(keys); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkVersionPolicy — антидаунгрейд и строгое возрастание.
func checkVersionPolicy(m *update.Manifest) error {
	cur, err := update.ParseVersion(runningVersion)
	if err != nil {
		return fmt.Errorf("update: текущая версия %q не разбирается: %w", runningVersion, err)
	}
	nv, _ := update.ParseVersion(m.Version) // Validate уже гарантировал разбор
	if nv.Compare(cur) <= 0 {
		return fmt.Errorf("update: версия %s не выше текущей %s — обновление отвергнуто", m.Version, runningVersion)
	}
	if floor := update.CurrentFloor(); nv.Compare(floor) <= 0 && floor != (update.Version{}) {
		return fmt.Errorf("update: версия %s не выше этажа принятых %s (антидаунгрейд) — отвергнуто", m.Version, floor.String())
	}
	return nil
}

// CheckUpdate — проверка каталога обновления (без применения). Честный
// превью-результат даже при отказе: OK=false + Reason.
func (a *App) CheckUpdate(updateDir string) (*UpdatePreview, error) {
	if updateDir == "" {
		return nil, errors.New("update: укажите каталог с update.json и payload")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prev := &UpdatePreview{CurrentVersion: runningVersion, Floor: update.CurrentFloor().String()}
	m, err := loadAndVerifyManifest(updateDir)
	if err != nil {
		prev.Reason = err.Error()
		return prev, nil // отказ — это результат проверки, не ошибка вызова
	}
	prev.Version = m.Version
	prev.Notes = m.Notes
	prev.ReleasedAt = m.ReleasedAt.Format(time.RFC3339)
	prev.Size = m.Size
	prev.SHA256 = m.SHA256
	prev.KeyID = m.KeyID

	payload := filepath.Join(updateDir, m.File)
	if err := m.VerifyPayload(payload); err != nil {
		prev.Reason = err.Error()
		return prev, nil
	}
	if err := checkVersionPolicy(m); err != nil {
		prev.Reason = err.Error()
		return prev, nil
	}
	prev.OK = true
	a.appendLogLocked("info", fmt.Sprintf("update: манифест %s проверен (подпись, sha256, версия) — готов к применению", m.Version))
	return prev, nil
}

// PickUpdateDir — диалог выбора каталога с update.json (нативный Wails).
func (a *App) PickUpdateDir() (string, error) {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Каталог с update.json и payload",
	})
	if err != nil {
		return "", err
	}
	return dir, nil // отмена — пустая строка, не ошибка
}

// ApplyUpdate — применить проверенное обновление: атомарная подмена exe и
// перезапуск. Возвращает текст для журнала UI (успех = процесс скоро умрёт).
func (a *App) ApplyUpdate(updateDir string) (string, error) {
	a.mu.Lock()
	// Повторная полная проверка (TOCTOU-гейт): между Check и Apply файлы могли
	// измениться — доверять можно только свежей проверке.
	m, err := loadAndVerifyManifest(updateDir)
	if err != nil {
		a.mu.Unlock()
		return "", err
	}
	if err := checkVersionPolicy(m); err != nil {
		a.mu.Unlock()
		return "", err
	}
	payload := filepath.Join(updateDir, m.File)
	if err := m.VerifyPayload(payload); err != nil {
		a.mu.Unlock()
		return "", err
	}
	a.appendLogLocked("info", fmt.Sprintf("update: применяю %s — подмена бинарника и перезапуск", m.Version))
	a.mu.Unlock()

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("update: путь exe: %w", err)
	}
	exe, _ = filepath.Abs(exe)

	// 1. Копируем payload в temp РЯДОМ с exe (rename работает только в рамках
	// одного тома).
	tmp := exe + ".update.tmp"
	if err := copyFile(payload, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update: staging: %w", err)
	}

	// 2. Останавливаем ядро (TUN/прокси/kill switch) ДО подмены — так новый
	// процесс стартует в чистом окружении, а системный прокси не останется
	// указывать на мёртвый порт.
	if err := a.stopCore(); err != nil {
		a.appendLog("warn", "update: остановка ядра перед подменой: "+err.Error())
	}

	// 3. Атомарная подмена: текущий exe → .old, staged → на место exe.
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update: rename текущего exe: %w (exe запущен?)", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		// Откат: вернуть старый exe на место — не оставлять машину без приложения.
		_ = os.Rename(old, exe)
		return "", fmt.Errorf("update: подмена: %w", err)
	}

	// 4. Перезапуск через detached-посредника: новый процесс должен стартовать
	// с теми же рабочими флагами (--auto-connect/--tun, если были).
	args := []string{}
	if autoConnectRequested() {
		args = append(args, autoConnectFlag)
	}
	if tunRequested() {
		args = append(args, "--tun")
	}
	// Перезапуск через detached-посредника: новый процесс должен стартовать
	// ПОСЛЕ освобождения SingleInstance-мьютекса (см. V2-050 урок: прямой
	// exec.Command до выхода старого давал новому процессу os.Exit(0) по
	// SingleInstanceLock — второй экземпляр уходит в WM_COPYDATA первому).
	// Посредник — отсоединённый powershell: ждёт смерти нашего PID, затем
	// стартует новый exe с теми же рабочими флагами. Он переживёт наш выход.
	pid := os.Getpid()
	flagStr := strings.Join(args, " ")
	// PS-строка: ожидание PID, затем Start-Process (экранирование: путь в кавычках).
	ps := fmt.Sprintf(
		`$p=Get-Process -Id %d -ErrorAction SilentlyContinue; while($p){Start-Sleep -Milliseconds 300; $p=Get-Process -Id %d -ErrorAction SilentlyContinue}; Start-Process -FilePath '%s'%s`,
		pid, pid, exe, func() string {
			if len(args) > 0 {
				return fmt.Sprintf(" -ArgumentList '%s'", flagStr)
			}
			return ""
		}())
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		// Новый не стартовал: exe на месте корректный (проверен sha256),
		// владелец запустит вручную. Честный отказ.
		return "", fmt.Errorf("update: запуск посредника перезапуска: %w (запустите приложение вручную)", err)
	}

	// 5. Выход старого. requestQuit делает cleanup-пути, но ядро уже остановлено;
	// короткая задержка — дать журналу записаться.
	a.appendLog("info", "update: новый процесс запущен — завершаю текущий")
	a.requestQuit(700 * time.Millisecond)
	return "обновление применено — приложение перезапускается", nil
}

// updateStartupHousekeeping — вызывается при старте нового процесса:
// поднять этаж версий (антидаунгрейд от отката) и удалить .old.
func updateStartupHousekeeping() {
	if runningVersion == "dev" {
		return // dev-сборки не участвуют в цепочке обновлений
	}
	if v, err := update.ParseVersion(runningVersion); err == nil {
		if _, err := update.RaiseFloor(v); err != nil {
			// Не фатально: этаж — усиление, а не условие старта.
			// Журнал ещё не поднят — пишем в crash-лог как факт.
			writeCrashLog("update: raise floor: " + err.Error())
		}
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
}

// copyFile — fsync-копия (данные должны дойти до диска до rename).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
