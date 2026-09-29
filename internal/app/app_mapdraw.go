package app

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/userdata"
)

// The lines drawn on the map with your own pen (map-draw.js): one record per
// line in map-drawings.json (internal/userdata Records), each with its map,
// floor, colour, width and points in game coordinates, so that they follow
// the user to another PC as the bookmarks do.

var mapDrawingMu sync.Mutex

// maxMapDrawingBytes bounds what the shell may hand over at once.
const maxMapDrawingBytes = 16 << 20

func mapDrawingPath() (string, error) { return appdir.Path("map-drawings.json") }

// MapDrawingLoad returns the lines kept, as a JSON array ("[]" for none).
func (a *App) MapDrawingLoad() (string, error) {
	mapDrawingMu.Lock()
	defer mapDrawingMu.Unlock()
	p, err := mapDrawingPath()
	if err != nil {
		return "[]", err
	}
	records, _, err := userdata.LoadRecords(p)
	list := []json.RawMessage{}
	for _, r := range records.Live() {
		list = append(list, r.Value)
	}
	out, _ := json.Marshal(list)
	return string(out), err
}

// MapDrawingSave keeps the lines given, all of them (a JSON array of
// objects with an id): a line no longer given becomes a tombstone.
func (a *App) MapDrawingSave(raw string) error {
	if len(raw) > maxMapDrawingBytes {
		return errors.New("map drawing too large")
	}
	var list []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return errors.New("invalid map drawing")
	}
	ids := make([]string, len(list))
	for i, item := range list {
		var line struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(item, &line)
		ids[i] = line.ID
	}
	mapDrawingMu.Lock()
	defer mapDrawingMu.Unlock()
	p, err := mapDrawingPath()
	if err != nil {
		return err
	}
	records, existed, _ := userdata.LoadRecords(p)
	if records.SetList(ids, list, time.Now()) || !existed {
		return userdata.SaveRecords(p, records)
	}
	return nil
}
