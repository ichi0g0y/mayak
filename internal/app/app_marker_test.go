package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/mayak/internal/applog"
	"github.com/local/mayak/internal/config"
)

func TestPlayerMarkerStylesAreKnownOrDefault(t *testing.T) {
	for _, style := range playerMarkerStyles {
		if normalizePlayerMarker(style) != style {
			t.Fatalf("%q not kept", style)
		}
	}
	for _, unknown := range []string{"", "huge", "Custom"} {
		if got := normalizePlayerMarker(unknown); got != playerMarkerDefault {
			t.Fatalf("%q -> %q", unknown, got)
		}
	}
	if normalizePlayerMarker("large") != "outline" {
		t.Fatal("the 0.1.15 name of the outline is not carried over")
	}
	// Nothing scales the icon: it keeps tarkov.dev's size on the map.
	for _, style := range playerMarkerStyles {
		if rules := playerMarkerRules(style, "IMG", "BOX", "data:image/png;base64,AAAA"); strings.Contains(rules, "IMG{transform") || strings.Contains(rules, "48px") {
			t.Fatalf("%s scales the icon: %s", style, rules)
		}
	}
	// Every style but tarkov.dev's own has rules on the marker image.
	for _, style := range playerMarkerStyles[1:] {
		rules := playerMarkerRules(style, "IMG", "BOX", "data:image/png;base64,AAAA")
		if !strings.HasPrefix(rules, "IMG{") {
			t.Fatalf("%s: %s", style, rules)
		}
	}
	if playerMarkerRules(playerMarkerDefault, "IMG", "BOX", "") != "" || playerMarkerRules("custom", "IMG", "BOX", "") != "" {
		t.Fatal("rules for the default marker, or a custom one without an image")
	}
	// Rules on the box reach each of the selectors it is made of.
	pulse := playerMarkerRules("pulse", "IMG", "A,B", "")
	if !strings.Contains(pulse, "A::before") || !strings.Contains(pulse, "B::before") || !strings.Contains(pulse, "@keyframes mayak-marker-pulse") {
		t.Fatal(pulse)
	}
}

func TestPlayerMarkerImageBecomesADataURL(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "marker.png")
	if err := os.WriteFile(png, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := playerMarkerImageData(png)
	if err != nil || !strings.HasPrefix(data, "data:image/png;base64,") {
		t.Fatalf("png: %q %v", data, err)
	}
	svg := filepath.Join(dir, "marker.svg")
	if err := os.WriteFile(svg, []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := playerMarkerImageData(svg); err != nil || !strings.HasPrefix(data, "data:image/svg+xml;base64,") {
		t.Fatalf("svg: %q %v", data, err)
	}
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(text, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := playerMarkerImageData(text); err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Fatalf("a text file was accepted, or the error does not name it: %v", err)
	}
	// An SVG in UTF-16 and a WebP the sniffing does not know go by extension.
	utf16 := filepath.Join(dir, "wide.svg")
	if err := os.WriteFile(utf16, []byte("\xff\xfe<\x00s\x00v\x00g\x00/\x00>\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := playerMarkerImageData(utf16); err != nil || !strings.HasPrefix(data, "data:image/svg+xml;base64,") {
		t.Fatalf("utf-16 svg: %q %v", data, err)
	}
	odd := filepath.Join(dir, "odd.webp")
	if err := os.WriteFile(odd, []byte("not really webp"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := playerMarkerImageData(odd); err != nil || !strings.HasPrefix(data, "data:image/webp;base64,") {
		t.Fatalf("webp by extension: %q %v", data, err)
	}
	if _, err := playerMarkerImageData(""); err == nil {
		t.Fatal("no path was accepted")
	}
	// The site's sheet carries the image; without one the default marker shows.
	a := &App{logs: applog.New(10)}
	if css := a.playerMarkerCSS(config.Settings{PlayerMarker: "custom", PlayerMarkerImage: png}); !strings.Contains(css, "data:image/png;base64,") || !strings.Contains(css, markerImageOnSite) {
		t.Fatal(css)
	}
	if css := a.playerMarkerCSS(config.Settings{PlayerMarker: "custom", PlayerMarkerImage: text}); css != "" {
		t.Fatal(css)
	}
	if css := a.playerMarkerCSS(config.Settings{PlayerMarker: "glow-red"}); !strings.Contains(css, "#ff3b30") {
		t.Fatal(css)
	}
}

func TestTarkovDevScriptCarriesTheMarkerSheet(t *testing.T) {
	script := tarkovDevScript("AB12", `IMG{transform:scale(2)}`)
	for _, want := range []string{`"tarkov.dev"`, `"/map/"`, `"/maps/"`, `"connection"`, `"sessionId"`, `"AB12"`, `mayak-player-marker`, `IMG{transform:scale(2)}`} {
		if !strings.Contains(script, want) {
			t.Fatalf("script lacks %s: %s", want, script)
		}
	}
	if plain := tarkovDevScript("AB12", ""); !strings.Contains(plain, `const css="";`) {
		t.Fatal(plain)
	}
}
