package main

import "github.com/getlantern/systray"

// Initial ready setup hands this state to the original command worker. Neither
// the polling loop nor another relay mutates this menu item/state afterwards.
type trayAutostart struct {
	app     *App
	session *traySession
	item    *systray.MenuItem
	known   bool
	on      bool
}

func newTrayAutostart(app *App, session *traySession) *trayAutostart {
	menu := &trayAutostart{app: app, session: session}
	menu.item = systray.AddMenuItemCheckbox("Автозапуск: проверить состояние", "Состояние неизвестно. Нажмите, чтобы прочитать его без изменения.", false)
	on, err := app.GetAutostart()
	menu.display(on, err == nil)
	return menu
}

func (menu *trayAutostart) display(on, known bool) {
	menu.on, menu.known = on, known
	if !known {
		menu.item.Uncheck()
		menu.item.SetTitle("Автозапуск: проверить состояние")
		menu.item.SetTooltip("Состояние неизвестно. Нажмите, чтобы прочитать его без изменения.")
		return
	}
	menu.item.SetTooltip("Команда прочитана для текущего приложения. Нажмите, чтобы изменить автозапуск.")
	if on {
		menu.item.SetTitle("Автозапуск при входе: включён")
		menu.item.Check()
	} else {
		menu.item.SetTitle("Автозапуск при входе: выключен")
		menu.item.Uncheck()
	}
}

func (menu *trayAutostart) click() {
	if menu.session.stopping() {
		return
	}
	observed, err := menu.app.GetAutostart()
	if err != nil {
		menu.display(false, false)
		menu.app.appendLog("warn", "автозапуск: состояние не прочитано; изменение не выполнялось")
		return
	}
	if !menu.known || observed != menu.on {
		menu.display(observed, true)
		return // unknown/external change first refreshes; it is never inverted blindly
	}
	wanted := !observed
	if menu.session.stopping() {
		return
	}
	if err := menu.app.SetAutostart(wanted); err != nil {
		menu.display(false, false)
		menu.app.appendLog("warn", "автозапуск: изменение не подтверждено; перечитайте состояние")
		return
	}
	actual, err := menu.app.GetAutostart()
	menu.display(actual, err == nil)
	if err != nil || actual != wanted {
		menu.app.appendLog("warn", "автозапуск: повторное чтение не подтвердило запрос")
	}
}
