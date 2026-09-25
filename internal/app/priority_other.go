//go:build !windows

package app

// raisePriority is a no-op outside Windows.
func raisePriority() {}
