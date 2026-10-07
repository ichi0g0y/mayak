//go:build !windows

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// toastExStyle is Windows' only.
const toastExStyle = 0

func toastHide(window *application.WebviewWindow) { window.Hide() }

// toastPlace shows the toast window at the bottom middle of main.
func toastPlace(window, main *application.WebviewWindow, width, height int) {
	x, y := main.Position()
	w, h := main.Size()
	window.SetSize(width, height)
	window.SetPosition(x+(w-width)/2, y+h-height-20)
	window.Show()
}
