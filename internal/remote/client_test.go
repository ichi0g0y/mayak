package remote

import (
	"encoding/json"
	"testing"

	"github.com/local/mayak/internal/model"
)

func TestPositionPayload(t *testing.T) {
	b, err := PositionPayload("abc", "customs", model.Position{X: 1, Y: 2, Z: 3, Rotation: 45})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["sessionID"] != "abc" {
		t.Fatalf("payload=%s", b)
	}
	data := got["data"].(map[string]any)
	if data["type"] != "playerPosition" || data["map"] != "customs" {
		t.Fatalf("payload=%s", b)
	}
}

func TestPositionMessagesCanSkipMapNavigation(t *testing.T) {
	position := model.Position{X: 1, Y: 2, Z: 3, Rotation: 45}
	withoutMap, err := PositionMessages("abc", "customs", position, false)
	if err != nil || len(withoutMap) != 1 {
		t.Fatalf("without map: count=%d err=%v", len(withoutMap), err)
	}
	withMap, err := PositionMessages("abc", "customs", position, true)
	if err != nil || len(withMap) != 2 {
		t.Fatalf("with map: count=%d err=%v", len(withMap), err)
	}
	var second envelope
	if err := json.Unmarshal(withMap[1], &second); err != nil {
		t.Fatal(err)
	}
	data, ok := second.Data.(map[string]any)
	if !ok || data["type"] != "map" {
		t.Fatalf("second message should navigate map: %s", withMap[1])
	}
}

func TestTaskPayload(t *testing.T) {
	b, err := TaskPayload("P0HZ", "capturing-outposts")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	data := got["data"].(map[string]any)
	if got["sessionID"] != "P0HZ" || data["type"] != "task" || data["value"] != "capturing-outposts" {
		t.Fatalf("payload=%s", b)
	}
}

func TestGroundZero21UsesStableRemoteMapName(t *testing.T) {
	mapMessage, err := MapPayload("abc", "ground-zero-21")
	if err != nil {
		t.Fatal(err)
	}
	var mapEnvelope envelope
	if err := json.Unmarshal(mapMessage, &mapEnvelope); err != nil {
		t.Fatal(err)
	}
	mapData := mapEnvelope.Data.(map[string]any)
	if mapData["value"] != "ground-zero" {
		t.Fatalf("map payload=%s", mapMessage)
	}

	positionMessage, err := PositionPayload("abc", "ground-zero-21", model.Position{X: 1, Y: 2, Z: 3})
	if err != nil {
		t.Fatal(err)
	}
	var positionEnvelope envelope
	if err := json.Unmarshal(positionMessage, &positionEnvelope); err != nil {
		t.Fatal(err)
	}
	positionData := positionEnvelope.Data.(map[string]any)
	if positionData["map"] != "ground-zero" {
		t.Fatalf("position payload=%s", positionMessage)
	}
}
