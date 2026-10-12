//go:build !windows

package capture

import (
	"image"
	"time"
)

// Hotkey is a registered key; none can be registered here.
type Hotkey struct{}

// StartHotkey cannot register a key on this platform.
func StartHotkey(Key, func()) (*Hotkey, error) { return nil, ErrUnsupported }

// Stop does nothing here.
func (h *Hotkey) Stop() {}

// ClickWatcher watches the mouse; it does nothing here.
type ClickWatcher struct{}

// WatchClicks never calls settled on this platform.
func WatchClicks(time.Duration, time.Duration, func(Click)) *ClickWatcher { return &ClickWatcher{} }

// Stop does nothing here.
func (w *ClickWatcher) Stop() {}

// Grab cannot take the screen on this platform.
func Grab() (*image.RGBA, error) { return nil, ErrUnsupported }
