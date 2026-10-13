//go:build windows

package browserview

import (
	"encoding/json"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// GameWheel turns the wheel of MAYAK's page in hwnd (a WebView's window: the
// shell's or a page view's) at (x, y), CSS pixels in its viewport, by dy
// pixels (down for more than 0), with no key held: the wheel turned in the
// game with Shift or Ctrl held (capture.ForwardWheel), which the page told
// the wheel as Windows' message would read as a sideways scroll or a zoom.
// It goes through DevTools, the way the page takes a wheel turned over it.
// (A scroll gesture glides, but felt less like the wheel than this.)
func GameWheel(hwnd uintptr, x, y, dy float64) bool {
	params, _ := json.Marshal(map[string]any{"type": "mouseWheel", "x": x, "y": y, "deltaX": 0, "deltaY": dy, "modifiers": 0, "pointerType": "mouse"})
	sent := false
	application.InvokeSync(func() {
		if e := edge.ForWindow(hwnd); e != nil {
			sent = e.DevToolsCall("Input.dispatchMouseEvent", string(params), func(string, error) {}) == nil
		}
	})
	return sent
}
