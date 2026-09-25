//go:build windows

package autostart

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "Mayak"
	// legacyValueName is the entry of the app's earlier name, removed either way.
	legacyValueName = "RaidLens"
)

// Apply adds or removes MAYAK from the current user's Windows startup apps.
func Apply(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	_ = key.DeleteValue(legacyValueName)

	if !enabled {
		err = key.DeleteValue(valueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	return key.SetStringValue(valueName, quoteExecutable(executable))
}

// RemoveLegacy removes the startup entry of the app's earlier name and
// reports whether there was one. That entry starts an executable that no
// longer exists; MAYAK's own entry follows the setting and is left alone.
func RemoveLegacy() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	err = key.DeleteValue(legacyValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func quoteExecutable(path string) string {
	return `"` + path + `"`
}
