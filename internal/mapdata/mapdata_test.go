package mapdata

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixture = `[
 {"normalizedName":"customs","maps":[
  {"key":"customs","projection":"interactive","transform":[0.239,168.65,0.239,136.35],"coordinateRotation":180,
   "bounds":[[698,-307],[-372,237]],"svgPath":"https://assets.tarkov.dev/maps/svg/Customs.svg","svgLayer":"Ground_Level",
   "minZoom":2,"maxZoom":6,"author":"Shebuka","authorLink":"https://github.com/the-hideout/tarkov-dev-svg-maps/",
   "heightRange":[-10,10],
   "labels":[{"position":[1,2],"text":"Dorms","rotation":"-9","size":80},{"position":[3,4,5],"text":"Roof","top":8,"bottom":2}],
   "layers":[
    {"name":"2nd Floor","svgLayer":"Second_Floor","extents":[{"height":[2.7,6.5],"bounds":[[[243,190],[165,125],"dorms"]]}]},
    {"name":"Underground","svgLayer":"Underground_Level","extents":[{"height":[-1000,-2]}]},
    {"name":"Tiles only","tilePath":"https://assets.tarkov.dev/maps/customs/4th/{z}/{x}/{y}.png","extents":[{"height":[0,1]}]},
    {"name":"Elsewhere","tilePath":"https://x/{z}/{x}/{y}.png","extents":[{"height":[0,1]}]}
   ]},
  {"key":"customs-2d","projection":"2D"}]},
 {"normalizedName":"factory","maps":[
  {"key":"factory","altMaps":["night-factory"],"projection":"interactive","transform":[1.629,119.9,1.629,139.3],"coordinateRotation":90,
   "bounds":[[77,-64.5],[-65.5,67.4]],"svgPath":"https://assets.tarkov.dev/maps/svg/Factory.svg","svgLayer":"Ground_Floor"}]},
 {"normalizedName":"the-lab","maps":[
  {"key":"the-lab","projection":"interactive","transform":[0.575,281.2,0.575,193.7],"coordinateRotation":270,
   "bounds":[[-80,-477],[-287,-193]],"tilePath":"https://assets.tarkov.dev/maps/labs_v4/1st/{z}/{x}/{y}.png"}]}
]`

