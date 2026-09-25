// Package iteminfo gathers what the item sidebar shows for one item: flea
// market and trader prices, and the tasks and hideout upgrades that need it.
// Prices come live from the tarkov.dev GraphQL API when it answers, and from
// the periodically refreshed catalog snapshot otherwise.
package iteminfo

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/locale"
)

type Info struct {
	ID        string `json:"id"`
	Mode      string `json:"mode"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	// Names and ShortNames are the names in other languages (see
	// internal/locale), when they differ; the sidebar shows the one in its
	// language.
	Names      map[string]string `json:"names,omitempty"`
	ShortNames map[string]string `json:"shortNames,omitempty"`
	IconURL    string            `json:"iconUrl"`
	Link       string            `json:"link"`
	WikiLink   string            `json:"wikiLink"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	BasePrice  int               `json:"basePrice"`
	// Flea is nil when the item cannot be sold on the flea market.
	Flea    *Flea         `json:"flea"`
	Traders []TraderPrice `json:"traders"`
	Tasks   []TaskNeed    `json:"tasks"`
	Hideout []HideoutNeed `json:"hideout"`
	// Live is true when the prices came from the GraphQL API just now; the
	// catalog snapshot is at most catalog.RefreshInterval old otherwise.
	Live bool `json:"live"`
	// QuestSite is the Host's preferred site for opening tasks.
	QuestSite string `json:"questSite,omitempty"`
	PricedAt  string `json:"pricedAt"`
	FetchedAt string `json:"fetchedAt"`
}

type Flea struct {
	LastLow       int     `json:"lastLow"`
	Avg24h        int     `json:"avg24h"`
	Low24h        int     `json:"low24h"`
	High24h       int     `json:"high24h"`
	ChangePercent float64 `json:"changePercent"`
	Offers        int     `json:"offers"`
	MinLevel      int     `json:"minLevel"`
}

type TraderPrice struct {
	Trader   string `json:"trader"`
	Price    int    `json:"price"`
	Currency string `json:"currency"`
	PriceRUB int    `json:"priceRub"`
}

type TaskNeed struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Names are the task's names in other languages, when they differ.
	Names       map[string]string `json:"names,omitempty"`
	Trader      string            `json:"trader"`
	Count       int               `json:"count"`
	FoundInRaid bool              `json:"foundInRaid"`
	// State is the TarkovTracker state ("completed", "failed",
	// "uncompleted") or empty when progress is unknown.
	State string `json:"state"`
	// URLs is the task's page per quest site (tarkov-dev, official-wiki,
	// japanese-wiki), for opening it from the item sidebar.
	URLs map[string]string `json:"urls,omitempty"`
	// NormalizedName and WikiLink are what the URLs are made from.
	NormalizedName string `json:"-"`
	WikiLink       string `json:"-"`
}

type HideoutNeed struct {
	LevelID     string `json:"levelId"`
	Station     string `json:"station"`
	Level       int    `json:"level"`
	Count       int    `json:"count"`
	FoundInRaid bool   `json:"foundInRaid"`
	// Complete is nil when hideout progress is unknown.
	Complete *bool `json:"complete"`
}

// Source provides catalog snapshots; *catalog.Client implements it.
type Source interface {
	Refresh(ctx context.Context, mode string, force bool) (*catalog.Snapshot, error)
}

type Service struct {
	source Source
	live   *liveClient
	mu     sync.Mutex
	// One index per mode, rebuilt when the catalog snapshot changes.
	indexes    map[string]*index
	history    map[string]cachedHistory
	historyURL string
}

func New(source Source) *Service {
	return &Service{source: source, live: newLiveClient(), indexes: make(map[string]*index), history: make(map[string]cachedHistory), historyURL: historyURL}
}

// Mode maps MAYAK' catalog modes; "auto" (unknown) uses the regular game.
func Mode(mode string) string {
	if catalog.ValidMode(mode) {
		return mode
	}
	return "regular"
}

// Catalog returns the item as the catalog snapshot describes it.
func (s *Service) Catalog(ctx context.Context, mode, id string) (Info, error) {
	mode = Mode(mode)
	ix, err := s.index(ctx, mode)
	if err != nil {
		return Info{}, err
	}
	info, ok := ix.info(id)
	if !ok {
		return Info{}, errors.New("item is not in the catalog")
	}
	info.Mode = mode
	info.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return info, nil
}

// Live replaces the catalog prices of info with current ones from the
// GraphQL API. On failure info is returned unchanged with the error.
func (s *Service) Live(ctx context.Context, info Info) (Info, error) {
	prices, err := s.live.prices(ctx, info.Mode, info.ID)
	if err != nil {
		return info, err
	}
	if info.Flea != nil || prices.flea != nil {
		minLevel := 0
		if info.Flea != nil {
			minLevel = info.Flea.MinLevel
		}
		info.Flea = prices.flea
		if info.Flea != nil {
			info.Flea.MinLevel = minLevel
		}
	}
	if len(prices.traders) > 0 {
		info.Traders = prices.traders
	}
	if prices.updated != "" {
		info.PricedAt = prices.updated
	}
	info.Live = true
	info.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return info, nil
}

