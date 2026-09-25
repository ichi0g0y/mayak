package app

// Events stay inside the Wails application; no browser bridge is started.
func (a *App) emitEvent(event string, args ...any) {
	if a.desktop != nil {
		a.desktop.Event.Emit(event, args...)
	}
}
