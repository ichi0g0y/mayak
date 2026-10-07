//go:build windows

package app

import (
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// toastExStyle keeps the toast window off the taskbar and from taking the
// focus, also when clicked: a game in front stays in front.
const toastExStyle = w32.WS_EX_CONTROLPARENT | w32.WS_EX_TOOLWINDOW | w32.WS_EX_NOACTIVATE

func toastHide(window *application.WebviewWindow) {
	application.InvokeSync(func() {
		if h := w32.HWND(uintptr(window.NativeWindow())); h != 0 {
			w32.ShowWindow(h, w32.SW_HIDE)
		}
	})
}

// toastPlace shows the toast window, width × height of the shell's pixels,
// at the bottom middle of main, with no focus. It works in the screen's
// pixels: the main window's own and its scale.
func toastPlace(window, main *application.WebviewWindow, width, height int) {
	application.InvokeSync(func() {
		h, m := w32.HWND(uintptr(window.NativeWindow())), w32.HWND(uintptr(main.NativeWindow()))
		if h == 0 || m == 0 {
			return
		}
		r := w32.GetWindowRect(m)
		scale := float64(w32.GetDpiForWindow(m)) / 96
		if scale <= 0 {
			scale = 1
		}
		w, ht, gap := int(float64(width)*scale), int(float64(height)*scale), int(20*scale)
		x := int(r.Left) + (int(r.Right-r.Left)-w)/2
		y := int(r.Bottom) - ht - gap
		w32.SetWindowPos(h, 0, x, y, w, ht, w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_SHOWWINDOW)
	})
}

// appInFront tells whether a window of MAYAK is the one in front (the main
// window, the popup, a menu).
func (a *App) appInFront() bool {
	h := w32.GetForegroundWindow()
	if h == 0 {
		return false
	}
	_, pid := w32.GetWindowThreadProcessId(h)
	return pid == os.Getpid()
}
