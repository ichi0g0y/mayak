package app

import (
	"context"
	"testing"
	"time"

	"github.com/local/mayak/internal/catalog"
)

// The stations on show win over the ten-minute copy: a catalog refresh that
// renamed a station shows in the next hideout event at once.
func TestHideoutStationsPreferThoseOnShow(t *testing.T) {
	a := &App{}
	a.hideoutStationsCache = map[string]hideoutStationsEntry{"pve": {stations: []catalog.HideoutStation{{AreaType: 1, Name: "old"}}, at: time.Now()}}
	a.hideoutStations = []catalog.HideoutStation{{AreaType: 1, Name: "new"}}
	a.hideoutCatalogMode = "pve"
	got := a.hideoutStationsFor(context.Background(), "pve")
	if len(got) != 1 || got[0].Name != "new" {
		t.Fatalf("stations %+v, want those on show", got)
	}
	// Another mode still has its copy.
	a.hideoutStationsCache["regular"] = hideoutStationsEntry{stations: []catalog.HideoutStation{{AreaType: 1, Name: "pvp"}}, at: time.Now()}
	if got := a.hideoutStationsFor(context.Background(), "regular"); len(got) != 1 || got[0].Name != "pvp" {
		t.Fatalf("other mode %+v", got)
	}
}
