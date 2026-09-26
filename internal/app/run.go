package app

import (
	"io/fs"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/update"
	"github.com/local/mayak/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Run starts MAYAK and returns when it quits. Package main embeds the files
// it needs: assets holds the built frontend (all:frontend/dist, as embedded)
// and icon is the tray icon (build/appicon.png).
func Run(assets fs.FS, icon []byte) error {
	trayIcon = icon
	raisePriority()
	// After an update, the version being replaced is still quitting: wait for
	// it (the app runs as a single instance), then drop the files it kept.
	restarted := update.WaitForPreviousInstance(30 * time.Second)
	if dir, err := update.InstallDir(); err == nil {
		update.CleanupOld(dir)
	}
	// The data folder of the app's earlier name (RaidLens) moves to Mayak
	// before anything reads it.
	moved, moveErr := appdir.Migrate()
	service := NewApp()
	// With the KeepPriority setting, the window's WebView2 processes are kept
	// at normal priority whatever a priority manager does to MAYAK while it
	// starts (priority_windows.go).
	go guardPriority(service.done, func() bool {
		service.mu.RLock()
		defer service.mu.RUnlock()
		return service.settings.KeepPriority
	})
	if restarted {
		service.addLog("Info", "Update", "Restarted into MAYAK "+version.Current())
	}
	if moveErr != nil {
		service.addLog("Warn", "Application", "Could not move the RaidLens data folder to Mayak: "+moveErr.Error())
	} else if moved {
		service.addLog("Info", "Application", "Moved the data folder from RaidLens to Mayak")
	}
	settings, _ := config.Load()
	desktop := application.New(application.Options{
		Name:           "MAYAK",
		Services:       []application.Service{application.NewService(service)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.ichi0g0y.mayak", OnSecondInstanceLaunch: func(application.SecondInstanceData) { service.showWindow() }},
	})
	service.desktop = desktop
	// The window's hooks (minimise to tray) may run before ServiceStartup
	// loads the settings; start from the saved ones.
	service.settings = normalizeSettings(settings)
	var restoreState config.WindowState
	windowOptions := application.WebviewWindowOptions{
		Name: "main", Title: "MAYAK", Width: 1120, Height: 760, MinWidth: 760, MinHeight: 560,
		StartState: application.WindowStateNormal, URL: "/", ZoomControlEnabled: false,
		// The shell draws its own title bar, so the sidebar reaches the top edge.
		// Windows keeps the shadow, rounded corners and snapping.
		Frameless: true,
	}
	// Set the initial geometry before native window creation. ServiceStartup
	// may run while Wails is still constructing the HWND/WebView controllers.
	if saved, err := config.LoadWindow(); err == nil {
		if !saved.Configured && settings.WindowConfigured {
			saved = config.WindowState{X: settings.WindowX, Y: settings.WindowY, Width: settings.WindowWidth, Height: settings.WindowHeight, Configured: true}
		}
		if saved.Configured && saved.Width <= 5000 && saved.Height <= 3000 && saved.X >= -10000 && saved.X <= 10000 && saved.Y >= -10000 && saved.Y <= 10000 {
			restoreState = saved
			windowOptions.Width, windowOptions.Height = max(760, saved.Width), max(560, saved.Height)
			windowOptions.InitialPosition = application.WindowXY
			windowOptions.X, windowOptions.Y = saved.X, saved.Y
		}
	}
	service.window = desktop.Window.NewWithOptions(windowOptions)
	var placementOnce sync.Once
	service.window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		placementOnce.Do(func() {
			if restoreState.Configured {
				restored := restoreWindowPlacement(restoreState, desktop.Screen.GetAll())
				service.window.SetPosition(restored.X, restored.Y)
				service.window.SetSize(restored.Width, restored.Height)
				if restored.Maximized {
					service.window.Maximise()
				}
			}
			service.windowPlacementReady.Store(true)
			// Minimising also sends it to the tray when minimize-to-tray is on.
			if settings.StartMinimized {
				service.window.Minimise()
			}
			go service.saveWindow(service.ctx)
		})
	})
	service.browserViews = browserview.New(service.window, func(e browserview.Event) { service.emitEvent("browser:navigation", e) })
	service.browserViews.SetKeyHandler(service.browserShortcut)
	for _, event := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize, events.Common.WindowMaximise, events.Common.WindowUnMaximise} {
		service.window.OnWindowEvent(event, func(*application.WindowEvent) { go service.saveWindow(service.ctx) })
	}
	service.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if service.beforeClose(service.ctx) {
			e.Cancel()
		}
	})
	service.window.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) {
		service.mu.RLock()
		hide := service.settings.MinimizeToTray
		service.mu.RUnlock()
		// Events are delivered asynchronously; a restore may already have
		// happened by the time a queued minimise event reaches this callback.
		if hide && service.window.IsMinimised() {
			service.window.Hide()
		}
	})
	return desktop.Run()
}
