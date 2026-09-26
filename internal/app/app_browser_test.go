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

func TestBrowserShortcutKeysMatchTheShell(t *testing.T) {
	key := func(name string, ctrl, shift, alt bool) browserview.Key {
		return browserview.Key{ID: "tab", Key: name, Ctrl: ctrl, Shift: shift, Alt: alt}
	}
	taken := []browserview.Key{key("t", true, false, false), key("t", true, true, false), key("w", true, false, false), key("f4", true, false, false), key("tab", true, false, false), key("tab", true, true, false), key("pageup", true, false, false), key("pagedown", true, false, false), key("l", true, false, false), key("d", true, false, false), key("d", false, false, true), key("f6", false, false, false), key("1", true, false, false), key("9", true, false, false)}
	for _, k := range taken {
		if !browserShortcutKey(k) {
			t.Errorf("not taken: %+v", k)
		}
	}
	left := []browserview.Key{key("t", false, false, false), key("t", true, false, true), key("w", true, true, false), key("0", true, false, false), key("l", true, true, false), key("r", true, false, false), key("f", true, false, false), key("c", true, false, false), key("f5", false, false, false), key("d", false, true, true), key("f6", true, false, false)}
	for _, k := range left {
		if browserShortcutKey(k) {
			t.Errorf("taken: %+v", k)
		}
	}
	a := &App{}
	if !a.browserShortcut(key("t", true, false, false)) || a.browserShortcut(key("r", true, false, false)) {
		t.Fatal("shortcut handler disagrees with the key set")
	}
}
