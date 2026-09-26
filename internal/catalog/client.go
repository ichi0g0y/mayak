package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/locale"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// RefreshInterval is how often the catalog is checked for changes. tarkov.dev
// republishes flea prices every few minutes; unchanged resources are not
// downloaded again (see fetch), so frequent checks stay cheap.
const RefreshInterval = 5 * time.Minute
const retryCooldown = time.Minute

var resources = []string{"items", "items_en", "maps", "maps_en", "traders", "traders_en", "tasks", "tasks_en", "hideout", "hideout_en"}

// optionalResources are the names in other languages (see internal/locale):
// a catalog without them still works, in English.
var optionalResources = func() []string {
	var names []string
	for _, lang := range locale.Languages {
		names = append(names, locale.Resource("tasks", lang), locale.Resource("items", lang))
	}
	return names
}()

func optional(name string) bool { return slices.Contains(optionalResources, name) }

// The resources each consumer reads, for Snapshot.Version: the task list
// (internal/questapi), the item data (internal/itemapi) and the hideout.
// Flea prices change the items every few minutes; tasks and the hideout
// rarely, so their consumers keep what they built while these stay the same.
var (
	TaskResources = append([]string{"tasks", "tasks_en", "maps", "maps_en", "traders", "traders_en"}, localized("tasks")...)
	ItemResources = append([]string{"items", "items_en"}, localized("items")...)
	// The hideout reads item and trader names: "items" carries the flea
	// prices and changes with them, but a new item brings a new name to
	// "items_en" too, so that one stands for it.
	HideoutResources = []string{"hideout", "hideout_en", "items_en", "traders", "traders_en"}
)

func localized(resource string) []string {
	names := make([]string, 0, len(locale.Languages))
	for _, lang := range locale.Languages {
		names = append(names, locale.Resource(resource, lang))
	}
	return names
}

// Version identifies the content of some of the snapshot's resources: it
// changes when any of them does. A resource without an ETag counts by its
// length and a checksum.
func (s *Snapshot) Version(names ...string) string {
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('=')
		if etag := s.ETags[name]; etag != "" {
			b.WriteString(etag)
		} else {
			data := s.Resources[name]
			fmt.Fprintf(&b, "%d:%08x", len(data), crc32.ChecksumIEEE(data))
		}
		b.WriteByte(';')
	}
	return b.String()
}

func allResources() []string {
	return append(append([]string(nil), resources...), optionalResources...)
}

type Snapshot struct {
	Mode      string                     `json:"mode"`
	UpdatedAt time.Time                  `json:"updatedAt"`
	Resources map[string]json.RawMessage `json:"resources"`
	// ETags identify each resource's version, so the next refresh downloads
	// only resources that changed.
	ETags               map[string]string `json:"etags,omitempty"`
	Items               int               `json:"items"`
	Maps                int               `json:"maps"`
	Traders             int               `json:"traders"`
	Tasks               int               `json:"tasks"`
	HideoutStations     int               `json:"hideoutStations"`
	ScavCooldownSeconds int               `json:"scavCooldownSeconds"`
	PlayerLevels        int               `json:"playerLevels"`
}

type Client struct {
	http       *http.Client
	baseURL    string
	cacheDir   string
	mu         sync.Mutex
	refreshMu  sync.Mutex
	snapshots  map[string]*Snapshot
	failures   map[string]time.Time
	lastErrors map[string]error
}

func New() *Client {
	dir, _ := os.UserConfigDir()
	if dir != "" {
		dir = filepath.Join(dir, appdir.Name, "catalog")
	}
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, baseURL: "https://json.tarkov.dev/", cacheDir: dir, snapshots: make(map[string]*Snapshot), failures: make(map[string]time.Time), lastErrors: make(map[string]error)}
}

func ValidMode(mode string) bool { return mode == "regular" || mode == "pve" || mode == "pvp-season" }

