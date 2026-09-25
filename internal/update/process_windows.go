package update

import (
	"golang.org/x/sys/windows"
)

// processRunning reports whether the process still exists.
func processRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied means it exists; anything else, that it is gone.
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && state == uint32(windows.WAIT_TIMEOUT)
}
