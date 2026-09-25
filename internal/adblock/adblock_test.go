package adblock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRules = `[Adblock Plus 2.0]
||ads.example^
||tracker.example^$third-party
@@||ads.example/allowed.js
##.ad-banner
example.com##.sponsor
example.com#@#.ad-banner
||tarkov-ads.example^
`

func newTestBlocker(t *testing.T) *Blocker {
	t.Helper()
	dir := t.TempDir()
	b := New(dir, func(string, string) {})
	b.lists = []List{{ID: 1, Name: "test"}}
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(testRules), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := b.rebuild(); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBlockRequests(t *testing.T) {
	b := newTestBlocker(t)
	page := "https://news.example.org/article"
	for _, tc := range []struct {
		url, kind string
		want      bool
	}{
		{"https://ads.example/banner.png", KindImage, true},
		{"https://ads.example/allowed.js", KindScript, false},
		{"https://tracker.example/pixel.gif", KindImage, true},
		{"https://cdn.example.org/app.js", KindScript, false},
	} {
		if got := b.Block(tc.url, page, tc.kind); got != tc.want {
			t.Errorf("Block(%s) = %v, want %v", tc.url, got, tc.want)
		}
	}
	// $third-party does not apply to the tracker's own pages.
	if b.Block("https://tracker.example/pixel.gif", "https://tracker.example/", KindImage) {
		t.Error("first-party request was blocked")
	}
}

func TestExemptSitesAndDisabling(t *testing.T) {
	b := newTestBlocker(t)
	for _, page := range []string{"https://tarkov.dev/map/customs", "https://api.tarkov.dev/"} {
		if b.Block("https://tarkov-ads.example/x.js", page, KindScript) || b.CosmeticCSS(page) != "" {
			t.Errorf("%s is not exempt", page)
		}
	}
	b.SetEnabled(false)
	if b.Block("https://ads.example/banner.png", "https://news.example.org/", KindImage) || b.CosmeticCSS("https://news.example.org/") != "" {
		t.Error("disabled blocker still filters")
	}
}

func TestCosmeticCSSHidesAdSlots(t *testing.T) {
	b := newTestBlocker(t)
	css := b.CosmeticCSS("https://news.example.org/")
	if !strings.Contains(css, ".ad-banner{display:none!important;--rl:1}") || strings.Contains(css, ".sponsor") {
		t.Errorf("generic CSS = %q", css)
	}
	css = b.CosmeticCSS("https://example.com/")
	if !strings.Contains(css, ".sponsor{display:none!important;--rl:1}") || strings.Contains(css, ".ad-banner") {
		t.Errorf("site CSS = %q", css)
	}
}

func TestCosmeticCSSAppliesSiteRulesToSubdomains(t *testing.T) {
	b := newTestBlocker(t)
	css := b.CosmeticCSS("https://wiki.example.com/page")
	if !strings.Contains(css, ".sponsor{") {
		t.Errorf("example.com rule missing on a subdomain: %q", css)
	}
	if got := parentDomains("a.b.example.com"); strings.Join(got, ",") != "b.example.com,example.com" {
		t.Errorf("parentDomains = %v", got)
	}
	if got := parentDomains("localhost"); len(got) != 0 {
		t.Errorf("parentDomains(localhost) = %v", got)
	}
}

func TestHidingCSSSkipsUnsafeSelectors(t *testing.T) {
	css := hidingCSS([]string{".a", ".a", "x{}", "</style>", " "}, []string{"#b"})
	if css != ".a{display:none!important;--rl:1}\n#b{display:none!important;--rl:1}\n" {
		t.Errorf("hidingCSS = %q", css)
	}
}

func TestDownloadRejectsNonFilterResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list.txt" {
			_, _ = w.Write([]byte(testRules))
			return
		}
		_, _ = w.Write([]byte("<html>captive portal</html>"))
	}))
	defer server.Close()
	b := New(t.TempDir(), func(string, string) {})
	b.lists = []List{{ID: 1, Name: "good", URL: server.URL + "/list.txt"}, {ID: 2, Name: "bad", URL: server.URL + "/portal"}}
	updated, err := b.update(context.Background())
	if !updated || err == nil {
		t.Fatalf("update = %v, %v", updated, err)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "bad.txt")); !os.IsNotExist(err) {
		t.Error("an HTML page was cached as a filter list")
	}
	if err := b.rebuild(); err != nil || !b.Block("https://ads.example/a.png", "https://site.example/", KindImage) {
		t.Errorf("downloaded list is not active: %v", err)
	}
}
