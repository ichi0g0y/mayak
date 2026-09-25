// Package appdir names the folder MAYAK keeps its data in under the user's
// configuration directory, and moves the data of its earlier name there.
package appdir

import (
	"os"
	"path/filepath"
)

// Name is the app's folder under os.UserConfigDir().
const Name = "Mayak"

// legacyName is the folder of the app's earlier name, RaidLens.
const legacyName = "RaidLens"

// legacyLog is the log file's earlier name, renamed inside the folder.
const legacyLog, logName = "raidlens.log", "mayak.log"

// Config is the app's data folder ("" when the user's configuration
// directory is unknown).
func Config() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, Name)
}

// Migrate moves the data folder of the earlier name to Name, once, when Name
// does not exist yet. It reports what it did, for the log.
func Migrate() (moved bool, err error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return false, err
	}
	return migrate(base)
}

func migrate(base string) (bool, error) {
	from, to := filepath.Join(base, legacyName), filepath.Join(base, Name)
	if _, err := os.Stat(to); err == nil {
		return false, nil
	}
	if info, err := os.Stat(from); err != nil || !info.IsDir() {
		return false, nil
	}
	if err := os.Rename(from, to); err != nil {
		return false, err
	}
	// A rename never fails for a file that is not there.
	_ = os.Rename(filepath.Join(to, legacyLog), filepath.Join(to, logName))
	return true, nil
}
