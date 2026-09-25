//go:build windows

package autostart

import "testing"

func TestQuoteExecutable(t *testing.T) {
	got := quoteExecutable(`C:\Program Files\Mayak\Mayak.exe`)
	want := `"C:\Program Files\Mayak\Mayak.exe"`
	if got != want {
		t.Fatalf("quoteExecutable() = %q, want %q", got, want)
	}
}
