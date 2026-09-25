package itemmatch

import "testing"

func TestMatchOCRNoise(t *testing.T) {
	items := []Item{{ID: "mask", Name: "Death Shadow mask", ShortName: "Death Shadow"}, {ID: "helmet", Name: "ULACH helmet", ShortName: "ULACH"}}
	results := Match("I Death Shad0w mask", items)
	if len(results) == 0 || results[0].Item.ID != "mask" || results[0].Confidence < .9 {
		t.Fatalf("unexpected match: %+v", results)
	}
}

// A title read from a Japanese game matches the item by its Japanese name,
// and the result keeps the English name (and so the English pages).
func TestMatchJapaneseAlias(t *testing.T) {
	items := []Item{
		{ID: "gpu", Name: "Graphics card", ShortName: "GPU", Aliases: []string{"グラフィックボード"}},
		{ID: "bolts", Name: "Bolts", ShortName: "Bolts", Aliases: []string{"ボルト"}},
	}
	results := Match("グラフィック ボード", items)
	if len(results) == 0 || results[0].Item.ID != "gpu" || results[0].Confidence < .9 || results[0].Item.Name != "Graphics card" {
		t.Fatalf("unexpected match: %+v", results)
	}
	if results := Match("ボルト", items); results[0].Item.ID != "bolts" {
		t.Fatalf("unexpected match: %+v", results)
	}
}

// Scraps read from the icon before a title do not lower the match.
func TestMatchLeadingScraps(t *testing.T) {
	items := []Item{{ID: "battery", Name: "GreenBat lithium battery", Aliases: []string{"GreenBat リチウムイオン電池"}}}
	if results := Match(". だ ん GreenBat リチウム イオ ン 電 池", items); results[0].Confidence < .98 {
		t.Fatalf("unexpected match: %+v", results)
	}
}

// A title with something added after the item's name (a weapon's own name)
// matches the item, not a preset whose suffix is as long as the addition.
func TestMatchAddedSuffix(t *testing.T) {
	items := []Item{
		{ID: "mp7", Name: "HK MP7A2 4.6x30 submachine gun", ShortName: "MP7A2"},
		{ID: "mp7-t1", Name: "HK MP7A2 4.6x30 submachine gun T-1", ShortName: "MP7A2 T-1"},
	}
	if results := Match("HK MP7A2 4.6x30 submachine gun _MP7", items); results[0].Item.ID != "mp7" || results[0].Confidence < .9 {
		t.Fatalf("unexpected match: %+v", results[:2])
	}
	if results := Match("HK MP7A2 4.6x30 submachine gun T-1", items); results[0].Item.ID != "mp7-t1" {
		t.Fatalf("the preset itself: %+v", results[:2])
	}
}
