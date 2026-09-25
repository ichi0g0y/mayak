package app

import (
	"github.com/local/mayak/internal/appdir"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/local/mayak/internal/adblock"
)

// setupAdblock filters the built-in browser's tabs. Filter lists are cached in
// the config directory and refreshed in the background.
func (a *App) setupAdblock() {
	if a.browserViews == nil || goruntime.GOOS != "windows" {
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		a.addLog("Warn", "Adblock", "Ad blocking is unavailable: "+err.Error())
		return
	}
	a.adblock = adblock.New(filepath.Join(dir, appdir.Name, "adblock"), func(level, message string) { a.addLog(level, "Adblock", message) })
	a.browserViews.SetContentBlocker(a.adblock)
	go a.adblock.Start(a.ctx)
}

// BrowserSetAdblock applies the browser shell's ad blocking setting. Open
// pages keep their current content until they are reloaded.
func (a *App) BrowserSetAdblock(enabled bool) {
	if a.adblock != nil {
		a.adblock.SetEnabled(enabled)
	}
}
