//go:build !windows

package browserview

// GameWheel has no WebViews to turn here.
func GameWheel(uintptr, float64, float64, float64) {}
