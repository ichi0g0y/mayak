//go:build !windows

package autostart

import "errors"

// Apply is unavailable because this PoC currently targets Windows startup.
func Apply(enabled bool) error {
	if enabled {
		return errors.New("launch at startup is available only on Windows")
	}
	return nil
}

// RemoveLegacy has nothing to remove outside Windows.
func RemoveLegacy() (bool, error) { return false, nil }
