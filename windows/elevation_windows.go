package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellExecuteRunAs — ShellExecuteW с глаголом "runas": стандартный путь
// запроса UAC у Windows. Возвращает ошибку, если владелец отклонил запрос.
//
// Альтернативы отвергнуты сознательно:
//   - manifest requireAdministrator — elevated на каждый запуск (и SOCKS-режим
//     получил бы права без нужды; принцип минимальных привилегий нарушен);
//   - служба — вне скоупа Phase A (A3 может пересмотреть).
func shellExecuteRunAs(exe string, args string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, err := syscall.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	// ShellExecuteW(window, verb, file, params, dir, show) → HINSTANCE (>32 ok).
	modShell32 := windows.NewLazySystemDLL("shell32.dll")
	proc := modShell32.NewProc("ShellExecuteW")
	const SW_SHOWNORMAL = 1
	ret, _, callErr := proc.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		0,
		SW_SHOWNORMAL,
	)
	// Смежные ошибки (отказ UAC) приходят как коды <=32.
	if ret <= 32 && callErr != syscall.Errno(0) {
		if callErr == windows.ERROR_CANCELLED {
			return fmt.Errorf("запрос прав администратора отклонён")
		}
		return fmt.Errorf("elevated-запуск: %w", callErr)
	}
	return nil
}

// restartWithArgs — перезапуск текущего exe с новыми аргументами. При
// requestElevation=true глагол runas (UAC), иначе обычный запуск.
// Вызывающий обязан остановить VPN и завершить процесс сам (двух
// экземпляров на одном SOCKS-порте не должно оставаться).
func restartWithArgs(requestElevation bool, args ...string) error {
	exe, err := osExecutable()
	if err != nil {
		return fmt.Errorf("не найден путь exe: %w", err)
	}
	clean := make([]string, 0, len(args))
	for _, a := range args {
		if a != "" {
			clean = append(clean, a)
		}
	}
	quoted := make([]string, 0, len(clean))
	for _, a := range clean {
		quoted = append(quoted, quoteArg(a))
	}
	argStr := strings.Join(quoted, " ")

	if requestElevation {
		return shellExecuteRunAs(exe, argStr)
	}
	cmd := exec.Command(exe, clean...)
	cmd.Dir = exeDir(exe)
	return cmd.Start()
}

// quoteArg — экранирование одного аргумента командной строки Windows
// (пробелы требуют кавычек, кавычка внутри экранируется).
func quoteArg(a string) string {
	if strings.ContainsAny(a, " \t\"") {
		return `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
	}
	return a
}

// tunRequested — запрошен ли TUN-режим флагом командной строки (--tun).
// Флаг задаётся при elevated-перезапуске из UI (EnableTUN/DisableTUN).
func tunRequested() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--tun" {
			return true
		}
	}
	return false
}
