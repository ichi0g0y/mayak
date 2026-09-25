package iteminfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"time"
)

const historyURL = "https://json.tarkov.dev/"

// The history file is republished a few times a day.
const historyTTL = 30 * time.Minute

// PricePoint is one flea market sample: the average and lowest offer at Time
// (Unix milliseconds). Older samples are daily, recent ones a few per day.
type PricePoint struct {
	Time   int64 `json:"t"`
	Price  int   `json:"price"`
	Min    int   `json:"min"`
	Offers int   `json:"offers,omitempty"`
}

type cachedHistory struct {
	points []PricePoint
	at     time.Time
}

var itemID = regexp.MustCompile(`^[0-9a-f]{24}$`)

// History returns the item's flea market price history, oldest first.
func (s *Service) History(ctx context.Context, mode, id string) ([]PricePoint, error) {
	mode = Mode(mode)
	if !itemID.MatchString(id) {
		return nil, errors.New("invalid item ID")
	}
	key := mode + "/" + id
	s.mu.Lock()
	cached, ok := s.history[key]
	s.mu.Unlock()
	if ok && time.Since(cached.at) < historyTTL {
		return cached.points, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.historyURL+mode+"/prices/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	resp, err := s.live.http.Do(req)
	if err != nil {
		if ok {
			return cached.points, nil
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if ok {
			return cached.points, nil
		}
		return nil, fmt.Errorf("tarkov.dev price history returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var points []PricePoint
	if err == nil {
		points, err = parseHistory(data)
	}
	// A cut-off or malformed answer falls back to the cached history too.
	if err != nil {
		if ok {
			return cached.points, nil
		}
		return nil, err
	}
	s.mu.Lock()
	s.history[key] = cachedHistory{points: points, at: time.Now()}
	s.mu.Unlock()
	return points, nil
}

func parseHistory(data []byte) ([]PricePoint, error) {
	var response struct {
		Data []struct {
			Price      float64 `json:"price"`
			PriceMin   float64 `json:"priceMin"`
			OfferCount int     `json:"offerCount"`
			Timestamp  int64   `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("invalid price history: %w", err)
	}
	points := make([]PricePoint, 0, len(response.Data))
	for _, p := range response.Data {
		if p.Timestamp <= 0 || p.Price <= 0 {
			continue
		}
		points = append(points, PricePoint{Time: p.Timestamp, Price: int(p.Price + .5), Min: int(p.PriceMin + .5), Offers: p.OfferCount})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Time < points[j].Time })
	return points, nil
}
