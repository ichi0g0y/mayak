package app

import (
	"encoding/json"
	"testing"

	"github.com/local/mayak/internal/catalog"
)

func TestLevelSteps(t *testing.T) {
	snapshot := &catalog.Snapshot{Resources: map[string]json.RawMessage{
		"items": json.RawMessage(`{"data":{"playerLevels":[
			{"level":2,"exp":1000,"levelBadgeImageLink":"b1"},
			{"level":1,"exp":0,"levelBadgeImageLink":"b1"},
			{"level":3,"exp":2000,"levelBadgeImageLink":"b2"}],
			"fleaMarket":{"minPlayerLevel":3}}}`),
		"traders": json.RawMessage(`{"data":{
			"p":{"name":"p Nickname","imageLink":"p.webp","levels":[{"level":1,"requiredPlayerLevel":0},{"level":2,"requiredPlayerLevel":3}]},
			"a":{"name":"a Nickname","levels":[{"level":2,"requiredPlayerLevel":3}]},
			"x":{"name":"x Nickname","levels":[{"level":2,"requiredPlayerLevel":3}]}}}`),
		"traders_en": json.RawMessage(`{"data":{"p Nickname":"Prapor","a Nickname":"Ragman"}}`),
	}}
	steps, err := levelSteps(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].Level != 1 || steps[1].Total != 1000 || steps[2].Total != 3000 || steps[2].Badge != "b2" {
		t.Fatalf("steps = %+v", steps)
	}
	// Level 3: a new badge, the flea market, then the traders by name (one
	// with no name left out).
	u := steps[2].Unlocks
	if len(u) != 4 || u[0].Kind != "badge" || u[0].Image != "b2" || u[1].Kind != "flea" ||
		u[2] != (LevelUnlock{Kind: "trader", Name: "Prapor", Loyalty: 2, Image: "p.webp"}) || u[3].Name != "Ragman" {
		t.Fatalf("level 3 unlocks = %+v", u)
	}
	if len(steps[1].Unlocks) != 0 {
		t.Fatalf("level 2 unlocks = %+v", steps[1].Unlocks)
	}
}
