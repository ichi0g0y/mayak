package app

import (
	"errors"
	"sync"
	"time"

	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The item popup: pages opened from the item sidebar show in a window of
// their own, so they can be moved anywhere, off the main window too. The
// window draws its header (popup.html) and holds the page as a native view
// below it. The popup closes when it loses the focus, like a popup, unless
// it is pinned; pinned, it stays until closed.
const popupViewID = "popup"

// popupHead is the height of the popup's header, above the page.
const popupHead = 44

// popupEdge keeps the popup's left edge free of the page, for resizing it
// there (the right and bottom edges are kept free by the view).
const popupEdge = 5

type itemPopup struct {
	mu     sync.Mutex
	window *application.WebviewWindow
	views  *browserview.Manager
	open   bool
	// url is the page the popup's view was last sent to, "" with no view.
	url    string
	pinned bool
	// gen counts the pages shown, so a close for losing the focus is dropped
	// when a page was shown meanwhile (clicking another task in the sidebar).
	gen      int
	placedAt time.Time
}

// PopupPlace is where the popup opens: x and y are relative to the main
// window, in the shell's pixels.
type PopupPlace struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// PopupPage is what the popup shows.
type PopupPage struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Task       bool   `json:"task"`
	Background string `json:"background"`
	// Theme and Language style the popup's header like the shell.
	Theme    string `json:"theme"`
	Language string `json:"language"`
	// Pinned is set by the Go side: whether the popup is pinned.
	Pinned bool `json:"pinned"`
}

// BrowserPopupShow opens page in the popup. An open popup stays where it is;
// a new one opens where the last one was left, or else at place.
func (a *App) BrowserPopupShow(page PopupPage, place PopupPlace) error {
	if a.desktop == nil || a.window == nil {
		return errors.New("browser is not ready")
	}
	if place.Width < 200 || place.Width > 4000 || place.Height < 150 || place.Height > 3000 || place.X < -4000 || place.X > 8000 || place.Y < -4000 || place.Y > 8000 {
		return errors.New("invalid popup place")
	}
	p := &a.popup
	p.mu.Lock()
	if p.window == nil {
		p.window = a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "popup", Title: "MAYAK", Width: place.Width, Height: place.Height, MinWidth: 320, MinHeight: 200,
			URL: "/popup.html", Frameless: true, Hidden: true, ZoomControlEnabled: false,
			Windows: application.WindowsWindow{HiddenOnTaskbar: true},
		})
		p.views = browserview.New(p.window, func(e browserview.Event) { a.emitEvent("browser:navigation", e) })
		if a.adblock != nil {
			p.views.SetContentBlocker(a.adblock)
		}
		if id := a.BrowserRemoteID(); id != "" {
			p.views.SetDocumentScript(tarkovDevConnectScript(id))
		}
		popup := p.window
		popup.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { ownPopup(popup, a.window) })
		p.window.OnWindowEvent(events.Windows.WindowInactive, func(*application.WindowEvent) {
			p.mu.Lock()
			gen := p.gen
			p.mu.Unlock()
			go func() {
				// The click that took the focus may open another page in it.
				time.Sleep(200 * time.Millisecond)
				p.mu.Lock()
				closing := p.open && !p.pinned && p.gen == gen
				p.mu.Unlock()
				if closing {
					a.closePopup(true)
				}
			}()
		})
		p.window.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
			p.mu.Lock()
			// Moves right after placing it are the placing itself.
			moved := p.open && time.Since(p.placedAt) > 500*time.Millisecond
			p.mu.Unlock()
			if moved {
				go a.savePopupWindow()
			}
		})
		p.window.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
			p.mu.Lock()
			resized := p.open && time.Since(p.placedAt) > 500*time.Millisecond
			p.mu.Unlock()
			if resized {
				go a.savePopupWindow()
			}
		})
		// The window's close button (Alt+F4) closes the popup, and keeps the window.
		p.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			e.Cancel()
			go a.closePopup(true)
		})
	}
	window, views := p.window, p.views
	// An open popup stays where it is. A new one opens where the last one
	// was left, or else beside the row clicked.
	if !p.open {
		p.placedAt = time.Now()
		if saved, err := config.LoadPopupWindow(); err == nil && saved.Configured {
			saved = restorePlacement(saved, a.desktop.Screen.GetAll(), 320, 200)
			window.SetSize(saved.Width, saved.Height)
			window.SetPosition(saved.X, saved.Y)
		} else {
			x, y := a.window.Position()
			window.SetSize(place.Width, place.Height)
			window.SetPosition(x+place.X, y+place.Y)
		}
	}
	p.gen++
	p.open = true
	pinned := p.pinned
	previous := p.url
	p.url = page.URL
	p.mu.Unlock()

	page.Pinned = pinned
	a.emitEvent("popup:page", page)
	ownPopup(window, a.window)
	window.Show()
	// A window made just now has no native window until it is shown.
	ownPopup(window, a.window)
	window.Focus()
	if err := views.Command("show", browserview.Options{ID: popupViewID, URL: page.URL, Left: popupEdge, Top: popupHead, Background: page.Background}); err != nil {
		return err
	}
	if previous != "" && previous != page.URL {
		return views.Command("navigate", browserview.Options{ID: popupViewID, URL: page.URL})
	}
	return nil
}

// savePopupWindow remembers where the popup is and its size, for the next
// popup, also after a restart.
func (a *App) savePopupWindow() {
	p := &a.popup
	p.mu.Lock()
	window := p.window
	p.mu.Unlock()
	if window == nil || window.IsMinimised() {
		return
	}
	x, y := window.Position()
	width, height := window.Size()
	if width < 320 || height < 200 {
		return
	}
	screen, _ := window.GetScreen()
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	saved, _ := config.LoadPopupWindow()
	_ = config.SavePopupWindow(rememberWindowPlacement(saved, x, y, width, height, false, screen))
}

// BrowserPopupPin pins the popup, so it stays when it loses the focus, or
// unpins it.
func (a *App) BrowserPopupPin(pinned bool) {
	a.popup.mu.Lock()
	defer a.popup.mu.Unlock()
	if a.popup.open {
		a.popup.pinned = pinned
	}
}

// BrowserPopupClose closes the popup.
func (a *App) BrowserPopupClose() { a.closePopup(false) }

// closePopup hides the popup and disposes of its page, so the next popup
// never shows an earlier page. notify tells the shell, when the popup closed
// by itself.
func (a *App) closePopup(notify bool) {
	p := &a.popup
	p.mu.Lock()
	if !p.open {
		p.mu.Unlock()
		return
	}
	p.open, p.pinned, p.url = false, false, ""
	window, views := p.window, p.views
	p.mu.Unlock()
	_ = views.Command("close", browserview.Options{ID: popupViewID})
	window.Hide()
	if notify {
		a.emitEvent("popup:closed")
	}
}
