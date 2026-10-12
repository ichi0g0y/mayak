package app

import (
	"strings"
	"testing"

	"github.com/local/mayak/internal/applog"
)

func TestBrowserLinkLog(t *testing.T) {
	a := &App{logs: applog.New(10)}
	a.BrowserLinkLog("Got browser:task\n(3s after the Host)")
	a.BrowserLinkLog("   ")
	a.BrowserLinkLog(strings.Repeat("あ", 300))
	got := a.logs.Entries()
	if len(got) != 2 {
		t.Fatalf("entries: %+v", got)
	}
	if got[0].Category != "Link" || got[0].Level != "Debug" || got[0].Message != "Got browser:task(3s after the Host)" {
		t.Fatalf("first entry: %+v", got[0])
	}
	if n := len([]rune(got[1].Message)); n != browserLinkLogMax {
		t.Fatalf("long line kept %d runes", n)
	}
}
