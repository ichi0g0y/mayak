package iteminfo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/mayak/internal/catalog"
)

type fixedSource struct{ snapshot *catalog.Snapshot }

func (s fixedSource) Refresh(context.Context, string, bool) (*catalog.Snapshot, error) {
	return s.snapshot, nil
}

func testSnapshot() *catalog.Snapshot {
	raw := map[string]string{
		"items":      `{"data":{"items":{"gpu":{"id":"gpu","name":"gpu Name","shortName":"gpu Short","width":2,"height":1,"lastLowPrice":400000,"avg24hPrice":410000,"changeLast48hPercent":-2.5,"lastOfferCount":12,"minLevelForFlea":20,"updated":"2026-09-23T12:00:00.000Z","sellToTrader":[{"trader":"prapor","price":90000,"priceRUB":90000,"currency":"RUB"},{"trader":"therapist","price":120000,"priceRUB":120000,"currency":"RUB"}],"buyFromTrader":[{"trader":"therapist","price":1000,"priceRUB":150000,"currency":"USD","minTraderLevel":2,"taskUnlock":"t2","buyLimit":3},{"trader":"prapor","price":140000,"priceRUB":140000,"currency":"RUB","minTraderLevel":1,"taskUnlock":null,"buyLimit":null},{"trader":"gone","price":1,"priceRUB":1,"currency":"RUB"}]},"thermo":{"id":"thermo","name":"thermo Name","shortName":"thermo Short","iconLink":"https://example.com/thermo.webp"}}}}`,
		"items_en":   `{"data":{"gpu Name":"Graphics card","gpu Short":"GPU","thermo Name":"Analog thermometer","thermo Short":"Termo"}}`,
		"items_ja":   `{"data":{"thermo Name":"アナログ温度計"}}`,
		"barters":    `{"data":[{"trader":"prapor","minTraderLevel":2,"taskUnlock":"t2","buyLimit":6,"requiredItems":[{"item":"thermo","count":1}],"offeredItem":{"item":"gpu","count":1}},{"trader":"therapist","minTraderLevel":1,"taskUnlock":null,"buyLimit":4,"requiredItems":[{"item":"gpu","count":7.51}],"offeredItem":{"item":"gpu","count":2}},{"trader":"prapor","requiredItems":[{"item":"unknown","count":1}],"offeredItem":{"item":"gpu","count":1}},{"trader":"prapor","requiredItems":[{"item":"gpu","count":1}],"offeredItem":{"item":"thermo","count":1}}]}`,
		"traders":    `{"data":{"prapor":{"name":"prapor Nickname","imageLink":"https://assets.tarkov.dev/prapor.webp","levels":[{"level":1,"requiredPlayerLevel":0,"requiredReputation":0},{"level":2,"requiredPlayerLevel":6,"requiredReputation":0.7}]},"therapist":{"name":"therapist Nickname","levels":[{"level":1,"requiredPlayerLevel":0},{"level":2,"requiredPlayerLevel":5,"requiredReputation":0.6}]}}}`,
		"traders_en": `{"data":{"prapor Nickname":"Prapor","therapist Nickname":"Therapist"}}`,
		"tasks":      `{"data":{"tasks":{"t1":{"id":"t1","name":"t1 name","trader":"prapor","objectives":[{"type":"findItem","items":["gpu"],"count":2,"foundInRaid":true},{"type":"giveItem","items":["gpu","other"],"count":2}]},"t2":{"id":"t2","name":"t2 name","trader":"prapor","objectives":[{"type":"visit"}]}}}}`,
		"tasks_en":   `{"data":{"t1 name":"Farming - Part 4","t2 name":"Other"}}`,
		"tasks_ja":   `{"data":{"t1 name":"農業 4"}}`,
		"hideout":    `{"data":{"s":{"id":"s","name":"s name","levels":[{"id":"s-2","level":2,"itemRequirements":[{"item":"gpu","count":1,"attributes":{"foundInRaid":true}}]}]}}}`,
		"hideout_en": `{"data":{"s name":"Bitcoin farm"}}`,
		"maps":       `{"data":{"maps":{"m1":{"normalizedName":"customs","locks":[{"key":"gpu","position":{"x":10,"y":2,"z":-5}},{"key":"other","position":{"x":1,"y":1,"z":1}}]},"m2":{"normalizedName":"bigmap-none","locks":[]}}}}`,
	}
	snapshot := &catalog.Snapshot{Resources: map[string]json.RawMessage{}}
	for name, data := range raw {
		snapshot.Resources[name] = json.RawMessage(data)
	}
	return snapshot
}