// staleAfter is how old catalog prices may be when the GraphQL API cannot
// answer; json.tarkov.dev republishes prices every few minutes.
const staleAfter = 5 * time.Minute

// Current returns the freshest prices available: live ones, or else the
// catalog, refreshed first when it is older than staleAfter. The error
// explains why live prices were not used.
func (s *Service) Current(ctx context.Context, mode, id string) (Info, error) {
	info, err := s.Catalog(ctx, mode, id)
	if err != nil {
		return Info{}, err
	}
	live, liveErr := s.Live(ctx, info)
	if liveErr == nil {
		return live, nil
	}
	if snapshot, _ := s.source.Refresh(ctx, info.Mode, false); snapshot != nil && time.Since(snapshot.UpdatedAt) > staleAfter {
		if _, err := s.source.Refresh(ctx, info.Mode, true); err == nil {
			if fresh, err := s.Catalog(ctx, info.Mode, id); err == nil {
				info = fresh
			}
		}
	}
	return info, liveErr
}

// Progress marks tasks and hideout levels with TarkovTracker progress. Nil
// maps mean the progress is unknown.
func Progress(info Info, tasks map[string]string, hideout map[string]bool) Info {
	info.Tasks = append([]TaskNeed(nil), info.Tasks...)
	for i := range info.Tasks {
		info.Tasks[i].State = ""
		if tasks != nil {
			info.Tasks[i].State = tasks[info.Tasks[i].ID]
			if info.Tasks[i].State == "" {
				info.Tasks[i].State = "uncompleted"
			}
		}
	}
	info.Hideout = append([]HideoutNeed(nil), info.Hideout...)
	for i := range info.Hideout {
		info.Hideout[i].Complete = nil
		if hideout != nil {
			complete := hideout[info.Hideout[i].LevelID]
			info.Hideout[i].Complete = &complete
		}
	}
	return info
}

