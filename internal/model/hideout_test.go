package model

import (
	"github.com/local/mayak/internal/catalog"
	"testing"
)

func TestHideoutProgressDoesNotInventCompletions(t *testing.T) {
	stations := []catalog.HideoutStation{{ID: "wall", Levels: []catalog.HideoutLevel{{ID: "wall-1", Level: 1}, {ID: "wall-2", Level: 2}, {ID: "wall-3", Level: 3}}}}
	result := HideoutProgress(stations, map[string]bool{"wall-1": true, "wall-3": true, "unrelated": true})
	station := result.Stations[0]
	if result.Completed != 2 || result.Total != 3 || station.CurrentLevel != 1 || station.NextLevel.Level != 2 {
		t.Fatalf("invented completion: %+v", result)
	}
	if result := HideoutProgress(stations, nil); result.Completed != 0 || result.Stations[0].CurrentLevel != 0 {
		t.Fatal("empty progress was treated as complete")
	}
}
