//go:build windows

package main

// instancelock_windows.go — ожидание освобождения SingleInstance-мьютекса
// при переключении режима TUN↔SOCKS (V2-080).
//
// ДЕФЕКТ (доказан live 2026-09-18/20): switchMode запускает новый процесс
// через ShellExecuteW «runas», и вызов возвращается СРАЗУ — до того, как
// старый процесс завершится. Новый процесс доходит до wails.Run →
// SetupSingleInstance, видит ERROR_ALREADY_EXISTS, находит окно старого
// процесса и вызывает os.Exit(0) — МОЛЧА: ни окна, ни строки в журнале,
// ни crash-лога. Для владельца это выглядит как «приложение вылетело при
// нажатии Включить TUN» (журнал обрывается строкой «перезапуск в TUN
// (elevated)», а нового процесса нет).
//
// Механика Wails проверена по исходникам pinned-версии:
//   internal/frontend/desktop/windows/single_instance.go (v2.13.0):
//     id := "wails-app-" + uniqueId
//     mutexName := id + "sim"
//     CreateMutex → ERROR_ALREADY_EXISTS → FindWindowW → SendMessage → os.Exit(0)
//
// РЕШЕНИЕ: процесс, запущенный переключением режима (флаг modeSwitchAwaitFlag),
// до wails.Run ждёт освобождения мьютекса предыдущим процессом. Мьютекс
// освобождается ядром автоматически при завершении владельца (в т.ч. по
// os.Exit), поэтому достаточно опроса CreateMutex с ограничением по времени.
// После ожидания handle сразу закрывается: мьютекс обязан создать именно
// Wails, иначе его SetupSingleInstance не создаст окно-приёмник WM_COPYDATA и
// защита «один экземпляр» перестанет работать для обычных повторных запусков.

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// singleInstanceMutexName — имя мьютекса Wails v2 для нашего UniqueId
// (options.SingleInstanceLock.UniqueId = "snowden-system-v3" в main.go):
// "wails-app-" + uniqueId + "sim".
const singleInstanceMutexName = "wails-app-snowden-system-v3sim"

// modeSwitchAwaitFlag — маркер перезапуска при переключении режима: «этот
// процесс поднят вместо предыдущего, дождись его выхода».
const modeSwitchAwaitFlag = "--await-previous"

// modeSwitchAwaitRequested — был ли процесс запущен переключением режима.
func modeSwitchAwaitRequested() bool {
	for _, arg := range os.Args[1:] {
		if arg == modeSwitchAwaitFlag {
			return true
		}
	}
	return false
}

// awaitSingleInstanceReleased — ждать, пока предыдущий экземпляр отдаст
// мьютекс. true = освобождён (можно стартовать); false = таймаут (стартуем
// всё равно: молчаливый выход хуже, чем повторный запуск — честный crash-лог
// пишет вызывающий).
func awaitSingleInstanceReleased(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(singleInstanceMutexName))
		if h != 0 {
			// Handle закрываем всегда: владельцем мьютекса должен остаться
			// только Wails (см. шапку файла).
			_ = windows.CloseHandle(h)
		}
		switch {
		case err == nil:
			return true // мьютекс свободен — предыдущий процесс завершился
		case err == windows.ERROR_ALREADY_EXISTS:
			if time.Now().After(deadline) {
				return false
			}
			time.Sleep(150 * time.Millisecond)
		default:
			// Неожиданная ошибка (права/имя): не блокируем старт, но и не
			// выдаём ожидание за успех.
			return false
		}
	}
}
