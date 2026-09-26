//go:build !windows

package app

// raisePriority is a no-op outside Windows.
func raisePriority() {}

// guardPriority is a no-op outside Windows.
func guardPriority(<-chan struct{}, func() bool) {}
