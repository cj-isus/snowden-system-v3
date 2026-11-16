package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/update"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Package init/loader code has already run, including Wails native init.
	// This order gates main's services only; it is not pre-Job containment.
	brokerSession, brokerHandled, brokerErr := core.DispatchBrokerGUI(os.Args[1:], nativeProbeHelperSHA256)
	if brokerHandled {
		activeElevatedGUISession = brokerSession
		if brokerErr != nil || brokerSession == nil {
			os.Exit(3) // partial custody/unaccepted gate is terminal, never ordinary GUI fallback
		}
	}
	elevatedSession, elevatedHandled, elevatedErr := core.DispatchElevatedGUI(os.Args[1:])
	if elevatedHandled {
		activeElevatedGUISession = elevatedSession
		if elevatedErr != nil || elevatedSession == nil {
			os.Exit(3)
		}
	}
	if handled, exitCode := core.DispatchDeliveryWorker(os.Args[1:]); handled {
		os.Exit(exitCode) // pure bounded public-byte SDK; no repair/store/GUI fallback
	}
	if handled, exitCode := core.DispatchUpdateRecovery(os.Args[1:], verifyRecoveryEntrypoint, repairFromRecoveryEntrypoint); handled {
		os.Exit(exitCode) // companion/invalid internal recovery flags never fall through to GUI or workers
	}
	restartSession, restartHandled, restartErr := core.DispatchRestartGUI(os.Args[1:])
	if restartHandled {
		activeRestartGUISession = restartSession // keep even partial original native custody
		if restartErr != nil || restartSession == nil {
			os.Exit(3)
		}
	}
	guiSession, guiHandled, guiErr := core.DispatchUpdateGUI(os.Args[1:], prepareUpdateGUIImage, verifyUpdateCodeAcceptance)
	if guiHandled {
		if guiErr != nil || guiSession == nil {
			os.Exit(3) // all partial bootstrap/native owners are terminal, never GUI fallback
		}
		activeUpdateGUISession = guiSession
	}
	if handled, exitCode := core.DispatchUpdateCodeAcceptance(os.Args[1:], verifyUpdateCodeAcceptance); handled {
		os.Exit(exitCode)
	}
	if core.EngineWorkerInvocation(os.Args[1:]) {
		if checkRunningVersionFloor() != nil || checkAcceptedInstalledPair() != nil {
			os.Exit(2) // no worker below floor, during intent, or with mismatched accepted images
		}
	}
	if handled, exitCode := core.DispatchEngineWorker(os.Args[1:]); handled {
		os.Exit(exitCode)
	}
	// Retired helper invocations must not become GUI launches, even when their
	// arguments are incomplete. No timer, PID cleanup or firewall effects.
	if legacyKillSwitchHelperRequested(os.Args[1:]) {
		return
	}
	// Паника на главной горутине не должна уходить молча: пишем crash-лог
	// (AppData, не рядом с exe — wails build стирает build/bin) и отдаем
	// стектрейс дальше, чтобы процесс завершился с видимой причиной.
	defer func() {
		if rec := recover(); rec != nil {
			writeCrashLog(fmt.Sprintf("panic: %v\n\n%s", rec, debug.Stack()))
			panic(rec)
		}
	}()

	// V2-080: процесс, запущенный переключением режима TUN↔SOCKS, обязан
	// ДОЖДАТЬСЯ выхода предыдущего экземпляра. ShellExecuteW «runas» возвращается
	// сразу, старый процесс ещё держит мьютекс SingleInstance — без ожидания
	// новый процесс в Wails SetupSingleInstance получает ERROR_ALREADY_EXISTS,
	// находит окно старого и вызывает os.Exit(0) молча: «приложение вылетело при
	// нажатии Включить TUN» (доказано live 2026-09-18/20 + повторяемый тест
	// mutex-race: экземпляр с --tun при живом экземпляре умирал через ~1с).
	if modeSwitchAwaitRequested() {
		if ok := awaitSingleInstanceReleased(8 * time.Second); !ok {
			// Timeout/ACL/Close uncertainty is not proof of parent exit. No
			// housekeeping, storage, Wails or network producer may follow it.
			writeCrashLog("перезапуск: освобождение SingleInstance-мьютекса не подтверждено — запуск остановлен")
			os.Exit(3) // also releases any original probe handle whose Close was uncertain
		}
	}

	// Один экземпляр: Wails SingleInstanceLock держит именованный мьютекс
	// (wails-app-<id>sim); второй запуск шлёт WM_COPYDATA первому окну
	// (OnSecondInstanceLaunch: показать поверх) и делает os.Exit(0).
	// Это закрывает и «второй SOCKS на 1080»: второй процесс не доходит до
	// ядра. Переключение режима совместимо: старый процесс умирает по Quit
	// в 300мс окне, его мьютекс освобождает ядро, новый стартует первым.
	// F15 (V2-050): после успешного обновления новый процесс поднимает этаж
	// версий (антидаунгрейд) и подчищает .old от подмены.
	exe, imageErr := os.Executable()
	if guiSession != nil {
		imageErr = update.ErrPairRecoveryRequired // no legacy housekeeping before DOM-ready acceptance
	} else if imageErr == nil {
		imageErr = update.CheckPairPending(filepath.Dir(exe))
	}
	if imageErr == nil {
		imageErr = checkAcceptedInstalledPair()
	}
	if imageErr == nil {
		imageErr = updateStartupHousekeeping()
	}
	app := NewApp()
	app.updateGUISession = guiSession
	if imageErr != nil {
		app.updateGUIStartupDone = make(chan struct{})
	}
	if imageErr != nil {
		app.installedImageChanged = true // restriction only; journal is not accepted authority
	}

	err := wails.Run(&options.App{
		Title:     "snowden.system — контроль VPN",
		Width:     1180,
		Height:    780,
		MinWidth:  940,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 15, B: 20, A: 1},
		OnStartup:        app.startup,
		OnDomReady:       app.updateGUIDOMReady,
		OnBeforeClose:    app.OnBeforeClose,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "snowden-system-v3",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				// Второй запуск уже работающего приложения: показать окно.
				if app.ctx != nil {
					wailsruntime.WindowUnminimise(app.ctx)
					wailsruntime.WindowShow(app.ctx)
				}
			},
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
	})
	if err != nil {
		println("fatal:", err.Error())
	}
}

// writeCrashLog — последняя запись перед смертью процесса.
func writeCrashLog(text string) {
	dir := logsDir()
	_ = os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, "crash-"+time.Now().Format("20060102-150405")+".log")
	_ = os.WriteFile(path, []byte(text+"\n"), 0o600)
}
