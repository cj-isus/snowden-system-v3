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
	"time"

	"github.com/getlantern/systray"
	"github.com/snowden-system/windows/backend/render"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

func (a *App) onTrayReady(session *traySession) {
	if session.stopping() {
		return
	}
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
	autostart := newTrayAutostart(a, session)
	mQuit := systray.AddMenuItem("Выход", "Завершить приложение")

	// Каналы: каждый пункт — собственная горутина-ретранслятор (индекс в ch).
	chSelected := make(chan int, len(channelItems))
	for i, it := range channelItems {
		idx, mi := i, it
		session.startWorker(func() {
			for {
				select {
				case <-session.stop:
					return
				case <-session.ctx.Done():
					return
				case _, ok := <-mi.ClickedCh:
					if !ok {
						session.fail()
						return
					}
					if session.stopping() {
						return
					}
					select {
					case <-session.stop:
						return
					case <-session.ctx.Done():
						return
					case chSelected <- idx:
					}
				}
			}
		})
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

	session.startWorker(func() {
		for {
			if session.stopping() {
				return
			}
			select {
			case <-session.stop:
				return
			case <-session.ctx.Done():
				return
			case _, ok := <-mConnect.ClickedCh:
				if !ok {
					session.fail()
					return
				}
				if session.stopping() {
					return
				}
				a.Start() // original App lifecycle outcome publishes the actual result
			case _, ok := <-mDisconnect.ClickedCh:
				if !ok {
					session.fail()
					return
				}
				if session.stopping() {
					return
				}
				_ = a.Stop()
			case idx, ok := <-chSelected:
				if !ok {
					session.fail()
					return
				}
				if session.stopping() {
					return
				}
				if idx >= 0 && idx < len(ids) {
					if err := a.SelectChannel(ids[idx]); err != nil {
						// Existing App lifecycle outcome owns failure/rollback; no live-channel guarantee here.
						a.appendLog("warn", "трей: переключение на "+ids[idx]+" не удалось: "+err.Error())
					}
				}
			case _, ok := <-autostart.item.ClickedCh:
				if !ok {
					session.fail()
					return
				}
				if session.stopping() {
					return
				}
				autostart.click()
			case _, ok := <-mQuit.ClickedCh:
				if !ok {
					session.fail()
					return
				}
				if session.stopping() {
					return
				}
				a.mu.Lock()
				ctx := a.ctx
				a.mu.Unlock()
				if !a.OnBeforeClose(ctx) {
					a.requestQuit(0) // accepted handoff or already authorized terminal exit
				}
				return // the original App cleanup now owns termination
			}
		}
	})

	// Зеркало состояния: опрос 2с, обновление только при изменении.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		if session.stopping() {
			return
		}
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
		select {
		case <-session.stop:
			return
		case <-session.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

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

// iconFileGuard — проверка, что embed-иконка на месте (сборка сломается раньше,
// но тест пусть даёт внятную ошибку).
func iconFileGuard() bool { return len(trayIcon) > 0 }
