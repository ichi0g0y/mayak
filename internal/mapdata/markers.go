package mapdata

import (
	"errors"
	"sort"
	"strings"
)

// The map view's markers, built from the tarkov.dev catalog the way
// tarkov.dev's own map builds them (src/pages/map/index.jsx): the same
// layers in the same groups, the same icons and the same details in the
// marker's popup.

// MarkerLayer is one entry of the map's filters: a kind of marker. Group is the
// filters' section (Groups); Name is the entry's name where the data names
// it (a container, a boss, a handbook category), else empty and the shell
// names it by Key. Icon is a picture in the shell's map-icons, or IconURL
// one on tarkov.dev.
type MarkerLayer struct {
	Key     string `json:"key"`
	Group   string `json:"group"`
	Name    string `json:"name,omitempty"`
	Icon    string `json:"icon,omitempty"`
	IconURL string `json:"iconUrl,omitempty"`
}

// Groups are the filters' sections, in tarkov.dev's (English) order.
var Groups = []string{"extracts", "hazards", "landmarks", "loose", "containers", "spawns", "tasks", "usable"}

// Ref is an item (or anything else) a popup names, with its picture.
type Ref struct {
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
	Count int    `json:"count,omitempty"`
}

// Chance is a boss that may spawn somewhere, and how likely it is (0-1).
type Chance struct {
	Name   string  `json:"name"`
	Chance float64 `json:"chance"`
}

// Target is something a switch opens: another switch or an extract (with
// its faction).
type Target struct {
	Name    string `json:"name"`
	Faction string `json:"faction,omitempty"`
}

// Detail is what a marker's popup tells beyond its name.
type Detail struct {
	LockType    string   `json:"lockType,omitempty"`
	NeedsPower  bool     `json:"needsPower,omitempty"`
	Bosses      []Chance `json:"bosses,omitempty"`
	ActivatedBy []string `json:"activatedBy,omitempty"`
	Activates   []Target `json:"activates,omitempty"`
	Item        *Ref     `json:"item,omitempty"`
	Items       []Ref    `json:"items,omitempty"`
	Task        string   `json:"task,omitempty"`
	TaskID      string   `json:"taskId,omitempty"`
	Objective   string   `json:"objective,omitempty"`
	// Active is a task marker of a task being done (see Sources.TaskActive).
	Active bool `json:"active,omitempty"`
}

// Marker is one thing on the map, in game coordinates. Layer is its filter
// entry (Layers when it is in several: loose loot of several categories);
// Icon a picture in map-icons, or IconURL one on tarkov.dev (with IconSize
// in pixels); Name is shown on it (extracts) or on hover and searched;
// Outline, when there is one, is the area it covers ([x, z] points); Top and
// Bottom bound its height, for the floor it is on.
type Marker struct {
	Layer    string       `json:"layer"`
	Layers   []string     `json:"layers,omitempty"`
	Icon     string       `json:"icon,omitempty"`
	IconURL  string       `json:"iconUrl,omitempty"`
	IconSize [2]float64   `json:"iconSize,omitzero"`
	Name     string       `json:"name,omitempty"`
	ID       string       `json:"id,omitempty"`
	X        float64      `json:"x"`
	Y        float64      `json:"y"`
	Z        float64      `json:"z"`
	Top      float64      `json:"top,omitempty"`
	Bottom   float64      `json:"bottom,omitempty"`
	Outline  [][2]float64 `json:"outline,omitempty"`
	Detail   *Detail      `json:"detail,omitempty"`
}

// MapMarkers is everything the map view shows of one map from the
// catalog.
type MapMarkers struct {
	Layers       []MarkerLayer `json:"layers"`
	Markers      []Marker      `json:"markers"`
	RaidDuration int           `json:"raidDuration,omitempty"`
	Players      string        `json:"players,omitempty"`
	// Mode is the catalog's game mode the markers come from.
	Mode string `json:"mode,omitempty"`
}

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type area struct {
	ID       string  `json:"id"`
	Position point   `json:"position"`
	Outline  []point `json:"outline"`
	Top      float64 `json:"top"`
	Bottom   float64 `json:"bottom"`
}

