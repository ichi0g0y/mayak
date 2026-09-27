//go:build !windows

package eftdetect

// GameRunning is false where the game does not run.
func GameRunning() bool { return false }
