//go:build windows

package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// ownPopup makes the main window the popup's owner, so the popup always
// stays above it (also once moved off it) and minimises with it.
func ownPopup(popup, main *application.WebviewWindow) {
	application.InvokeSync(func() {
		p, m := uintptr(popup.NativeWindow()), uintptr(main.NativeWindow())
		if p != 0 && m != 0 {
			w32.SetWindowLongPtr(w32.HWND(p), w32.GWLP_HWNDPARENT, m)
		}
	})
}
