package questapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/local/mayak/internal/questmatch"
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

// wikiRecent limits the wiki's tasks to those added to its task list lately:
// the list also has Arena tasks and past events, which are not in the game
// and would only add wrong matches.
const wikiRecent = 180 * 24 * time.Hour

// EnableWiki makes the client add the wiki's tasks (off by default, so
// tests and tools stay offline).
func (c *Client) EnableWiki() *Client {
	c.wiki = true
	return c
}

// wikiQuestTitles returns the titles of the wiki's recent task pages that
// are at hand, at once: building the task list never waits for the wiki. A
// stale or missing list is fetched in the background; when it changes, the
// task lists are built again with it.
func (c *Client) wikiQuestTitles() []string {
	if !c.wiki {
		return nil
	}
	c.wikiMu.Lock()
	defer c.wikiMu.Unlock()
	if !c.wikiFetching && time.Now().After(c.wikiNext) {
		c.wikiFetching = true
		go c.refreshWiki()
	}
	return c.wikiTitles
}

func (c *Client) refreshWiki() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	titles, err := c.fetchWikiQuestTitles(ctx)
	c.wikiMu.Lock()
	c.wikiFetching = false
	// A failed fetch keeps the last list (or none) and is tried again soon.
	if err != nil || len(titles) == 0 {
		c.wikiNext = time.Now().Add(wikiRetry)
		c.wikiMu.Unlock()
		return
	}
	c.wikiNext = time.Now().Add(wikiRefresh)
	changed := !slices.Equal(c.wikiTitles, titles)
	c.wikiTitles = titles
	c.wikiMu.Unlock()
	if changed {
		c.Invalidate()
	}
}

func (c *Client) fetchWikiQuestTitles(ctx context.Context) ([]string, error) {
	quests, err := c.wikiCategory(ctx, "Quests")
	if err != nil {
		return nil, err
	}
	// Past events' tasks are marked on the wiki; they are not in the game.
	past := map[string]bool{}
	for _, category := range []string{"Event content", "Historical content"} {
		members, err := c.wikiCategory(ctx, category)
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			past[member.title] = true
		}
	}
	var titles []string
	for _, member := range quests {
		if !past[member.title] && time.Since(member.added) < wikiRecent {
			titles = append(titles, member.title)
		}
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
		if !past[member.title] && !slices.Contains(titles, member.title) {
			titles = append(titles, member.title)
		}
	}
	return titles, nil
}

type wikiMember struct {
	title string
	added time.Time
}

// wikiCategory lists a wiki category's pages, with when each was added.
func (c *Client) wikiCategory(ctx context.Context, category string) ([]wikiMember, error) {
	var members []wikiMember
	next := ""
	// The API returns 500 pages at a time; the categories have a few hundred.
	for page := 0; page < 10; page++ {
		query := url.Values{"action": {"query"}, "list": {"categorymembers"}, "cmtitle": {"Category:" + category}, "cmnamespace": {"0"}, "cmlimit": {"500"}, "cmprop": {"title|timestamp"}, "format": {"json"}}
		if next != "" {
			query.Set("cmcontinue", next)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikiAPI+"?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "MAYAK/0.1.0")
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
					Title string    `json:"title"`
					Added time.Time `json:"timestamp"`
				} `json:"categorymembers"`
			} `json:"query"`
		}
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, member := range body.Query.Members {
			members = append(members, wikiMember{member.Title, member.Added})
		}
		if next = body.Continue.Next; next == "" {
			break
		}
	}
	return members, nil
}

// appendWiki adds the wiki's tasks that quests does not have by name.
func appendWiki(quests []Quest, titles []string) []Quest {
	known := make(map[string]bool, len(quests))
	for _, quest := range quests {
		known[questmatch.Normalize(quest.Name)] = true
	}
	for _, title := range titles {
		title = strings.TrimSpace(title)
		key := questmatch.Normalize(title)
		// "Quests" is the category's own overview page.
		if key == "" || known[key] || strings.EqualFold(title, "Quests") {
			continue
		}
		known[key] = true
		quests = append(quests, Quest{
			Quest:    questmatch.Quest{ID: "wiki:" + title, Name: title},
			WikiLink: wikiPage + url.PathEscape(strings.ReplaceAll(title, " ", "_")),
		})
	}
	return quests
}
