package app

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/local/mayak/internal/catalog"
)

// The player's levels, for the sidebar's level ladder (view-levels.js): what
// each level takes and what reaching it unlocks, from the catalog of the mode
// played. The experience the player has comes with the Overall screen
// (model.PlayerProfile).

// LevelStep is one level: the experience it takes from the level before
// (Exp) and from the start (Total), its badge's picture (tarkov.dev's), and
// what reaching it unlocks.
type LevelStep struct {
	Level   int           `json:"level"`
	Exp     int           `json:"exp"`
	Total   int           `json:"total"`
	Badge   string        `json:"badge,omitempty"`
	Unlocks []LevelUnlock `json:"unlocks,omitempty"`
}

// LevelUnlock is what a level unlocks: a trader's loyalty level ("trader",
// Name and Loyalty, Image its picture), the flea market ("flea"), or a new
// level badge ("badge", Image the badge).
type LevelUnlock struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Loyalty int    `json:"loyalty,omitempty"`
	Image   string `json:"image,omitempty"`
}

// BrowserPlayerLevels is the level ladder of the mode played.
func (a *App) BrowserPlayerLevels() ([]LevelStep, error) {
	a.mu.RLock()
	mode := a.effectiveCatalogMode(a.settings.GameMode)
	a.mu.RUnlock()
	parent := a.parentContext()
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	snapshot, err := a.catalogClient.Refresh(ctx, mode, false)
	if snapshot == nil {
		if err == nil {
			err = errors.New("catalog is not available")
		}
		return nil, err
	}
	return levelSteps(snapshot)
}

// levelSteps reads the ladder from a catalog: items' playerLevels and
// fleaMarket, the traders' loyalty levels and their names in English.
func levelSteps(snapshot *catalog.Snapshot) ([]LevelStep, error) {
	var items struct {
		Data struct {
			PlayerLevels []struct {
				Level int    `json:"level"`
				Exp   int    `json:"exp"`
				Badge string `json:"levelBadgeImageLink"`
			} `json:"playerLevels"`
			FleaMarket struct {
				MinPlayerLevel int `json:"minPlayerLevel"`
			} `json:"fleaMarket"`
		} `json:"data"`
	}
	if err := json.Unmarshal(snapshot.Resources["items"], &items); err != nil {
		return nil, err
	}
	if len(items.Data.PlayerLevels) == 0 {
		return nil, errors.New("catalog has no player levels")
	}
	var traders struct {
		Data map[string]struct {
			Name      string `json:"name"`
			ImageLink string `json:"imageLink"`
			Levels    []struct {
				Level               int `json:"level"`
				RequiredPlayerLevel int `json:"requiredPlayerLevel"`
			} `json:"levels"`
		} `json:"data"`
	}
	var names struct {
		Data map[string]string `json:"data"`
	}
	_ = json.Unmarshal(snapshot.Resources["traders"], &traders)
	_ = json.Unmarshal(snapshot.Resources["traders_en"], &names)
	levels := items.Data.PlayerLevels
	sort.Slice(levels, func(i, j int) bool { return levels[i].Level < levels[j].Level })
	steps := make([]LevelStep, 0, len(levels))
	byLevel := map[int]int{}
	total, badge := 0, ""
	for i, l := range levels {
		total += l.Exp
		step := LevelStep{Level: l.Level, Exp: l.Exp, Total: total, Badge: l.Badge}
		if i > 0 && l.Badge != "" && l.Badge != badge {
			step.Unlocks = append(step.Unlocks, LevelUnlock{Kind: "badge", Image: l.Badge})
		}
		badge = l.Badge
		byLevel[l.Level] = len(steps)
		steps = append(steps, step)
	}
	if at, ok := byLevel[items.Data.FleaMarket.MinPlayerLevel]; ok && items.Data.FleaMarket.MinPlayerLevel > 1 {
		steps[at].Unlocks = append(steps[at].Unlocks, LevelUnlock{Kind: "flea"})
	}
	var unlocks []struct {
		level int
		u     LevelUnlock
	}
	for _, t := range traders.Data {
		name := strings.TrimSpace(names.Data[t.Name])
		if name == "" {
			continue
		}
		for _, l := range t.Levels {
			if l.Level > 1 && l.RequiredPlayerLevel > 1 {
				unlocks = append(unlocks, struct {
					level int
					u     LevelUnlock
				}{l.RequiredPlayerLevel, LevelUnlock{Kind: "trader", Name: name, Loyalty: l.Level, Image: t.ImageLink}})
			}
		}
	}
	// The traders by name at each level, the same every time.
	sort.Slice(unlocks, func(i, j int) bool {
		if unlocks[i].level != unlocks[j].level {
			return unlocks[i].level < unlocks[j].level
		}
		return unlocks[i].u.Name < unlocks[j].u.Name
	})
	for _, u := range unlocks {
		if at, ok := byLevel[u.level]; ok {
			steps[at].Unlocks = append(steps[at].Unlocks, u.u)
		}
	}
	return steps, nil
}
