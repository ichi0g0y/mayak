package squad

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

// Endpoint is the relay's squad rooms; the room ID follows.
const Endpoint = "wss://mayak-relay.ich.sh/squad/"

// MaxName is the longest display name sent, in characters.
const MaxName = 24

const (
	pingEvery   = 30 * time.Second
	readTimeout = 75 * time.Second
	writeWait   = 5 * time.Second
)

// Position is a player's place in a raid, in the game's coordinates, and
// which way they face (degrees).
type Position struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Z   float64 `json:"z"`
	Rot float64 `json:"rot"`
}

// Report is what a member tells the room: their name and, while in a raid,
// the map and their last position. An empty Map is "not in a raid".
type Report struct {
	Name string `json:"name"`
	// Viewer is a PC that only watches: a client of a Host in the squad,
	// the same player, so the others leave it out of their list.
	Viewer bool      `json:"viewer,omitempty"`
	Map    string    `json:"map,omitempty"`
	Pos    *Position `json:"pos,omitempty"`
	// At is when the position was taken.
	At time.Time `json:"at,omitzero"`
	// Color is the squad colour chosen ("#rrggbb"), or empty for one given
	// by the others (map-geo.js assignColors). Older builds leave it out.
	Color string `json:"color,omitempty"`
	// Key is this PC's member key (32 hex digits), the same across
	// reconnections, which the squad's lines are owned by (map-draw.js).
	Key string `json:"key,omitempty"`
	// Tracker is this player's TarkovTracker progress in short, when they
	// sync with it: shown on their card in the others' sidebar.
	Tracker *Tracker `json:"tracker,omitempty"`
	// Profile is what this player's Overall screen showed when last read.
	Profile *Profile `json:"profile,omitempty"`
}

// Profile is a player's Overall screen in short: the nickname, the level,
// raids, kills, the survival rate, K/D, hours online, and when the picture
// of their character was taken (unix seconds; 0 for none), which the
// others fetch from the squad's store (StoreIn "picture").
type Profile struct {
	Name         string  `json:"name,omitempty"`
	Level        int     `json:"level,omitempty"`
	Raids        int     `json:"raids,omitempty"`
	Kills        int     `json:"kills,omitempty"`
	SurvivalRate float64 `json:"sr,omitempty"`
	KD           float64 `json:"kd,omitempty"`
	Hours        float64 `json:"hours,omitempty"`
	Picture      int64   `json:"picture,omitempty"`
}

