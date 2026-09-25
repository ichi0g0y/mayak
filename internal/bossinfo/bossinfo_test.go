package bossinfo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const mapsJSON = `{"data":{
 "maps":{
  "m1":{"id":"m1","name":"m1 Name","normalizedName":"customs","bosses":[
   {"mob":"bossBully","spawnChance":0.75,"spawnLocations":[{"name":"ZoneDormitory","chance":0.5},{"name":"ZoneGasStation","chance":0.5}],"escorts":[{"mob":"followerBully","amount":[{"count":2,"chance":0.5},{"count":4,"chance":0.5}]}]},
   {"mob":"bossKnight","spawnChance":0.25,"spawnLocations":[{"name":"ZoneScavBase","chance":1}],"escorts":[{"mob":"followerBigPipe","amount":[{"count":1,"chance":1}]}]},
   {"mob":"bossKnight","spawnChance":0.3,"spawnLocations":[{"name":"ZoneScavBase","chance":0.4}],"escorts":[]},
   {"mob":"pmcUSEC","spawnChance":0.5,"spawnLocations":[],"escorts":[]}
  ]},
  "m2":{"id":"m2","name":"m2 Name","normalizedName":"woods","bosses":[]}
 },
 "goonReports":[{"map":"m2","timestamp":"1790000000000"},{"map":"m1","timestamp":"1790003600000"},{"map":"gone","timestamp":"1790007200000"}],
 "mobs":{"bossKnight":{"id":"bossKnight","imagePortraitLink":"https://assets.tarkov.dev/knight-portrait.png"}}
}}`

func server(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pve/maps":
			*hits++
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte(mapsJSON))
		case "/pve/maps_en":
			_, _ = w.Write([]byte(`{"data":{"m1 Name":"Customs","m2 Name":"Woods","bossBully":"Reshala","bossKnight":"Knight","followerBully":"Reshala's guard","followerBigPipe":"Big Pipe","ZoneDormitory":"Dorms","ZoneGasStation":"New Gas Station","ZoneScavBase":"Stronghold"}}`))
		case "/pve/maps_ja":
			_, _ = w.Write([]byte(`{"data":{"m1 Name":"税関","bossBully":""}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestInfo(t *testing.T) {
	hits := 0
	srv := server(t, &hits)
	defer srv.Close()
	c := New()
	c.base = srv.URL + "/"
	info, err := c.Info(context.Background(), "pve", "ja")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != "pve" || len(info.Maps) != 2 {
		t.Fatalf("info = %+v", info)
	}
	var customs Map
	for _, m := range info.Maps {
		if m.Key == "customs" {
			customs = m
		}
	}
	// Japanese where translated, English otherwise (an empty one too).
	if customs.Name != "税関" || len(customs.Bosses) != 2 || customs.Bosses[0].Name != "Reshala" {
		t.Fatalf("customs = %+v", customs)
	}
	reshala, knight := customs.Bosses[0], customs.Bosses[1]
	if reshala.Chance != 0.75 || len(reshala.Locations) != 2 || reshala.Escorts[0] != (Escort{Name: "Reshala's guard", Min: 2, Max: 4}) {
		t.Fatalf("reshala = %+v", reshala)
	}
	// Two spawn entries make one row: the higher chance, the places merged.
	if knight.Chance != 0.3 || len(knight.Locations) != 1 || knight.Locations[0] != (Location{"Stronghold", 1}) || knight.Portrait == "" {
		t.Fatalf("knight = %+v", knight)
	}
	// Newest report first; one on an unknown map is dropped.
	if len(info.Goons) != 2 || info.Goons[0].MapKey != "customs" || info.Goons[1].Map != "Woods" || !info.Goons[0].Time.Equal(time.UnixMilli(1790003600000)) {
		t.Fatalf("goons = %+v", info.Goons)
	}
	// Within the TTL nothing is asked; after it, an unchanged ETag keeps the data.
	if _, err := c.Info(context.Background(), "pve", "ja"); err != nil || hits != 1 {
		t.Fatalf("hits = %d, err = %v", hits, err)
	}
	c.mu.Lock()
	entry := c.maps["pve"]
	entry.checked = time.Now().Add(-time.Hour)
	c.maps["pve"] = entry
	c.mu.Unlock()
	again, err := c.Info(context.Background(), "pve", "ja")
	if err != nil || hits != 2 || len(again.Goons) != 2 {
		t.Fatalf("hits = %d, err = %v, goons = %d", hits, err, len(again.Goons))
	}
}

func TestMode(t *testing.T) {
	for in, want := range map[string]string{"pve": "pve", "regular": "regular", "pvp-season": "regular", "auto": "regular", "": "regular"} {
		if got := Mode(in); got != want {
			t.Errorf("Mode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSendReport(t *testing.T) {
	var got map[string]any
	status := http.StatusOK
	answer := `{"status":"success"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request = %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	c := New()
	c.reportURL = srv.URL
	at := time.UnixMilli(1790000000000)
	if err := c.SendReport(context.Background(), Report{MapNameID: "bigmap", Mode: "pve", Time: at, AccountID: "1234567"}); err != nil {
		t.Fatal(err)
	}
	if got["map"] != "bigmap" || got["gameMode"] != "pve" || got["timestamp"] != float64(1790000000000) || got["accountId"] != float64(1234567) {
		t.Fatalf("body = %v", got)
	}
	for _, bad := range []Report{{MapNameID: "bigmap", Mode: "pve", Time: at, AccountID: "12a"}, {MapNameID: "bigmap", Mode: "arena", Time: at, AccountID: "1"}, {Mode: "pve", Time: at, AccountID: "1"}} {
		if err := c.SendReport(context.Background(), bad); err == nil {
			t.Errorf("%+v was sent", bad)
		}
	}
	status, answer = http.StatusBadRequest, "invalid map"
	if err := c.SendReport(context.Background(), Report{MapNameID: "bigmap", Mode: "pve", Time: at, AccountID: "1"}); err == nil {
		t.Fatal("a refused report passed")
	}
	status, answer = http.StatusOK, `{"status":"failure"}`
	if err := c.SendReport(context.Background(), Report{MapNameID: "bigmap", Mode: "pve", Time: at, AccountID: "1"}); err == nil {
		t.Fatal("a failed report passed")
	}
}
