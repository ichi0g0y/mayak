package catalog

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestHideoutTypedModes(t *testing.T) {
	c := New()
	c.cacheDir = ""
	for _, mode := range []string{"regular", "pve", "pvp-season"} {
		data := map[string]json.RawMessage{
			"hideout":    json.RawMessage(`{"data":{"wall":{"id":"wall","name":"wall-name","normalizedName":"defective-wall","areaType":22,"levels":[{"id":"` + mode + `-1","level":1,"constructionTime":123,"itemRequirements":[{"item":"item","count":3,"attributes":{"foundInRaid":true}}],"stationLevelRequirements":[{"station":"wall","level":0}],"traderRequirements":[{"trader":"trader","level":2}]}]}}}`),
			"hideout_en": json.RawMessage(`{"data":{"wall-name":"Defective Wall"}}`), "items": json.RawMessage(`{"data":{"items":{"item":{"name":"item-name"}}}}`), "items_en": json.RawMessage(`{"data":{"item-name":"Tool"}}`), "traders": json.RawMessage(`{"data":{"trader":{"name":"trader-name"}}}`), "traders_en": json.RawMessage(`{"data":{"trader-name":"Trader"}}`),
		}
		c.snapshots[mode] = &Snapshot{Mode: mode, UpdatedAt: time.Now(), Resources: data}
	}
	for _, mode := range []string{"regular", "pve", "pvp-season"} {
		stations, err := c.Hideout(context.Background(), mode)
		if err != nil || len(stations) != 1 {
			t.Fatal(stations, err)
		}
		s := stations[0]
		l := s.Levels[0]
		if s.AreaType != 22 || s.Name != "Defective Wall" || l.ID != mode+"-1" || l.ItemRequirements[0].Name != "Tool" || l.TraderRequirements[0].Name != "Trader" || l.ConstructionTime != 123 {
			t.Fatalf("catalog mapping/isolation failed: %+v", s)
		}
	}
}
func TestLiveHideoutAreaMapping(t *testing.T) {
	if os.Getenv("MAYAK_LIVE_CATALOG") == "" {
		t.Skip("opt-in public API check")
	}
	c := New()
	c.cacheDir = t.TempDir()
	for _, mode := range []string{"regular", "pve", "pvp-season"} {
		stations, err := c.Hideout(context.Background(), mode)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range stations {
			if s.AreaType == 22 {
				found = s.NormalizedName == "defective-wall" && s.Name == "Defective Wall"
			}
		}
		if !found {
			t.Fatalf("area 22 mapping failed for %s", mode)
		}
	}
}