func TestCatalogInfo(t *testing.T) {
	info, err := New(fixedSource{testSnapshot()}).Catalog(context.Background(), "pve", "gpu")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Graphics card" || info.ShortName != "GPU" || info.Mode != "pve" || info.PricedAt != "2026-09-23T12:00:00.000Z" {
		t.Fatalf("info = %+v", info)
	}
	if info.Flea == nil || info.Flea.LastLow != 400000 || info.Flea.MinLevel != 20 {
		t.Fatalf("flea = %+v", info.Flea)
	}
	if len(info.Traders) != 2 || info.Traders[0].Trader != "Therapist" {
		t.Fatalf("traders = %+v", info.Traders)
	}
	// The traders' pictures by name; one without stays out.
	if len(info.TraderImages) != 1 || info.TraderImages["Prapor"] != "https://assets.tarkov.dev/prapor.webp" {
		t.Fatalf("trader images = %v", info.TraderImages)
	}
	if len(info.Tasks) != 1 || info.Tasks[0].Name != "Farming - Part 4" || info.Tasks[0].Names["ja"] != "農業 4" || info.Tasks[0].Count != 2 || !info.Tasks[0].FoundInRaid || info.Tasks[0].Trader != "Prapor" {
		t.Fatalf("tasks = %+v", info.Tasks)
	}
	if len(info.Hideout) != 1 || info.Hideout[0].Station != "Bitcoin farm" || info.Hideout[0].Level != 2 || info.Hideout[0].LevelID != "s-2" {
		t.Fatalf("hideout = %+v", info.Hideout)
	}
	if len(info.Locks) != 1 || info.Locks[0] != (Lock{Map: "customs", X: 10, Y: 2, Z: -5}) {
		t.Fatalf("locks = %+v", info.Locks)
	}
	// Cash offers, cheapest in roubles first, then barters from the lowest
	// loyalty level; a trader or a barter item not in the catalog is left out.
	if len(info.Offers) != 4 {
		t.Fatalf("offers = %+v", info.Offers)
	}
	first, second := info.Offers[0], info.Offers[1]
	if first.Trader != "Prapor" || first.Price != 140000 || first.Level != 1 || first.PlayerLevel != 0 || first.BuyLimit != 0 || first.Task != nil {
		t.Fatalf("first offer = %+v", first)
	}
	if second.Trader != "Therapist" || second.Price != 1000 || second.Currency != "USD" || second.PriceRUB != 150000 || second.Level != 2 || second.PlayerLevel != 5 || second.Reputation != 0.6 || second.BuyLimit != 3 || second.Task == nil || second.Task.ID != "t2" || second.Task.Name != "Other" || second.Task.Trader != "Prapor" {
		t.Fatalf("second offer = %+v %+v", second, second.Task)
	}
	coins, thermo := info.Offers[2], info.Offers[3]
	if coins.Trader != "Therapist" || coins.Level != 1 || coins.Price != 0 || coins.Count != 2 || coins.BuyLimit != 4 || coins.Task != nil || len(coins.Barter) != 1 || coins.Barter[0].ID != "gpu" || coins.Barter[0].Count != 7.51 || coins.Barter[0].Name != "Graphics card" {
		t.Fatalf("coin barter = %+v", coins)
	}
	if thermo.Trader != "Prapor" || thermo.Level != 2 || thermo.PlayerLevel != 6 || thermo.Reputation != 0.7 || thermo.Count != 0 || thermo.BuyLimit != 6 || thermo.Task == nil || thermo.Task.ID != "t2" {
		t.Fatalf("thermometer barter = %+v", thermo)
	}
	if b := thermo.Barter; len(b) != 1 || b[0].Name != "Analog thermometer" || b[0].Names["ja"] != "アナログ温度計" || b[0].ShortName != "Termo" || b[0].IconURL != "https://example.com/thermo.webp" || b[0].Count != 1 {
		t.Fatalf("thermometer barter items = %+v", b)
	}
}

