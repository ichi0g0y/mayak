package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/local/mayak/internal/model"
)

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
		name = strings.TrimSuffix(strings.ReplaceAll(name, ",", ""), " [PVE ZONE]")
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
	status, site := a.status, a.settings.QuestSite
	a.mu.RUnlock()
	if status.LastQuest == "" {
		return errors.New("no recognized task")
	}
	a.showBrowserTask(status, site)
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
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
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
		urls := map[string]string{}
		for _, s := range []string{"tarkov-dev", "official-wiki", "japanese-wiki"} {
			urls[s] = questStatusURL(s, status)
		}
		return urls
	}
	return nil
}
