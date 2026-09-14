package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Паника на главной горутине не должна уходить молча: пишем crash-лог
	// (AppData, не рядом с exe — wails build стирает build/bin) и отдаем
	// стектрейс дальше, чтобы процесс завершился с видимой причиной.
	defer func() {
		if rec := recover(); rec != nil {
			writeCrashLog(fmt.Sprintf("panic: %v\n\n%s", rec, debug.Stack()))
			panic(rec)
		}
	}()

	// Один экземпляр: Wails SingleInstanceLock держит именованный мьютекс
	// (wails-app-<id>sim); второй запуск шлёт WM_COPYDATA первому окну
	// (OnSecondInstanceLaunch: показать поверх) и делает os.Exit(0).
	// Это закрывает и «второй SOCKS на 1080»: второй процесс не доходит до
	// ядра. Переключение режима совместимо: старый процесс умирает по Quit
	// в 300мс окне, его мьютекс освобождает ядро, новый стартует первым.
	app := NewApp()

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
