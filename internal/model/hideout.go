package model

import (
	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/hideoutlog"
)

type HideoutStatus struct {
	State     string                   `json:"state"`
	Mode      string                   `json:"mode"`
	AccountID string                   `json:"accountId"`
	ProfileID string                   `json:"profileId"`
	Completed int                      `json:"completed"`
	Total     int                      `json:"total"`
	Stations  []HideoutStationProgress `json:"stations"`
	Events    []hideoutlog.Event       `json:"events"`
	LastError string                   `json:"lastError"`
}
type HideoutStationProgress struct {
	catalog.HideoutStation
	CurrentLevel    int                   `json:"currentLevel"`
	CompletedLevels []int                 `json:"completedLevels"`
	NextLevel       *catalog.HideoutLevel `json:"nextLevel,omitempty"`
}

// Missing/non-contiguous completions never imply that earlier levels are complete.
func HideoutProgress(stations []catalog.HideoutStation, complete map[string]bool) HideoutStatus {
	result := HideoutStatus{State: "ready", Stations: []HideoutStationProgress{}}
	for _, station := range stations {
		p := HideoutStationProgress{HideoutStation: station, CompletedLevels: []int{}}
		gap := false
		for _, level := range station.Levels {
			result.Total++
			if complete[level.ID] {
				result.Completed++
				p.CompletedLevels = append(p.CompletedLevels, level.Level)
				if !gap {
					p.CurrentLevel = level.Level
				}
			} else {
				if !gap {
					next := level
					p.NextLevel = &next
				}
				gap = true
			}
		}
		result.Stations = append(result.Stations, p)
	}
	return result
}
