//go:build windows

package app

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// applyWindowTheme uses DWM, so snapping, shadows and the caption buttons keep
// their native behavior. Caption colors need Windows 11; older versions keep
// the default title bar but still get dark or light caption buttons.
func (a *App) applyWindowTheme(caption, text, border uint32, dark bool) error {
	if a.window == nil {
		return errors.New("window is not ready")
	}
	application.InvokeSync(func() {
		hwnd := uintptr(a.window.NativeWindow())
		if hwnd == 0 {
			return
		}
		w32.SetTheme(hwnd, dark)
		w32.SetTitleBarColour(hwnd, caption)
		w32.SetTitleTextColour(hwnd, text)
		w32.SetBorderColour(hwnd, border)
	})
	return nil
}
