//go:build !windows

package browserview

import "errors"

// capture is Windows-only for now: macOS and Linux views have no capture.
func (m *Manager) capture(id string, full bool) ([]byte, error) {
	return nil, errors.New("capturing a page is not supported on this platform")
}