func TestProgress(t *testing.T) {
	info, _ := New(fixedSource{testSnapshot()}).Catalog(context.Background(), "pve", "gpu")
	unknown := Progress(info, nil, nil)
	if unknown.Tasks[0].State != "" || unknown.Hideout[0].Complete != nil {
		t.Fatalf("unknown progress = %+v %+v", unknown.Tasks, unknown.Hideout)
	}
	known := Progress(info, map[string]string{"t1": "completed"}, map[string]bool{})
	if known.Tasks[0].State != "completed" || known.Hideout[0].Complete == nil || *known.Hideout[0].Complete {
		t.Fatalf("known progress = %+v %+v", known.Tasks, known.Hideout)
	}
	// The task unlocking an offer is marked too, on a copy.
	if unknown.Offers[1].Task.State != "" || known.Offers[1].Task.State != "uncompleted" || info.Offers[1].Task.State != "" {
		t.Fatalf("offer task states = %q %q", unknown.Offers[1].Task.State, known.Offers[1].Task.State)
	}
	if done := Progress(info, map[string]string{"t2": "completed"}, nil); done.Offers[1].Task.State != "completed" {
		t.Fatalf("completed offer task = %+v", done.Offers[1].Task)
	}
}

func TestParseLive(t *testing.T) {
	prices, err := parseLive([]byte(`{"data":{"item":{"lastLowPrice":390000,"avg24hPrice":405000,"changeLast48hPercent":1.5,"lastOfferCount":30,"updated":"2026-09-23T13:00:00.000Z","sellFor":[{"price":395000,"currency":"RUB","priceRUB":395000,"vendor":{"name":"Flea Market","normalizedName":"flea-market"}},{"price":88000,"currency":"RUB","priceRUB":88000,"vendor":{"name":"Prapor","normalizedName":"prapor"}},{"price":1000,"currency":"USD","priceRUB":130000,"vendor":{"name":"Peacekeeper","normalizedName":"peacekeeper"}}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if prices.flea.LastLow != 390000 || prices.updated != "2026-09-23T13:00:00.000Z" || len(prices.traders) != 2 || prices.traders[0].Trader != "Peacekeeper" {
		t.Fatalf("prices = %+v", prices)
	}
	if _, err := parseLive([]byte(`{"errors":["GraphQL server unavailable. Try again later."]}`)); err == nil {
		t.Fatal("string errors must fail")
	}
	if _, err := parseLive([]byte(`{"errors":[{"message":"bad"}],"data":null}`)); err == nil {
		t.Fatal("object errors must fail")
	}
}

func TestHistory(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/pve/prices/57347ca924597744596b4e71" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"price":739444.4,"priceMin":728888,"offerCount":119,"timestamp":1790154485000},{"price":1223743,"priceMin":1090000,"timestamp":1721347200000},{"price":0,"timestamp":1721347200001}]}`))
	}))
	defer server.Close()
	s := New(fixedSource{testSnapshot()})
	s.historyURL = server.URL + "/"
	points, err := s.History(context.Background(), "pve", "57347ca924597744596b4e71")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].Time != 1721347200000 || points[1].Price != 739444 || points[1].Min != 728888 || points[1].Offers != 119 {
		t.Fatalf("points = %+v", points)
	}
	if _, err := s.History(context.Background(), "pve", "57347ca924597744596b4e71"); err != nil || calls != 1 {
		t.Fatalf("history was not cached: calls=%d err=%v", calls, err)
	}
	if _, err := s.History(context.Background(), "pve", "../items"); err == nil {
		t.Fatal("invalid IDs must be rejected")
	}
}
