package app

import (
	"fmt"
	"strconv"
	"strings"
)

// BrowserSetWindowTheme colors the native title bar and border to match the
// browser shell's theme. Colors are "#rrggbb"; dark selects light or dark
// caption buttons. Platforms without a colorable title bar ignore it.
func (a *App) BrowserSetWindowTheme(caption, text, border string, dark bool) error {
	colors := make([]uint32, 3)
	for i, value := range []string{caption, text, border} {
		c, err := colorRef(value)
		if err != nil {
			return err
		}
		colors[i] = c
	}
	return a.applyWindowTheme(colors[0], colors[1], colors[2], dark)
}

// colorRef converts "#rrggbb" to a Win32 COLORREF (0x00BBGGRR).
func colorRef(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return 0, fmt.Errorf("invalid color %q", value)
	}
	rgb, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid color %q", value)
	}
	r, g, b := uint32(rgb>>16)&0xff, uint32(rgb>>8)&0xff, uint32(rgb)&0xff
	return b<<16 | g<<8 | r, nil
}
