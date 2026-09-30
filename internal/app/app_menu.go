package app

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The shell's menus over a page: the page is a native view above the shell,
// so a menu the shell draws would be hidden under it. A menu opens instead in
// a small frameless window of its own (menu.html), owned by the main window
// so it stays above it. The shell sends the items (BrowserMenuShow); the menu
// page draws them, reports its height (BrowserMenuReady), and a choice
// (BrowserMenuChoose) closes it and goes back to the shell as "menu:choice".
// Losing the focus closes it with no choice.
type shellMenu struct {
	mu     sync.Mutex
	window *application.WebviewWindow
	open   bool
	width  int
	// request is the menu shown now, for a menu page that loads after it
	// was sent (the window's first menu): it asks for it (BrowserMenuCurrent).
	request MenuRequest
	// gen counts the menus shown, so a focus loss of an older one is dropped.
	gen int
}

// MenuItem is one row of a menu: an item to choose (Kind "item", the
// default), a heading (Kind "label"), or a card (Kind "card": a picture,
// Title and Hint over Facts, not to choose). Icon names an icon of the
// shell's set; Thumb is a picture (a JPEG or PNG data URL).
type MenuItem struct {
	ID    string     `json:"id"`
	Kind  string     `json:"kind"`
	Icon  string     `json:"icon"`
	Title string     `json:"title"`
	Hint  string     `json:"hint"`
	Thumb string     `json:"thumb"`
	Facts []MenuFact `json:"facts"`
	// Color is a colour for the card's mark (#rrggbb).
	Color string `json:"color"`
}

// MenuFact is one line of a card: a label and its value.
type MenuFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// MenuRequest is a menu to show: at X, Y (the shell's pixels, relative to the
// main window), Width wide.
type MenuRequest struct {
	ID       string     `json:"id"`
	X        int        `json:"x"`
	Y        int        `json:"y"`
	Width    int        `json:"width"`
	Items    []MenuItem `json:"items"`
	Theme    string     `json:"theme"`
	Language string     `json:"language"`
}

// BrowserMenuShow opens a menu. It shows once the menu page has drawn it
// (BrowserMenuReady), so it never flashes at a wrong size.
func (a *App) BrowserMenuShow(request MenuRequest) error {
	if a.desktop == nil || a.window == nil {
		return errors.New("browser is not ready")
	}
	if request.Width < 120 || request.Width > 800 || request.X < -4000 || request.X > 8000 || request.Y < -4000 || request.Y > 8000 || len(request.Items) == 0 || len(request.Items) > 40 {
		a.addLog("Warn", "Browser", "Menu not shown: invalid menu")
		return errors.New("invalid menu")
	}
	for i, item := range request.Items {
		if len(item.ID) > 200 || len(item.Title) > 300 || len(item.Hint) > 300 || len(item.Icon) > 40 || len(item.Thumb) > 200_000 || len(item.Facts) > 12 || len(item.Color) > 7 {
			return errors.New("invalid menu item")
		}
		for _, fact := range item.Facts {
			if len(fact.Label) > 80 || len(fact.Value) > 80 {
				return errors.New("invalid menu item")
			}
		}
		if item.Thumb != "" && !strings.HasPrefix(item.Thumb, "data:image/jpeg;base64,") && !strings.HasPrefix(item.Thumb, "data:image/png;base64,") {
			request.Items[i].Thumb = ""
		}
	}
	m := &a.menu
	m.mu.Lock()
	if m.window == nil {
		m.window = a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "menu", Title: "MAYAK", Width: request.Width, Height: 60,
			URL: "/menu.html", Frameless: true, Hidden: true, ZoomControlEnabled: false, DisableResize: true,
			Windows: application.WindowsWindow{HiddenOnTaskbar: true},
		})
		window := m.window
		window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { ownPopup(window, a.window) })
		window.OnWindowEvent(events.Windows.WindowInactive, func(*application.WindowEvent) {
			m.mu.Lock()
			gen := m.gen
			m.mu.Unlock()
			go func() {
				// A click on a row takes the focus back to the menu first.
				time.Sleep(120 * time.Millisecond)
				m.mu.Lock()
				closing := m.open && m.gen == gen
				m.mu.Unlock()
				if closing {
					a.closeMenu("")
				}
			}()
		})
		window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			e.Cancel()
			go a.closeMenu("")
		})
	}
	m.gen++
	m.open = true
	m.width = request.Width
	m.request = request
	window := m.window
	m.mu.Unlock()
	// The window shows at once, at the height its rows should take; the
	// menu page sets the exact height when it has drawn them.
	x, y := a.window.Position()
	window.SetPosition(x+request.X, y+request.Y)
	window.SetSize(request.Width, menuHeight(request.Items))
	a.emitEvent("menu:page", request)
	ownPopup(window, a.window)
	window.Show()
	ownPopup(window, a.window)
	window.Focus()
	return nil
}

// menuHeight estimates a menu's height from its rows (menu.js draws them).
func menuHeight(items []MenuItem) int {
	height := 14
	for _, item := range items {
		switch {
		case item.Kind == "label":
			height += 24
		case item.Hint != "" || item.Thumb != "":
			height += 54
		default:
			height += 38
		}
	}
	return min(height, 900)
}

// BrowserMenuCurrent is the menu shown now, for the menu page to draw when it
// has just loaded; none when no menu is open.
func (a *App) BrowserMenuCurrent() *MenuRequest {
	m := &a.menu
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.open {
		return nil
	}
	request := m.request
	return &request
}

// BrowserMenuReady shows the menu at the height its page drew it.
func (a *App) BrowserMenuReady(height int) {
	m := &a.menu
	m.mu.Lock()
	window, open, width := m.window, m.open, m.width
	m.mu.Unlock()
	if window == nil || !open {
		return
	}
	window.SetSize(width, min(max(height, 40), 900))
}

// BrowserMenuChoose closes the menu with a choice ("" for none).
func (a *App) BrowserMenuChoose(id string) { a.closeMenu(id) }

func (a *App) closeMenu(choice string) {
	m := &a.menu
	m.mu.Lock()
	if !m.open {
		m.mu.Unlock()
		return
	}
	m.open = false
	window := m.window
	m.mu.Unlock()
	if window != nil {
		window.Hide()
	}
	a.emitEvent("menu:choice", map[string]string{"id": choice})
}
