//go:build !darwin

package browserview

// blockerSet has nothing to do here: Windows asks the blocker about each
// request as tabs make them (filter_windows.go).
func blockerSet(ContentBlocker) {}
