// Package appdir names the folder MAYAK keeps its data in under the user's
// configuration directory, and moves the data of its earlier name there.
package appdir

import (
	"os"
	"path/filepath"
)

// Name is the app's folder under os.UserConfigDir(): Mayak, or DevName for
// the `task dev` build (UseDev), set before anything reads the folder.
var Name = "Mayak"

// DevName is the `task dev` build's folder, apart from the installed app's
// so that the two run side by side as two PCs would (their own pairing,
// squad member key, window and web data).
const DevName = "Mayak-dev"

// devSeed is what the `task dev` build takes from the installed app's
// folder when its own does not exist yet: the preferences, folders and
// keys the user set, but nothing that makes it the same PC (browser.json
// with the pairing and the squad joined, the squad member key, the window).
var devSeed = []string{
	"settings.json", "preferences.json", "browser-preferences.json", "bookmarks.json",
	"map-drawings.json", "tracker-tokens.dat", "sounds",
}

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

// Path is a file or folder in the app's data folder (parts joined under it).
// With the user's configuration directory unknown it returns the error, and
// the parts under the app's folder name alone.
func Path(parts ...string) (string, error) {
	base, err := os.UserConfigDir()
	return filepath.Join(append([]string{base, Name}, parts...)...), err
}

// FreshName is the folder of `task dev:fresh`: emptied at every start, never
// seeded, so the app starts as on a new PC (the folder detection, the
// tutorial and the setup notice show as they would).
const FreshName = "Mayak-fresh"

// UseFresh makes Name FreshName and empties that folder.
func UseFresh() error {
	base, err := os.UserConfigDir()
	Name = FreshName
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(base, FreshName))
}

// UseDev makes Name DevName and, when that folder does not exist yet,
// seeds it from the installed app's (devSeed). It tells whether it seeded.
func UseDev() (bool, error) {
	base, err := os.UserConfigDir()
	Name = DevName
	if err != nil {
		return false, err
	}
	return seed(filepath.Join(base, "Mayak"), filepath.Join(base, DevName))
}

func seed(from, to string) (bool, error) {
	if _, err := os.Stat(to); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return false, err
	}
	if _, err := os.Stat(from); err != nil {
		return false, nil
	}
	for _, name := range devSeed {
		if err := copyTree(filepath.Join(from, name), filepath.Join(to, name)); err != nil && !os.IsNotExist(err) {
			return true, err
		}
	}
	return true, nil
}

func copyTree(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(from)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(to, 0o700); err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o600)
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
