//go:build !windows

package sysfonts

// List is Windows-only so far: elsewhere the shell's own fonts are offered.
func List() []string { return nil }
