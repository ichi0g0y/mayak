// Package mapdata is the geometry of the interactive maps the squad map
// draws: how game coordinates land on each map's picture, its floors, and
// the picture itself.
//
// Both come from tarkov.dev as they are now: the map list is tarkov.dev's
// own src/data/maps.json (MIT), the pictures are its SVG maps on
// assets.tarkov.dev (Shebuka and others, CC BY-NC-SA 4.0). They are kept on
// disk (the "maps" cache folder), checked again once a day, and the last
// good copy is used when tarkov.dev cannot be reached.
package mapdata

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/userdata"
)

// ListURL is tarkov.dev's map list.
const ListURL = "https://raw.githubusercontent.com/the-hideout/tarkov-dev/main/src/data/maps.json"

// svgHost is where map pictures may come from.
const svgHost = "https://assets.tarkov.dev/"

const (
	checkEvery = 24 * time.Hour
	maxList    = 8 << 20
	maxSVG     = 16 << 20
)

// Extent is a height range of a floor, optionally only within some areas
// ([[x1, z1], [x2, z2]] in game coordinates).
type Extent struct {
	Height [2]float64      `json:"height"`
	Bounds [][2][2]float64 `json:"bounds,omitempty"`
}

// Layer is a floor above or below the ground level, drawn from the SVG map
// (SVGLayer, a group in it) and/or from tiles of its own (TilePath). ID
// names it: its SVGLayer, or "tile-<n>" for a floor only in tiles. Show is
// the floor tarkov.dev shows first (Icebreaker's), over an undimmed base.
type Layer struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	SVGLayer string   `json:"svgLayer,omitempty"`
	TilePath string   `json:"tilePath,omitempty"`
	Show     bool     `json:"show,omitempty"`
	Extents  []Extent `json:"extents"`
}

// Label is a place name written on the map: at (X, Z), turned by Rotation
// degrees, Size percent of the normal size; it shows on the floors between
// Bottom and Top (and Y, its height). Ground is a name given no height: it
// spans every floor, as tarkov.dev has it, but names a place on the ground.
type Label struct {
	Text     string  `json:"text"`
	X        float64 `json:"x"`
	Z        float64 `json:"z"`
	Y        float64 `json:"y"`
	Rotation float64 `json:"rotation,omitempty"`
	Size     float64 `json:"size,omitempty"`
	Top      float64 `json:"top"`
	Bottom   float64 `json:"bottom"`
	Ground   bool    `json:"ground,omitempty"`
}

// Map is one interactive map. Keys are tarkov.dev's map names; Aliases are
// the other names that use the same map (night-factory, ground-zero-21). A
// map has an SVG picture (SVG, with its ground level SVGLayer), tiles on
// tarkov.dev (TilePath, a URL template with {z}/{x}/{y}, TileSize pixels
// square), or both, as tarkov.dev's Abstract and Satellite.
type Map struct {
	Key      string   `json:"key"`
	Aliases  []string `json:"aliases,omitempty"`
	SVG      string   `json:"svg,omitempty"`
	SVGLayer string   `json:"svgLayer,omitempty"`
	TilePath string   `json:"tilePath,omitempty"`
	TileSize float64  `json:"tileSize,omitempty"`
	// HeightRange bounds the ground level's heights (none: all of them).
	HeightRange *[2]float64    `json:"heightRange,omitempty"`
	Transform   [4]float64     `json:"transform"`
	Rotation    float64        `json:"rotation"`
	Bounds      [2][2]float64  `json:"bounds"`
	SVGBounds   *[2][2]float64 `json:"svgBounds,omitempty"`
	MinZoom     float64        `json:"minZoom"`
	MaxZoom     float64        `json:"maxZoom"`
	Author      string         `json:"author,omitempty"`
	AuthorLink  string         `json:"authorLink,omitempty"`
	Layers      []Layer        `json:"layers,omitempty"`
	Labels      []Label        `json:"labels,omitempty"`
}

// Source fetches and keeps the map list and pictures.
type Source struct {
	http    *http.Client
	dir     string
	agent   string
	listURL string

	mu   sync.Mutex
	maps []Map
}

// New keeps its files in dir.
func New(dir, userAgent string) *Source {
	return &Source{http: &http.Client{Timeout: 30 * time.Second}, dir: dir, agent: userAgent, listURL: ListURL}
}