func TestParseListKeepsInteractiveMaps(t *testing.T) {
	maps, err := parseList([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 3 {
		t.Fatalf("maps = %d", len(maps))
	}
	c, ok := Find(maps, "customs")
	if !ok || c.Rotation != 180 || c.Transform[1] != 168.65 || c.SVGLayer != "Ground_Level" || len(c.Layers) != 3 {
		t.Fatalf("customs = %+v", c)
	}
	if b := c.Layers[0].Extents[0].Bounds; len(b) != 1 || b[0][0][0] != 243 || b[0][1][1] != 125 {
		t.Fatalf("areas = %+v", b)
	}
	// Place names, one with its rotation written as a string.
	if l := c.Labels; len(l) != 2 || l[0].Rotation != -9 || l[0].Size != 80 || l[0].Y != 0 || l[0].Top != 1000 || !l[0].Ground || l[1].Ground || l[1].Y != 5 || l[1].Top != 8 {
		t.Fatalf("labels = %+v", l)
	}
	if f, ok := Find(maps, "night-factory"); !ok || f.Key != "factory" {
		t.Fatal("night-factory does not find the factory")
	}
	// A floor only in tiles is kept, named tile-<n>; tiles from elsewhere
	// are not.
	if l := c.Layers[2]; l.ID != "tile-2" || l.SVGLayer != "" || l.TilePath == "" || c.Layers[0].ID != "Second_Floor" {
		t.Fatalf("floors = %+v", c.Layers)
	}
	if l, _ := Find(maps, "the-lab"); l.TilePath == "" || l.TileSize != 256 {
		t.Fatalf("the lab has no tiles: %+v", l)
	}
	if l, _ := Find(maps, "the-lab"); l.SVG != "" {
		t.Fatal("a tile map got a picture")
	}
	if _, err := parseList([]byte(`[]`)); err == nil {
		t.Fatal("an empty list was accepted")
	}
}

func TestFloorFollowsHeightAndAreas(t *testing.T) {
	maps, _ := parseList([]byte(fixture))
	c, _ := Find(maps, "customs")
	cases := []struct {
		x, y, z float64
		want    string
	}{
		{200, 4, 150, "Second_Floor"}, // in the dorms, second floor height
		{0, 4, 0, ""},                 // same height elsewhere: the ground
		{200, 1, 150, ""},             // in the dorms on the ground floor
		{0, -10, 0, "Underground_Level"},
	}
	for _, tc := range cases {
		if got := Floor(c, tc.x, tc.y, tc.z); got != tc.want {
			t.Errorf("Floor(%v, %v, %v) = %q, want %q", tc.x, tc.y, tc.z, got, tc.want)
		}
	}
}

func TestWithLayerStylesTheLevels(t *testing.T) {
	svg := []byte(`<?xml version="1.0"?><svg viewBox="0 0 1 1"><g id="Ground_Level"/><g id="Second_Floor"/></svg>`)
	out, err := withLayer(svg, "Ground_Level", "Second_Floor", 20)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.HasPrefix(s, `<?xml version="1.0"?><svg viewBox="0 0 1 1"><style>`) || !strings.Contains(s, `:root>g[id="Second_Floor"]{display:inline}`) || !strings.Contains(s, `{opacity:0.20}`) {
		t.Fatalf("styled = %s", s)
	}
	ground, _ := withLayer(svg, "Ground_Level", "", 20)
	if strings.Contains(string(ground), "opacity") {
		t.Fatal("the ground level is faded without a floor")
	}
	if _, err := withLayer([]byte("<html>"), "a", "", 20); err == nil {
		t.Fatal("not an SVG was accepted")
	}
}

func TestCachedChecksDailyAndKeepsTheLastGoodCopy(t *testing.T) {
	var hits, fail atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() == 1 {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()
	s := New(t.TempDir(), "test")
	s.listURL = srv.URL
	ctx := context.Background()
	if maps, err := s.Maps(ctx); err != nil || len(maps) != 3 {
		t.Fatalf("Maps = %d, %v", len(maps), err)
	}
	if _, err := s.Maps(ctx); err != nil || hits.Load() != 1 {
		t.Fatalf("checked again within a day: hits %d, %v", hits.Load(), err)
	}
	// A day later: a conditional request, answered 304.
	back(t, s.dir+"/maps.json.meta")
	if _, err := s.Maps(ctx); err != nil || hits.Load() != 2 {
		t.Fatalf("not checked after a day: hits %d, %v", hits.Load(), err)
	}
	// tarkov.dev down: the copy on disk.
	back(t, s.dir+"/maps.json.meta")
	fail.Store(1)
	s.maps = nil
	if maps, err := s.Maps(ctx); err != nil || len(maps) != 3 {
		t.Fatalf("no fallback to the disk: %d, %v", len(maps), err)
	}
}

// back makes a cached file look two days old.
func back(t *testing.T, metaFile string) {
	t.Helper()
	m := `{"etag":"\"v1\"","checked":"` + time.Now().Add(-48*time.Hour).Format(time.RFC3339) + `"}`
	if err := writeFile(metaFile, m); err != nil {
		t.Fatal(err)
	}
}

func TestImageServesTheStyledPicture(t *testing.T) {
	svg := `<svg viewBox="0 0 1 1"><g id="Ground_Floor"/></svg>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(svg)) }))
	defer srv.Close()
	s := New(t.TempDir(), "test")
	m := Map{Key: "factory", SVG: svgHost + "maps/svg/Factory.svg", SVGLayer: "Ground_Floor"}
	// Only assets.tarkov.dev is fetched; point the client at the test server.
	s.http = &http.Client{Transport: rewrite{srv.URL}}
	url, err := s.Image(context.Background(), m, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/svg+xml;base64,"))
	if !strings.HasPrefix(url, "data:image/svg+xml;base64,") || !strings.Contains(string(raw), "<style>") {
		t.Fatalf("image = %s", raw)
	}
	if _, err := s.Image(context.Background(), Map{SVG: "https://evil.example/x.svg", SVGLayer: "a"}, "", 20); err == nil {
		t.Fatal("a picture from another host was fetched")
	}
	if _, err := s.Image(context.Background(), m, `x"}*{display:none`, 20); err == nil {
		t.Fatal("an unsafe layer name was accepted")
	}
}

type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	target, _ := http.NewRequest(req.Method, r.to+u.Path, nil)
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}

func writeFile(name, content string) error { return os.WriteFile(name, []byte(content), 0o644) }

func TestTileRefusesOtherURLs(t *testing.T) {
	s := New(t.TempDir(), "test")
	for _, url := range []string{
		"https://example.com/maps/labs_v4/main/0/0/0.png",
		"https://assets.tarkov.dev/maps/../secret/0/0/0.png",
		"https://assets.tarkov.dev/maps/labs_v4/main/0/0/0.svg",
		"http://assets.tarkov.dev/maps/labs_v4/main/0/0/0.png",
		"https://assets.tarkov.dev/other/labs_v4/0/0/0.png",
	} {
		if _, err := s.Tile(context.Background(), url); err == nil {
			t.Errorf("Tile(%q) was fetched", url)
		}
	}
	if !tileURL.MatchString("https://assets.tarkov.dev/maps/labs_v4/main/3/5/7.png") {
		t.Error("a tile URL is refused")
	}
}