func (s *Service) index(ctx context.Context, mode string) (*index, error) {
	snapshot, err := s.source.Refresh(ctx, mode, false)
	if snapshot == nil {
		if err == nil {
			err = errors.New("catalog is not available")
		}
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ix := s.indexes[mode]; ix != nil && ix.snapshot == snapshot {
		return ix, nil
	}
	ix, err := build(snapshot)
	if err != nil {
		return nil, err
	}
	s.indexes[mode] = ix
	return ix, nil
}

type rawItem struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ShortName       string  `json:"shortName"`
	Link            string  `json:"link"`
	WikiLink        string  `json:"wikiLink"`
	IconLink        string  `json:"iconLink"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	BasePrice       int     `json:"basePrice"`
	LastLowPrice    *int    `json:"lastLowPrice"`
	Avg24hPrice     *int    `json:"avg24hPrice"`
	Low24hPrice     *int    `json:"low24hPrice"`
	High24hPrice    *int    `json:"high24hPrice"`
	ChangePercent   float64 `json:"changeLast48hPercent"`
	LastOfferCount  int     `json:"lastOfferCount"`
	MinLevelForFlea int     `json:"minLevelForFlea"`
	Updated         string  `json:"updated"`
	LastScan        string  `json:"lastScan"`
	SellToTrader    []struct {
		Trader   string `json:"trader"`
		Price    int    `json:"price"`
		PriceRUB int    `json:"priceRUB"`
		Currency string `json:"currency"`
	} `json:"sellToTrader"`
}

type index struct {
	snapshot *catalog.Snapshot
	items    map[string]rawItem
	text     map[string]string
	// languages are the names in other languages: by language, then by
	// resource (items, tasks; kept apart so their keys cannot collide), then key.
	languages map[string]map[string]map[string]string
	traders   map[string]string
	tasks     map[string][]TaskNeed
	hideout   map[string][]HideoutNeed
}

func build(snapshot *catalog.Snapshot) (*index, error) {
	decode := func(name string, target any) error {
		raw := snapshot.Resources[name]
		if len(raw) == 0 {
			return errors.New("catalog is missing " + name)
		}
		return json.Unmarshal(raw, target)
	}
	var items struct {
		Data struct {
			Items map[string]rawItem `json:"items"`
		} `json:"data"`
	}
	var text, traderText, taskText, hideoutText struct {
		Data map[string]string `json:"data"`
	}
	var traders struct {
		Data map[string]struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	var tasks struct {
		Data struct {
			Tasks map[string]struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				NormalizedName string `json:"normalizedName"`
				WikiLink       string `json:"wikiLink"`
				Trader         string `json:"trader"`
				Objectives     []struct {
					Type        string   `json:"type"`
					Items       []string `json:"items"`
					Count       int      `json:"count"`
					FoundInRaid bool     `json:"foundInRaid"`
				} `json:"objectives"`
			} `json:"tasks"`
		} `json:"data"`
	}
	var stations struct {
		Data map[string]catalog.HideoutStation `json:"data"`
	}
	for name, target := range map[string]any{"items": &items, "items_en": &text, "traders": &traders, "traders_en": &traderText, "tasks": &tasks, "tasks_en": &taskText, "hideout": &stations, "hideout_en": &hideoutText} {
		if err := decode(name, target); err != nil {
			return nil, err
		}
	}
	// Names in other languages; each is optional.
	languages := map[string]map[string]map[string]string{}
	for _, lang := range locale.Languages {
		languages[lang] = map[string]map[string]string{}
		for _, base := range []string{"items", "tasks"} {
			var resource struct {
				Data map[string]string `json:"data"`
			}
			if decode(locale.Resource(base, lang), &resource) == nil {
				languages[lang][base] = resource.Data
			}
		}
	}
	localized := func(dict map[string]string, key string) string {
		if v := strings.TrimSpace(dict[key]); v != "" {
			return v
		}
		return key
	}
	ix := &index{snapshot: snapshot, items: items.Data.Items, text: text.Data, languages: languages, traders: make(map[string]string), tasks: make(map[string][]TaskNeed), hideout: make(map[string][]HideoutNeed)}
	for id, trader := range traders.Data {
		ix.traders[id] = localized(traderText.Data, trader.Name)
	}
	for _, task := range tasks.Data.Tasks {
		// One row per task and item: "find in raid" and "hand over" objectives
		// for the same item are merged.
		needs := map[string]*TaskNeed{}
		for _, objective := range task.Objectives {
			switch objective.Type {
			case "giveItem", "findItem", "plantItem", "sellItem":
			default:
				continue
			}
			for _, item := range objective.Items {
				need := needs[item]
				if need == nil {
					need = &TaskNeed{ID: task.ID, Name: localized(taskText.Data, task.Name), Trader: ix.traders[task.Trader], NormalizedName: task.NormalizedName, WikiLink: task.WikiLink}
					need.Names = ix.names("tasks", task.Name, need.Name)
					needs[item] = need
				}
				need.Count = max(need.Count, objective.Count)
				need.FoundInRaid = need.FoundInRaid || objective.FoundInRaid
			}
		}
		for item, need := range needs {
			ix.tasks[item] = append(ix.tasks[item], *need)
		}
	}
	for _, station := range stations.Data {
		for _, level := range station.Levels {
			for _, r := range level.ItemRequirements {
				ix.hideout[r.Item] = append(ix.hideout[r.Item], HideoutNeed{LevelID: level.ID, Station: localized(hideoutText.Data, station.Name), Level: level.Level, Count: int(r.Count), FoundInRaid: r.Attributes.FoundInRaid})
			}
		}
	}
	for _, needs := range ix.tasks {
		sort.Slice(needs, func(i, j int) bool { return needs[i].Name < needs[j].Name })
	}
	for _, needs := range ix.hideout {
		sort.Slice(needs, func(i, j int) bool {
			if needs[i].Station != needs[j].Station {
				return needs[i].Station < needs[j].Station
			}
			return needs[i].Level < needs[j].Level
		})
	}
	return ix, nil
}

// names are key's names (from resource: items or tasks) in the other
// languages that differ from english.
func (ix *index) names(resource, key, english string) map[string]string {
	var out map[string]string
	for lang, resources := range ix.languages {
		if name := strings.TrimSpace(resources[resource][key]); name != "" && name != english {
			if out == nil {
				out = map[string]string{}
			}
			out[lang] = name
		}
	}
	return out
}

func (ix *index) info(id string) (Info, bool) {
	item, ok := ix.items[id]
	if !ok {
		return Info{}, false
	}
	info := Info{ID: id, Name: localize(ix.text, item.Name), ShortName: localize(ix.text, item.ShortName), IconURL: item.IconLink, Link: item.Link, WikiLink: item.WikiLink, Width: item.Width, Height: item.Height, BasePrice: item.BasePrice, PricedAt: item.Updated}
	info.Names = ix.names("items", item.Name, info.Name)
	info.ShortNames = ix.names("items", item.ShortName, info.ShortName)
	if info.PricedAt == "" {
		info.PricedAt = item.LastScan
	}
	if item.LastLowPrice != nil || item.Avg24hPrice != nil {
		info.Flea = &Flea{LastLow: deref(item.LastLowPrice), Avg24h: deref(item.Avg24hPrice), Low24h: deref(item.Low24hPrice), High24h: deref(item.High24hPrice), ChangePercent: item.ChangePercent, Offers: item.LastOfferCount, MinLevel: item.MinLevelForFlea}
	}
	for _, sale := range item.SellToTrader {
		info.Traders = append(info.Traders, TraderPrice{Trader: ix.traders[sale.Trader], Price: sale.Price, Currency: sale.Currency, PriceRUB: sale.PriceRUB})
	}
	sortTraders(info.Traders)
	info.Tasks = append([]TaskNeed(nil), ix.tasks[id]...)
	info.Hideout = append([]HideoutNeed(nil), ix.hideout[id]...)
	return info, true
}

func sortTraders(traders []TraderPrice) {
	sort.SliceStable(traders, func(i, j int) bool { return traders[i].PriceRUB > traders[j].PriceRUB })
}

func localize(dict map[string]string, key string) string {
	if v := strings.TrimSpace(dict[key]); v != "" {
		return v
	}
	return key
}

func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
