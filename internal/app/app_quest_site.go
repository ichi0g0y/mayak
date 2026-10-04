package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/version"
)

// questSiteKeys are the task sites, in their default order.
var questSiteKeys = []string{"tarkov-dev", "official-wiki", "japanese-wiki"}

// normalizeQuestSites is the task sites in the order set: every site once,
// the ones left out after in their default order. first, a site set alone
// (by a version that knows one site only), goes first.
func normalizeQuestSites(order []string, first string) []string {
	out := make([]string, 0, len(questSiteKeys))
	for _, site := range order {
		if slices.Contains(questSiteKeys, site) && !slices.Contains(out, site) {
			out = append(out, site)
		}
	}
	for _, site := range questSiteKeys {
		if !slices.Contains(out, site) {
			out = append(out, site)
		}
	}
	if slices.Contains(questSiteKeys, first) && out[0] != first {
		out = append([]string{first}, slices.DeleteFunc(out, func(site string) bool { return site == first })...)
	}
	return out
}

// questSiteURLs is a task's page on each task site.
func questSiteURLs(status model.Status) map[string]string {
	urls := make(map[string]string, len(questSiteKeys))
	for _, site := range questSiteKeys {
		urls[site] = questStatusURL(site, status)
	}
	return urls
}

// questSiteFor is the first site in order that has the task's page (urls,
// questSiteURLs), or the first site when none has.
func questSiteFor(ctx context.Context, order []string, urls map[string]string) string {
	order = normalizeQuestSites(order, "")
	for _, site := range order {
		if questPages.exists(ctx, urls[site]) {
			return site
		}
	}
	return order[0]
}

// A wiki's answer about a page is kept a while: a page found for long, a
// missing one for less, as it may be written soon (a task new to the game).
const questPageFoundFor = 12 * time.Hour
const questPageMissingFor = 30 * time.Minute

type questPageCheck struct {
	exists bool
	at     time.Time
}

type questPageChecker struct {
	client *http.Client
	mu     sync.Mutex
	known  map[string]questPageCheck
}

var questPages = &questPageChecker{client: &http.Client{Timeout: 5 * time.Second}, known: map[string]questPageCheck{}}

// exists tells whether a task's page is there. tarkov.dev has a page for the
// tasks in its catalog (/task/…; the task list stands in for the others).
// A wiki is asked (HEAD): a page it says is not there (404, 410) is missing;
// one it does not answer for is taken as there, so a wiki down does not move
// tasks to another site.
func (c *questPageChecker) exists(ctx context.Context, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	switch u.Host {
	case "tarkov.dev":
		return strings.HasPrefix(u.Path, "/task/")
	case "escapefromtarkov.fandom.com", "wikiwiki.jp":
	default:
		return false
	}
	c.mu.Lock()
	check, ok := c.known[raw]
	c.mu.Unlock()
	keep := questPageMissingFor
	if check.exists {
		keep = questPageFoundFor
	}
	if ok && time.Since(check.at) < keep {
		return check.exists
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return true
	}
	req.Header.Set("User-Agent", version.UserAgent())
	response, err := c.client.Do(req)
	if err != nil {
		return true
	}
	response.Body.Close()
	if response.StatusCode >= 500 {
		return true
	}
	exists := response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusGone
	c.mu.Lock()
	// A few thousand pages at most; a list grown past that starts again.
	if len(c.known) > 4000 {
		c.known = map[string]questPageCheck{}
	}
	c.known[raw] = questPageCheck{exists: exists, at: time.Now()}
	c.mu.Unlock()
	return exists
}

// QuestSiteFor is the site a task opens on, for the shell: the first in order
// that has the task's page (urls, as QuestSiteURLs gives them).
func (a *App) QuestSiteFor(order []string, urls map[string]string) string {
	ctx, cancel := context.WithTimeout(a.parentContext(), 8*time.Second)
	defer cancel()
	return questSiteFor(ctx, order, urls)
}

func (a *App) parentContext() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}

func normalizeQuestSite(site string) string {
	switch site {
	case "official-wiki", "japanese-wiki":
		return site
	default:
		return "tarkov-dev"
	}
}

func questStatusURL(site string, status model.Status) string {
	if normalizeQuestSite(site) == "official-wiki" {
		if parsed, err := url.Parse(status.QuestWikiURL); err == nil && parsed.Scheme == "https" && parsed.Host == "escapefromtarkov.fandom.com" && parsed.User == nil && strings.HasPrefix(parsed.Path, "/wiki/") {
			return parsed.String()
		}
	}
	return questPageURL(site, status.LastQuest, status.QuestTrader, status.QuestURL)
}

func questPageURL(site, name, trader, tarkovURL string) string {
	switch normalizeQuestSite(site) {
	case "official-wiki":
		return "https://escapefromtarkov.fandom.com/wiki/" + url.PathEscape(strings.ReplaceAll(name, " ", "_"))
	case "japanese-wiki":
		// The wiki titles pages without commas or the " [PVE ZONE]" suffix
		// ("Camera Action!" for "Camera, Action!").
		// The official wiki tells a story chapter from a task of the same name
		// with " (story chapter)"; the Japanese one files it under ストーリータスク.
		name = strings.TrimSuffix(strings.ReplaceAll(name, ",", ""), " [PVE ZONE]")
		name = strings.TrimSuffix(name, " (story chapter)")
		if trader != "" {
			return "https://wikiwiki.jp/eft/" + url.PathEscape(trader) + "/" + url.PathEscape(name)
		}
		// A task without a trader is a story chapter ("Batya", "Tour"): the
		// wiki has them under ストーリータスク.
		if name != "" {
			return "https://wikiwiki.jp/eft/" + url.PathEscape("ストーリータスク") + "/" + url.PathEscape(name)
		}
		return "https://wikiwiki.jp/eft/" + url.PathEscape("タスク")
	default:
		if tarkovURL != "" {
			return tarkovURL
		}
		return "https://tarkov.dev/tasks/"
	}
}

func (a *App) OpenQuestPage() error {
	a.mu.RLock()
	status, order := a.status, a.settings.QuestSites
	a.mu.RUnlock()
	if status.LastQuest == "" {
		return errors.New("no recognized task")
	}
	go a.showBrowserTask(status, order)
	return nil
}

// QuestSiteURLs is a task's page on each task site, from the task list at
// hand: a task tab kept from before keeps the pages it opened with, which a
// newer list may know better (a trader read since, for the Japanese wiki).
// Empty when the task is not in the list.
func (a *App) QuestSiteURLs(id, name string) map[string]string {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	ctx, cancel := context.WithTimeout(a.parentContext(), 15*time.Second)
	defer cancel()
	quests, err := a.questClient.QuestsForMode(ctx, a.effectiveCatalogMode(settings.GameMode))
	if err != nil {
		return nil
	}
	for _, q := range quests {
		if (id == "" || q.ID != id) && (name == "" || q.Name != name) {
			continue
		}
		status := model.Status{LastQuest: q.Name, QuestTrader: q.Trader, QuestWikiURL: q.WikiLink}
		if q.NormalizedName != "" {
			status.QuestURL = "https://tarkov.dev/task/" + q.NormalizedName
		}
		return questSiteURLs(status)
	}
	return nil
}
