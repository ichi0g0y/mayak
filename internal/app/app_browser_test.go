package app

import (
	"github.com/local/mayak/internal/browserview"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserStateReplacement(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	a := &App{}
	for _, raw := range []string{`{"tabs":[{"id":"one"}]}`, `{"tabs":[{"id":"one"},{"id":"two"}]}`} {
		if e := a.BrowserSave(raw); e != nil {
			t.Fatal(e)
		}
		got, e := a.BrowserLoad()
		if e != nil || got != raw {
			t.Fatalf("state roundtrip: %q %v", got, e)
		}
	}
	p, _ := browserStatePath()
	files, e := os.ReadDir(filepath.Dir(p))
	if e != nil || len(files) != 1 {
		t.Fatalf("temporary files left behind: %v %v", files, e)
	}
	if e := a.BrowserSave(`not json`); e == nil {
		t.Fatal("invalid state accepted")
	}
}
func TestBrowserRejectsPrivilegedNavigationBeforeNativeCalls(t *testing.T) {
	a := &App{}
	for _, u := range []string{"file:///C:/settings.json", "javascript:alert(1)", "http://wails.localhost/", "https://user:secret@example.com"} {
		if e := a.BrowserView("show", browserview.Options{ID: "test", URL: u}); e == nil {
			t.Fatalf("accepted %q", u)
		}
	}
	if e := a.BrowserView("show", browserview.Options{ID: "../../outside", URL: "https://tarkov.dev"}); e == nil {
		t.Fatal("accepted traversal ID")
	}
}
