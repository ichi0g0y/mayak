package iteminfo

import "sort"

// TraderOffer is a trader selling the item, for cash or in a barter: its
// price or the items it takes, the loyalty level it needs and what reaching
// that level takes, and the task that unlocks it.
type TraderOffer struct {
	Trader string `json:"trader"`
	// Price, Currency and PriceRUB are zero for a barter.
	Price    int    `json:"price"`
	Currency string `json:"currency"`
	PriceRUB int    `json:"priceRub"`
	// Barter is what the trader takes instead of money, nil for a cash offer.
	Barter []BarterItem `json:"barter,omitempty"`
	// Count is how many of the item a barter gives, when more than one.
	Count int `json:"count,omitempty"`
	// Level is the trader loyalty level the offer needs; PlayerLevel and
	// Reputation are what that level needs.
	Level       int     `json:"level"`
	PlayerLevel int     `json:"playerLevel"`
	Reputation  float64 `json:"reputation"`
	// BuyLimit is how many can be bought per restock, 0 when unlimited.
	BuyLimit int `json:"buyLimit"`
	// Task is the task whose completion unlocks the offer, nil for none. It
	// gets its pages and TarkovTracker state like the item's tasks.
	Task *TaskNeed `json:"task,omitempty"`
}

// BarterItem is one of the items a barter takes. Count can be a fraction:
// Ref's offers take GP coins as money.
type BarterItem struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Names     map[string]string `json:"names,omitempty"`
	ShortName string            `json:"shortName"`
	IconURL   string            `json:"iconUrl"`
	Count     float64           `json:"count"`
}

// rawOffer is one of a catalog item's buyFromTrader entries.
type rawOffer struct {
	Trader         string `json:"trader"`
	Price          int    `json:"price"`
	PriceRUB       int    `json:"priceRUB"`
	Currency       string `json:"currency"`
	MinTraderLevel int    `json:"minTraderLevel"`
	TaskUnlock     string `json:"taskUnlock"`
	BuyLimit       int    `json:"buyLimit"`
}

// rawBarter is one of the catalog's barters.
type rawBarter struct {
	Trader         string `json:"trader"`
	MinTraderLevel int    `json:"minTraderLevel"`
	TaskUnlock     string `json:"taskUnlock"`
	BuyLimit       int    `json:"buyLimit"`
	RequiredItems  []struct {
		Item  string  `json:"item"`
		Count float64 `json:"count"`
	} `json:"requiredItems"`
	OfferedItem struct {
		Item  string  `json:"item"`
		Count float64 `json:"count"`
	} `json:"offeredItem"`
}

// traderLevel is a trader loyalty level and what reaching it needs.
type traderLevel struct {
	Level               int     `json:"level"`
	RequiredPlayerLevel int     `json:"requiredPlayerLevel"`
	RequiredReputation  float64 `json:"requiredReputation"`
}

// offers are the item's cash offers, cheapest (in roubles) first, then its
// barters, from the lowest loyalty level.
func (ix *index) offers(item rawItem) []TraderOffer {
	var offers, barters []TraderOffer
	for _, o := range item.BuyFromTrader {
		if offer, ok := ix.offer(o.Trader, o.MinTraderLevel, o.TaskUnlock, o.BuyLimit); ok {
			offer.Price, offer.Currency, offer.PriceRUB = o.Price, o.Currency, o.PriceRUB
			offers = append(offers, offer)
		}
	}
	sort.SliceStable(offers, func(i, j int) bool {
		if offers[i].PriceRUB != offers[j].PriceRUB {
			return offers[i].PriceRUB < offers[j].PriceRUB
		}
		return offers[i].Level < offers[j].Level
	})
	for _, b := range ix.barters[item.ID] {
		offer, ok := ix.offer(b.Trader, b.MinTraderLevel, b.TaskUnlock, b.BuyLimit)
		if !ok || len(b.RequiredItems) == 0 {
			continue
		}
		if b.OfferedItem.Count > 1 {
			offer.Count = int(b.OfferedItem.Count)
		}
		// A barter taking an item the catalog does not know is left out:
		// the panel could not say what it takes.
		for _, r := range b.RequiredItems {
			required, known := ix.items[r.Item]
			if !known {
				ok = false
				break
			}
			name := localize(ix.text, required.Name)
			offer.Barter = append(offer.Barter, BarterItem{ID: r.Item, Name: name, Names: ix.names("items", required.Name, name), ShortName: localize(ix.text, required.ShortName), IconURL: required.IconLink, Count: r.Count})
		}
		if ok {
			barters = append(barters, offer)
		}
	}
	sort.SliceStable(barters, func(i, j int) bool {
		if barters[i].Level != barters[j].Level {
			return barters[i].Level < barters[j].Level
		}
		if barters[i].Trader != barters[j].Trader {
			return barters[i].Trader < barters[j].Trader
		}
		return barters[i].Barter[0].ID < barters[j].Barter[0].ID
	})
	return append(offers, barters...)
}

// offer is an offer by trader at a loyalty level, with what that level takes
// and the task unlocking it; false for a trader the catalog does not know.
func (ix *index) offer(trader string, level int, task string, limit int) (TraderOffer, bool) {
	offer := TraderOffer{Trader: ix.traders[trader], Level: max(1, level), BuyLimit: max(0, limit)}
	if offer.Trader == "" {
		return offer, false
	}
	for _, l := range ix.levels[trader] {
		if l.Level == offer.Level {
			offer.PlayerLevel, offer.Reputation = l.RequiredPlayerLevel, l.RequiredReputation
		}
	}
	if need, ok := ix.taskByID[task]; ok && task != "" {
		offer.Task = &need
	}
	return offer, true
}

// offerProgress marks the tasks unlocking offers like Progress marks the
// item's tasks, on copies so the catalog's stay unmarked.
func offerProgress(offers []TraderOffer, tasks map[string]string) []TraderOffer {
	offers = append([]TraderOffer(nil), offers...)
	for i := range offers {
		if offers[i].Task == nil {
			continue
		}
		task := *offers[i].Task
		task.State = ""
		if tasks != nil {
			task.State = tasks[task.ID]
			if task.State == "" {
				task.State = "uncompleted"
			}
		}
		offers[i].Task = &task
	}
	return offers
}
