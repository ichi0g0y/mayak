package questapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/questmatch"
)

const baseURL = "https://json.tarkov.dev/"

type Objective struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Maps        []string `json:"maps"`
}
type rawObjective struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Maps        []string `json:"maps"`
}
type rawTask struct {
	WikiLink       string         `json:"wikiLink"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Trader         string         `json:"trader"`
	Map            string         `json:"map"`
	Objectives     []rawObjective `json:"objectives"`
	NormalizedName string         `json:"normalizedName"`
}
type rawNamed struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
}
type taskEnvelope struct {
	Data struct {
		Tasks map[string]rawTask `json:"tasks"`
	} `json:"data"`
}
type mapEnvelope struct {
	Data struct {
		Maps map[string]rawNamed `json:"maps"`
	} `json:"data"`
}
type traderEnvelope struct {
	Data map[string]rawNamed `json:"data"`
}
type translationEnvelope struct {
	Data map[string]string `json:"data"`
}
type Quest struct {
	questmatch.Quest
	WikiLink   string      `json:"wikiLink"`
	Objectives []Objective `json:"objectives"`
}
type Client struct {
	source interface {
		Get(context.Context, string, string, any) error
	}
	http       *http.Client
	mu         sync.Mutex
	quests     []Quest
	loaded     time.Time
	modeQuests map[string][]Quest
	modeLoaded map[string]time.Time

	wiki       bool
	wikiMu     sync.Mutex
	wikiTitles []string
	// wikiNext is when the wiki's list is due again; wikiFetching while it is fetched.
	wikiNext     time.Time
	wikiFetching bool
	// wikiDone is closed when the fetch under way ends.
	wikiDone chan struct{}
}

func NewWithSource(source interface {
	Get(context.Context, string, string, any) error
}) *Client {
	c := New()
	c.source = source
	return c
}

func (c *Client) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = time.Time{}
	c.modeLoaded = make(map[string]time.Time)
}

func New() *Client {
	return &Client{
		http:       &http.Client{Timeout: 20 * time.Second},
		modeQuests: make(map[string][]Quest),
		modeLoaded: make(map[string]time.Time),
	}
}

func (c *Client) QuestsForMode(ctx context.Context, mode string) ([]Quest, error) {
	if mode == "" || mode == "auto" {
		return c.Quests(ctx)
	}
	if mode != "regular" && mode != "pve" && mode != "pvp-season" {
		return nil, errors.New("unsupported game mode")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if quests := c.modeQuests[mode]; len(quests) > 0 && time.Since(c.modeLoaded[mode]) < 12*time.Hour {
		return append([]Quest(nil), quests...), nil
	}
	quests, err := c.loadMode(ctx, mode)
	if err != nil {
		return nil, err
	}
	c.modeQuests[mode] = quests
	c.modeLoaded[mode] = time.Now()
	return append([]Quest(nil), quests...), nil
}

func (c *Client) Quests(ctx context.Context) ([]Quest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.quests) > 0 && time.Since(c.loaded) < 12*time.Hour {
		return append([]Quest(nil), c.quests...), nil
	}
	var tasks taskEnvelope
	var maps mapEnvelope
	var traders traderEnvelope
	var taskText, mapText, traderText translationEnvelope
	for _, item := range []struct {
		path   string
		target any
	}{{"tasks", &tasks}, {"tasks_en", &taskText}, {"maps", &maps}, {"maps_en", &mapText}, {"traders", &traders}, {"traders_en", &traderText}} {
		if err := c.get(ctx, "regular", item.path, item.target); err != nil {
			return nil, err
		}
	}
	if len(tasks.Data.Tasks) == 0 {
		return nil, errors.New("tarkov.dev returned an empty quest catalog")
	}
	out := buildQuests(tasks, taskText, maps, mapText, traders, traderText)
	c.addLocaleNames(ctx, "regular", tasks, out)
	knownIDs := make(map[string]bool, len(out))
	for _, quest := range out {
		knownIDs[quest.ID] = true
	}
	for _, mode := range []string{"pve", "pvp-season"} {
		var modeTasks taskEnvelope
		var modeTaskText translationEnvelope
		if c.get(ctx, mode, "tasks", &modeTasks) == nil && c.get(ctx, mode, "tasks_en", &modeTaskText) == nil {
			modeQuests := buildQuests(modeTasks, modeTaskText, maps, mapText, traders, traderText)
			c.addLocaleNames(ctx, mode, modeTasks, modeQuests)
			for _, quest := range modeQuests {
				if !knownIDs[quest.ID] {
					out = append(out, quest)
					knownIDs[quest.ID] = true
				}
			}
		}
	}
	known := make(map[string]bool, len(out))
	for _, quest := range out {
		known[questmatch.Normalize(quest.Name)] = true
	}
	for _, quest := range supplementalQuests {
		if !known[questmatch.Normalize(quest.Name)] {
			out = append(out, quest)
		}
	}
	out = appendWiki(out, c.wikiQuestTitles(ctx))
	c.quests = out
	c.loaded = time.Now()
	return append([]Quest(nil), out...), nil
}

func (c *Client) loadMode(ctx context.Context, mode string) ([]Quest, error) {
	var tasks taskEnvelope
	var maps mapEnvelope
	var traders traderEnvelope
	var taskText, mapText, traderText translationEnvelope
	for _, item := range []struct {
		path   string
		target any
	}{{"tasks", &tasks}, {"tasks_en", &taskText}, {"maps", &maps}, {"maps_en", &mapText}, {"traders", &traders}, {"traders_en", &traderText}} {
		if err := c.get(ctx, mode, item.path, item.target); err != nil {
			return nil, err
		}
	}
	if len(tasks.Data.Tasks) == 0 {
		return nil, errors.New("tarkov.dev returned an empty quest catalog")
	}
	quests := buildQuests(tasks, taskText, maps, mapText, traders, traderText)
	c.addLocaleNames(ctx, mode, tasks, quests)
	return appendWiki(appendSupplemental(quests), c.wikiQuestTitles(ctx)), nil
}

// Aliases share the stable task ID. Display names and Wiki links stay canonical.
// A missing locale must not make the English/offline catalog unusable.
func (c *Client) addLocaleNames(ctx context.Context, mode string, tasks taskEnvelope, quests []Quest) {
	for _, lang := range locale.Languages {
		c.addNames(ctx, mode, locale.Resource("tasks", lang), tasks, quests)
	}
}

func (c *Client) addNames(ctx context.Context, mode, resource string, tasks taskEnvelope, quests []Quest) {
	var names translationEnvelope
	if c.get(ctx, mode, resource, &names) != nil {
		return
	}
	keys := make(map[string]string, len(tasks.Data.Tasks))
	for id, task := range tasks.Data.Tasks {
		if task.ID != "" {
			id = task.ID
		}
		keys[id] = task.Name
	}
	for i := range quests {
		if name := names.Data[keys[quests[i].ID]]; name != "" && name != quests[i].Name {
			quests[i].Aliases = append(quests[i].Aliases, name)
		}
	}
}

func appendSupplemental(quests []Quest) []Quest {
	known := make(map[string]bool, len(quests))
	for _, quest := range quests {
		known[questmatch.Normalize(quest.Name)] = true
	}
	for _, quest := range supplementalQuests {
		if !known[questmatch.Normalize(quest.Name)] {
			quests = append(quests, quest)
		}
	}
	return quests
}

func buildQuests(tasks taskEnvelope, taskText translationEnvelope, maps mapEnvelope, mapText translationEnvelope, traders traderEnvelope, traderText translationEnvelope) []Quest {
	out := make([]Quest, 0, len(tasks.Data.Tasks))
	for id, q := range tasks.Data.Tasks {
		if q.ID == "" {
			q.ID = id
		}
		name := translate(taskText.Data, q.Name)
		trader := traders.Data[q.Trader]
		traderName := translate(traderText.Data, trader.Name)
		mapName := ""
		if m, ok := maps.Data.Maps[q.Map]; ok {
			mapName = m.NormalizedName
			if mapName == "" {
				mapName = translate(mapText.Data, m.Name)
			}
		}
		objectives := make([]Objective, 0, len(q.Objectives))
		for _, o := range q.Objectives {
			names := make([]string, 0, len(o.Maps))
			for _, mid := range o.Maps {
				if m, ok := maps.Data.Maps[mid]; ok {
					names = append(names, m.NormalizedName)
				}
			}
			objectives = append(objectives, Objective{ID: o.ID, Description: translate(taskText.Data, o.Description), Maps: names})
		}
		if mapName == "" {
			for _, objective := range objectives {
				if len(objective.Maps) > 0 {
					mapName = objective.Maps[0]
					break
				}
			}
		}
		out = append(out, Quest{Quest: questmatch.Quest{ID: q.ID, Name: name, Trader: traderName, Map: mapName, NormalizedName: q.NormalizedName}, WikiLink: q.WikiLink, Objectives: objectives})
	}
	return out
}

func (c *Client) get(ctx context.Context, mode, path string, target any) error {
	if c.source != nil {
		return c.source.Get(ctx, mode, path, target)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+mode+"/"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("tarkov.dev JSON API returned " + resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
func translate(dict map[string]string, key string) string {
	if v := dict[key]; v != "" {
		return v
	}
	return key
}
