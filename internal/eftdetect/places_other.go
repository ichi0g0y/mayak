//go:build !windows

package eftdetect

import (
	"os"
	"path/filepath"
)

// documentsDirectory is the home's Documents folder off Windows (EFT runs on
// Windows only; this keeps the package building elsewhere).
func documentsDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents")
}

// installedGame is "" off Windows.
func installedGame() string { return "" }
