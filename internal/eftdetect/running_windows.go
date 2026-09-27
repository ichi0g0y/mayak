//go:build windows

package eftdetect

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GameRunning reports whether Escape from Tarkov is running (its process,
// gameExe, is in the process list).
func GameRunning() bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), gameExe) {
			return true
		}
	}
	return false
}
