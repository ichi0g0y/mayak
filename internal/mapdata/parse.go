package mapdata

import (
	"encoding/json"
	"errors"
	"slices"
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
	HeightRange        []float64    `json:"heightRange"`
	MinZoom            float64      `json:"minZoom"`
	MaxZoom            float64      `json:"maxZoom"`
	Author             string       `json:"author"`
	AuthorLink         string       `json:"authorLink"`
	Layers             []rawLayer   `json:"layers"`
}

type rawLayer struct {
	Name     string `json:"name"`
	SVGLayer string `json:"svgLayer"`
	Extents  []struct {
		Height []float64 `json:"height"`
		// Each area is [[x1, z1], [x2, z2], "label"].
		Bounds [][]json.RawMessage `json:"bounds"`
	} `json:"extents"`
}

var errNoMaps = errors.New("the map list has no interactive maps")

// parseList reads tarkov.dev's maps.json into the interactive maps, leaving
// out what does not have the geometry the squad map needs.
func parseList(body []byte) ([]Map, error) {
	var list rawList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	var maps []Map
	svgs := 0
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
				svgs++
			}
			for _, rl := range r.Layers {
				if !layerID.MatchString(rl.SVGLayer) {
					continue
				}
				l := Layer{Name: rl.Name, SVGLayer: rl.SVGLayer}
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
			maps = append(maps, m)
		}
	}
	if svgs == 0 {
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

// Floor returns the SVGLayer of the floor a player at (x, y, z) is on, or ""
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
				return l.SVGLayer
			}
			for _, b := range e.Bounds {
				if between(x, b[0][0], b[1][0]) && between(z, b[0][1], b[1][1]) {
					return l.SVGLayer
				}
			}
		}
	}
	return ""
}

func between(v, a, b float64) bool { return v >= min(a, b) && v <= max(a, b) }
