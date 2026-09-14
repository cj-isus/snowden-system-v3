package main

// tray_windows.go — системный трей + автозапуск (V2-048/F13+F14).
//
// Трей: getlantern/systray в отдельной горутине (message pump живёт внутри
// библиотеки, Wails-окну не мешает). Меню зеркалит фактическое состояние:
// Подключить/Отключить (по state), выбор канала (SelectChannel с probe-гейтом —
// политика не обходится), автозапуск, Выход. Состояние приходит опросом раз
// в 2с (дешёвое чтение под мьютексом; событие UI «state» в трей не пробрасываем,
// чтобы не связывать потоки Wails и systray).
//
// Автозапуск: HKCU\...\Run — пользовательская настройка без UAC. Значение:
// "<exe>" --auto-connect (существующий флаг честного failover-on-start).
// Пишет в реестр ТОЛЬКО явное действие владельца (меню трея/настройки).

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/getlantern/systray"
	"github.com/snowden-system/windows/backend/render"
	"golang.org/x/sys/windows/registry"
)

const autostartRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const autostartValueName = "snowden-system"

//go:embed build/windows/icon.ico
var trayIcon []byte

// startTray — запустить иконку трея (горутина; вызывается из startup).
func (a *App) startTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("snowden.system")

	mConnect := systray.AddMenuItem("Подключить", "Запустить защищённый туннель")
	mDisconnect := systray.AddMenuItem("Отключить", "Остановить туннель")
	systray.AddSeparator()

	ids := a.trayChannelIDs()
	channelItems := make([]*systray.MenuItem, 0, len(ids))
	for _, id := range ids {
		it := systray.AddMenuItem("Канал: "+id, "Переключиться на канал (с probe-проверкой)")
		channelItems = append(channelItems, it)
	}
	systray.AddSeparator()
	mAutostart := systray.AddMenuItemCheckbox("Автозапуск с системой", "Запускать приложение при входе в систему", autostartEnabled())
	mQuit := systray.AddMenuItem("Выход", "Завершить приложение")

	// Каналы: каждый пункт — собственная горутина-ретранслятор (индекс в ch).
	chSelected := make(chan int, len(channelItems))
	for i, it := range channelItems {
		go func(idx int, mi *systray.MenuItem) {
			for range mi.ClickedCh {
				chSelected <- idx
			}
		}(i, it)
	}

	setConnected := func(on bool) {
		if on {
			mConnect.Disable()
			mDisconnect.Enable()
		} else {
			mConnect.Enable()
			mDisconnect.Disable()
		}
	}
	setConnected(false)

	go func() {
		for {
			select {
			case <-mConnect.ClickedCh:
				a.Start() // асинхронный: результат виден и в трее, и в окне
			case <-mDisconnect.ClickedCh:
				_ = a.Stop()
			case idx := <-chSelected:
				if idx >= 0 && idx < len(ids) {
					if err := a.SelectChannel(ids[idx]); err != nil {
						// честный факт в журнал; откат гарантирует живой канал
						a.appendLog("warn", "трей: переключение на "+ids[idx]+" не удалось: "+err.Error())
					}
				}
			case <-mAutostart.ClickedCh:
				want := !autostartEnabled()
				if err := setAutostart(want); err != nil {
					a.appendLog("error", "автозапуск: "+err.Error())
				} else if want {
					mAutostart.Check()
				} else {
					mAutostart.Uncheck()
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Зеркало состояния: опрос 2с, обновление только при изменении.
	last := ""
	for {
		a.mu.Lock()
		st := a.state.State
		active := a.state.ActiveID
		a.mu.Unlock()
		if st != last {
			last = st
			switch st {
			case "running":
				systray.SetTooltip("snowden.system — подключено (" + active + ")")
				setConnected(true)
			case "starting", "stopping":
				systray.SetTooltip("snowden.system — " + st)
				setConnected(false)
			case "blocked", "error":
				systray.SetTooltip("snowden.system — проблема (см. журнал)")
				setConnected(false)
			default:
				systray.SetTooltip("snowden.system — отключено")
				setConnected(false)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func (a *App) onTrayExit() {}

// trayChannelIDs — идентификаторы selectable каналов для меню.
func (a *App) trayChannelIDs() []string {
	channels, err := render.LoadDescriptors()
	if err != nil {
		return nil
	}
	var ids []string
	for _, ch := range render.Selectable(channels) {
		ids = append(ids, ch.ID)
	}
	return ids
}

// ---------- автозапуск (F14) ----------

// autostartEnabled — есть ли запись в HKCU Run.
func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autostartValueName)
	return err == nil && v != ""
}

// setAutostart — включить/выключить автозапуск (HKCU, без UAC).
// Значение: "<exe>" --auto-connect (путь в кавычках: Program Files с пробелами).
func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("реестр недоступен: %w", err)
	}
	defer k.Close()
	if !on {
		return k.DeleteValue(autostartValueName)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("путь приложения неизвестен: %w", err)
	}
	return k.SetStringValue(autostartValueName, `"`+exe+`" --auto-connect`)
}

// автозапуск-биндинг для UI-настроек (вызывается из SettingsView).
func (a *App) GetAutostart() bool { return autostartEnabled() }

func (a *App) SetAutostart(on bool) error {
	if err := setAutostart(on); err != nil {
		return err
	}
	if on {
		a.appendLog("info", "автозапуск включён (HKCU Run, --auto-connect)")
	} else {
		a.appendLog("info", "автозапуск выключен")
	}
	return nil
}

// iconFileGuard — проверка, что embed-иконка на месте (сборка сломается раньше,
// но тест пусть даёт внятную ошибку).
func iconFileGuard() bool { return len(trayIcon) > 0 }

var _ = filepath.Join
