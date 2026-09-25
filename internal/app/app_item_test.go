package app

import (
	"strings"
	"testing"

	"github.com/local/mayak/internal/iteminfo"
	"github.com/local/mayak/internal/itemmatch"
)

func TestWithTaskURLs(t *testing.T) {
	info := withTaskURLs(iteminfo.Info{Tasks: []iteminfo.TaskNeed{{ID: "t", Name: "Half-Empty", Trader: "Prapor", NormalizedName: "half-empty", WikiLink: "https://escapefromtarkov.fandom.com/wiki/Half-Empty"}}}, "official-wiki")
	urls := info.Tasks[0].URLs
	if info.QuestSite != "official-wiki" || urls["tarkov-dev"] != "https://tarkov.dev/task/half-empty" || urls["official-wiki"] != "https://escapefromtarkov.fandom.com/wiki/Half-Empty" || urls["japanese-wiki"] != "https://wikiwiki.jp/eft/Prapor/Half-Empty" {
		t.Fatalf("info = %+v", info)
	}
	if got := questPageURL("japanese-wiki", "Camera, Action!", "Mechanic", ""); got != "https://wikiwiki.jp/eft/Mechanic/Camera%20Action%21" {
		t.Fatalf("japanese wiki URL = %s", got)
	}
	if got := questPageURL("japanese-wiki", "Surprise [PVE ZONE]", "Ref", ""); got != "https://wikiwiki.jp/eft/Ref/Surprise" {
		t.Fatalf("japanese wiki URL = %s", got)
	}
}

func TestSearchItems(t *testing.T) {
	items := []itemmatch.Item{{ID: "1", Name: "Graphics card", ShortName: "GPU"}, {ID: "2", Name: "Military power filter", ShortName: "MPF"}, {ID: "3", Name: "Power cord", ShortName: "Cord"}, {ID: "4", Name: "Powerbank", ShortName: "Powerbank"}, {ID: "5", Name: "Superwater", ShortName: "Water"}}
	got := searchItems(items, "power", 10)
	names := make([]string, len(got))
	for i, h := range got {
		names[i] = h.Name
	}
	// Prefix matches (shorter first), then a word inside a name.
	want := []string{"Powerbank", "Power cord", "Military power filter"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("power = %v", names)
	}
	items = append(items, itemmatch.Item{ID: "6", Name: "Magpul handguard", ShortName: "MOE"})
	if got := searchItems(items, "gpu", 10); len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("gpu = %+v", got)
	}
	if got := searchItems(items, "water", 1); len(got) != 1 || got[0].ID != "5" {
		t.Fatalf("limit = %+v", got)
	}
}

// Japanese names find items too, also by a short word inside them, and the
// hit carries the Japanese name for the sidebar.
func TestSearchItemsJapanese(t *testing.T) {
	items := []itemmatch.Item{
		{ID: "gpu", Name: "Graphics card", ShortName: "GPU", Aliases: []string{"グラフィックボード"}, Names: map[string]string{"ja": "グラフィックボード"}},
		{ID: "battery", Name: "GreenBat lithium battery", ShortName: "GreenBat", Aliases: []string{"GreenBat リチウムイオン電池"}},
	}
	if got := searchItems(items, "グラフィック", 10); len(got) != 1 || got[0].ID != "gpu" || got[0].Names["ja"] != "グラフィックボード" {
		t.Fatalf("got %+v", got)
	}
	if got := searchItems(items, "電池", 10); len(got) != 1 || got[0].ID != "battery" {
		t.Fatalf("got %+v", got)
	}
}
