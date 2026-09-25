package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
