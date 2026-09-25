package app

import (
	"testing"

	"github.com/local/mayak/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWindowPlacementRestore(t *testing.T) {
	primary := &application.Screen{ID: "primary", IsPrimary: true, WorkArea: application.Rect{Width: 2560, Height: 1400}}
	left := &application.Screen{ID: "second", Name: "DISPLAY2", WorkArea: application.Rect{X: -1920, Width: 1920, Height: 1040}}
	for _, tt := range []struct {
		name                string
		saved               config.WindowState
		screens             []*application.Screen
		x, y, width, height int
		id                  string
	}{
		{"legacy negative coordinates", config.WindowState{X: -1646, Y: 25, Width: 1632, Height: 892}, []*application.Screen{primary, left}, -1646, 25, 1632, 892, "second"},
		{"display moved", config.WindowState{ScreenID: "second", ScreenX: 2560, X: 2660, Y: 40, Width: 1200, Height: 760}, []*application.Screen{primary, left}, -1820, 40, 1200, 760, "second"},
		{"monitor handle changed", config.WindowState{ScreenID: "old-handle", ScreenName: "DISPLAY2", ScreenX: 2560, X: 2660, Y: 40, Width: 1200, Height: 760}, []*application.Screen{primary, left}, -1820, 40, 1200, 760, "second"},
		{"display unplugged", config.WindowState{ScreenID: "second", ScreenX: -1920, X: -1800, Y: 40, Width: 1200, Height: 760}, []*application.Screen{primary}, 0, 40, 1200, 760, "primary"},
		{"resolution reduced", config.WindowState{ScreenID: "second", ScreenX: -1920, X: -1900, Y: 0, Width: 2200, Height: 1300}, []*application.Screen{primary, left}, -1920, 0, 1920, 1040, "second"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.saved.Configured, tt.saved.Maximized = true, true
			got := restoreWindowPlacement(tt.saved, tt.screens)
			if got.X != tt.x || got.Y != tt.y || got.Width != tt.width || got.Height != tt.height || got.ScreenID != tt.id || !got.Maximized {
				t.Fatalf("unexpected restored placement: %+v", got)
			}
		})
	}
}

func TestWindowPlacementMaximizedMonitorTransfer(t *testing.T) {
	saved := config.WindowState{ScreenID: "primary", X: 100, Y: 50, Width: 1200, Height: 760, Configured: true}
	left := &application.Screen{ID: "second", WorkArea: application.Rect{X: -1920, Width: 1920, Height: 1040}}
	got := rememberWindowPlacement(saved, -1928, -8, 1936, 1056, true, left)
	if got.X != -1820 || got.Y != 50 || got.Width != 1200 || got.Height != 760 || got.ScreenID != "second" || !got.Maximized {
		t.Fatalf("normal rectangle did not follow maximized window: %+v", got)
	}
	if again := rememberWindowPlacement(got, -1928, -8, 1936, 1056, true, left); again != got {
		t.Fatalf("repeated resize drifted: %+v", again)
	}
}

func TestColorRef(t *testing.T) {
	if c, err := colorRef("#1f1e1d"); err != nil || c != 0x001d1e1f {
		t.Fatalf("colorRef = %#x, %v", c, err)
	}
	for _, bad := range []string{"", "1f1e1d", "#fff", "#gggggg", "rgb(0,0,0)"} {
		if _, err := colorRef(bad); err == nil {
			t.Errorf("colorRef(%q) accepted", bad)
		}
	}
}
