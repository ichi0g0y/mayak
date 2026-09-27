package itemapi

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
)

// fakeSource serves resources by mode and path, as tarkov.dev's JSON API
// would, and counts the requests.
type fakeSource struct {
	resources map[string]any
	requests  int
}

func (f *fakeSource) Get(_ context.Context, mode, path string, target any) error {
	f.requests++
	value, ok := f.resources[mode+"/"+path]
	if !ok {
		return errors.New("not found: " + mode + "/" + path)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func catalog(mode string, withJapanese bool) *fakeSource {
	items := map[string]any{"data": map[string]any{"items": map[string]any{
		"a": map[string]string{"id": "a", "name": "item-a-name", "shortName": "item-a-short", "normalizedName": "salewa"},
		// The ID comes from the key when the item has none.
		"b": map[string]string{"name": "item-b-name", "shortName": "item-b-short", "normalizedName": "bolts"},
	}}}
	resources := map[string]any{
		mode + "/items": items,
		mode + "/items_en": map[string]any{"data": map[string]string{
			"item-a-name": "Salewa first aid kit", "item-a-short": "Salewa",
			"item-b-name": "Bolts", "item-b-short": "Bolts",
		}},
	}
	if withJapanese {
		resources[mode+"/items_ja"] = map[string]any{"data": map[string]string{
			"item-a-name": "Salewa 救急キット", "item-a-short": "Salewa",
			// The same as English: not an alias.
			"item-b-name": "Bolts",
		}}
	}
	return &fakeSource{resources: resources}
}

func TestItemsForModeNamesAndAliases(t *testing.T) {
	c := NewWithSource(catalog("regular", true))
	items, err := c.ItemsForMode(context.Background(), "auto")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) != 2 || items[0].ID != "a" || items[1].ID != "b" {
		t.Fatalf("items = %+v", items)
	}
	a, b := items[0], items[1]
	if a.Name != "Salewa first aid kit" || a.ShortName != "Salewa" || a.NormalizedName != "salewa" {
		t.Errorf("a = %+v", a)
	}
	if len(a.Aliases) != 1 || a.Aliases[0] != "Salewa 救急キット" || a.Names["ja"] != "Salewa 救急キット" {
		t.Errorf("a aliases = %v, names = %v", a.Aliases, a.Names)
	}
	if len(a.ShortAliases) != 0 {
		t.Errorf("a short aliases = %v, want none (same as English)", a.ShortAliases)
	}
	if len(b.Aliases) != 0 || b.Names != nil {
		t.Errorf("b aliases = %v, names = %v, want none", b.Aliases, b.Names)
	}
}

func TestItemsForModeWithoutTranslations(t *testing.T) {
	items, err := NewWithSource(catalog("pve", false)).ItemsForMode(context.Background(), "pve")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.Name == "" || len(item.Aliases) != 0 {
			t.Errorf("item = %+v", item)
		}
	}
}

func TestItemsForModeCachesUntilInvalidated(t *testing.T) {
	source := catalog("regular", true)
	c := NewWithSource(source)
	ctx := context.Background()
	first, err := c.ItemsForMode(ctx, "regular")
	if err != nil {
		t.Fatal(err)
	}
	requests := source.requests
	// A caller changing its copy does not change the cache.
	first[0].Name = "changed"
	again, err := c.ItemsForMode(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if source.requests != requests {
		t.Errorf("requests = %d, want %d (cached)", source.requests, requests)
	}
	for _, item := range again {
		if item.Name == "changed" {
			t.Error("the cache shares its slice with callers")
		}
	}
	c.Invalidate()
	if _, err := c.ItemsForMode(ctx, "regular"); err != nil {
		t.Fatal(err)
	}
	if source.requests == requests {
		t.Error("Invalidate did not reload the catalog")
	}
}

func TestItemsForModeErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := NewWithSource(catalog("regular", false)).ItemsForMode(ctx, "arena"); err == nil {
		t.Error("an unsupported mode loaded")
	}
	empty := &fakeSource{resources: map[string]any{
		"regular/items":    map[string]any{"data": map[string]any{"items": map[string]any{}}},
		"regular/items_en": map[string]any{"data": map[string]string{}},
	}}
	if _, err := NewWithSource(empty).ItemsForMode(ctx, "regular"); err == nil {
		t.Error("an empty catalog loaded")
	}
	if _, err := NewWithSource(&fakeSource{}).ItemsForMode(ctx, "regular"); err == nil {
		t.Error("a failed request loaded")
	}
}
