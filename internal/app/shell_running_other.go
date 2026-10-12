//go:build !darwin

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// keepShellRunning has nothing to do: only macOS holds the shell back here.
func keepShellRunning(*application.WebviewWindow) {}
