package itemmatch

import (
	"sort"

	"github.com/local/mayak/internal/questmatch"
)

type Item struct {
	ID             string
	Name           string
	ShortName      string
	NormalizedName string
	Link           string
	IconLink       string
	// Aliases and ShortAliases are the names in other languages (Japanese),
	// so a title read from a game in that language matches the same item.
	Aliases      []string
	ShortAliases []string
	// Names are those names by language (see internal/locale), for display.
	Names map[string]string
}

type Result struct {
	Item       Item
	Confidence float64
}

func Match(raw string, items []Item) []Result {
	if questmatch.Normalize(raw) == "" {
		return nil
	}
	results := make([]Result, 0, len(items))
	for _, item := range items {
		score := questmatch.Similarity(raw, item.Name)
		for _, alias := range item.Aliases {
			score = max(score, questmatch.Similarity(raw, alias))
		}
		for _, short := range append([]string{item.ShortName}, item.ShortAliases...) {
			if short != "" {
				score = max(score, questmatch.Similarity(raw, short)*.97)
			}
		}
		results = append(results, Result{Item: item, Confidence: score})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence == results[j].Confidence {
			return results[i].Item.Name < results[j].Item.Name
		}
		return results[i].Confidence > results[j].Confidence
	})
	return results
}
