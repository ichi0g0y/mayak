package config

import "testing"

func TestWindowGeometryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	want := WindowState{ScreenID: "display-2", ScreenName: "DISPLAY2", ScreenX: -1920, X: -1646, Y: 25, Width: 1120, Height: 760, Configured: true, Maximized: true}
	if err := SaveWindow(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadWindow()
	if err != nil || got != want {
		t.Fatalf("window state: got %+v, want %+v, error %v", got, want, err)
	}
}
