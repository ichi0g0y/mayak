package app

import (
	"sync"

	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/capture"
	"github.com/local/mayak/internal/config"
)

// With config GameWheel on and GameScrollCombo held in EFT (a button or key
// of capture.WheelCombos), the wheel scrolls MAYAK's window; with
// GameTabCombo held, it goes through MAYAK's tabs (the shell's cycleView: the
// sidebar's views, then the tabs). A page can be read on without leaving the
// game (capture.ForwardWheel).
type wheelState struct {
	mu        sync.Mutex
	combos    [2]string
	forwarder *capture.WheelForwarder
}

// applyGameWheel starts taking the wheel with the combos held (to scroll, to
// switch tabs), in place of those it was started with, or stops for none.
func (a *App) applyGameWheel(scroll, tabs string) {
	w := &a.wheel
	w.mu.Lock()
	defer w.mu.Unlock()
	if a.quitting.Load() {
		scroll, tabs = "", ""
	}
	if tabs == scroll {
		tabs = "" // as normalizeSettings keeps it
	}
	combos := [2]string{scroll, tabs}
	if combos == w.combos {
		return
	}
	if w.forwarder != nil {
		w.forwarder.Stop()
		w.forwarder = nil
	}
	w.combos = [2]string{}
	if scroll == "" && tabs == "" {
		return
	}
	forwarder, err := capture.ForwardWheel(capture.WheelOptions{
		Scroll:     scroll,
		Tabs:       tabs,
		SwitchTab:  func(step int) { a.emitEvent("browser:game-tab", step) },
		ScrollPage: a.scrollPageFromGame,
		Point: func() (int32, int32, bool) {
			if a.browserViews == nil {
				return 0, 0, false
			}
			return a.browserViews.PageMiddle()
		},
		Note: func(text string) { a.addLog("Debug", "Capture", "The "+text) },
	})
	if err != nil {
		a.addLog("Warn", "Capture", "Could not take the wheel from EFT: "+err.Error())
		return
	}
	w.combos, w.forwarder = combos, forwarder
	a.addLog("Info", "Capture", "The wheel turned in EFT scrolls MAYAK with "+orNone(scroll)+" held, switches its tabs with "+orNone(tabs))
}

// gameWheelCombos are the combos to take the wheel with, none while
// GameWheel is off.
func (a *App) gameWheelCombos(s config.Settings) (scroll, tabs string) {
	if !s.GameWheel {
		return "", ""
	}
	return s.GameScrollCombo, s.GameTabCombo
}

// scrollPageFromGame turns the wheel of the page in hwnd with no key held.
func (a *App) scrollPageFromGame(hwnd uintptr, x, y, dy float64) {
	browserview.GameWheel(hwnd, x, y, dy)
}

func orNone(combo string) string {
	if combo == "" {
		return "none"
	}
	return combo
}
