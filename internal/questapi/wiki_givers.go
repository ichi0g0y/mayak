package questapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/local/mayak/internal/version"
)

// knownWikiGivers is who gives each task of the wiki's list at hand.
func (c *Client) knownWikiGivers() map[string]string {
	c.wikiMu.Lock()
	defer c.wikiMu.Unlock()
	out := make(map[string]string, len(c.wikiTitles))
	for _, wq := range c.wikiTitles {
		if wq.Trader != "" {
			out[wq.Title] = wq.Trader
		}
	}
	return out
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