// Tracker is a player's TarkovTracker progress in short: the name there,
// the level, the game mode (pvp, pve), the tasks completed and failed, and
// the TarkovTracker user (for the link to their shared profile).
type Tracker struct {
	Name   string `json:"name,omitempty"`
	Level  int    `json:"level,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Done   int    `json:"done,omitempty"`
	Failed int    `json:"failed,omitempty"`
	User   string `json:"user,omitempty"`
}

type sealedReport struct {
	V int `json:"v"`
	Report
}

// sealedMessage is a message of the shell's (the squad pen's lines, its
// position; version 2), passed on as it is. Builds before it drop it.
type sealedMessage struct {
	V int             `json:"v"`
	D json.RawMessage `json:"d"`
}

// MaxMessage is the longest message the relay takes, in characters.
const MaxMessage = 4096

// Ephemeral marks a message the relay passes on without keeping it as the
// member's last one (which it hands to those who join after): only reports
// are kept.
const Ephemeral = "~"

// The relay closes a member that sends more than its rate (told in its
// welcome; 120 before it did) in 10 s. This PC keeps to five sixths of it in
// any 10 s, and keeps a sixth of that for what must go (lines, erasing,
// shares): what may be dropped (the pen's position, the points of a line
// being drawn) stops short of it.
const (
	relayRate  = 120
	sendWindow = 10 * time.Second
)

// limits are the most messages sent in sendWindow, and the most of them
// droppable, for a relay taking rate.
func limits(rate int) (all, droppable int) {
	if rate <= 0 {
		rate = relayRate
	}
	all = rate * 5 / 6
	return all, all - all/6
}

// A relay deployed while this PC stays connected keeps the connection (the
// room hibernates), so its welcome, which tells the relay's version, is not
// said again: connected to one that did not take the squad pen, the client
// connects again this often to ask anew (and when asked, Recheck).
const recheckEvery = 10 * time.Minute

// Member is one player in the room, as last reported.
type Member struct {
	ID string `json:"id"`
	Me bool   `json:"me"`
	Report
}

// Phases of the connection.
const (
	PhaseConnecting = "connecting"
	PhaseConnected  = "connected"
	PhaseOffline    = "offline"
	PhaseFull       = "full"
)

// State is the room as this PC sees it.
type State struct {
	Code    string   `json:"code"`
	Phase   string   `json:"phase"`
	Members []Member `json:"members"`
	// Drawing is whether the relay takes the squad pen's messages (it
	// said so in its welcome); an older relay would drop this PC for them.
	Drawing bool `json:"drawing"`
}

// Client is this PC in a squad room. It reconnects on its own until Close.
type Client struct {
	code      string
	url       string
	agent     string
	seal      sealer
	onChange  func(State)
	onMessage func(from string, data json.RawMessage)

	mu      sync.Mutex
	writeMu sync.Mutex
	conn    *websocket.Conn
	phase   string
	myID    string
	mine    Report
	members map[string]Report
	drawing bool
	// rate is the relay's (its welcome's), 0 for one that did not say.
	rate int
	// recheck makes the next connection at once, without the pause.
	recheck bool
	done    chan struct{}
	closed  bool

	// When the messages of the last sendWindow were sent, oldest first.
	sentMu sync.Mutex
	sent   []time.Time
}

// Join starts a client for code (canonical, see Normalize) against the
// rooms at endpoint, reporting every change of the room to onChange and
// every message of the shell's (Send) from another member to onMessage.
func Join(code, endpoint, userAgent string, mine Report, onChange func(State), onMessage func(from string, data json.RawMessage)) (*Client, error) {
	seal, err := newSealer(code)
	if err != nil {
		return nil, err
	}
	c := &Client{
		code:      code,
		url:       endpoint + RoomID(code),
		agent:     userAgent,
		seal:      seal,
		onChange:  onChange,
		onMessage: onMessage,
		phase:     PhaseConnecting,
		mine:      clean(mine),
		members:   map[string]Report{},
		done:      make(chan struct{}),
	}
	go c.run()
	return c, nil
}

// Code is the room's code (canonical).
func (c *Client) Code() string { return c.code }

// Done is closed when the client is closed.
func (c *Client) Done() <-chan struct{} { return c.done }

// Mine is what this PC tells the room now.
func (c *Client) Mine() Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mine
}

// Close leaves the room.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.done)
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		c.writeMu.Lock()
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "leave"))
		c.writeMu.Unlock()
		_ = conn.Close()
	}
}

// Report replaces what this PC tells the room and sends it now when
// connected (otherwise on the next connection).
func (c *Client) Report(r Report) {
	c.mu.Lock()
	c.mine = clean(r)
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		c.send(conn)
	}
	c.changed()
}

// State returns the room now.
func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Client) stateLocked() State {
	s := State{Code: c.code, Phase: c.phase, Drawing: c.drawing, Members: []Member{{ID: c.myID, Me: true, Report: c.mine}}}
	for id, r := range c.members {
		s.Members = append(s.Members, Member{ID: id, Report: r})
	}
	sort.SliceStable(s.Members[1:], func(i, j int) bool {
		a, b := s.Members[1+i], s.Members[1+j]
		if a.Name != b.Name {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.ID < b.ID
	})
	return s
}

func (c *Client) changed() {
	if c.onChange != nil {
		c.onChange(c.State())
	}
}

func (c *Client) setPhase(phase string) {
	c.mu.Lock()
	same := c.phase == phase
	c.phase = phase
	c.mu.Unlock()
	if !same {
		c.changed()
	}
}

func (c *Client) run() {
	wait := 5 * time.Second
	for {
		d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
		conn, resp, err := d.Dial(c.url, http.Header{"User-Agent": []string{c.agent}})
		if err == nil {
			wait = 5 * time.Second
			c.serve(conn)
			c.mu.Lock()
			again := c.recheck && !c.closed
			c.recheck = false
			c.mu.Unlock()
			if again {
				c.setPhase(PhaseConnecting)
				continue
			}
		} else if resp != nil && resp.StatusCode == http.StatusConflict {
			c.setPhase(PhaseFull)
			wait = time.Minute
		} else {
			c.setPhase(PhaseOffline)
		}
		select {
		case <-c.done:
			return
		case <-time.After(wait):
		}
		// At most 15 s apart: in a raid a longer wait keeps the squad away.
		wait = min(wait*2, 15*time.Second)
		c.setPhase(PhaseConnecting)
	}
}

// relayMessage is what the relay sends: welcome (with the members already
// there and their last messages), join, leave, and msg (a member's message).
type relayMessage struct {
	T    string `json:"t"`
	ID   string `json:"id"`
	From string `json:"from"`
	Data string `json:"data"`
	// V is the relay's version (2 takes the squad pen's messages).
	V int `json:"v"`
	// Rate is how many messages a member may send in 10 s (0: 120).
	Rate    int `json:"rate"`
	Members []struct {
		ID   string `json:"id"`
		Last string `json:"last"`
	} `json:"members"`
}

func (c *Client) serve(conn *websocket.Conn) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close()
		return
	}
	c.conn = conn
	c.mu.Unlock()
	stop := make(chan struct{})
	go c.ping(conn, stop)
	defer func() {
		close(stop)
		_ = conn.Close()
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.members = map[string]Report{}
		c.mu.Unlock()
		c.setPhase(PhaseOffline)
	}()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	for {
		kind, body, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		if kind != websocket.TextMessage || string(body) == "pong" {
			continue
		}
		var m relayMessage
		if json.Unmarshal(body, &m) != nil {
			continue
		}
		switch m.T {
		case "welcome":
			c.mu.Lock()
			c.myID = m.ID
			c.phase = PhaseConnected
			c.drawing = m.V >= 2
			c.rate = m.Rate
			c.members = map[string]Report{}
			for _, other := range m.Members {
				if r, ok := c.openReport(other.Last); ok {
					c.members[other.ID] = r
				}
			}
			drawing := c.drawing
			c.mu.Unlock()
			c.send(conn)
			c.changed()
			if !drawing {
				go func() {
					select {
					case <-stop:
					case <-time.After(recheckEvery):
						c.Recheck()
					}
				}()
			}
		case "msg":
			if r, ok := c.openReport(m.Data); ok {
				c.mu.Lock()
				c.members[m.From] = r
				c.mu.Unlock()
				c.changed()
			} else if d, ok := c.openMessage(m.Data); ok && c.onMessage != nil {
				c.onMessage(m.From, d)
			}
		case "leave":
			c.mu.Lock()
			_, known := c.members[m.ID]
			delete(c.members, m.ID)
			c.mu.Unlock()
			if known {
				c.changed()
			}
		}
	}
}

func (c *Client) ping(conn *websocket.Conn, stop <-chan struct{}) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if c.write(conn, []byte("ping")) != nil {
				_ = conn.Close()
				return
			}
		}
	}
}

func (c *Client) send(conn *websocket.Conn) {
	c.mu.Lock()
	body, err := json.Marshal(sealedReport{V: 1, Report: c.mine})
	c.mu.Unlock()
	if err != nil {
		return
	}
	c.take(false)
	if c.write(conn, []byte(c.seal.seal(body))) != nil {
		_ = conn.Close()
	}
}

// Recheck connects again at once when connected to a relay that did not
// say it takes the squad pen, to hear its version again (a relay deployed
// since this PC connected).
func (c *Client) Recheck() {
	c.mu.Lock()
	conn := c.conn
	stale := conn != nil && !c.drawing && c.phase == PhaseConnected
	if stale {
		c.recheck = true
	}
	c.mu.Unlock()
	if stale {
		_ = conn.Close()
	}
}

// Send seals data (JSON) as a message of the shell's and sends it to the
// others, ephemeral (not kept by the relay for those who join after). A
// droppable one (the pen's position, the points of a line being drawn) is
// dropped when this PC has sent its fill lately; any other waits for room.
// It tells whether the message went.
func (c *Client) Send(data json.RawMessage, droppable bool) bool {
	body, err := json.Marshal(sealedMessage{V: 2, D: data})
	if err != nil {
		return false
	}
	text := Ephemeral + c.seal.seal(body)
	if len(text) > MaxMessage {
		return false
	}
	c.mu.Lock()
	conn, drawing := c.conn, c.drawing
	c.mu.Unlock()
	if conn == nil || !drawing || !c.take(droppable) {
		return false
	}
	if c.write(conn, []byte(text)) != nil {
		_ = conn.Close()
		return false
	}
	return true
}

// take counts a message about to be sent against the limits: it tells
// false for a droppable one over its limit, and waits for room for any other.
func (c *Client) take(droppable bool) bool {
	c.mu.Lock()
	all, few := limits(c.rate)
	c.mu.Unlock()
	limit := all
	if droppable {
		limit = few
	}
	for {
		c.sentMu.Lock()
		now := time.Now()
		for len(c.sent) > 0 && now.Sub(c.sent[0]) >= sendWindow {
			c.sent = c.sent[1:]
		}
		if len(c.sent) < limit {
			c.sent = append(c.sent, now)
			c.sentMu.Unlock()
			return true
		}
		wait := sendWindow - now.Sub(c.sent[0])
		c.sentMu.Unlock()
		if droppable {
			return false
		}
		select {
		case <-c.done:
			return false
		case <-time.After(wait):
		}
	}
}

func (c *Client) write(conn *websocket.Conn, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) openReport(text string) (Report, bool) {
	if text == "" || strings.HasPrefix(text, Ephemeral) {
		return Report{}, false
	}
	plain, err := c.seal.open(text)
	if err != nil {
		return Report{}, false
	}
	var r sealedReport
	if json.Unmarshal(plain, &r) != nil || r.V != 1 {
		return Report{}, false
	}
	return clean(r.Report), true
}

// openMessage opens a message of the shell's (version 2).
func (c *Client) openMessage(text string) (json.RawMessage, bool) {
	plain, err := c.seal.open(strings.TrimPrefix(text, Ephemeral))
	if err != nil {
		return nil, false
	}
	var m sealedMessage
	if json.Unmarshal(plain, &m) != nil || m.V != 2 || len(m.D) == 0 {
		return nil, false
	}
	return m.D, true
}

// clean bounds what a report may hold, from this PC or another.
func clean(r Report) Report {
	r.Name = strings.TrimSpace(r.Name)
	if utf8.RuneCountInString(r.Name) > MaxName {
		r.Name = string([]rune(r.Name)[:MaxName])
	}
	if !validMap(r.Map) {
		r.Map = ""
	}
	if !ValidColor(r.Color) {
		r.Color = ""
	}
	if !ValidKey(r.Key) {
		r.Key = ""
	}
	if r.Map == "" {
		r.Pos = nil
		r.At = time.Time{}
	}
	if r.Tracker != nil {
		t := *r.Tracker
		t.Name = strings.TrimSpace(t.Name)
		if utf8.RuneCountInString(t.Name) > MaxName*2 {
			t.Name = string([]rune(t.Name)[:MaxName*2])
		}
		if t.Level < 0 || t.Level > 200 {
			t.Level = 0
		}
		if t.Mode != "pvp" && t.Mode != "pve" {
			t.Mode = ""
		}
		if t.Done < 0 || t.Done > 10000 {
			t.Done = 0
		}
		if t.Failed < 0 || t.Failed > 10000 {
			t.Failed = 0
		}
		if !trackerUser.MatchString(t.User) {
			t.User = ""
		}
		r.Tracker = &t
		if t == (Tracker{}) {
			r.Tracker = nil
		}
	}
	if r.Profile != nil {
		p := *r.Profile
		if !nickname.MatchString(p.Name) {
			p.Name = ""
		}
		if p.Level < 0 || p.Level > 79 {
			p.Level = 0
		}
		p.Raids, p.Kills = min(max(p.Raids, 0), 1000000), min(max(p.Kills, 0), 1000000)
		if p.SurvivalRate < 0 || p.SurvivalRate > 100 {
			p.SurvivalRate = 0
		}
		if p.KD < 0 || p.KD > 1000 {
			p.KD = 0
		}
		if p.Hours < 0 || p.Hours > 100000 {
			p.Hours = 0
		}
		p.Picture = max(p.Picture, 0)
		r.Profile = &p
		if p == (Profile{}) {
			r.Profile = nil
		}
	}
	return r
}

// nickname is an EFT nickname.
var nickname = regexp.MustCompile(`^[A-Za-z0-9_-]{3,15}$`)

// trackerUser is a TarkovTracker user id: a UUID.
var trackerUser = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidColor tells whether c is a colour as a report carries one: "#rrggbb"
// in lower case.
func ValidColor(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// ValidKey tells whether k is a member key: 32 hex digits in lower case.
func ValidKey(k string) bool {
	if len(k) != 32 {
		return false
	}
	for _, r := range k {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func validMap(name string) bool {
	if name == "" || len(name) > 60 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
