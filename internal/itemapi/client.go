package itemapi

import (
	"strings"

	"context"
	"encoding/json"
	"errors"
	"github.com/local/mayak/internal/locale"
	"net/http"
	"sync"
	"time"

	"github.com/local/mayak/internal/itemmatch"
)

const baseURL = "https://json.tarkov.dev/"

type rawItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ShortName      string `json:"shortName"`
	NormalizedName string `json:"normalizedName"`
	Link           string `json:"link"`
	IconLink       string `json:"iconLink"`
}

type itemEnvelope struct {
	Data struct {
		Items map[string]rawItem `json:"items"`
	} `json:"data"`
}

type translationEnvelope struct {
	Data map[string]string `json:"data"`
}

type Client struct {
	source interface {
		Get(context.Context, string, string, any) error
	}
	http   *http.Client
	mu     sync.Mutex
	items  map[string][]itemmatch.Item
	loaded map[string]time.Time
}

func NewWithSource(source interface {
	Get(context.Context, string, string, any) error
}) *Client {
	c := New()
	c.source = source
	return c
}

func (c *Client) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = make(map[string]time.Time)
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, items: make(map[string][]itemmatch.Item), loaded: make(map[string]time.Time)}
}

func (c *Client) ItemsForMode(ctx context.Context, mode string) ([]itemmatch.Item, error) {
	if mode == "" || mode == "auto" {
		mode = "regular"
	}
	if mode != "regular" && mode != "pve" && mode != "pvp-season" {
		return nil, errors.New("unsupported game mode")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if items := c.items[mode]; len(items) > 0 && time.Since(c.loaded[mode]) < 12*time.Hour {
		return append([]itemmatch.Item(nil), items...), nil
	}
	items, err := c.load(ctx, mode)
	if err != nil {
		return nil, err
	}
	c.items[mode] = items
	c.loaded[mode] = time.Now()
	return append([]itemmatch.Item(nil), items...), nil
}

func (c *Client) load(ctx context.Context, mode string) ([]itemmatch.Item, error) {
	var items itemEnvelope
	var text translationEnvelope
	if err := c.get(ctx, mode, "items", &items); err != nil {
		return nil, err
	}
	if err := c.get(ctx, mode, "items_en", &text); err != nil {
		return nil, err
	}
	if len(items.Data.Items) == 0 {
		return nil, errors.New("tarkov.dev returned an empty item catalog")
	}
	// Names in the other languages match titles read from a game in them. A
	// missing language leaves the English names working.
	languages := map[string]map[string]string{}
	for _, lang := range locale.Languages {
		var names translationEnvelope
		if c.get(ctx, mode, locale.Resource("items", lang), &names) == nil {
			languages[lang] = names.Data
		}
	}
	result := make([]itemmatch.Item, 0, len(items.Data.Items))
	for id, item := range items.Data.Items {
		if item.ID == "" {
			item.ID = id
		}
		entry := itemmatch.Item{
			ID:             item.ID,
			Name:           translate(text.Data, item.Name),
			ShortName:      translate(text.Data, item.ShortName),
			NormalizedName: item.NormalizedName,
			Link:           item.Link,
			IconLink:       item.IconLink,
		}
		for _, lang := range locale.Languages {
			names := languages[lang]
			if name := alias(names, item.Name, text.Data); name != "" {
				entry.Aliases = append(entry.Aliases, name)
				if entry.Names == nil {
					entry.Names = map[string]string{}
				}
				entry.Names[lang] = name
			}
			if short := alias(names, item.ShortName, text.Data); short != "" {
				entry.ShortAliases = append(entry.ShortAliases, short)
			}
		}
		result = append(result, entry)
	}
	return result, nil
}

func (c *Client) get(ctx context.Context, mode, path string, target any) error {
	if c.source != nil {
		return c.source.Get(ctx, mode, path, target)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+mode+"/"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("tarkov.dev JSON API returned " + resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// alias is key's name in names when it differs from the English one.
func alias(names map[string]string, key string, english map[string]string) string {
	if name := strings.TrimSpace(names[key]); name != "" && name != translate(english, key) {
		return name
	}
	return ""
}

func translate(dictionary map[string]string, key string) string {
	if value := dictionary[key]; value != "" {
		return value
	}
	return key
}
