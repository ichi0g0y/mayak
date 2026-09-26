package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/local/mayak/internal/config"
)

// The player marker on tarkov.dev's map is a Leaflet divIcon (class
// "marker") whose HTML is <img src="/maps/interactive/player-position.png"
// style="width:24px;height:24px;rotate:Ndeg"> (player-position-no-rotation.png
// without a heading), anchored at its centre. A style sheet the document
// script adds to map pages (tarkovDevScript) gives that image an effect (an
// outline, a glow, a pulsing ring, a beacon) in a colour of the user's
// choice. The icon stays tarkov.dev's, at its 24 px and place: the effects
// reach beyond it. tarkov.dev's rotation stays too: "rotate" is an inline
// property of its own, so a filter from the sheet composes with it. Rules
// for the marker's box (the ring, the beacon's halo) reach it through
// :has().

const playerMarkerNoEffect = "none"

// playerMarkerEffects are the effects to choose from, in the settings' order.
var playerMarkerEffects = []string{playerMarkerNoEffect, "outline", "glow", "pulse", "beacon"}

// The colour an effect has until one is chosen: white for the outline (as
// the icon's own rim), a warning red for the rest.
const playerMarkerRed = "#ff3b30"

var playerMarkerColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// The selectors on tarkov.dev: the marker image, and its box.
const (
	markerImageOnSite  = `.leaflet-marker-icon img[src$="/player-position.png"],.leaflet-marker-icon img[src$="/player-position-no-rotation.png"]`
	markerParentOnSite = `.leaflet-marker-icon:has(> img[src$="/player-position.png"]),.leaflet-marker-icon:has(> img[src$="/player-position-no-rotation.png"])`
)

func normalizePlayerMarkerEffect(effect string) string {
	for _, known := range playerMarkerEffects {
		if effect == known {
			return effect
		}
	}
	return playerMarkerNoEffect
}

// normalizePlayerMarkerColor keeps a #rrggbb colour, lower-cased; anything
// else is no colour, so the effect's own applies.
func normalizePlayerMarkerColor(color string) string {
	color = strings.TrimSpace(color)
	if !playerMarkerColorPattern.MatchString(color) {
		return ""
	}
	return strings.ToLower(color)
}

// legacyPlayerMarker reads 0.1.15's single choice (playerMarker), which had
// the glow in two colours and a custom image, as an effect and a colour.
func legacyPlayerMarker(style string) (effect, color string) {
	switch style {
	case "large", "outline":
		return "outline", ""
	case "glow-red":
		return "glow", ""
	case "glow-green":
		return "glow", "#3dff6e"
	case "pulse", "beacon":
		return style, ""
	}
	return playerMarkerNoEffect, ""
}

// playerMarkerEffectColor is the colour effect is drawn in: color when
// chosen, else the effect's own.
func playerMarkerEffectColor(effect, color string) string {
	if color = normalizePlayerMarkerColor(color); color != "" {
		return color
	}
	if effect == "outline" {
		return "#ffffff"
	}
	return playerMarkerRed
}

// rgba is color (#rrggbb) with an alpha, for a gradient that fades it out.
func rgba(color string, alpha float64) string {
	var r, g, b int
	fmt.Sscanf(color, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", r, g, b, alpha)
}

// playerMarkerRules is the style sheet of one effect in one colour, with
// img the selector of the marker image and parent that of its box.
func playerMarkerRules(effect, color, img, parent string) string {
	c := playerMarkerEffectColor(effect, color)
	const ring = `{content:"";position:absolute;left:50%;top:50%;border-radius:50%;pointer-events:none;box-sizing:border-box`
	switch effect {
	case "outline":
		// Three thin shadows make a solid rim, the dark one lifts it off the map.
		return img + `{filter:drop-shadow(0 0 1.5px ` + c + `) drop-shadow(0 0 1.5px ` + c + `) drop-shadow(0 0 2px ` + c + `) drop-shadow(0 1px 3px rgba(0,0,0,.9))}`
	case "glow":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 5px ` + c + `) drop-shadow(0 0 12px ` + c + `)}`
	case "pulse":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 1px 3px rgba(0,0,0,.9))}` +
			parentEach(parent, `::before`+ring+`;width:24px;height:24px;margin:-12px 0 0 -12px;border:3px solid `+c+`;animation:mayak-marker-pulse 1.4s ease-out infinite}`) +
			`@keyframes mayak-marker-pulse{from{transform:scale(1);opacity:.9}to{transform:scale(3.4);opacity:0}}`
	case "beacon":
		return img + `{filter:drop-shadow(0 0 1px #000) drop-shadow(0 0 4px ` + c + `) drop-shadow(0 0 14px ` + c + `)}` +
			parentEach(parent, `::before`+ring+`;width:72px;height:72px;margin:-36px 0 0 -36px;background:radial-gradient(circle,`+rgba(c, .45)+`,`+rgba(c, 0)+` 70%)}`)
	}
	return ""
}

// parentEach appends suffix to each comma-separated selector in parent.
func parentEach(parent, suffix string) string {
	parts := strings.Split(parent, ",")
	for i := range parts {
		parts[i] += suffix
	}
	return strings.Join(parts, ",")
}

// playerMarkerCSS is the style sheet for tarkov.dev's map pages under
// settings s: empty without an effect.
func (a *App) playerMarkerCSS(s config.Settings) string {
	return playerMarkerRules(normalizePlayerMarkerEffect(s.PlayerMarkerEffect), s.PlayerMarkerColor, markerImageOnSite, markerParentOnSite)
}

// PlayerMarkerEffects lists the effects the settings offer.
func (a *App) PlayerMarkerEffects() []string { return append([]string(nil), playerMarkerEffects...) }

// PlayerMarkerPreviewCSS is a style sheet for the settings page: each
// effect's rules, in color (or its own), on
// .marker-preview[data-effect=<effect>] .marker-icon (the box) and its img.
func (a *App) PlayerMarkerPreviewCSS(color string) string {
	var sheet strings.Builder
	for _, effect := range playerMarkerEffects {
		parent := `.marker-preview[data-effect="` + effect + `"] .marker-icon`
		sheet.WriteString(playerMarkerRules(effect, color, parent+" img", parent))
	}
	return sheet.String()
}

// PlayerMarkerEffectColor is the colour effect shows in without a colour
// chosen, for the settings' colour control.
func (a *App) PlayerMarkerEffectColor(effect string) string {
	return playerMarkerEffectColor(normalizePlayerMarkerEffect(effect), "")
}
