package main

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/windows/registry"
)

const (
	autostartRunKey        = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValueName     = "snowden-system"
	maxAutostartValueBytes = 64 * 1024
)

func autostartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil || len(exe) > maxAutostartValueBytes-32 || !utf8.ValidString(exe) ||
		!filepath.IsAbs(exe) || strings.ContainsAny(exe, "\"\x00\r\n") {
		return "", errors.New("путь приложения для автозапуска не подтверждён")
	}
	return `"` + exe + `" --auto-connect`, nil
}

func readAutostartValue(key registry.Key, expected string) (bool, error) {
	if expected == "" || len(expected) > maxAutostartValueBytes || !utf8.ValidString(expected) {
		return false, errAutostartUnavailable
	}
	units, err := syscall.UTF16FromString(expected)
	if err != nil || len(units) > maxAutostartValueBytes/2 {
		return false, errAutostartUnavailable
	}
	// One bounded native query. Do not retry/resize to a registry-supplied length
	// or let UTF16ToString truncate a foreign value at an embedded zero.
	buffer := make([]byte, maxAutostartValueBytes)
	n, kind, err := key.GetValue(autostartValueName, buffer)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil || kind != registry.SZ || n < 2 || n > len(buffer) || n%2 != 0 {
		return false, errors.New("настройка автозапуска не прочитана в поддерживаемом формате")
	}
	if n == 2 && binary.LittleEndian.Uint16(buffer[:2]) == 0 {
		return false, nil // canonical empty REG_SZ; a zero-length malformed value is unknown
	}
	if n == len(units)*2 {
		matches := true
		for index, unit := range units {
			if binary.LittleEndian.Uint16(buffer[index*2:index*2+2]) != unit {
				matches = false
				break
			}
		}
		if matches {
			return true, nil
		}
	}
	return false, errors.New("запись автозапуска содержит другую команду; изменение не разрешено")
}

// Private compatibility helpers share the same original process owner. The
// production tray/UI use App methods so foreign App ownership is also checked.
func readAutostart() (bool, error) { return accessAutostart(nil, false, false) }
func setAutostart(on bool) error {
	_, err := accessAutostart(nil, true, on)
	return err
}

// Existing Wails API: no binding/schema change or synthetic preview result.
func (a *App) GetAutostart() (bool, error) { return accessAutostart(a, false, false) }

func (a *App) SetAutostart(on bool) error {
	if _, err := accessAutostart(a, true, on); err != nil {
		return err
	}
	if on {
		a.appendLog("info", "автозапуск: запрос включения соответствует чтению реестра; будущий вход Windows не проверен")
	} else {
		a.appendLog("info", "автозапуск: запрос отключения соответствует чтению реестра")
	}
	return nil
}
