package mapdata

import (
	"encoding/json"
	"testing"
)

const catalogFixture = `{"data":{
 "maps":{"m1":{"id":"m1","normalizedName":"customs","raidDuration":35,"players":"10-12",
  "bosses":[
   {"mob":"bossBully","spawnChance":0.6,"spawnLocations":[{"spawnKey":"ZoneDormitory"},{"spawnKey":"ZoneGasStation"}]},
   {"mob":"bossKnight","spawnChance":0.4,"spawnLocations":[{"spawnKey":"ZoneScavBase"}]},
   {"mob":"sectantPriest","spawnChance":0.1,"spawnLocations":[{"spawnKey":"ZoneScavBase"},{"spawnKey":"ZoneWoodCutter"}]}],
  "spawns":[
   {"position":{"x":1,"y":0,"z":1},"sides":["scav"],"categories":["boss"],"zoneName":"ZoneDormitory"},
   {"position":{"x":2,"y":0,"z":2},"sides":["scav"],"categories":["boss"],"zoneName":"ZoneScavBase"},
   {"position":{"x":3,"y":0,"z":3},"sides":["scav"],"categories":["boss"],"zoneName":"ZoneWoodCutter"},
   {"position":{"x":4,"y":0,"z":4},"sides":["scav"],"categories":["boss","bot"],"zoneName":"ZoneNobody"},
   {"position":{"x":10,"y":0,"z":10},"sides":["pmc"],"categories":["player"]},
   {"position":{"x":10,"y":0,"z":10},"sides":["pmc"],"categories":["player"]},
   {"position":{"x":12,"y":0,"z":12},"sides":["all"],"categories":["player"]},
   {"position":{"x":13,"y":0,"z":13},"sides":["scav"],"categories":["player"]},
   {"position":{"x":14,"y":0,"z":14},"sides":["scav"],"categories":["sniper"]},
   {"position":{"x":11,"y":0,"z":11},"sides":["scav"],"categories":["bot"]}],
  "extracts":[{"id":"e1","name":"Crossroads","faction":"shared","switches":["s1"],"transferItem":{"item":"i1","count":2},"position":{"x":5,"y":1,"z":5},"outline":[{"x":4,"y":0,"z":4},{"x":6,"y":0,"z":4},{"x":6,"y":0,"z":6}],"top":3,"bottom":0},
   {"id":"e2","name":"Odd","position":{"x":0,"y":0,"z":0}}],
  "transits":[{"id":"9","description":"CUS_TRANSIT_9_DESC","position":{"x":7,"y":0,"z":7}}],
  "locks":[{"lockType":"door","key":"k1","needsPower":true,"position":{"x":3,"y":0,"z":3}},{"lockType":"door","key":"unknown","position":{"x":3,"y":0,"z":3}}],
  "hazards":[{"hazardType":"minefield","name":"DamageType_Landmine","position":{"x":8,"y":0,"z":8}}],
  "lootContainers":[{"lootContainer":"c1","position":{"x":9,"y":0,"z":9}},{"lootContainer":"c2","position":{"x":9,"y":0,"z":8}}],
  "lootLoose":[{"items":["i1"],"position":{"x":1,"y":1,"z":1}},{"items":["i1","i2"],"position":{"x":2,"y":1,"z":2}}],
  "switches":[{"id":"s1","name":"switch_power","activatedBy":false,"activates":[{"extract":"e1"}],"position":{"x":2,"y":0,"z":2}}],
  "stationaryWeapons":[{"stationaryWeapon":"w1","position":{"x":4,"y":0,"z":4}}],
  "btrStops":[{"name":"stop1","x":1,"y":2,"z":3}],
  "artillery":{"zones":[{"id":"13","position":{"x":1,"y":0,"z":1},"outline":[{"x":0,"y":0,"z":0},{"x":2,"y":0,"z":0},{"x":2,"y":0,"z":2}]}]}},
  "m2":{"id":"m2","normalizedName":"factory"}},
 "mobs":{"bossBully":{"normalizedName":"reshala"},"bossKnight":{"normalizedName":"knight"},"sectantPriest":{"normalizedName":"cultist-priest"}},
 "lootContainers":{"c1":{"name":"c1 Name","normalizedName":"toolbox"},"c2":{"name":"c2 Name","normalizedName":"scav-body"}},
 "stationaryWeapons":{"w1":{"name":"w1 Name"}}}}`

