package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayIcon is build/appicon.png, embedded by package main (see Run).
var trayIcon []byte

// trayLabels are the tray menu's items in language.
func trayLabels(language string) (show, check, quit string) {
	if language == "en" {
		return "Open MAYAK", "Check for updates", "Quit"
	}
	return "MAYAKを開く", "更新を確認", "終了"
}

func startTray(a *App) {
	show, check, quit := trayLabels(a.settings.Language)
	tray := a.desktop.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("MAYAK")
	tray.OnClick(a.showWindow).OnDoubleClick(a.showWindow)
	menu := a.desktop.Menu.New()
	a.trayShow = menu.Add(show).OnClick(func(*application.Context) { a.showWindow() })
	// The window comes up so the result (the status strip, About) is seen.
	a.trayCheck = menu.Add(check).OnClick(func(*application.Context) {
		a.showWindow()
		go func() {
			if _, err := a.CheckForUpdates(); err != nil {
				a.addLog("Warn", "Update", "Update check failed: "+err.Error())
			}
		}()
	})
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
	show, check, quit := trayLabels(language)
	// Native menus change on the UI thread.
	application.InvokeSync(func() {
		a.trayShow.SetLabel(show)
		a.trayCheck.SetLabel(check)
		a.trayQuit.SetLabel(quit)
		a.trayMenu.Update()
	})
}