// Get shares the same mode-specific snapshot with all consumers. On network
// failure, a validated last-known-good snapshot remains usable.
func (c *Client) Get(ctx context.Context, mode, resource string, target any) error {
	allowed := false
	for _, name := range allResources() {
		if name == resource {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("unsupported catalog resource")
	}
	snapshot, err := c.Refresh(ctx, mode, false)
	if snapshot == nil {
		return err
	}
	return json.Unmarshal(snapshot.Resources[resource], target)
}

// Refresh publishes all resources together: a partial or malformed download
// must never replace a previously working catalog.
func (c *Client) Refresh(ctx context.Context, mode string, force bool) (*Snapshot, error) {
	if !ValidMode(mode) {
		return nil, errors.New("unsupported catalog game mode")
	}
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	c.mu.Lock()
	previous := c.snapshots[mode]
	c.mu.Unlock()
	if previous == nil && c.cacheDir != "" {
		var cached Snapshot
		if data, err := os.ReadFile(filepath.Join(c.cacheDir, mode+".json")); err == nil && json.Unmarshal(data, &cached) == nil && cached.Mode == mode && validate(&cached) == nil {
			previous = &cached
			c.mu.Lock()
			c.snapshots[mode] = previous
			c.mu.Unlock()
		}
	}
	if !force && previous != nil && time.Since(previous.UpdatedAt) >= 0 && time.Since(previous.UpdatedAt) < RefreshInterval {
		return previous, nil
	}
	if !force && time.Since(c.failures[mode]) < retryCooldown {
		return previous, c.lastErrors[mode]
	}
	next := &Snapshot{Mode: mode, UpdatedAt: time.Now().UTC(), Resources: make(map[string]json.RawMessage), ETags: make(map[string]string)}
	type result struct {
		name string
		data json.RawMessage
		etag string
		err  error
	}
	names := allResources()
	results := make(chan result, len(names))
	for _, name := range names {
		// A resource that has not changed since the previous snapshot is
		// reused as it is.
		var previousData json.RawMessage
		var previousETag string
		if previous != nil && len(previous.Resources[name]) > 0 {
			previousData, previousETag = previous.Resources[name], previous.ETags[name]
		}
		go func(name string) {
			data, etag, notModified, err := c.fetch(ctx, mode, name, previousETag)
			if notModified {
				data, etag = previousData, previousETag
			}
			results <- result{name, data, etag, err}
		}(name)
	}
	var downloadErr error
	for range names {
		result := <-results
		if optional(result.name) {
			var locale struct {
				Data map[string]string `json:"data"`
			}
			if result.err == nil && json.Unmarshal(result.data, &locale) == nil && len(locale.Data) > 0 {
				next.Resources[result.name] = result.data
				next.ETags[result.name] = result.etag
			} else if previous != nil {
				next.Resources[result.name] = previous.Resources[result.name]
				next.ETags[result.name] = previous.ETags[result.name]
			}
			continue
		}
		if result.err != nil {
			downloadErr = errors.Join(downloadErr, result.err)
		} else {
			next.Resources[result.name] = result.data
			next.ETags[result.name] = result.etag
		}
	}
	if downloadErr != nil {
		c.failures[mode] = time.Now()
		c.lastErrors[mode] = downloadErr
		return previous, downloadErr
	}
	if err := validate(next); err != nil {
		c.failures[mode] = time.Now()
		c.lastErrors[mode] = err
		return previous, err
	}
	delete(c.failures, mode)
	delete(c.lastErrors, mode)
	c.mu.Lock()
	c.snapshots[mode] = next
	c.mu.Unlock()
	if c.cacheDir != "" {
		if err := c.save(next); err != nil {
			return next, fmt.Errorf("catalog loaded but disk cache could not be saved: %w", err)
		}
	}
	return next, nil
}

// fetch downloads one resource. With the ETag of the copy in hand it asks
// for changes only; notModified reports that the copy is still current.
func (c *Client) fetch(ctx context.Context, mode, name, etag string) (data json.RawMessage, newETag string, notModified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+mode+"/"+name, nil)
	if err != nil {
		return nil, "", false, err
	}
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("%s: %w", name, err)
	}
	defer response.Body.Close()
	if etag != "" && response.StatusCode == http.StatusNotModified {
		return nil, etag, true, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("%s: %s", name, response.Status)
	}
	const maxResponseBytes = 64 << 20
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, "", false, err
	}
	if len(data) > maxResponseBytes || !json.Valid(data) {
		return nil, "", false, fmt.Errorf("invalid catalog response: %s", name)
	}
	return data, response.Header.Get("ETag"), false, nil
}

func validate(snapshot *Snapshot) error {
	counts := make(map[string]int)
	for _, name := range resources {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(snapshot.Resources[name], &envelope) != nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return fmt.Errorf("missing catalog data: %s", name)
		}
		var entries map[string]json.RawMessage
		if json.Unmarshal(envelope.Data, &entries) != nil {
			return fmt.Errorf("invalid catalog data: %s", name)
		}
		switch name {
		case "items", "maps", "tasks":
			var nested map[string]json.RawMessage
			if json.Unmarshal(entries[name], &nested) != nil || len(nested) == 0 {
				return fmt.Errorf("empty catalog: %s", name)
			}
			counts[name] = len(nested)
		case "traders", "hideout":
			if len(entries) == 0 {
				return fmt.Errorf("empty catalog: %s", name)
			}
			counts[name] = len(entries)
		}
		if name == "items" {
			var settings struct {
				ScavCooldownSeconds int `json:"scavCooldownSeconds"`
			}
			_ = json.Unmarshal(entries["settings"], &settings)
			var levels []json.RawMessage
			_ = json.Unmarshal(entries["playerLevels"], &levels)
			snapshot.ScavCooldownSeconds = settings.ScavCooldownSeconds
			snapshot.PlayerLevels = len(levels)
		}
	}
	snapshot.Items = counts["items"]
	snapshot.Maps = counts["maps"]
	snapshot.Tasks = counts["tasks"]
	snapshot.Traders = counts["traders"]
	snapshot.HideoutStations = counts["hideout"]
	return nil
}

func (c *Client) save(snapshot *Snapshot) error {
	if err := os.MkdirAll(c.cacheDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(c.cacheDir, ".catalog-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(c.cacheDir, snapshot.Mode+".json"))
}
