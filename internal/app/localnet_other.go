//go:build !darwin

package app

// requestLocalNetwork is a no-op outside macOS, which alone asks before an
// app reaches the local network.
func requestLocalNetwork() {}
