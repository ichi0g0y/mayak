//go:build !darwin

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// macWindow is only used on macOS (window_buttons_darwin.go).
var macWindow application.MacWindow

// keepWindowButtonsPlaced has nothing to do: the shell draws the buttons.
func keepWindowButtonsPlaced(*application.WebviewWindow) {}
