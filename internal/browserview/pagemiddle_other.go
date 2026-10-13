//go:build !windows

package browserview

func (m *Manager) pageMiddle([4]int) (int32, int32, bool) { return 0, 0, false }
