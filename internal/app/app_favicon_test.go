package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFaviconURLAllowed(t *testing.T) {
	for _, u := range []string{"https://tarkov.dev/favicon.ico", "http://example.com/a.png"} {
		if !faviconURLAllowed(u) {
			t.Errorf("%s was rejected", u)
		}
	}
	for _, u := range []string{"file:///c:/a.ico", "https://localhost/a.ico", "http://127.0.0.1/a.ico", "http://192.168.1.10/a.ico", "https://user:pw@example.com/a.ico", "javascript:alert(1)"} {
		if faviconURLAllowed(u) {
			t.Errorf("%s was allowed", u)
		}
	}
}

func TestFaviconCacheServesAndRefreshes(t *testing.T) {
	icon := []byte{0, 0, 1, 0, 1, 2, 3}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(icon)
	}))
	defer server.Close()
	c := &faviconCache{dir: t.TempDir(), client: server.Client(), allow: func(string) bool { return true }}
	u := server.URL + "/favicon.ico"
	first, err := c.get(context.Background(), u, false)
	if err != nil || !strings.HasPrefix(first, "data:image/x-icon;base64,") || calls != 1 {
		t.Fatalf("first = %q, %v, calls %d", first, err, calls)
	}
	// Cached: neither a plain read nor an early refresh fetches again.
	if again, _ := c.get(context.Background(), u, false); again != first || calls != 1 {
		t.Fatalf("cache miss: calls %d", calls)
	}
	if again, _ := c.get(context.Background(), u, true); again != first || calls != 1 {
		t.Fatalf("refreshed too early: calls %d", calls)
	}
	// Once the copy is old, a refresh picks up a changed icon.
	old := time.Now().Add(-2 * faviconRefresh)
	files, _ := filepath.Glob(filepath.Join(c.dir, "*.img"))
	for _, f := range files {
		os.Chtimes(f, old, old)
	}
	icon = []byte{0, 0, 1, 0, 9, 9, 9}
	if changed, _ := c.get(context.Background(), u, true); changed == first || calls != 2 {
		t.Fatalf("changed icon not picked up: calls %d", calls)
	}
}

func TestIconLinksReadsTheHead(t *testing.T) {
	page, _ := url.Parse("https://example.com/wiki/Page")
	doc := `<html><head><link rel="mask-icon" href="/mask.svg"><link rel="apple-touch-icon" href="/touch.png">
<link rel="shortcut icon" href="/static/fav.ico"><base href="https://cdn.example.com/"><link rel="icon" href="i.png" sizes="32x32"></head>
<body><link rel="icon" href="/late.png"></body></html>`
	got := iconLinks(strings.NewReader(doc), page)
	want := []string{"https://example.com/static/fav.ico", "https://cdn.example.com/i.png", "https://example.com/touch.png"}
	if !slices.Equal(got, want) {
		t.Fatalf("iconLinks = %v, want %v", got, want)
	}
}

func TestSiteIconFindsTheIconOfAPageNeverOpened(t *testing.T) {
	icon := []byte{0, 0, 1, 0, 1, 2, 3}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/old":
			http.Redirect(w, r, "/page", http.StatusFound)
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<head><link rel="icon" href="/missing.png"><link rel="icon" href="/img/icon.ico"></head>`))
		case "/img/icon.ico", "/favicon.ico":
			w.Write(icon)
		case "/bare":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<head><title>no icon named</title></head>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := &faviconCache{dir: t.TempDir(), client: server.Client(), allow: func(string) bool { return true }}
	// Through a redirect, the first named icon that loads.
	if got, err := c.siteIcon(context.Background(), server.URL+"/old"); err != nil || got != server.URL+"/img/icon.ico" {
		t.Fatalf("siteIcon = %q, %v", got, err)
	}
	// A page naming none falls back to /favicon.ico, and it is cached.
	got, err := c.siteIcon(context.Background(), server.URL+"/bare")
	if err != nil || got != server.URL+"/favicon.ico" {
		t.Fatalf("siteIcon = %q, %v", got, err)
	}
	if files, _ := filepath.Glob(filepath.Join(c.dir, "*.img")); len(files) != 2 {
		t.Fatalf("cached %d icons, want 2", len(files))
	}
	// A page the app may not fetch is refused.
	c.allow = faviconURLAllowed
	if _, err := c.siteIcon(context.Background(), "http://127.0.0.1/"); err == nil {
		t.Fatal("siteIcon fetched a local address")
	}
}
