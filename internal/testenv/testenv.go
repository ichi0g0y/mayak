// Package testenv keeps a package's tests off this PC's own MAYAK data.
package testenv

import (
	"os"
	"testing"
)

// Run runs a package's tests with the config and cache folders
// (os.UserConfigDir, os.UserCacheDir: %AppData%, %LocalAppData%,
// XDG_CONFIG_HOME, XDG_CACHE_HOME, and HOME on macOS) in a temporary folder,
// so a test never writes the user's settings or log (an App's applog wrote
// to the real mayak.log). A package's TestMain calls it:
//
//	func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }
func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "mayak-test-")
	if err != nil {
		return m.Run()
	}
	defer os.RemoveAll(dir)
	for _, name := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "HOME"} {
		_ = os.Setenv(name, dir)
	}
	return m.Run()
}