const itemsFixture = `{"data":{
 "items":{
  "k1":{"iconLink":"k1.webp","width":1,"height":1},
  "i1":{"baseImageLink":"i1.webp","iconLink":"i1-icon.webp","width":2,"height":1,"handbookCategories":["h1","hp"]},
  "i2":{"baseImageLink":"i2.webp","width":1,"height":1,"handbookCategories":["h2"]}},
 "handbookCategories":{"h1":{"name":"h1","normalizedName":"ammo","imageLink":"h1.webp"},"h2":{"name":"h2","normalizedName":"medical","imageLink":"h2.webp"}}}}`

const tasksFixture = `{"data":{
 "tasks":{"t1":{"id":"t1","name":"t1 name","objectives":[
  {"id":"o1","description":"o1","zones":[{"id":"z1","map":"m1","position":{"x":1,"y":0,"z":1},"outline":[{"x":0,"y":0,"z":0}]},{"id":"z2","map":"m2","position":{"x":1,"y":0,"z":1}}]},
  {"id":"o2","description":"o2","questItem":"q1","possibleLocations":[{"map":"m1","positions":[{"x":5,"y":0,"z":5},{"x":6,"y":0,"z":6}]},{"map":"m2","positions":[{"x":1,"y":0,"z":1}]}]}]},
  "t2":{"id":"t2","name":"t2 name","objectives":[{"id":"o3","description":"o3","zones":[{"id":"z3","map":"m1","position":{"x":2,"y":0,"z":2}}]}]}},
 "questItems":{"q1":{"name":"q1 Name","iconLink":"q1.webp"}}}}`

func fixtureSources(t *testing.T) Sources {
	t.Helper()
	var src Sources
	for _, x := range []struct {
		raw string
		to  any
	}{{catalogFixture, &src.Maps}, {itemsFixture, &src.Items}, {tasksFixture, &src.Tasks}} {
		if err := json.Unmarshal([]byte(x.raw), x.to); err != nil {
			t.Fatal(err)
		}
	}
	src.MapNames = Names{Data: map[string]string{"bossBully": "Reshala", "bossKnight": "Knight", "sectantPriest": "Cultist Priest", "CUS_TRANSIT_9_DESC": "Transit to Reserve", "c1 Name": "Toolbox", "c2 Name": "Scav body", "w1 Name": "NSV", "switch_power": "Power Switch", "DamageType_Landmine": "Landmine", "stop1": "Cinema"}}
	src.ItemNames = Names{Data: map[string]string{"k1 Name": "検問所の鍵", "h1": "弾薬"}}
	src.ItemNamesEn = Names{Data: map[string]string{"k1 Name": "Checkpoint key", "i1 Name": "Ammo box", "i2 Name": "Medkit", "h1": "Ammo", "h2": "Medical"}}
	src.TaskNames = Names{Data: map[string]string{"t1 name": "任務 1", "o1": "目的地に行く"}}
	src.TaskNamesEn = Names{Data: map[string]string{"t2 name": "Task 2", "o3": "Go there", "q1 Name": "Folder"}}
	src.TaskActive = func(id string) bool { return id == "t1" }
	return src
}

