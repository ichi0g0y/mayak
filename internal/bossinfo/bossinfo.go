// Package bossinfo reads the bosses of each map and the Goons sightings from
// json.tarkov.dev: the spawn chances, places and escorts the game has now
// (events change them), and where players last reported the Goons.
package bossinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://json.tarkov.dev/"

// The maps data is large (about 0.8 MB compressed): it is asked again at most
// this often, and then only if it changed (its ETag).
const (
	mapsTTL  = 4 * time.Minute
	namesTTL = 6 * time.Hour
	maxBody  = 64 << 20
	goonMax  = 20
)

type Location struct {
	Name   string  `json:"name"`
	Chance float64 `json:"chance"`
}

// Escort is a follower with the counts it comes in, e.g. 0–3.
type Escort struct {
	Name string `json:"name"`
	Min  int    `json:"min"`
	Max  int    `json:"max"`
}

type Boss struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Portrait  string     `json:"portrait"`
	Chance    float64    `json:"chance"`
	Locations []Location `json:"locations"`
	Escorts   []Escort   `json:"escorts"`
}

type Map struct {
	ID     string `json:"id"`
	Key    string `json:"key"`    // normalized name, as the map detection reports it
	NameID string `json:"nameId"` // the game's name, as Goons reports take it
	Name   string `json:"name"`
	Bosses []Boss `json:"bosses"`
}

// Goon is one player report of the Goons on a map.
type Goon struct {
	MapKey string    `json:"mapKey"`
	Map    string    `json:"map"`
	Time   time.Time `json:"time"`
}

type Info struct {
	Mode    string    `json:"mode"`
	Fetched time.Time `json:"fetched"`
	Maps    []Map     `json:"maps"`
	Goons   []Goon    `json:"goons"`
}

// Raw data, as far as it is used.
type rawSpawn struct {
	Mob            string  `json:"mob"`
	SpawnChance    float64 `json:"spawnChance"`
	SpawnLocations []struct {
		Name   string  `json:"name"`
		Chance float64 `json:"chance"`
	} `json:"spawnLocations"`
	Escorts []struct {
		Mob    string `json:"mob"`
		Amount []struct {
			Count  int     `json:"count"`
			Chance float64 `json:"chance"`
		} `json:"amount"`
	} `json:"escorts"`
}
type rawMaps struct {
	Data struct {
		Maps map[string]struct {
			ID             string     `json:"id"`
			Name           string     `json:"name"`
			NormalizedName string     `json:"normalizedName"`
			NameID         string     `json:"nameId"`
			Bosses         []rawSpawn `json:"bosses"`
		} `json:"maps"`
		GoonReports []struct {
			Map       string `json:"map"`
			Timestamp string `json:"timestamp"`
		} `json:"goonReports"`
		Mobs map[string]struct {
			ID                string `json:"id"`
			ImagePortraitLink string `json:"imagePortraitLink"`
		} `json:"mobs"`
	} `json:"data"`
}

type mapsEntry struct {
	etag    string
	data    *rawMaps
	checked time.Time
	fetched time.Time
}
type namesEntry struct {
	names   map[string]string
	checked time.Time
}

type Client struct {
	http *http.Client
	base string
	// reportURL replaces the Goons report address (tests).
	reportURL string
	mu        sync.Mutex
	fetch     sync.Mutex
	maps      map[string]mapsEntry
	names     map[string]namesEntry
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}, base: baseURL, maps: map[string]mapsEntry{}, names: map[string]namesEntry{}}
}

// Mode is the data set for a catalog mode: json.tarkov.dev has the regular
// game and PvE.
func Mode(mode string) string {
	if mode == "pve" {
		return "pve"
	}
	return "regular"
}

// Info returns the bosses and Goons reports of a game mode, with names in
// lang (English where there is no translation).
func (c *Client) Info(ctx context.Context, mode, lang string) (Info, error) {
	mode = Mode(mode)
	c.fetch.Lock()
	defer c.fetch.Unlock()
	entry, err := c.loadMaps(ctx, mode)
	if entry.data == nil {
		return Info{}, err
	}
	names := c.loadNames(ctx, mode, "en")
	if lang != "" && lang != "en" {
		local := c.loadNames(ctx, mode, lang)
		merged := make(map[string]string, len(names)+len(local))
		for k, v := range names {
			merged[k] = v
		}
		for k, v := range local {
			if v != "" {
				merged[k] = v
			}
		}
		names = merged
	}
	return build(entry.data, names, mode, entry.fetched), nil
}

func (c *Client) loadMaps(ctx context.Context, mode string) (mapsEntry, error) {
	c.mu.Lock()
	entry := c.maps[mode]
	c.mu.Unlock()
	if entry.data != nil && time.Since(entry.checked) < mapsTTL {
		return entry, nil
	}
	body, etag, err := c.get(ctx, mode+"/maps", entry.etag)
	now := time.Now()
	switch {
	case err != nil:
		return entry, err
	case body == nil: // not modified
		entry.checked = now
	default:
		var data rawMaps
		if err := json.Unmarshal(body, &data); err != nil {
			return entry, fmt.Errorf("boss data: %w", err)
		}
		entry = mapsEntry{etag: etag, data: &data, checked: now, fetched: now}
	}
	c.mu.Lock()
	c.maps[mode] = entry
	c.mu.Unlock()
	return entry, nil
}

