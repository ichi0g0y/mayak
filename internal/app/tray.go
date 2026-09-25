package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayIcon is build/appicon.png, embedded by package main (see Run).
var trayIcon []byte

// trayLabels are the tray menu's items in language.
func trayLabels(language string) (show, quit string) {
	if language == "en" {
		return "Open MAYAK", "Quit"
	}
	return "MAYAKを開く", "終了"
}

func startTray(a *App) {
	show, quit := trayLabels(a.settings.Language)
	tray := a.desktop.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("MAYAK")
	tray.OnClick(a.showWindow).OnDoubleClick(a.showWindow)
	menu := a.desktop.Menu.New()
	a.trayShow = menu.Add(show).OnClick(func(*application.Context) { a.showWindow() })
	menu.AddSeparator()
	a.trayQuit = menu.Add(quit).OnClick(func(*application.Context) { a.quit() })
	a.trayMenu = menu
	tray.SetMenu(menu)
}

// setTrayLanguage follows a change of the settings' language.
func (a *App) setTrayLanguage(language string) {
	if a.trayMenu == nil {
		return
	}
	show, quit := trayLabels(language)
	// Native menus change on the UI thread.
	application.InvokeSync(func() {
		a.trayShow.SetLabel(show)
		a.trayQuit.SetLabel(quit)
		a.trayMenu.Update()
	})
}