func TestMarkersFollowTarkovDev(t *testing.T) {
	mm, err := Markers(fixtureSources(t), "no-such-map", "customs")
	if err != nil {
		t.Fatal(err)
	}
	if mm.RaidDuration != 35 || mm.Players != "10-12" {
		t.Fatalf("raid info = %d %q", mm.RaidDuration, mm.Players)
	}
	byLayer := map[string][]Marker{}
	for _, m := range mm.Markers {
		byLayer[m.Layer] = append(byLayer[m.Layer], m)
	}
	// The filters in tarkov.dev's groups and order.
	var groups []string
	layers := map[string]MarkerLayer{}
	for _, l := range mm.Layers {
		if len(groups) == 0 || groups[len(groups)-1] != l.Group {
			groups = append(groups, l.Group)
		}
		layers[l.Key] = l
	}
	want := []string{"extracts", "hazards", "landmarks", "loose", "containers", "spawns", "tasks", "usable"}
	if len(groups) != len(want) {
		t.Fatalf("groups = %v", groups)
	}
	for i := range want {
		if groups[i] != want[i] {
			t.Fatalf("groups = %v", groups)
		}
	}
	// Extracts: an unknown faction counts as shared; the popup names the
	// switch and the item to hand over.
	if len(byLayer["extract_shared"]) != 2 {
		t.Fatalf("shared extracts = %d", len(byLayer["extract_shared"]))
	}
	e := byLayer["extract_shared"][0]
	if e.Name != "Crossroads" || len(e.Outline) != 3 || e.Top != 3 || e.Detail == nil || e.Detail.ActivatedBy[0] != "Power Switch" || e.Detail.Item.Name != "Ammo box" || e.Detail.Item.Count != 2 {
		t.Fatalf("extract = %+v %+v", e, e.Detail)
	}
	// Bosses as tarkov.dev draws them.
	bosses := map[string]string{}
	for _, l := range []string{"spawn_boss", "spawn_cultist-priest"} {
		for _, b := range byLayer[l] {
			bosses[b.Name] = b.Icon
		}
	}
	if len(bosses) != 3 || bosses["Reshala"] != "spawn_boss" || bosses["Knight, Cultist Priest"] != "spawn_boss" || bosses["Cultist Priest"] != "spawn_cultist-priest" {
		t.Fatalf("boss spawns = %v", bosses)
	}
	if layers["spawn_cultist-priest"].Name != "Cultist Priest" || byLayer["spawn_boss"][0].Detail.Bosses[0].Chance != 0.6 {
		t.Fatal("boss layer or chance")
	}
	if len(byLayer["spawn_pmc"]) != 2 || len(byLayer["spawn_scav"]) != 2 || len(byLayer["spawn_sniper_scav"]) != 1 {
		t.Fatalf("spawns: pmc %d scav %d sniper %d", len(byLayer["spawn_pmc"]), len(byLayer["spawn_scav"]), len(byLayer["spawn_sniper_scav"]))
	}
	// Containers by type, loose loot by handbook category.
	if layers["container_toolbox"].Name != "Toolbox" || byLayer["container_scav-body"][0].Icon != "container_dead-scav" {
		t.Fatal("containers")
	}
	if l := layers["loose_ammo"]; l.Name != "弾薬" || l.IconURL != "h1.webp" || layers["loose_medical"].Name != "Medical" {
		t.Fatalf("loose layers = %+v", layers)
	}
	one := byLayer["loose_ammo"]
	if len(one) != 2 || one[0].IconURL != "i1.webp" || one[0].IconSize != [2]float64{24, 64 * 24 / 127.0} || one[1].Icon != "loose_loot" || len(one[1].Layers) != 2 {
		t.Fatalf("loose loot = %+v", one)
	}
	// Tasks: this map's only, named in the language shown, active per
	// TaskActive.
	if q := byLayer["quest_item"]; len(q) != 2 || q[0].Name != "Folder" || q[0].Detail.Task != "任務 1" || !q[0].Detail.Active {
		t.Fatalf("quest items = %+v", q)
	}
	obj := map[string]bool{}
	for _, o := range byLayer["quest_objective"] {
		obj[o.Detail.Objective] = o.Detail.Active
	}
	if len(obj) != 2 || !obj["目的地に行く"] || obj["Go there"] {
		t.Fatalf("objectives = %v", obj)
	}
	// Usable things, hazards and landmarks.
	if l := byLayer["lock"]; len(l) != 1 || l[0].Name != "検問所の鍵" || !l[0].Detail.NeedsPower || l[0].Detail.LockType != "door" {
		t.Fatalf("locks = %+v", l)
	}
	if s := byLayer["switch"]; len(s) != 1 || s[0].Name != "Power Switch" || s[0].Detail.Activates[0].Name != "Crossroads" {
		t.Fatalf("switch = %+v", s)
	}
	if len(byLayer["hazard_minefield"]) != 1 || layers["hazard_minefield"].Name != "Landmine" || len(byLayer["hazard_mortar"]) != 1 {
		t.Fatal("hazards")
	}
	if b := byLayer["btr_stop"]; len(b) != 1 || b[0].Name != "Cinema" || byLayer["stationarygun"][0].Name != "NSV" || byLayer["extract_transit"][0].Name != "Transit to Reserve" {
		t.Fatal("landmarks or guns")
	}
	if _, err := Markers(fixtureSources(t), "woods"); err == nil {
		t.Fatal("a map the catalog lacks gave markers")
	}
}
