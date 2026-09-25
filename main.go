// Command mayak is the MAYAK desktop app. The app itself is internal/app;
// this package only embeds the files that must sit beside it (the built
// frontend and the tray icon) and links the Windows resources
// (mayak_windows_*.syso).
package main

import (
	"embed"
	"log"

	"github.com/local/mayak/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func main() {
	if err := app.Run(assets, trayIcon); err != nil {
		log.Fatal(err)
	}
}
