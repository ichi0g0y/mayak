package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/local/mayak/internal/iteminfo"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/model"
)

// showBrowserItem sends the recognized item to the browser's item sidebar:
// catalog data at once, then again with live prices when they arrive.
func (a *App) showBrowserItem(mode, id string) {
	if a.itemInfo == nil || id == "" {
		return
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	info, err := a.itemInfo.Catalog(ctx, mode, id)
	if err != nil {
		a.addLog("Warn", "Item", "Item details are unavailable: "+err.Error())
		return
	}
	a.emitEvent("browser:item", a.itemProgress(info))
	current, err := a.itemInfo.Current(ctx, mode, id)
	if err != nil {
		a.addLog("Debug", "Item", "Live prices are unavailable, using the catalog: "+err.Error())
	}
	if a.latestItemID() != id {
		return
	}
	a.emitEvent("browser:item", a.itemProgress(current))
}

// BrowserItemInfo returns current details of an item for the item sidebar's
// refresh button.
func (a *App) BrowserItemInfo(mode, id string) (iteminfo.Info, error) {
	if a.itemInfo == nil || id == "" {
		return iteminfo.Info{}, errors.New("item details are unavailable")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	info, err := a.itemInfo.Current(ctx, a.itemMode(mode), id)
	if info.ID == "" {
		return info, err
	}
	return a.itemProgress(info), nil
}

func (a *App) latestItemID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status.ItemID
}

// itemProgress marks the item's tasks and hideout levels with TarkovTracker
// progress when it belongs to the same game mode.
func (a *App) itemProgress(info iteminfo.Info) iteminfo.Info {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info = withTaskURLs(info, a.settings.QuestSite)
	if catalogMode(a.status.Tracker.Mode) != info.Mode || a.status.Tracker.Connection != "connected" {
		return iteminfo.Progress(info, nil, nil)
	}
	tasks := make(map[string]string, len(a.trackerTasks))
	for id, state := range a.trackerTasks {
		tasks[id] = state
	}
	var hideout map[string]bool
	if a.hideoutProgress != nil {
		hideout = make(map[string]bool, len(a.hideoutProgress))
		for id, complete := range a.hideoutProgress {
			hideout[id] = complete
		}
	}
	return iteminfo.Progress(info, tasks, hideout)
}

// BrowserItemHistory returns the item's flea market price history for the
// item sidebar's chart.
func (a *App) BrowserItemHistory(mode, id string) ([]iteminfo.PricePoint, error) {
	if a.itemInfo == nil {
		return nil, errors.New("item details are unavailable")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	return a.itemInfo.History(ctx, mode, id)
}

// withTaskURLs adds each task's page on every quest site, the same pages a
// recognized task opens.
func withTaskURLs(info iteminfo.Info, site string) iteminfo.Info {
	info.QuestSite = normalizeQuestSite(site)
	info.Tasks = append([]iteminfo.TaskNeed(nil), info.Tasks...)
	for i, task := range info.Tasks {
		status := model.Status{LastQuest: task.Name, QuestTrader: task.Trader, QuestWikiURL: task.WikiLink}
		if task.NormalizedName != "" {
			status.QuestURL = "https://tarkov.dev/task/" + task.NormalizedName
		}
		urls := map[string]string{}
		for _, s := range []string{"tarkov-dev", "official-wiki", "japanese-wiki"} {
			urls[s] = questStatusURL(s, status)
		}
		info.Tasks[i].URLs = urls
	}
	return info
}

// ItemSearchHit is one match of the item sidebar's search box.
type ItemSearchHit struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	// Names are the names in other languages, when they differ.
	Names   map[string]string `json:"names,omitempty"`
	IconURL string            `json:"iconUrl"`
}

// BrowserItemSearch finds catalog items by name or short name, so items can
// be looked up without a screenshot.
func (a *App) BrowserItemSearch(query string) ([]ItemSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 80 {
		return nil, nil
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	items, err := a.itemClient.ItemsForMode(ctx, a.itemMode(""))
	if err != nil {
		return nil, err
	}
	return searchItems(items, query, 20), nil
}

// itemMode is the catalog mode for item lookups: the given one, or the
// detected game mode.
func (a *App) itemMode(mode string) string {
	if mode != "" {
		return mode
	}
	a.mu.RLock()
	configured := a.settings.GameMode
	a.mu.RUnlock()
	return a.effectiveCatalogMode(configured)
}

// searchItems ranks exact names first, then names and short names starting
// with the query, then words starting with it, then (for queries of four or
// more characters) anything containing it; shorter names first within a rank.
// searchRank is how well name matches q: 0 exactly, 1 at its start, 2 at a
// word's start, 3 inside a word, or -1 not at all.
func searchRank(name, q string) int {
	switch {
	case name == q:
		return 0
	case strings.HasPrefix(name, q):
		return 1
	case strings.Contains(" "+name, " "+q):
		return 2
	// Inside a word only for longer queries: "gpu" should not find "Magpul".
	// Japanese has no spaces between words, so two characters are enough.
	case strings.Contains(name, q) && (len([]rune(q)) >= 4 || japaneseQuery(q) && len([]rune(q)) >= 2):
		return 3
	}
	return -1
}

func japaneseQuery(q string) bool {
	for _, r := range q {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

func searchItems(items []itemmatch.Item, query string, limit int) []ItemSearchHit {
	q := strings.ToLower(query)
	type ranked struct {
		item itemmatch.Item
		rank int
	}
	var found []ranked
	for _, item := range items {
		rank := -1
		// The English and the Japanese names count alike; the best one ranks.
		names := append([]string{item.Name, item.ShortName}, item.Aliases...)
		names = append(names, item.ShortAliases...)
		for _, n := range names {
			if r := searchRank(strings.ToLower(n), q); r >= 0 && (rank < 0 || r < rank) {
				rank = r
			}
		}
		if rank >= 0 {
			found = append(found, ranked{item, rank})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].rank != found[j].rank {
			return found[i].rank < found[j].rank
		}
		if len(found[i].item.Name) != len(found[j].item.Name) {
			return len(found[i].item.Name) < len(found[j].item.Name)
		}
		return found[i].item.Name < found[j].item.Name
	})
	hits := make([]ItemSearchHit, 0, min(limit, len(found)))
	for _, f := range found[:min(limit, len(found))] {
		hits = append(hits, ItemSearchHit{ID: f.item.ID, Name: f.item.Name, ShortName: f.item.ShortName, Names: f.item.Names, IconURL: f.item.IconLink})
	}
	return hits
}