type rawCatalogMap struct {
	ID             string `json:"id"`
	NormalizedName string `json:"normalizedName"`
	RaidDuration   int    `json:"raidDuration"`
	Players        string `json:"players"`
	Bosses         []struct {
		Mob            string  `json:"mob"`
		SpawnChance    float64 `json:"spawnChance"`
		SpawnLocations []struct {
			SpawnKey string `json:"spawnKey"`
		} `json:"spawnLocations"`
	} `json:"bosses"`
	Spawns []struct {
		Position   point    `json:"position"`
		Sides      []string `json:"sides"`
		Categories []string `json:"categories"`
		ZoneName   string   `json:"zoneName"`
	} `json:"spawns"`
	Extracts []struct {
		area
		Name         string   `json:"name"`
		Faction      string   `json:"faction"`
		Switches     []string `json:"switches"`
		TransferItem *struct {
			Item  string `json:"item"`
			Count int    `json:"count"`
		} `json:"transferItem"`
	} `json:"extracts"`
	Transits []struct {
		area
		Description string `json:"description"`
	} `json:"transits"`
	Locks []struct {
		LockType   string `json:"lockType"`
		Key        string `json:"key"`
		NeedsPower bool   `json:"needsPower"`
		Position   point  `json:"position"`
	} `json:"locks"`
	Hazards []struct {
		area
		HazardType string `json:"hazardType"`
		Name       string `json:"name"`
	} `json:"hazards"`
	LootContainers []struct {
		LootContainer string `json:"lootContainer"`
		Position      point  `json:"position"`
	} `json:"lootContainers"`
	LootLoose []struct {
		Items    []string `json:"items"`
		Position point    `json:"position"`
	} `json:"lootLoose"`
	Switches []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ActivatedBy any    `json:"activatedBy"`
		Activates   []struct {
			Switch  string `json:"switch"`
			Extract string `json:"extract"`
		} `json:"activates"`
		Position point `json:"position"`
	} `json:"switches"`
	StationaryWeapons []struct {
		StationaryWeapon string `json:"stationaryWeapon"`
		Position         point  `json:"position"`
	} `json:"stationaryWeapons"`
	BtrStops []struct {
		Name string  `json:"name"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Z    float64 `json:"z"`
	} `json:"btrStops"`
	Artillery struct {
		Zones []area `json:"zones"`
	} `json:"artillery"`
}

type catalogThing struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
}

// CatalogMaps is the catalog's "maps" resource (json.tarkov.dev), as far as
// the markers need it.
type CatalogMaps struct {
	Data struct {
		Maps              map[string]rawCatalogMap `json:"maps"`
		Mobs              map[string]catalogThing  `json:"mobs"`
		LootContainers    map[string]catalogThing  `json:"lootContainers"`
		StationaryWeapons map[string]catalogThing  `json:"stationaryWeapons"`
	} `json:"data"`
}

// CatalogItems is the catalog's "items" resource, as far as locks, loose
// loot and extract requirements need it.
type CatalogItems struct {
	Data struct {
		Items map[string]struct {
			Name               string   `json:"name"`
			BaseImageLink      string   `json:"baseImageLink"`
			IconLink           string   `json:"iconLink"`
			Width              int      `json:"width"`
			Height             int      `json:"height"`
			HandbookCategories []string `json:"handbookCategories"`
		} `json:"items"`
		HandbookCategories map[string]struct {
			Name           string `json:"name"`
			NormalizedName string `json:"normalizedName"`
			ImageLink      string `json:"imageLink"`
		} `json:"handbookCategories"`
	} `json:"data"`
}

// CatalogTasks is the catalog's "tasks" resource, as far as the task
// markers need it.
type CatalogTasks struct {
	Data struct {
		Tasks map[string]struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Objectives []struct {
				ID          string `json:"id"`
				Description string `json:"description"`
				QuestItem   string `json:"questItem"`
				Zones       []struct {
					area
					Map string `json:"map"`
				} `json:"zones"`
				PossibleLocations []struct {
					Map       string  `json:"map"`
					Positions []point `json:"positions"`
				} `json:"possibleLocations"`
			} `json:"objectives"`
		} `json:"tasks"`
		QuestItems map[string]struct {
			Name     string `json:"name"`
			IconLink string `json:"iconLink"`
		} `json:"questItems"`
	} `json:"data"`
}

// Names is a catalog translation resource ("maps_en", "items_ja"…): the
// text for each key the data names.
type Names struct {
	Data map[string]string `json:"data"`
}

func (n Names) text(key string) string { return n.Data[key] }

// Sources are the catalog resources the markers come from. MapNames
// translates the maps' own names (maps_en); ItemNames and TaskNames are in
// the language shown, with the English ones as a fallback; TaskActive tells
// a task being done (every task when nil).
type Sources struct {
	Maps                             CatalogMaps
	Items                            CatalogItems
	Tasks                            CatalogTasks
	MapNames, ItemNames, ItemNamesEn Names
	TaskNames, TaskNamesEn           Names
	TaskActive                       func(taskID string) bool
}

var errUnknownMap = errors.New("the catalog does not have this map")

// bossIcons are the bosses with a spawn icon of their own; the others share
// spawn_boss.
var bossIcons = map[string]bool{"cultist-priest": true, "rogue": true, "black-div": true, "af": true, "bloodhound": true}

// containerIcons maps a container (its normalizedName) to its icon, as
// tarkov.dev's map-images.mjs does; one not listed uses its own name.
var containerIcons = map[string]string{
	"bank-cash-register": "cash-register", "cash-register-tar2-2": "cash-register",
	"bank-safe": "safe", "dead-civilian": "dead-scav", "pmc-body": "dead-scav",
	"civilian-body": "dead-scav", "lab-technician-body": "dead-scav", "scav-body": "dead-scav",
	"medical-supply-crate": "crate", "ration-supply-crate": "crate", "technical-supply-crate": "crate",
	"shturmans-stash": "weapon-box",
}

// Markers returns the markers of the catalog map named any of names (a map
// key first, then its aliases).
func Markers(src Sources, names ...string) (MapMarkers, error) {
	var m *rawCatalogMap
	for _, name := range names {
		for id, candidate := range src.Maps.Data.Maps {
			if candidate.NormalizedName == name {
				c := candidate
				if c.ID == "" {
					c.ID = id
				}
				m = &c
				break
			}
		}
		if m != nil {
			break
		}
	}
	if m == nil {
		return MapMarkers{}, errUnknownMap
	}
	b := builder{src: src, layers: map[string]bool{}}
	b.extracts(m)
	b.hazards(m)
	b.landmarks(m)
	b.looseLoot(m)
	b.containers(m)
	b.spawns(m)
	b.tasks(m)
	b.usable(m)
	sort.SliceStable(b.out.Layers, func(i, j int) bool {
		return groupOrder(b.out.Layers[i].Group) < groupOrder(b.out.Layers[j].Group)
	})
	b.out.RaidDuration, b.out.Players = m.RaidDuration, m.Players
	return b.out, nil
}

type builder struct {
	src    Sources
	out    MapMarkers
	layers map[string]bool
}

func (b *builder) mapName(key string) string {
	if v := b.src.MapNames.text(key); v != "" {
		return v
	}
	return key
}

func (b *builder) itemName(id string) string {
	if v := b.src.ItemNames.text(id + " Name"); v != "" {
		return v
	}
	return b.src.ItemNamesEn.text(id + " Name")
}

func (b *builder) taskText(key string) string {
	if v := b.src.TaskNames.text(key); v != "" {
		return v
	}
	if v := b.src.TaskNamesEn.text(key); v != "" {
		return v
	}
	return key
}

func (b *builder) layer(l MarkerLayer) {
	if b.layers[l.Key] {
		return
	}
	b.layers[l.Key] = true
	b.out.Layers = append(b.out.Layers, l)
}

func (b *builder) add(m Marker) { b.out.Markers = append(b.out.Markers, m) }

func placed(m Marker, a area) Marker {
	m.X, m.Y, m.Z, m.Top, m.Bottom = a.Position.X, a.Position.Y, a.Position.Z, a.Top, a.Bottom
	for _, o := range a.Outline {
		m.Outline = append(m.Outline, [2]float64{o.X, o.Z})
	}
	return m
}

func at(m Marker, p point) Marker {
	m.X, m.Y, m.Z = p.X, p.Y, p.Z
	return m
}

func (b *builder) extracts(m *rawCatalogMap) {
	for _, f := range []string{"pmc", "scav", "shared"} {
		b.layer(MarkerLayer{Key: "extract_" + f, Group: "extracts", Icon: "extract_" + f})
	}
	b.layer(MarkerLayer{Key: "extract_transit", Group: "extracts", Icon: "extract_transit"})
	switchNames := map[string]string{}
	for _, s := range m.Switches {
		switchNames[s.ID] = b.mapName(s.Name)
	}
	for _, e := range m.Extracts {
		faction := e.Faction
		if faction != "pmc" && faction != "scav" {
			faction = "shared"
		}
		mk := placed(Marker{Layer: "extract_" + faction, Icon: "extract_" + faction, Name: b.mapName(e.Name), ID: e.ID}, e.area)
		d := &Detail{}
		for _, id := range e.Switches {
			if n := switchNames[id]; n != "" {
				d.ActivatedBy = append(d.ActivatedBy, n)
			}
		}
		if e.TransferItem != nil && e.TransferItem.Item != "" {
			it := b.src.Items.Data.Items[e.TransferItem.Item]
			d.Item = &Ref{Name: b.itemName(e.TransferItem.Item), Image: it.IconLink, Count: e.TransferItem.Count}
		}
		if len(d.ActivatedBy) > 0 || d.Item != nil {
			mk.Detail = d
		}
		b.add(mk)
	}
	for _, t := range m.Transits {
		b.add(placed(Marker{Layer: "extract_transit", Icon: "extract_transit", Name: b.mapName(t.Description), ID: t.ID}, t.area))
	}
}

func (b *builder) hazards(m *rawCatalogMap) {
	for _, h := range m.Hazards {
		key := "hazard_" + h.HazardType
		b.layer(MarkerLayer{Key: key, Group: "hazards", Name: b.mapName(h.Name), Icon: "hazard"})
		b.add(placed(Marker{Layer: key, Icon: "hazard", Name: b.mapName(h.Name)}, h.area))
	}
	for _, z := range m.Artillery.Zones {
		b.layer(MarkerLayer{Key: "hazard_mortar", Group: "hazards", Icon: "hazard_mortar"})
		b.add(placed(Marker{Layer: "hazard_mortar", Icon: "hazard_mortar", ID: z.ID}, z))
	}
}

func (b *builder) landmarks(m *rawCatalogMap) {
	// The place names come from maps.json (Map.Labels); the shell adds their
	// layer.
	for _, s := range m.BtrStops {
		b.layer(MarkerLayer{Key: "btr_stop", Group: "landmarks", Icon: "btr_stop"})
		b.add(Marker{Layer: "btr_stop", Icon: "btr_stop", Name: b.mapName(s.Name), X: s.X, Y: s.Y, Z: s.Z, Top: s.Y, Bottom: s.Y})
	}
}

func (b *builder) looseLoot(m *rawCatalogMap) {
	items := b.src.Items.Data.Items
	cats := b.src.Items.Data.HandbookCategories
	for _, l := range m.LootLoose {
		var refs []Ref
		var layers []string
		seen := map[string]bool{}
		for _, id := range l.Items {
			it, ok := items[id]
			if !ok {
				continue
			}
			refs = append(refs, Ref{Name: b.itemName(id), Image: it.BaseImageLink})
			if len(it.HandbookCategories) == 0 {
				continue
			}
			cat, ok := cats[it.HandbookCategories[0]]
			if !ok || seen[cat.NormalizedName] {
				continue
			}
			seen[cat.NormalizedName] = true
			key := "loose_" + cat.NormalizedName
			name := b.src.ItemNames.text(cat.Name)
			if name == "" {
				name = b.src.ItemNamesEn.text(cat.Name)
			}
			b.layer(MarkerLayer{Key: key, Group: "loose", Name: name, IconURL: cat.ImageLink})
			layers = append(layers, key)
		}
		if len(refs) == 0 || len(layers) == 0 {
			continue
		}
		mk := at(Marker{Layer: layers[0], Icon: "loose_loot", Name: refs[0].Name, Detail: &Detail{Items: refs}}, l.Position)
		if len(layers) > 1 {
			mk.Layers = layers
		}
		if len(refs) == 1 {
			// One item: its own picture, its longer side 24 px (a grid
			// cell is 63 px).
			it := items[l.Items[0]]
			for _, id := range l.Items {
				if x, ok := items[id]; ok {
					it = x
					break
				}
			}
			w, h := float64(it.Width*63+1), float64(it.Height*63+1)
			if w >= h {
				mk.IconSize = [2]float64{24, h * 24 / w}
			} else {
				mk.IconSize = [2]float64{w * 24 / h, 24}
			}
			mk.Icon, mk.IconURL = "", it.BaseImageLink
		} else if len(layers) == 1 {
			for _, id := range l.Items {
				if it, ok := items[id]; ok && len(it.HandbookCategories) > 0 {
					if cat, ok := cats[it.HandbookCategories[0]]; ok {
						mk.Icon, mk.IconURL = "", cat.ImageLink
						for _, l := range b.out.Layers {
							if l.Key == layers[0] {
								mk.Name = l.Name
							}
						}
					}
					break
				}
			}
		}
		b.add(mk)
	}
}

func (b *builder) containers(m *rawCatalogMap) {
	var layers []MarkerLayer
	seen := map[string]bool{}
	for _, c := range m.LootContainers {
		t := b.src.Maps.Data.LootContainers[c.LootContainer]
		if t.NormalizedName == "" {
			continue
		}
		icon := containerIcons[t.NormalizedName]
		if icon == "" {
			icon = t.NormalizedName
		}
		key := "container_" + t.NormalizedName
		name := b.mapName(t.Name)
		if !seen[key] {
			seen[key] = true
			layers = append(layers, MarkerLayer{Key: key, Group: "containers", Name: name, Icon: "container_" + icon})
		}
		b.add(at(Marker{Layer: key, Icon: "container_" + icon, Name: name}, c.Position))
	}
	// In the filters by name, as tarkov.dev lists them.
	sort.Slice(layers, func(i, j int) bool { return strings.ToLower(layers[i].Name) < strings.ToLower(layers[j].Name) })
	for _, l := range layers {
		b.layer(l)
	}
}

func (b *builder) spawns(m *rawCatalogMap) {
	for _, k := range []string{"pmc", "scav", "sniper_scav", "boss"} {
		b.layer(MarkerLayer{Key: "spawn_" + k, Group: "spawns", Icon: "spawn_" + k})
	}
	for _, s := range m.Spawns {
		switch {
		case contains(s.Categories, "boss"):
			// The bosses that spawn in the spawn's zone, each once.
			var bosses []Chance
			var normalized []string
			seen := map[string]bool{}
			for _, bo := range m.Bosses {
				mob := b.src.Maps.Data.Mobs[bo.Mob]
				nn := mob.NormalizedName
				if nn == "" {
					nn = bo.Mob
				}
				for _, l := range bo.SpawnLocations {
					if l.SpawnKey == s.ZoneName && !seen[nn] {
						seen[nn] = true
						bosses = append(bosses, Chance{Name: b.mapName(bo.Mob), Chance: bo.SpawnChance})
						normalized = append(normalized, nn)
					}
				}
			}
			switch {
			case len(bosses) == 0:
				if contains(s.Categories, "bot") && contains(s.Sides, "scav") {
					b.add(at(Marker{Layer: "spawn_scav", Icon: "spawn_scav"}, s.Position))
				}
			case len(bosses) == 1 && bossIcons[normalized[0]]:
				key := "spawn_" + normalized[0]
				b.layer(MarkerLayer{Key: key, Group: "spawns", Name: bosses[0].Name, Icon: key})
				b.add(at(Marker{Layer: key, Icon: key, Name: bosses[0].Name, Detail: &Detail{Bosses: bosses}}, s.Position))
			default:
				names := make([]string, len(bosses))
				for i, c := range bosses {
					names[i] = c.Name
				}
				b.add(at(Marker{Layer: "spawn_boss", Icon: "spawn_boss", Name: strings.Join(names, ", "), Detail: &Detail{Bosses: bosses}}, s.Position))
			}
		case contains(s.Categories, "player"):
			if contains(s.Sides, "pmc") || contains(s.Sides, "all") {
				b.add(at(Marker{Layer: "spawn_pmc", Icon: "spawn_pmc"}, s.Position))
			}
		case contains(s.Categories, "sniper"):
			b.add(at(Marker{Layer: "spawn_sniper_scav", Icon: "spawn_sniper_scav"}, s.Position))
		case contains(s.Sides, "scav") && (contains(s.Categories, "bot") || contains(s.Categories, "all")):
			b.add(at(Marker{Layer: "spawn_scav", Icon: "spawn_scav"}, s.Position))
		}
	}
	b.out.Markers = dedupe(b.out.Markers)
}

func (b *builder) tasks(m *rawCatalogMap) {
	b.layer(MarkerLayer{Key: "quest_item", Group: "tasks", Icon: "quest_item"})
	b.layer(MarkerLayer{Key: "quest_objective", Group: "tasks", Icon: "quest_objective"})
	ids := make([]string, 0, len(b.src.Tasks.Data.Tasks))
	for id := range b.src.Tasks.Data.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := b.src.Tasks.Data.Tasks[id]
		if t.ID == "" {
			t.ID = id
		}
		active := b.src.TaskActive == nil || b.src.TaskActive(t.ID)
		task := b.taskText(t.Name)
		for _, o := range t.Objectives {
			for _, loc := range o.PossibleLocations {
				if loc.Map != m.ID || o.QuestItem == "" {
					continue
				}
				qi := b.src.Tasks.Data.QuestItems[o.QuestItem]
				name := b.taskText(qi.Name)
				for _, p := range loc.Positions {
					b.add(at(Marker{Layer: "quest_item", Icon: "quest_item", Name: name, ID: o.QuestItem, Detail: &Detail{Task: task, TaskID: t.ID, Item: &Ref{Name: name, Image: qi.IconLink}, Active: active}}, p))
				}
			}
			for _, z := range o.Zones {
				if z.Map != m.ID {
					continue
				}
				desc := b.taskText(o.Description)
				b.add(placed(Marker{Layer: "quest_objective", Icon: "quest_objective", Name: desc, ID: z.ID, Detail: &Detail{Task: task, TaskID: t.ID, Objective: desc, Active: active}}, z.area))
			}
		}
	}
}

func (b *builder) usable(m *rawCatalogMap) {
	b.layer(MarkerLayer{Key: "switch", Group: "usable", Icon: "switch"})
	b.layer(MarkerLayer{Key: "stationarygun", Group: "usable", Icon: "stationarygun"})
	b.layer(MarkerLayer{Key: "lock", Group: "usable", Icon: "lock"})
	switchNames := map[string]string{}
	for _, s := range m.Switches {
		switchNames[s.ID] = b.mapName(s.Name)
	}
	extracts := map[string]Target{}
	for _, e := range m.Extracts {
		extracts[e.ID] = Target{Name: b.mapName(e.Name), Faction: e.Faction}
	}
	for _, s := range m.Switches {
		d := &Detail{}
		if id, ok := s.ActivatedBy.(string); ok && switchNames[id] != "" {
			d.ActivatedBy = []string{switchNames[id]}
		}
		for _, a := range s.Activates {
			if n := switchNames[a.Switch]; n != "" {
				d.Activates = append(d.Activates, Target{Name: n})
			} else if e, ok := extracts[a.Extract]; ok {
				d.Activates = append(d.Activates, e)
			}
		}
		mk := at(Marker{Layer: "switch", Icon: "switch", Name: switchNames[s.ID], ID: s.ID}, s.Position)
		if len(d.ActivatedBy) > 0 || len(d.Activates) > 0 {
			mk.Detail = d
		}
		b.add(mk)
	}
	for _, w := range m.StationaryWeapons {
		b.add(at(Marker{Layer: "stationarygun", Icon: "stationarygun", Name: b.mapName(b.src.Maps.Data.StationaryWeapons[w.StationaryWeapon].Name)}, w.Position))
	}
	for _, l := range m.Locks {
		it, ok := b.src.Items.Data.Items[l.Key]
		if !ok {
			continue
		}
		name := b.itemName(l.Key)
		b.add(at(Marker{Layer: "lock", Icon: "lock", Name: name, ID: l.Key, Detail: &Detail{LockType: l.LockType, NeedsPower: l.NeedsPower, Item: &Ref{Name: name, Image: it.IconLink}}}, l.Position))
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func groupOrder(group string) int {
	for i, g := range Groups {
		if g == group {
			return i
		}
	}
	return len(Groups)
}

// dedupe drops a spawn at the same place as one of its layer and name
// before it (spawns listed once per side).
func dedupe(in []Marker) []Marker {
	type key struct {
		layer, name string
		x, y, z     int64
	}
	seen := map[key]bool{}
	out := in[:0]
	for _, m := range in {
		if strings.HasPrefix(m.Layer, "spawn_") {
			k := key{m.Layer, m.Name, int64(m.X * 10), int64(m.Y * 10), int64(m.Z * 10)}
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, m)
	}
	return out
}