func (c *Client) loadNames(ctx context.Context, mode, lang string) map[string]string {
	key := mode + "/" + lang
	c.mu.Lock()
	entry := c.names[key]
	c.mu.Unlock()
	if entry.names != nil && time.Since(entry.checked) < namesTTL {
		return entry.names
	}
	body, _, err := c.get(ctx, mode+"/maps_"+lang, "")
	if err == nil && body != nil {
		var raw struct {
			Data map[string]string `json:"data"`
		}
		if json.Unmarshal(body, &raw) == nil && raw.Data != nil {
			entry = namesEntry{names: raw.Data, checked: time.Now()}
			c.mu.Lock()
			c.names[key] = entry
			c.mu.Unlock()
		}
	}
	return entry.names
}

// get fetches a path; with an ETag it returns a nil body when unchanged.
func (c *Client) get(ctx context.Context, path, etag string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && etag != "" {
		return nil, etag, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxBody {
		return nil, "", errors.New(path + ": response too large")
	}
	return body, resp.Header.Get("ETag"), nil
}

// shown tells the bosses worth listing: the bosses, the Goons, cultists and
// raiders; not the AI PMCs and scav-like guards.
func shown(mob string) bool {
	return strings.HasPrefix(mob, "boss") || mob == "sectantPriest" || mob == "PmcBot"
}

func build(data *rawMaps, names map[string]string, mode string, fetched time.Time) Info {
	name := func(key string) string {
		if v := names[key]; v != "" {
			return v
		}
		return key
	}
	info := Info{Mode: mode, Fetched: fetched, Maps: []Map{}, Goons: []Goon{}}
	keys := map[string]Map{}
	for _, raw := range data.Data.Maps {
		m := Map{ID: raw.ID, Key: raw.NormalizedName, NameID: raw.NameID, Name: name(raw.Name), Bosses: []Boss{}}
		// A boss can have several spawn entries on a map: one row each, with
		// the highest chance and the places of all of them.
		index := map[string]int{}
		for _, spawn := range raw.Bosses {
			if !shown(spawn.Mob) || spawn.SpawnChance <= 0 {
				continue
			}
			i, ok := index[spawn.Mob]
			if !ok {
				i = len(m.Bosses)
				index[spawn.Mob] = i
				m.Bosses = append(m.Bosses, Boss{ID: spawn.Mob, Name: name(spawn.Mob), Portrait: data.Data.Mobs[spawn.Mob].ImagePortraitLink, Locations: []Location{}, Escorts: []Escort{}})
			}
			b := &m.Bosses[i]
			b.Chance = max(b.Chance, spawn.SpawnChance)
			for _, l := range spawn.SpawnLocations {
				b.Locations = addLocation(b.Locations, Location{Name: name(l.Name), Chance: l.Chance})
			}
			for _, e := range spawn.Escorts {
				if len(e.Amount) == 0 {
					continue
				}
				low, high := e.Amount[0].Count, e.Amount[0].Count
				for _, a := range e.Amount {
					low, high = min(low, a.Count), max(high, a.Count)
				}
				if high > 0 {
					b.Escorts = addEscort(b.Escorts, Escort{Name: name(e.Mob), Min: low, Max: high})
				}
			}
		}
		for i := range m.Bosses {
			sort.SliceStable(m.Bosses[i].Locations, func(a, b int) bool { return m.Bosses[i].Locations[a].Chance > m.Bosses[i].Locations[b].Chance })
		}
		sort.SliceStable(m.Bosses, func(a, b int) bool { return m.Bosses[a].Chance > m.Bosses[b].Chance })
		keys[raw.ID] = m
		info.Maps = append(info.Maps, m)
	}
	sort.Slice(info.Maps, func(a, b int) bool { return info.Maps[a].Name < info.Maps[b].Name })
	for _, r := range data.Data.GoonReports {
		ms, err := strconv.ParseInt(r.Timestamp, 10, 64)
		m, ok := keys[r.Map]
		if err != nil || !ok {
			continue
		}
		info.Goons = append(info.Goons, Goon{MapKey: m.Key, Map: m.Name, Time: time.UnixMilli(ms).UTC()})
	}
	sort.Slice(info.Goons, func(a, b int) bool { return info.Goons[a].Time.After(info.Goons[b].Time) })
	if len(info.Goons) > goonMax {
		info.Goons = info.Goons[:goonMax]
	}
	return info
}

func addLocation(list []Location, l Location) []Location {
	for i := range list {
		if list[i].Name == l.Name {
			list[i].Chance = max(list[i].Chance, l.Chance)
			return list
		}
	}
	return append(list, l)
}

func addEscort(list []Escort, e Escort) []Escort {
	for i := range list {
		if list[i].Name == e.Name {
			list[i].Min, list[i].Max = min(list[i].Min, e.Min), max(list[i].Max, e.Max)
			return list
		}
	}
	return append(list, e)
}
