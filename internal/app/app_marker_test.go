package app

import (
	"strings"
	"testing"

	"github.com/local/mayak/internal/config"
)

func TestPlayerMarkerEffectsAndColours(t *testing.T) {
	for _, effect := range playerMarkerEffects {
		if normalizePlayerMarkerEffect(effect) != effect {
			t.Fatalf("%q not kept", effect)
		}
	}
	for _, unknown := range []string{"", "huge", "Outline", "custom", "default", "glow-red"} {
		if got := normalizePlayerMarkerEffect(unknown); got != playerMarkerNoEffect {
			t.Fatalf("%q -> %q", unknown, got)
		}
	}
	for raw, want := range map[string]string{"#FF3B30": "#ff3b30", " #abcdef ": "#abcdef", "red": "", "#fff": "", "": ""} {
		if got := normalizePlayerMarkerColor(raw); got != want {
			t.Fatalf("colour %q -> %q, want %q", raw, got, want)
		}
	}
	// 0.1.15's single choice: its effects carry over, the green glow keeps
	// its colour, the custom image and the default are no effect.
	for style, want := range map[string][2]string{"large": {"outline", ""}, "glow-red": {"glow", ""}, "glow-green": {"glow", "#3dff6e"}, "pulse": {"pulse", ""}, "custom": {"none", ""}, "default": {"none", ""}} {
		if effect, color := legacyPlayerMarker(style); effect != want[0] || color != want[1] {
			t.Fatalf("%q -> %q %q, want %v", style, effect, color, want)
		}
	}
	// Every effect but none has rules on the marker image in its colour, and
	// none scales the icon.
	for _, effect := range playerMarkerEffects[1:] {
		rules := playerMarkerRules(effect, "#123456", "IMG", "BOX")
		if !strings.HasPrefix(rules, "IMG{") || strings.Contains(rules, "IMG{transform") || !strings.Contains(rules, "#123456") {
			t.Fatalf("%s: %s", effect, rules)
		}
	}
	if playerMarkerRules(playerMarkerNoEffect, "#123456", "IMG", "BOX") != "" {
		t.Fatal("rules for no effect")
	}
	// Without a colour the outline is white and the rest red.
	if !strings.Contains(playerMarkerRules("outline", "", "IMG", "BOX"), "#ffffff") || !strings.Contains(playerMarkerRules("glow", "", "IMG", "BOX"), playerMarkerRed) {
		t.Fatal("own colours")
	}
	// Rules on the box reach each of the selectors it is made of; the
	// beacon's halo fades the colour out.
	pulse := playerMarkerRules("pulse", "", "IMG", "A,B")
	if !strings.Contains(pulse, "A::before") || !strings.Contains(pulse, "B::before") || !strings.Contains(pulse, "@keyframes mayak-marker-pulse") {
		t.Fatal(pulse)
	}
	if beacon := playerMarkerRules("beacon", "#ffd60a", "IMG", "BOX"); !strings.Contains(beacon, "rgba(255,214,10,0.45)") || !strings.Contains(beacon, "rgba(255,214,10,0)") {
		t.Fatal(beacon)
	}
	a := &App{}
	if css := a.playerMarkerCSS(config.Settings{PlayerMarkerEffect: "glow", PlayerMarkerColor: "#00ff00"}); !strings.Contains(css, markerImageOnSite) || !strings.Contains(css, "#00ff00") {
		t.Fatal(css)
	}
	if css := a.playerMarkerCSS(config.Settings{}); css != "" {
		t.Fatal(css)
	}
	if preview := a.PlayerMarkerPreviewCSS("#00ff00"); !strings.Contains(preview, `.marker-preview[data-effect="beacon"] .marker-icon::before`) || strings.Count(preview, "#00ff00") < 4 {
		t.Fatal(preview)
	}
	if a.PlayerMarkerEffectColor("outline") != "#ffffff" || a.PlayerMarkerEffectColor("pulse") != playerMarkerRed {
		t.Fatal("effect colours")
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
