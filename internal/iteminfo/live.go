package iteminfo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const graphqlURL = "https://api.tarkov.dev/graphql"

// Prices change a few times per scan cycle; asking again within this window
// returns the previous answer.
const liveTTL = time.Minute

const liveQuery = `query($id: ID, $mode: GameMode) { item(id: $id, gameMode: $mode) { lastLowPrice avg24hPrice low24hPrice high24hPrice changeLast48hPercent lastOfferCount updated sellFor { price currency priceRUB vendor { name normalizedName } } } }`

type livePrices struct {
	flea    *Flea
	traders []TraderPrice
	updated string
	at      time.Time
}

type liveClient struct {
	http  *http.Client
	url   string
	mu    sync.Mutex
	cache map[string]livePrices
}

func newLiveClient() *liveClient {
	return &liveClient{http: &http.Client{Timeout: 10 * time.Second}, url: graphqlURL, cache: make(map[string]livePrices)}
}

func (c *liveClient) prices(ctx context.Context, mode, id string) (livePrices, error) {
	// The GraphQL API knows the regular game and PvE only.
	if mode != "regular" && mode != "pve" {
		return livePrices{}, errors.New("live prices are not available for " + mode)
	}
	key := mode + "/" + id
	c.mu.Lock()
	cached, ok := c.cache[key]
	c.mu.Unlock()
	if ok && time.Since(cached.at) < liveTTL {
		return cached, nil
	}
	body, _ := json.Marshal(map[string]any{"query": liveQuery, "variables": map[string]string{"id": id, "mode": mode}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return livePrices{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return livePrices{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return livePrices{}, err
	}
	prices, err := parseLive(data)
	if err != nil {
		if resp.StatusCode != http.StatusOK {
			return livePrices{}, fmt.Errorf("tarkov.dev GraphQL returned %s: %w", resp.Status, err)
		}
		return livePrices{}, err
	}
	prices.at = time.Now()
	c.mu.Lock()
	c.cache[key] = prices
	c.mu.Unlock()
	return prices, nil
}

func parseLive(data []byte) (livePrices, error) {
	var response struct {
		// tarkov.dev reports errors as plain strings or GraphQL error objects.
		Errors []json.RawMessage `json:"errors"`
		Data   struct {
			Item *struct {
				LastLowPrice   *int    `json:"lastLowPrice"`
				Avg24hPrice    *int    `json:"avg24hPrice"`
				Low24hPrice    *int    `json:"low24hPrice"`
				High24hPrice   *int    `json:"high24hPrice"`
				ChangePercent  float64 `json:"changeLast48hPercent"`
				LastOfferCount int     `json:"lastOfferCount"`
				Updated        string  `json:"updated"`
				SellFor        []struct {
					Price    int    `json:"price"`
					Currency string `json:"currency"`
					PriceRUB int    `json:"priceRUB"`
					Vendor   struct {
						Name           string `json:"name"`
						NormalizedName string `json:"normalizedName"`
					} `json:"vendor"`
				} `json:"sellFor"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return livePrices{}, fmt.Errorf("invalid tarkov.dev GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		var message string
		if json.Unmarshal(response.Errors[0], &message) != nil {
			var object struct{ Message string }
			_ = json.Unmarshal(response.Errors[0], &object)
			message = object.Message
		}
		return livePrices{}, errors.New("tarkov.dev GraphQL: " + message)
	}
	item := response.Data.Item
	if item == nil {
		return livePrices{}, errors.New("tarkov.dev GraphQL does not know this item")
	}
	prices := livePrices{updated: item.Updated}
	if item.LastLowPrice != nil || item.Avg24hPrice != nil {
		prices.flea = &Flea{LastLow: deref(item.LastLowPrice), Avg24h: deref(item.Avg24hPrice), Low24h: deref(item.Low24hPrice), High24h: deref(item.High24hPrice), ChangePercent: item.ChangePercent, Offers: item.LastOfferCount}
	}
	for _, sale := range item.SellFor {
		if sale.Vendor.NormalizedName == "flea-market" {
			continue
		}
		prices.traders = append(prices.traders, TraderPrice{Trader: sale.Vendor.Name, Price: sale.Price, Currency: sale.Currency, PriceRUB: sale.PriceRUB})
	}
	sortTraders(prices.traders)
	return prices, nil
}
