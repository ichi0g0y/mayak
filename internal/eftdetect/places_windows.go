//go:build windows

package eftdetect

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// documentsDirectory is Windows' own Documents folder (the known folder,
// wherever it was moved: another drive, OneDrive). EFT, a Unity game, saves
// its screenshots under it.
func documentsDirectory() string {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		return ""
	}
	return path
}

// installedGame is where Windows has EFT installed (the launcher registers
// it for uninstalling), "" when it is not.
func installedGame() string {
	const key = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\EscapeFromTarkov`
	for _, access := range []uint32{registry.QUERY_VALUE | registry.WOW64_32KEY, registry.QUERY_VALUE | registry.WOW64_64KEY} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, access)
		if err != nil {
			continue
		}
		location, _, err := k.GetStringValue("InstallLocation")
		k.Close()
		if err == nil && location != "" {
			return location
		}
	}
	return ""
}
