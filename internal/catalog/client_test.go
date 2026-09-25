package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalogModesCacheAndLastKnownGood(t *testing.T) {
	var calls atomic.Int32
	var failing atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failing.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		name := parts[1]
		data := map[string]any{"id": map[string]string{"name": "example"}}
		if name == "items" || name == "maps" || name == "tasks" {
			data = map[string]any{name: map[string]any{parts[0]: map[string]string{"id": parts[0]}}}
			if name == "items" {
				data["settings"] = map[string]int{"scavCooldownSeconds": 1500}
				data["playerLevels"] = []int{1, 2}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	c := New()
	c.baseURL = server.URL + "/"
	c.cacheDir = t.TempDir()
	first, err := c.Refresh(context.Background(), "pve", false)
	if err != nil || first.Items != 1 || first.HideoutStations != 1 || first.ScavCooldownSeconds != 1500 || first.PlayerLevels != 2 {
		t.Fatalf("snapshot=%#v err=%v", first, err)
	}
	if _, err = c.Refresh(context.Background(), "pve", false); err != nil || calls.Load() != int32(len(allResources())) {
		t.Fatalf("fresh cache was not used: %v %d", err, calls.Load())
	}
	pvp, err := c.Refresh(context.Background(), "regular", false)
	if err != nil || string(pvp.Resources["tasks"]) == string(first.Resources["tasks"]) {
		t.Fatalf("mode data mixed: %v", err)
	}
	failing.Store(true)
	stale, err := c.Refresh(context.Background(), "pve", true)
	if err == nil || stale != first {
		t.Fatal("failed refresh discarded the last-known-good snapshot")
	}
	first.UpdatedAt = time.Now().Add(-time.Hour)
	if err = c.save(first); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.baseURL = server.URL + "/"
	restarted.cacheDir = c.cacheDir
	restored, err := restarted.Refresh(context.Background(), "pve", false)
	if err == nil || restored == nil || restored.Items != 1 {
		t.Fatalf("offline disk fallback failed: %v", err)
	}
	before := calls.Load()
	if _, err = restarted.Refresh(context.Background(), "pve", false); err == nil || calls.Load() != before {
		t.Fatal("offline retry cooldown was not respected")
	}
}

func TestPartialDownloadCannotPublishCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()
	c := New()
	c.cacheDir = ""
	c.baseURL = server.URL + "/"
	if snapshot, err := c.Refresh(context.Background(), "pve", true); err == nil || snapshot != nil {
		t.Fatal("empty resource bundle was published")
	}
}

func TestCatalogRejectsUnknownModeAndResources(t *testing.T) {
	c := New()
	c.cacheDir = ""
	if _, err := c.Refresh(context.Background(), "auto", false); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if err := c.Get(context.Background(), "pve", "../token", nil); err == nil {
		t.Fatal("invalid resource accepted")
	}
}

func TestUnchangedResourcesAreNotDownloadedAgain(t *testing.T) {
	var downloads, notModified atomic.Int32
	var version atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		name := parts[1]
		// Only items change between refreshes, like flea prices do.
		etag := `"` + name + `-1"`
		if name == "items" {
			etag = `"items-` + string(rune('0'+version.Load())) + `"`
		}
		if r.Header.Get("If-None-Match") == etag {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		downloads.Add(1)
		w.Header().Set("ETag", etag)
		data := map[string]any{"id": map[string]string{"name": "example"}}
		if name == "items" || name == "maps" || name == "tasks" {
			data = map[string]any{name: map[string]any{parts[0]: map[string]string{"id": parts[0]}}}
		}
		// Translations map keys to text, like tarkov.dev's.
		if strings.HasSuffix(name, "_en") || strings.HasSuffix(name, "_ja") {
			data = map[string]any{"example": "Example"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	c := New()
	c.baseURL = server.URL + "/"
	c.cacheDir = t.TempDir()
	first, err := c.Refresh(context.Background(), "pve", true)
	if err != nil || first.ETags["tasks"] != `"tasks-1"` {
		t.Fatalf("first refresh: %v %v", err, first.ETags)
	}
	total := int32(len(allResources()))
	if downloads.Load() != total {
		t.Fatalf("first refresh downloaded %d of %d", downloads.Load(), total)
	}
	version.Store(1)
	second, err := c.Refresh(context.Background(), "pve", true)
	if err != nil {
		t.Fatal(err)
	}
	if downloads.Load() != total+1 || notModified.Load() != total-1 {
		t.Fatalf("second refresh: downloads %d, not modified %d", downloads.Load()-total, notModified.Load())
	}
	if string(second.Resources["tasks"]) != string(first.Resources["tasks"]) || second.ETags["items"] != `"items-1"` {
		t.Fatalf("second snapshot: %v", second.ETags)
	}
	// The ETags are saved with the snapshot, so a restart asks for changes only.
	restarted := New()
	restarted.baseURL = server.URL + "/"
	restarted.cacheDir = c.cacheDir
	if _, err := restarted.Refresh(context.Background(), "pve", true); err != nil || downloads.Load() != total+1 {
		t.Fatalf("after restart: downloads %d, %v", downloads.Load()-total-1, err)
	}
}
