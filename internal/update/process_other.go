//go:build !windows

package update

import (
	"errors"
	"syscall"
)

// processRunning reports whether the process still exists.
func processRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
