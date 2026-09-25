//go:build !windows

package app

func (a *App) applyWindowTheme(caption, text, border uint32, dark bool) error { return nil }
