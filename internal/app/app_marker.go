package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/mayak/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The player marker on tarkov.dev's map is a Leaflet divIcon (class
// "marker") whose HTML is <img src="/maps/interactive/player-position.png"
// style="width:24px;height:24px;rotate:Ndeg"> (player-position-no-rotation.png
// without a heading), anchored at its centre. A style sheet the document
// script adds to map pages (tarkovDevScript) restyles that image, and
// tarkov.dev's rotation stays: "rotate" is an inline property of its own,
// so a transform or filter from the sheet composes with it. Rules for the
// marker's own box (the ring of "pulse", the halo of "beacon") reach it
// through :has().

const playerMarkerDefault = "default"

// playerMarkerStyles are the markers to choose from, in the settings' order:
// tarkov.dev's own, five that stand out, and the user's own image. None is
// drawn larger than tarkov.dev's 24 px: the effects reach beyond the icon,
// the icon itself keeps its size and place on the map.
var playerMarkerStyles = []string{playerMarkerDefault, "outline", "glow-red", "glow-green", "pulse", "beacon", "custom"}

// The selectors on tarkov.dev: the marker image, and its box.
const (
	markerImageOnSite  = `.leaflet-marker-icon img[src$="/player-position.png"],.leaflet-marker-icon img[src$="/player-position-no-rotation.png"]`
	markerParentOnSite = `.leaflet-marker-icon:has(> img[src$="/player-position.png"]),.leaflet-marker-icon:has(> img[src$="/player-position-no-rotation.png"])`
)

// playerMarkerImageLimit is the largest custom image, as a file; it goes
// into the page as a data URL.
const playerMarkerImageLimit = 1 << 20

func normalizePlayerMarker(style string) string {
	if style == "large" { // the outline's name in 0.1.15, when it also scaled the icon
		return "outline"
	}
	for _, known := range playerMarkerStyles {
		if style == known {
			return style
		}
	}
	return playerMarkerDefault
}

// playerMarkerRules is the style sheet for one marker style, with img the
// selector of the marker image and parent that of its box. image is the
// custom image as a data URL; "custom" without one is no rule at all.
func playerMarkerRules(style, img, parent, image string) string {
	const ring = `{content:"";position:absolute;left:50%;top:50%;border-radius:50%;pointer-events:none;box-sizing:border-box}`
	switch style {
	case "outline":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 2px #fff) drop-shadow(0 1px 3px rgba(0,0,0,.9))}`
	case "glow-red":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 5px #ff3b30) drop-shadow(0 0 12px #ff3b30)}`
	case "glow-green":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 5px #3dff6e) drop-shadow(0 0 12px #3dff6e)}`
	case "pulse":
		return img + `{filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 1px 3px rgba(0,0,0,.9))}` +
			parentEach(parent, `::before`+ring[:len(ring)-1]+`;width:24px;height:24px;margin:-12px 0 0 -12px;border:3px solid #ff3b30;animation:mayak-marker-pulse 1.4s ease-out infinite}`) +
			`@keyframes mayak-marker-pulse{from{transform:scale(1);opacity:.9}to{transform:scale(3.4);opacity:0}}`
	case "beacon":
		return img + `{filter:drop-shadow(0 0 1px #000) drop-shadow(0 0 4px #ffd60a) drop-shadow(0 0 14px #ffd60a)}` +
			parentEach(parent, `::before`+ring[:len(ring)-1]+`;width:72px;height:72px;margin:-36px 0 0 -36px;background:radial-gradient(circle,rgba(255,214,10,.45),rgba(255,214,10,0) 70%)}`)
	case "custom":
		if image == "" {
			return ""
		}
		// The image replaces tarkov.dev's in its 24 px box (sized here too:
		// an SVG without a size of its own would be drawn at none).
		return img + `{content:url("` + image + `");width:24px!important;height:24px!important;object-fit:contain}`
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

// playerMarkerImageData reads a custom marker image as a data URL: PNG,
// JPEG, GIF, WebP or SVG, up to playerMarkerImageLimit.
func playerMarkerImageData(path string) (string, error) {
	if path == "" {
		return "", errors.New("no image chosen")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("the marker image is a folder")
	}
	if info.Size() > playerMarkerImageLimit {
		return "", fmt.Errorf("the marker image is larger than %d KB", playerMarkerImageLimit/1024)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// The type from the content where it can be told (PNG, JPEG, GIF, WebP),
	// else from the extension: an SVG is XML in any encoding, and a file the
	// sniffing does not know is still handed to the page, which shows it or
	// nothing. Only a file of no image kind at all is refused.
	sniffed := http.DetectContentType(data)
	sniffed = sniffed[:strings.IndexByte(sniffed+";", ';')]
	byExtension := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml"}[strings.ToLower(filepath.Ext(path))]
	mime := sniffed
	if !strings.HasPrefix(sniffed, "image/") {
		mime = byExtension
	}
	if mime == "" {
		return "", fmt.Errorf("%s is not an image MAYAK can use (a PNG, JPEG, GIF, WebP or SVG file; this reads as %s)", filepath.Base(path), sniffed)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// playerMarkerCSS is the style sheet for tarkov.dev's map pages under
// settings s: empty for tarkov.dev's own marker, and for a custom image
// that cannot be read (logged; the map shows tarkov.dev's marker then).
func (a *App) playerMarkerCSS(s config.Settings) string {
	style := normalizePlayerMarker(s.PlayerMarker)
	image := ""
	if style == "custom" {
		data, err := playerMarkerImageData(s.PlayerMarkerImage)
		if err != nil {
			a.addLog("Warn", "Browser", "The player marker image cannot be used, so tarkov.dev's own marker shows: "+err.Error())
			return ""
		}
		image = data
	}
	return playerMarkerRules(style, markerImageOnSite, markerParentOnSite, image)
}

// PlayerMarkerStyles lists the marker styles the settings offer.
func (a *App) PlayerMarkerStyles() []string { return append([]string(nil), playerMarkerStyles...) }

// PlayerMarkerPreviewCSS is a style sheet for the settings page's gallery:
// every style's rules on .marker-preview[data-style=<style>] .marker-icon
// (the box) and its img, the custom one with the image at imagePath.
func (a *App) PlayerMarkerPreviewCSS(imagePath string) string {
	var sheet strings.Builder
	for _, style := range playerMarkerStyles {
		image := ""
		if style == "custom" {
			if data, err := playerMarkerImageData(imagePath); err == nil {
				image = data
			}
		}
		parent := `.marker-preview[data-style="` + style + `"] .marker-icon`
		sheet.WriteString(playerMarkerRules(style, parent+" img", parent, image))
	}
	return sheet.String()
}

// ChoosePlayerMarkerFile asks for a custom marker image and checks it can be used.
func (a *App) ChoosePlayerMarkerFile() (string, error) {
	path, err := a.desktop.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{CanChooseFiles: true, Window: a.window,
		Title:   "Select player marker image",
		Filters: []application.FileFilter{{DisplayName: "Images (*.png, *.svg, *.jpg, *.jpeg, *.webp, *.gif)", Pattern: "*.png;*.svg;*.jpg;*.jpeg;*.webp;*.gif"}},
	}).PromptForSingleSelection()
	if err != nil || path == "" {
		return path, err
	}
	if _, err := playerMarkerImageData(path); err != nil {
		return "", err
	}
	return filepath.Clean(path), nil
}
