package app

import (
	"github.com/local/mayak/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Keep the normal rectangle, but move it with the display even when the
// maximized window is transferred without passing through a normal state.
func rememberWindowPlacement(saved config.WindowState, x, y, width, height int, maximized bool, screen *application.Screen) config.WindowState {
	if !maximized || !saved.Configured {
		saved.X, saved.Y, saved.Width, saved.Height = x, y, width, height
		saved.Configured = true
	} else if screen != nil && saved.ScreenID != "" {
		saved.X += screen.WorkArea.X - saved.ScreenX
		saved.Y += screen.WorkArea.Y - saved.ScreenY
	}
	if screen != nil {
		saved.ScreenID = screen.ID
		saved.ScreenName = screen.Name
		saved.ScreenX, saved.ScreenY = screen.WorkArea.X, screen.WorkArea.Y
	}
	saved.Maximized = maximized
	return saved
}

// Resolve the display after Wails has enumerated screens. Screen-relative
// coordinates survive changes to the desktop's layout and work-area origin.
func restoreWindowPlacement(saved config.WindowState, screens []*application.Screen) config.WindowState {
	return restorePlacement(saved, screens, 760, 560)
}

// restorePlacement is restoreWindowPlacement for a window at least
// minWidth×minHeight in size.
func restorePlacement(saved config.WindowState, screens []*application.Screen, minWidth, minHeight int) config.WindowState {
	var target *application.Screen
	for _, screen := range screens {
		// Windows IDs are monitor handles and can change across OS sessions.
		if (saved.ScreenName != "" && screen.Name == saved.ScreenName) || (saved.ScreenName == "" && saved.ScreenID != "" && screen.ID == saved.ScreenID) {
			target = screen
			break
		}
	}
	if target != nil {
		saved.X += target.WorkArea.X - saved.ScreenX
		saved.Y += target.WorkArea.Y - saved.ScreenY
	} else {
		// Legacy files have no display ID. Choose the largest overlap, not
		// just the top-left corner (which may lie in the invisible border).
		best := 0
		for _, screen := range screens {
			r := screen.WorkArea
			overlap := max(0, min(saved.X+saved.Width, r.X+r.Width)-max(saved.X, r.X)) * max(0, min(saved.Y+saved.Height, r.Y+r.Height)-max(saved.Y, r.Y))
			if overlap > best {
				best, target = overlap, screen
			}
		}
		if target == nil && len(screens) > 0 {
			target = screens[0]
			for _, screen := range screens {
				if screen.IsPrimary {
					target = screen
					break
				}
			}
		}
	}
	saved.Width, saved.Height = max(minWidth, saved.Width), max(minHeight, saved.Height)
	if target != nil {
		r := target.WorkArea
		saved.Width = max(minWidth, min(saved.Width, r.Width))
		saved.Height = max(minHeight, min(saved.Height, r.Height))
		saved.X = max(r.X, min(saved.X, r.X+max(0, r.Width-saved.Width)))
		saved.Y = max(r.Y, min(saved.Y, r.Y+max(0, r.Height-saved.Height)))
		saved.ScreenID, saved.ScreenX, saved.ScreenY = target.ID, r.X, r.Y
		saved.ScreenName = target.Name
	}
	return saved
}
