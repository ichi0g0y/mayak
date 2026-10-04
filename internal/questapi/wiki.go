package questapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/version"
)

// The official wiki lists new tasks before tarkov.dev's catalog has them.
// Tasks only the wiki knows are added by name, so a screenshot of one still
// matches; their page is the wiki's. They have no tarkov.dev route
// (NormalizedName stays empty), like the supplemental tasks.
const wikiAPI = "https://escapefromtarkov.fandom.com/api.php"
const wikiPage = "https://escapefromtarkov.fandom.com/wiki/"

// wikiRefresh is how long the wiki's task list is kept, and wikiRetry how
// soon a failed fetch is tried again.
const wikiRefresh = 12 * time.Hour
const wikiRetry = 30 * time.Minute

// wikiWait is how long building a task list waits for the wiki's first list.
// Story chapters are known from the wiki only, so a list built without it
// misses them: a story screenshot taken right after a start would fail once
// and match on the next try.
const wikiWait = 10 * time.Second

// EnableWiki makes the client add the wiki's tasks (off by default, so
// tests and tools stay offline).
func (c *Client) EnableWiki() *Client {
	c.wiki = true
	return c
}

// WarmWiki starts fetching the wiki's task list now, so that the first
// recognition after a start finds it at hand.
func (c *Client) WarmWiki() {
	if !c.wiki {
		return
	}
	c.wikiMu.Lock()
	c.startWikiLocked()
	c.wikiMu.Unlock()
}

// startWikiLocked fetches the wiki's list in the background when it is due.
// The caller holds wikiMu.
func (c *Client) startWikiLocked() {
	if !c.wikiFetching && time.Now().After(c.wikiNext) {
		c.wikiFetching = true
		c.wikiDone = make(chan struct{})
		go c.refreshWiki(c.wikiDone)
	}
}

// wikiQuest is a task the wiki lists: its page's title and the trader who
// gives it ("" for a story chapter, or when the page does not say).
type wikiQuest struct {
	Title  string
	Trader string
}

// wikiQuestTitles returns the wiki's tasks at hand, at once. A
// missing or stale list is fetched in the background; when it changes, the
// task lists are built again with it.
func (c *Client) wikiQuestTitles() []wikiQuest {
	if !c.wiki {
		return nil
	}
	c.wikiMu.Lock()
	defer c.wikiMu.Unlock()
	c.startWikiLocked()
	return c.wikiTitles
}