// Maps returns the interactive maps, from tarkov.dev when the copy on disk
// is a day old, else from the disk.
func (s *Source) Maps(ctx context.Context) ([]Map, error) {
	body, fresh, err := s.cached(ctx, s.listURL, filepath.Join(s.dir, "maps.json"), maxList, func(b []byte) error {
		_, err := parseList(b)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maps == nil || fresh {
		maps, err := parseList(body)
		if err != nil {
			return nil, err
		}
		s.maps = maps
	}
	return s.maps, nil
}

// Find returns the map named name (a key or an alias).
func Find(maps []Map, name string) (Map, bool) {
	for _, m := range maps {
		if m.Key == name {
			return m, true
		}
		for _, a := range m.Aliases {
			if a == name {
				return m, true
			}
		}
	}
	return Map{}, false
}

var layerID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

// Image returns the map's picture as a data URL showing the ground level,
// or with layer (a Layer's SVGLayer) that floor over the ground level faded
// to fade percent (tarkov.dev: 20). It is meant for an <img>, where nothing
// in the SVG runs.
func (s *Source) Image(ctx context.Context, m Map, layer string, fade int) (string, error) {
	if !strings.HasPrefix(m.SVG, svgHost) || !layerID.MatchString(m.SVGLayer) {
		return "", errors.New("the map has no picture")
	}
	if layer != "" && !layerID.MatchString(layer) {
		return "", errors.New("invalid map layer")
	}
	name := path.Base(m.SVG)
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+\.svg$`).MatchString(name) {
		return "", errors.New("invalid map picture name")
	}
	body, _, err := s.cached(ctx, m.SVG, filepath.Join(s.dir, "svg", name), maxSVG, func(b []byte) error {
		if !strings.Contains(string(b[:min(len(b), 4096)]), "<svg") {
			return errors.New("not an SVG")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	styled, err := withLayer(body, m.SVGLayer, layer, fade)
	if err != nil {
		return "", err
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(styled), nil
}

// A tile of a map drawn from tiles (a Layer's or a Map's TilePath filled in).
var tileURL = regexp.MustCompile(`^https://assets\.tarkov\.dev/maps/[A-Za-z0-9_/-]{1,120}/\d{1,2}/\d{1,6}/\d{1,6}\.(png|jpg|webp)$`)

const maxTile = 4 << 20

// Tile returns one tile of a tile map as a data URL, for a picture of the
// map made in the shell: tarkov.dev serves its tiles to be shown, not to be
// read by a page.
func (s *Source) Tile(ctx context.Context, url string) (string, error) {
	match := tileURL.FindStringSubmatch(url)
	if match == nil || strings.Contains(url, "..") {
		return "", errors.New("invalid map tile")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", s.agent)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxTile+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxTile {
		return "", errors.New("the tile is too large")
	}
	kind := map[string]string{"png": "image/png", "jpg": "image/jpeg", "webp": "image/webp"}[match[1]]
	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// withLayer adds a style to the SVG that shows the ground level (the base
// group and the groups kept with it) and, with layer, that floor over the
// ground level faded to fade percent, as tarkov.dev shows it. The SVG's
// top-level groups with an id are its levels.
func withLayer(svg []byte, base, layer string, fade int) ([]byte, error) {
	open := strings.Index(string(svg), "<svg")
	if open < 0 {
		return nil, errors.New("not an SVG")
	}
	end := strings.IndexByte(string(svg[open:]), '>')
	if end < 0 {
		return nil, errors.New("not an SVG")
	}
	at := open + end + 1
	baseSel := fmt.Sprintf(`:root>g[id="%s"],:root>g[data-keep-with-group="%s"]`, base, base)
	style := `:root>g[id]{display:none}` + baseSel + `{display:inline}`
	if layer != "" && layer != base {
		style += baseSel + fmt.Sprintf(`{opacity:%.2f}`, float64(min(100, max(0, fade)))/100) + fmt.Sprintf(`:root>g[id="%s"]{display:inline}`, layer)
	}
	out := make([]byte, 0, len(svg)+len(style)+32)
	out = append(out, svg[:at]...)
	out = append(out, "<style>"+style+"</style>"...)
	return append(out, svg[at:]...), nil
}

type meta struct {
	ETag    string    `json:"etag,omitempty"`
	Checked time.Time `json:"checked"`
}

// cached returns the file for url from the disk, fetching it again (with
// its ETag) when it was last checked a day ago. A download that fails or
// does not pass valid leaves the copy on disk in use. fresh reports that
// the content came from the network just now.
func (s *Source) cached(ctx context.Context, url, file string, limit int64, valid func([]byte) error) (body []byte, fresh bool, err error) {
	var m meta
	if b, err := os.ReadFile(file + ".meta"); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	disk, diskErr := os.ReadFile(file)
	if diskErr == nil && valid(disk) != nil {
		disk, diskErr = nil, errors.New("the copy on disk is invalid")
	}
	if diskErr == nil && time.Since(m.Checked) < checkEvery {
		return disk, false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", s.agent)
	if diskErr == nil && m.ETag != "" {
		req.Header.Set("If-None-Match", m.ETag)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		if diskErr == nil {
			return disk, false, nil
		}
		return nil, false, err
	}
	defer resp.Body.Close()
	save := func(etag string) {
		m = meta{ETag: etag, Checked: time.Now()}
		if b, err := json.Marshal(m); err == nil {
			_ = userdata.WriteAtomic(file+".meta", b)
		}
	}
	switch {
	case resp.StatusCode == http.StatusNotModified && diskErr == nil:
		save(m.ETag)
		return disk, false, nil
	case resp.StatusCode == http.StatusOK:
		b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if err == nil && int64(len(b)) > limit {
			err = errors.New("the download is too large")
		}
		if err == nil {
			err = valid(b)
		}
		if err != nil {
			if diskErr == nil {
				return disk, false, nil
			}
			return nil, false, err
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
			_ = userdata.WriteAtomic(file, b)
			save(resp.Header.Get("ETag"))
		}
		return b, true, nil
	default:
		if diskErr == nil {
			return disk, false, nil
		}
		return nil, false, fmt.Errorf("%s: %s", url, resp.Status)
	}
}
