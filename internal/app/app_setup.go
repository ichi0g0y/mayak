package app

// The setup notice: an EFT folder missing after the start's detection (the
// first-run tutorial says so once; this stays until it is set). Set, the
// notice goes, and monitoring left off for it starts when it starts on its
// own (AutoStartMonitoring).

// setupKey is the notice's key (one at a time, replaced in place).
const setupKey = "setupFolders"

// missingFolders names the EFT folders not set or gone: "screenshots",
// "logs".
func (a *App) missingFolders() []string {
	a.mu.RLock()
	screenshots, logs := a.settings.ScreenshotDirectory, a.settings.LogsDirectory
	a.mu.RUnlock()
	var missing []string
	if screenshots == "" || !isDir(screenshots) {
		missing = append(missing, "screenshots")
	}
	if logs == "" || !isDir(logs) {
		missing = append(missing, "logs")
	}
	return missing
}

// noteSetupFolders shows the notice when a folder is missing and show is
// set (at start) or the notice already shows (a folder set, the other not);
// with both folders there it takes the notice away and starts monitoring
// that could not start without them.
func (a *App) noteSetupFolders(show bool) {
	if a.browserClient.Load() || a.BrowserPlatform() != "windows" {
		return
	}
	missing := a.missingFolders()
	if len(missing) == 0 {
		if a.setupShown.CompareAndSwap(true, false) {
			a.endToast(setupKey)
		}
		a.mu.RLock()
		start := a.settings.AutoStartMonitoring && !a.status.Monitoring
		a.mu.RUnlock()
		if start {
			_ = a.StartMonitoring()
		}
		return
	}
	if !show && !a.setupShown.Load() {
		return
	}
	which := "both"
	if len(missing) == 1 {
		which = missing[0]
	}
	a.setupShown.Store(true)
	a.toast(Toast{
		Key: setupKey, Category: ToastError, Level: "warn", Persistent: true,
		Message: "toastSetupFolders_" + which,
		Actions: []ToastAction{{ID: "setupFolders", Label: "toastOpenFolders"}},
	})
}
