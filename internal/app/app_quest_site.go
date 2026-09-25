package app

import (
	"errors"
	"github.com/local/mayak/internal/model"
	"net/url"
	"strings"
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
		return "https://wikiwiki.jp/eft/?cmd=search&word=" + url.QueryEscape(name)
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
