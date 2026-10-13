//go:build windows

package browserview

import "github.com/wailsapp/wails/v3/pkg/w32"

func (m *Manager) pageMiddle(area [4]int) (int32, int32, bool) {
	hwnd := w32.HWND(m.native.hwnd)
	if hwnd == 0 {
		return 0, 0, false
	}
	bounds := w32.GetClientRect(hwnd)
	if bounds == nil {
		return 0, 0, false
	}
	dpi := int(w32.GetDpiForWindow(hwnd))
	if dpi == 0 {
		dpi = 96
	}
	left, top, right, bottom := area[0]*dpi/96, area[1]*dpi/96, area[2]*dpi/96, area[3]*dpi/96
	if int(bounds.Right)-left-right <= 0 || int(bounds.Bottom)-top-bottom <= 0 {
		return 0, 0, false
	}
	x, y := w32.ClientToScreen(hwnd, (left+int(bounds.Right)-right)/2, (top+int(bounds.Bottom)-bottom)/2)
	return int32(x), int32(y), true
}
