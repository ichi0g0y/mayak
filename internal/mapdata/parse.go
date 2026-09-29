package mapdata

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
)

type rawList []struct {
	NormalizedName string   `json:"normalizedName"`
	Maps           []rawMap `json:"maps"`
}

type rawMap struct {
	Key                string       `json:"key"`
	AltMaps            []string     `json:"altMaps"`
	Projection         string       `json:"projection"`
	Transform          []float64    `json:"transform"`
	CoordinateRotation float64      `json:"coordinateRotation"`
	Bounds             [][2]float64 `json:"bounds"`
	SVGBounds          [][2]float64 `json:"svgBounds"`
	SVGPath            string       `json:"svgPath"`
	SVGLayer           string       `json:"svgLayer"`
	TilePath           string       `json:"tilePath"`
	TileSize           float64      `json:"tileSize"`
	HeightRange        []float64    `json:"heightRange"`
	MinZoom            float64      `json:"minZoom"`
	MaxZoom            float64      `json:"maxZoom"`
	Author             string       `json:"author"`
	AuthorLink         string       `json:"authorLink"`
	Layers             []rawLayer   `json:"layers"`
	Labels             []struct {
		Position []number `json:"position"`
		Text     string   `json:"text"`
		Rotation number   `json:"rotation"`
		Size     number   `json:"size"`
		Top      *number  `json:"top"`
		Bottom   *number  `json:"bottom"`
	} `json:"labels"`
}

// number is a number in maps.json that is sometimes written as a string
// ("rotation": "6"); anything else reads as 0.
type number float64

func (n *number) UnmarshalJSON(b []byte) error {
	var f float64
	if json.Unmarshal(b, &f) == nil {
		*n = number(f)
		return nil
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			*n = number(v)
		}
	}
	return nil
}

type rawLayer struct {
	Name     string `json:"name"`
	SVGLayer string `json:"svgLayer"`
	TilePath string `json:"tilePath"`
	Show     bool   `json:"show"`
	Extents  []struct {
		Height []float64 `json:"height"`
		// Each area is [[x1, z1], [x2, z2], "label"].
		Bounds [][]json.RawMessage `json:"bounds"`
	} `json:"extents"`
}

var errNoMaps = errors.New("the map list has no interactive maps")

// tilePath keeps a tile URL template on tarkov.dev's assets, else "".
func tilePath(p string) string {
	if strings.HasPrefix(p, svgHost) && strings.Contains(p, "{z}") && strings.Contains(p, "{x}") && strings.Contains(p, "{y}") {
		return p
	}
	return ""
}

// parseList reads tarkov.dev's maps.json into the interactive maps, leaving
// out what does not have the geometry the squad map needs.
func parseList(body []byte) ([]Map, error) {
	var list rawList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	var maps []Map
	drawable := 0
	for _, group := range list {
		for _, r := range group.Maps {
			if r.Projection != "interactive" || !validMap(r.Key) || len(r.Transform) != 4 || len(r.Bounds) != 2 {
				continue
			}
			m := Map{
				Key:        r.Key,
				Transform:  [4]float64(r.Transform),
				Rotation:   r.CoordinateRotation,
				Bounds:     [2][2]float64(r.Bounds),
				MinZoom:    r.MinZoom,
				MaxZoom:    r.MaxZoom,
				Author:     r.Author,
				AuthorLink: r.AuthorLink,
			}
			for _, a := range append(append([]string(nil), r.AltMaps...), group.NormalizedName) {
				if validMap(a) && a != m.Key && !slices.Contains(m.Aliases, a) {
					m.Aliases = append(m.Aliases, a)
				}
			}
			if len(r.SVGBounds) == 2 {
				b := [2][2]float64(r.SVGBounds)
				m.SVGBounds = &b
			}
			if r.SVGPath != "" && layerID.MatchString(r.SVGLayer) {
				m.SVG, m.SVGLayer = r.SVGPath, r.SVGLayer
			}
			if m.TilePath = tilePath(r.TilePath); m.TilePath != "" {
				m.TileSize = r.TileSize
				if m.TileSize <= 0 {
					m.TileSize = 256
				}
			}
			if m.SVG != "" || m.TilePath != "" {
				drawable++
			}
			if len(r.HeightRange) == 2 {
				h := [2]float64(r.HeightRange)
				m.HeightRange = &h
			}
			for i, rl := range r.Layers {
				l := Layer{Name: rl.Name, TilePath: tilePath(rl.TilePath), Show: rl.Show}
				if m.SVG != "" && layerID.MatchString(rl.SVGLayer) {
					l.SVGLayer = rl.SVGLayer
				}
				if l.SVGLayer == "" && l.TilePath == "" {
					continue
				}
				l.ID = l.SVGLayer
				if l.ID == "" {
					l.ID = "tile-" + strconv.Itoa(i)
				}
				for _, re := range rl.Extents {
					if len(re.Height) != 2 {
						continue
					}
					e := Extent{Height: [2]float64(re.Height)}
					for _, area := range re.Bounds {
						var a, b [2]float64
						if len(area) < 2 || json.Unmarshal(area[0], &a) != nil || json.Unmarshal(area[1], &b) != nil {
							continue
						}
						e.Bounds = append(e.Bounds, [2][2]float64{a, b})
					}
					l.Extents = append(l.Extents, e)
				}
				if len(l.Extents) > 0 {
					m.Layers = append(m.Layers, l)
				}
			}
			// Labels as tarkov.dev places them: the height given, else the
			// middle of their top and bottom, else of the map's height range.
			for _, rl := range r.Labels {
				if len(rl.Position) < 2 || rl.Text == "" {
					continue
				}
				l := Label{Text: rl.Text, X: float64(rl.Position[0]), Z: float64(rl.Position[1]), Rotation: float64(rl.Rotation), Size: float64(rl.Size), Top: 1000, Bottom: -1000}
				if rl.Top != nil {
					l.Top = float64(*rl.Top)
				}
				if rl.Bottom != nil {
					l.Bottom = float64(*rl.Bottom)
				}
				switch {
				case len(rl.Position) > 2:
					l.Y = float64(rl.Position[2])
				case rl.Top != nil || rl.Bottom != nil:
					l.Y = (l.Top + l.Bottom) / 2
				default:
					l.Ground = true
				}
				if l.Ground && len(r.HeightRange) == 2 {
					l.Y = (r.HeightRange[0] + r.HeightRange[1]) / 2
				}
				m.Labels = append(m.Labels, l)
			}
			maps = append(maps, m)
		}
	}
	if drawable == 0 {
		return nil, errNoMaps
	}
	return maps, nil
}

// validMap is the same check as the rest of MAYAK's map names.
func validMap(name string) bool {
	if name == "" || len(name) > 60 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// Floor returns the ID of the floor a player at (x, y, z) is on, or ""
// for the ground level: the first layer with an extent whose height range
// holds y and, if it has areas, one of them holds (x, z). This is how
// tarkov.dev places markers on its floors.
func Floor(m Map, x, y, z float64) string {
	for _, l := range m.Layers {
		for _, e := range l.Extents {
			if y < e.Height[0] || y >= e.Height[1] {
				continue
			}
			if len(e.Bounds) == 0 {
				return l.ID
			}
			for _, b := range e.Bounds {
				if between(x, b[0][0], b[1][0]) && between(z, b[0][1], b[1][1]) {
					return l.ID
				}
			}
		}
	}
	return ""
}

func between(v, a, b float64) bool { return v >= min(a, b) && v <= max(a, b) }
