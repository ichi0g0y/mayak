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
// tarkov.dev's own, five that stand out, and the user's own image.
var playerMarkerStyles = []string{playerMarkerDefault, "large", "glow-red", "glow-green", "pulse", "beacon", "custom"}

// The selectors on tarkov.dev: the marker image, and its box.
const (
	markerImageOnSite  = `.leaflet-marker-icon img[src$="/player-position.png"],.leaflet-marker-icon img[src$="/player-position-no-rotation.png"]`
	markerParentOnSite = `.leaflet-marker-icon:has(> img[src$="/player-position.png"]),.leaflet-marker-icon:has(> img[src$="/player-position-no-rotation.png"])`
)

// playerMarkerImageLimit is the largest custom image, as a file; it goes
// into the page as a data URL.
const playerMarkerImageLimit = 1 << 20

func normalizePlayerMarker(style string) string {
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
	case "large":
		return img + `{transform:scale(2);filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 1.5px #fff) drop-shadow(0 1px 3px rgba(0,0,0,.9))}`
	case "glow-red":
		return img + `{transform:scale(1.8);filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 5px #ff3b30) drop-shadow(0 0 12px #ff3b30)}`
	case "glow-green":
		return img + `{transform:scale(1.8);filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 5px #3dff6e) drop-shadow(0 0 12px #3dff6e)}`
	case "pulse":
		return img + `{transform:scale(1.8);filter:drop-shadow(0 0 1.5px #fff) drop-shadow(0 1px 3px rgba(0,0,0,.9))}` +
			parentEach(parent, `::before`+ring[:len(ring)-1]+`;width:24px;height:24px;margin:-12px 0 0 -12px;border:3px solid #ff3b30;animation:mayak-marker-pulse 1.4s ease-out infinite}`) +
			`@keyframes mayak-marker-pulse{from{transform:scale(1);opacity:.9}to{transform:scale(3.4);opacity:0}}`
	case "beacon":
		return img + `{transform:scale(2.4);filter:drop-shadow(0 0 1px #000) drop-shadow(0 0 4px #ffd60a) drop-shadow(0 0 14px #ffd60a)}` +
			parentEach(parent, `::before`+ring[:len(ring)-1]+`;width:72px;height:72px;margin:-36px 0 0 -36px;background:radial-gradient(circle,rgba(255,214,10,.45),rgba(255,214,10,0) 70%)}`)
	case "custom":
		if image == "" {
			return ""
		}
		// The image replaces tarkov.dev's at twice the size, still centred on
		// the position (the box stays 24 px, so the extra hangs over evenly).
		return img + `{content:url("` + image + `");width:48px!important;height:48px!important;margin:-12px 0 0 -12px;object-fit:contain;filter:drop-shadow(0 0 2px rgba(0,0,0,.85))}`
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
	mime := http.DetectContentType(data)
	switch {
	case strings.HasPrefix(mime, "image/png"), strings.HasPrefix(mime, "image/jpeg"), strings.HasPrefix(mime, "image/gif"), strings.HasPrefix(mime, "image/webp"):
		mime = mime[:strings.IndexByte(mime+";", ';')]
	case strings.EqualFold(filepath.Ext(path), ".svg") && strings.Contains(string(data[:min(len(data), 4096)]), "<svg"):
		mime = "image/svg+xml"
	default:
		return "", errors.New("the marker image must be a PNG, JPEG, GIF, WebP or SVG file")
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
