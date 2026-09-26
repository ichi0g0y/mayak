//go:build windows

package browserview

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// MaxCaptureHeight caps a whole-page capture (CSS pixels): a very long page
// is cut there rather than making an image too large to draw on.
const MaxCaptureHeight = 15000

const captureTimeout = 30 * time.Second

// evaluate is the parameters of Runtime.evaluate for a script.
func evaluate(script string) string {
	params, _ := json.Marshal(map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true})
	return string(params)
}

// capture takes a PNG of the tab id through the DevTools protocol
// (Page.captureScreenshot): what is on screen, or with full the whole page
// down to MaxCaptureHeight. A page that scrolls inside an element of its own
// (a map) has nothing beyond the screen, so full shows the same. prepare,
// when set, runs in the page first (to hide a bar), and restore after.
func (m *Manager) capture(id string, full bool, prepare, restore string) ([]byte, error) {
	type result struct {
		png []byte
		err error
	}
	done := make(chan result, 1)
	var view *nativeView
	// finish hands the result over once. Every callback runs on the UI
	// thread; restore goes first, so the page looks as before.
	finish := func(png []byte, err error) {
		if restore != "" && view != nil {
			_ = view.chromium.DevToolsCall("Runtime.evaluate", evaluate(restore), func(string, error) {})
		}
		select {
		case done <- result{png, err}:
		default:
		}
	}
	shoot := func(params string) error {
		return view.chromium.DevToolsCall("Page.captureScreenshot", params, func(text string, err error) {
			if err != nil {
				finish(nil, err)
				return
			}
			var shot struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal([]byte(text), &shot); err != nil || shot.Data == "" {
				finish(nil, errors.New("the page gave no image"))
				return
			}
			png, err := base64.StdEncoding.DecodeString(shot.Data)
			finish(png, err)
		})
	}
	take := func() error {
		if !full {
			return shoot(`{"format":"png","fromSurface":true}`)
		}
		return view.chromium.DevToolsCall("Page.getLayoutMetrics", "{}", func(text string, err error) {
			if err != nil {
				finish(nil, err)
				return
			}
			var metrics struct {
				CSSContentSize struct{ Width, Height float64 } `json:"cssContentSize"`
				ContentSize    struct{ Width, Height float64 } `json:"contentSize"`
			}
			_ = json.Unmarshal([]byte(text), &metrics)
			size := metrics.CSSContentSize
			if size.Width <= 0 || size.Height <= 0 {
				size = metrics.ContentSize
			}
			if size.Width <= 0 || size.Height <= 0 {
				finish(nil, errors.New("the page's size is unknown"))
				return
			}
			width, height := math.Ceil(size.Width), math.Min(math.Ceil(size.Height), MaxCaptureHeight)
			params := fmt.Sprintf(`{"format":"png","fromSurface":true,"captureBeyondViewport":true,"clip":{"x":0,"y":0,"width":%g,"height":%g,"scale":1}}`, width, height)
			if err := shoot(params); err != nil {
				finish(nil, err)
			}
		})
	}
	err := application.InvokeSyncWithError(func() error {
		v := m.native.views[id]
		if v == nil || m.native.closed {
			return errors.New("the page is not open")
		}
		view = v
		if prepare == "" {
			return take()
		}
		return v.chromium.DevToolsCall("Runtime.evaluate", evaluate(prepare), func(string, error) {
			if err := take(); err != nil {
				finish(nil, err)
			}
		})
	})
	if err != nil {
		return nil, err
	}
	select {
	case r := <-done:
		return r.png, r.err
	case <-time.After(captureTimeout):
		return nil, errors.New("capturing the page timed out")
	}
}