// awaitFirstWiki waits for the wiki's first list after a start, up to
// wikiWait (or until ctx ends), so the first task list has the story
// chapters. It waits once: a wiki that does not answer makes no later list
// wait. The caller holds no lock of the client while it waits.
func (c *Client) awaitFirstWiki(ctx context.Context) {
	if !c.wiki {
		return
	}
	c.wikiMu.Lock()
	if c.wikiWaited || len(c.wikiTitles) > 0 {
		c.wikiMu.Unlock()
		return
	}
	c.wikiWaited = true
	c.startWikiLocked()
	fetching, done := c.wikiFetching, c.wikiDone
	c.wikiMu.Unlock()
	if !fetching {
		return
	}
	timer := time.NewTimer(wikiWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
}

// refreshWiki fetches the wiki's list and closes done when it is stored.
func (c *Client) refreshWiki(done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	titles, err := c.fetchWikiQuestTitles(ctx)
	c.wikiMu.Lock()
	c.wikiFetching = false
	// A failed fetch keeps the last list (or none) and is tried again soon.
	if err != nil || len(titles) == 0 {
		c.wikiNext = time.Now().Add(wikiRetry)
		close(done)
		c.wikiMu.Unlock()
		return
	}
	c.wikiNext = time.Now().Add(wikiRefresh)
	changed := !slices.Equal(c.wikiTitles, titles)
	c.wikiTitles = titles
	close(done)
	c.wikiMu.Unlock()
	if changed {
		c.Invalidate()
	}
}

func (c *Client) fetchWikiQuestTitles(ctx context.Context) ([]wikiQuest, error) {
	quests, err := c.wikiCategory(ctx, "Quests")
	if err != nil {
		return nil, err
	}
	// Past events' tasks are marked Historical content on the wiki; they are
	// not in the game. Event content alone is an event under way (its tasks,
	// "All-Inclusive Support", are in the game and not on tarkov.dev yet).
	past := map[string]bool{}
	historical, err := c.wikiCategory(ctx, "Historical content")
	if err != nil {
		return nil, err
	}
	for _, member := range historical {
		past[member.title] = true
	}
	// Every task in the game counts, however old its page: a task can come
	// to the game long after its page was made ("To the Light - Getting
	// Acquainted", made in 2023). The ones tarkov.dev has are dropped by name.
	var titles []wikiQuest
	var current []string
	for _, member := range quests {
		if !past[member.title] {
			current = append(current, member.title)
		}
	}
	// Who gives each (the page's "given by"): the Japanese wiki files a task
	// under its trader. Without it (the pages not read) the tasks still match.
	givers := c.wikiGivers(ctx, current)
	for _, title := range current {
		titles = append(titles, wikiQuest{Title: title, Trader: givers[title]})
	}
	// The story chapters (the Story tab of the Tasks screen: "Tour", "The
	// Ticket"…) are tasks tarkov.dev's catalog does not carry; the wiki
	// keeps them in a category of their own, all of them in the game, so
	// none is too old to count.
	chapters, err := c.wikiCategory(ctx, "Story chapters")
	if err != nil {
		return nil, err
	}
	for _, member := range chapters {
		if !past[member.title] && !slices.ContainsFunc(titles, func(q wikiQuest) bool { return q.Title == member.title }) {
			titles = append(titles, wikiQuest{Title: member.title})
		}
	}
	return titles, nil
}

// givenBy is the trader in a task page's infobox: "|given by =[[Mechanic]]".
var givenBy = regexp.MustCompile(`(?i)\|\s*given by\s*=\s*\[\[([^\]|]+)`)

// wikiGivers reads who gives each task from the pages, 50 pages a request.
// A page that fails to come is left out.
func (c *Client) wikiGivers(ctx context.Context, titles []string) map[string]string {
	out := map[string]string{}
	for start := 0; start < len(titles); start += 50 {
		batch := titles[start:min(start+50, len(titles))]
		query := url.Values{"action": {"query"}, "prop": {"revisions"}, "rvprop": {"content"}, "rvslots": {"main"}, "titles": {strings.Join(batch, "|")}, "format": {"json"}, "formatversion": {"2"}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikiAPI+"?"+query.Encode(), nil)
		if err != nil {
			return out
		}
		req.Header.Set("User-Agent", version.UserAgent())
		response, err := c.http.Do(req)
		if err != nil {
			return out
		}
		var body struct {
			Query struct {
				Pages []struct {
					Title     string `json:"title"`
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
		ok := response.StatusCode == http.StatusOK && json.NewDecoder(response.Body).Decode(&body) == nil
		response.Body.Close()
		if !ok {
			return out
		}
		for _, page := range body.Query.Pages {
			if len(page.Revisions) == 0 {
				continue
			}
			if m := givenBy.FindStringSubmatch(page.Revisions[0].Slots.Main.Content); m != nil {
				out[page.Title] = strings.TrimSpace(m[1])
			}
		}
	}
	return out
}

type wikiMember struct {
	title string
}

// wikiCategory lists a wiki category's pages.
func (c *Client) wikiCategory(ctx context.Context, category string) ([]wikiMember, error) {
	var members []wikiMember
	next := ""
	// The API returns 500 pages at a time; the categories have a few hundred.
	for page := 0; page < 10; page++ {
		query := url.Values{"action": {"query"}, "list": {"categorymembers"}, "cmtitle": {"Category:" + category}, "cmnamespace": {"0"}, "cmlimit": {"500"}, "cmprop": {"title"}, "format": {"json"}}
		if next != "" {
			query.Set("cmcontinue", next)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikiAPI+"?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", version.UserAgent())
		response, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("wiki %s: %s", category, response.Status)
		}
		var body struct {
			Continue struct {
				Next string `json:"cmcontinue"`
			} `json:"continue"`
			Query struct {
				Members []struct {
					Title string `json:"title"`
				} `json:"categorymembers"`
			} `json:"query"`
		}
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, member := range body.Query.Members {
			members = append(members, wikiMember{member.Title})
		}
		if next = body.Continue.Next; next == "" {
			break
		}
	}
	return members, nil
}

// appendWiki adds the wiki's tasks that quests does not have by name.
func appendWiki(quests []Quest, titles []wikiQuest) []Quest {
	known := make(map[string]bool, len(quests))
	for _, quest := range quests {
		known[questmatch.Normalize(quest.Name)] = true
	}
	for _, wq := range titles {
		title := strings.TrimSpace(wq.Title)
		key := questmatch.Normalize(title)
		// "Quests" and "Story chapters" are the categories' own overview pages.
		if key == "" || known[key] || strings.EqualFold(title, "Quests") || strings.EqualFold(title, "Story chapters") {
			continue
		}
		known[key] = true
		quests = append(quests, Quest{
			Quest:    questmatch.Quest{ID: "wiki:" + title, Name: title, Trader: wq.Trader},
			WikiLink: wikiPage + url.PathEscape(strings.ReplaceAll(title, " ", "_")),
		})
	}
	return quests
}
