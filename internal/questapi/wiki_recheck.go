package questapi

import (
	"context"
	"time"
)

// wikiRecheck is the least time between a fetch of the wiki's list and one
// made for a title that matched no task. A new task's page can come to the
// wiki hours before the list is due again: a screenshot of the task fetches
// it sooner, but however many screenshots fail, not more often than this.
const wikiRecheck = 15 * time.Minute

// RecheckWiki fetches the wiki's task list again for a title that matched no
// task, and waits for it up to wikiWait (or until ctx ends). It tells whether
// the list changed, so the title can be matched again. A list fetched less
// than wikiRecheck ago is not fetched again; a fetch under way is waited for.
func (c *Client) RecheckWiki(ctx context.Context) bool {
	if !c.wiki {
		return false
	}
	c.wikiMu.Lock()
	version := c.wikiVersion
	if !c.wikiFetching {
		if time.Since(c.wikiStarted) < wikiRecheck {
			c.wikiMu.Unlock()
			return false
		}
		c.wikiNext = time.Time{}
		c.startWikiLocked()
	}
	done := c.wikiDone
	c.wikiMu.Unlock()
	timer := time.NewTimer(wikiWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
	c.wikiMu.Lock()
	defer c.wikiMu.Unlock()
	return c.wikiVersion != version
}
