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
	current   [4]string // wheelButtons of the options running
	forwarder *capture.WheelForwarder
}

// wheelButtons are o's buttons and keys.
func wheelButtons(o capture.WheelOptions) [4]string {
	return [4]string{o.Scroll, o.Tabs, o.CloseTab, o.PinTab}
}

// gameWheelSettings are the buttons and keys to take the wheel with, none
// while GameWheel is off.
func (a *App) gameWheelSettings(s config.Settings) capture.WheelOptions {
	if !s.GameWheel {
		return capture.WheelOptions{}
	}
	o := capture.WheelOptions{Scroll: s.GameScrollCombo, Tabs: s.GameTabCombo, CloseTab: s.GameCloseTab, PinTab: s.GamePinTab}
	if o.Tabs == o.Scroll {
		o.Tabs = "" // as normalizeSettings keeps it
	}
	return o
}

// applyGameWheel starts taking the wheel with the buttons and keys of o, in
// place of those it was started with, or stops for none.
func (a *App) applyGameWheel(o capture.WheelOptions) {
	w := &a.wheel
	w.mu.Lock()
	defer w.mu.Unlock()
	if a.quitting.Load() {
		o = capture.WheelOptions{}
	}
	if wheelButtons(o) == w.current {
		return
	}
	if w.forwarder != nil {
		w.forwarder.Stop()
		w.forwarder = nil
	}
	w.current = [4]string{}
	if o.Scroll == "" && o.Tabs == "" && o.CloseTab == "" && o.PinTab == "" {
		return
	}
	set := o
	o.SwitchTab = func(step int) { a.emitEvent("browser:game-tab", step) }
	o.TabAction = func(action string) { a.emitEvent("browser:game-tab-action", action) }
	o.ScrollPage = a.scrollPageFromGame
	o.Point = func() (int32, int32, bool) {
		if a.browserViews == nil {
			return 0, 0, false
		}
		return a.browserViews.PageMiddle()
	}
	o.Note = func(text string) { a.addLog("Debug", "Capture", "The "+text) }
	forwarder, err := capture.ForwardWheel(o)
	if err != nil {
		a.addLog("Warn", "Capture", "Could not take the wheel from EFT: "+err.Error())
		return
	}
	w.current, w.forwarder = wheelButtons(set), forwarder
	a.addLog("Info", "Capture", "The wheel turned in EFT scrolls MAYAK with "+orNone(set.Scroll)+" held, switches its tabs with "+orNone(set.Tabs)+" (close: "+orNone(set.CloseTab)+", pin: "+orNone(set.PinTab)+")")
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

// stopGameWheel takes the hook out, as MAYAK quits.
func (a *App) stopGameWheel() { a.applyGameWheel(capture.WheelOptions{}) }
