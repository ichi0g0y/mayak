package keyusage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/local/mayak/internal/version"
)

const wikiAPI = "https://escapefromtarkov.fandom.com/api.php"

// The key pages' categories: keys and keycards (246 of tarkov.dev's 259
// keys, 2026-10-08).
var categories = []string{"Keys", "Keycards"}

// refresh is how long a fetched list is kept, and retry how soon a failed
// fetch is tried again.
const (
	refresh = 24 * time.Hour
	retry   = 30 * time.Minute
)

// Store keeps what the wiki says of each key, by item id. It reads its cache
// file at once and fetches the wiki in the background when the list is due;
// nothing waits for it. It stays offline until Enable (tests and tools).
type Store struct {
	path string
	api  string
	http *http.Client

	mu       sync.RWMutex
	enabled  bool
	kinds    map[string]Kind
	loaded   bool
	fetching bool
	next     time.Time
}

// cacheFile is what the cache keeps: the classified keys and when they were
// fetched. Unclassified keys are left out.
type cacheFile struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Kinds     map[string]Kind `json:"kinds"`
}

// New makes a store with its cache at path ("" keeps none).
func New(path string) *Store {
	return &Store{path: path, api: wikiAPI, http: &http.Client{Timeout: 30 * time.Second}}
}

// Enable lets the store fetch the wiki.
func (s *Store) Enable() *Store {
	s.mu.Lock()
	s.enabled = true
	s.mu.Unlock()
	return s
}

// Warm reads the cache and starts a fetch when it is due, so the first key
// shown after a start finds the list at hand.
func (s *Store) Warm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dueLocked()
}

// Kind is what the wiki says of the key id; Unclassified while the list is
// not at hand.
func (s *Store) Kind(id string) Kind {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dueLocked()
	return s.kinds[id]
}

// dueLocked reads the cache once and starts a fetch when the list is stale.
// The caller holds mu.
func (s *Store) dueLocked() {
	if !s.loaded {
		s.loaded = true
		if cached, ok := s.readCache(); ok {
			s.kinds = cached.Kinds
			s.next = cached.FetchedAt.Add(refresh)
		}
	}
	if s.enabled && !s.fetching && time.Now().After(s.next) {
		s.fetching = true
		go s.fetch()
	}
}

func (s *Store) readCache() (cacheFile, bool) {
	if s.path == "" {
		return cacheFile{}, false
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return cacheFile{}, false
	}
	var cached cacheFile
	if json.Unmarshal(data, &cached) != nil || cached.Kinds == nil {
		return cacheFile{}, false
	}
	return cached, true
}

// fetch reads every key page and keeps the result. A failed fetch keeps the
// last list (or none) and is tried again soon.
func (s *Store) fetch() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	kinds, err := s.fetchKinds(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetching = false
	if err != nil {
		s.next = time.Now().Add(retry)
		return
	}
	now := time.Now()
	s.kinds, s.next = kinds, now.Add(refresh)
	if s.path == "" {
		return
	}
	if data, err := json.Marshal(cacheFile{FetchedAt: now, Kinds: kinds}); err == nil {
		_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
		_ = os.WriteFile(s.path, data, 0o644)
	}
}

func (s *Store) fetchKinds(ctx context.Context) (map[string]Kind, error) {
	kinds := map[string]Kind{}
	for _, category := range categories {
		if err := s.fetchCategory(ctx, category, kinds); err != nil {
			return nil, err
		}
	}
	return kinds, nil
}

// fetchCategory reads a category's pages, 50 a request, with their text.
func (s *Store) fetchCategory(ctx context.Context, category string, kinds map[string]Kind) error {
	next := url.Values{}
	for page := 0; page < 20; page++ {
		query := url.Values{"action": {"query"}, "generator": {"categorymembers"}, "gcmtitle": {"Category:" + category}, "gcmnamespace": {"0"}, "gcmlimit": {"50"}, "prop": {"revisions"}, "rvprop": {"content"}, "rvslots": {"main"}, "format": {"json"}, "formatversion": {"2"}}
		for key, values := range next {
			query[key] = values
		}
		var body struct {
			Continue map[string]string `json:"continue"`
			Query    struct {
				Pages []struct {
					Revisions []struct {
						Slots struct {
							Main struct {
								Content string `json:"content"`
							} `json:"main"`
						} `json:"slots"`
					} `json:"revisions"`
				} `json:"pages"`
			} `json:"query"`
		}
		if err := s.get(ctx, query, &body); err != nil {
			return fmt.Errorf("wiki %s: %w", category, err)
		}
		for _, p := range body.Query.Pages {
			if len(p.Revisions) == 0 {
				continue
			}
			node, usage := parsePage(p.Revisions[0].Slots.Main.Content)
			if kind := Classify(usage); node != "" && kind != Unclassified {
				kinds[node] = kind
			}
		}
		if len(body.Continue) == 0 {
			return nil
		}
		next = url.Values{}
		for key, value := range body.Continue {
			next.Set(key, value)
		}
	}
	return nil
}

func (s *Store) get(ctx context.Context, query url.Values, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.api+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	response, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}
