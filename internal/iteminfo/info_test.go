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
		"items":      `{"data":{"items":{"gpu":{"id":"gpu","name":"gpu Name","shortName":"gpu Short","width":2,"height":1,"lastLowPrice":400000,"avg24hPrice":410000,"changeLast48hPercent":-2.5,"lastOfferCount":12,"minLevelForFlea":20,"updated":"2026-09-23T12:00:00.000Z","sellToTrader":[{"trader":"prapor","price":90000,"priceRUB":90000,"currency":"RUB"},{"trader":"therapist","price":120000,"priceRUB":120000,"currency":"RUB"}]}}}}`,
		"items_en":   `{"data":{"gpu Name":"Graphics card","gpu Short":"GPU"}}`,
		"traders":    `{"data":{"prapor":{"name":"prapor Nickname"},"therapist":{"name":"therapist Nickname"}}}`,
		"traders_en": `{"data":{"prapor Nickname":"Prapor","therapist Nickname":"Therapist"}}`,
		"tasks":      `{"data":{"tasks":{"t1":{"id":"t1","name":"t1 name","trader":"prapor","objectives":[{"type":"findItem","items":["gpu"],"count":2,"foundInRaid":true},{"type":"giveItem","items":["gpu","other"],"count":2}]},"t2":{"id":"t2","name":"t2 name","trader":"prapor","objectives":[{"type":"visit"}]}}}}`,
		"tasks_en":   `{"data":{"t1 name":"Farming - Part 4","t2 name":"Other"}}`,
		"tasks_ja":   `{"data":{"t1 name":"農業 4"}}`,
		"hideout":    `{"data":{"s":{"id":"s","name":"s name","levels":[{"id":"s-2","level":2,"itemRequirements":[{"item":"gpu","count":1,"attributes":{"foundInRaid":true}}]}]}}}`,
		"hideout_en": `{"data":{"s name":"Bitcoin farm"}}`,
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
	if len(info.Tasks) != 1 || info.Tasks[0].Name != "Farming - Part 4" || info.Tasks[0].Names["ja"] != "農業 4" || info.Tasks[0].Count != 2 || !info.Tasks[0].FoundInRaid || info.Tasks[0].Trader != "Prapor" {
		t.Fatalf("tasks = %+v", info.Tasks)
	}
	if len(info.Hideout) != 1 || info.Hideout[0].Station != "Bitcoin farm" || info.Hideout[0].Level != 2 || info.Hideout[0].LevelID != "s-2" {
		t.Fatalf("hideout = %+v", info.Hideout)
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
