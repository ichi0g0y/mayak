package squad

import (
	"encoding/json"
	"net/http"
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
}

type sealedReport struct {
	V int `json:"v"`
	Report
}

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
}

// Client is this PC in a squad room. It reconnects on its own until Close.
type Client struct {
	code     string
	url      string
	agent    string
	seal     sealer
	onChange func(State)

	mu      sync.Mutex
	writeMu sync.Mutex
	conn    *websocket.Conn
	phase   string
	myID    string
	mine    Report
	members map[string]Report
	done    chan struct{}
	closed  bool
}

// Join starts a client for code (canonical, see Normalize) against the
// rooms at endpoint, reporting every change of the room to onChange.
func Join(code, endpoint, userAgent string, mine Report, onChange func(State)) (*Client, error) {
	seal, err := newSealer(code)
	if err != nil {
		return nil, err
	}
	c := &Client{
		code:     code,
		url:      endpoint + RoomID(code),
		agent:    userAgent,
		seal:     seal,
		onChange: onChange,
		phase:    PhaseConnecting,
		mine:     clean(mine),
		members:  map[string]Report{},
		done:     make(chan struct{}),
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
	s := State{Code: c.code, Phase: c.phase, Members: []Member{{ID: c.myID, Me: true, Report: c.mine}}}
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
		wait = min(wait*2, time.Minute)
		c.setPhase(PhaseConnecting)
	}
}

// relayMessage is what the relay sends: welcome (with the members already
// there and their last messages), join, leave, and msg (a member's message).
type relayMessage struct {
	T       string `json:"t"`
	ID      string `json:"id"`
	From    string `json:"from"`
	Data    string `json:"data"`
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
			c.members = map[string]Report{}
			for _, other := range m.Members {
				if r, ok := c.openReport(other.Last); ok {
					c.members[other.ID] = r
				}
			}
			c.mu.Unlock()
			c.send(conn)
			c.changed()
		case "msg":
			if r, ok := c.openReport(m.Data); ok {
				c.mu.Lock()
				c.members[m.From] = r
				c.mu.Unlock()
				c.changed()
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
	if c.write(conn, []byte(c.seal.seal(body))) != nil {
		_ = conn.Close()
	}
}

func (c *Client) write(conn *websocket.Conn, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) openReport(text string) (Report, bool) {
	if text == "" {
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

// clean bounds what a report may hold, from this PC or another.
func clean(r Report) Report {
	r.Name = strings.TrimSpace(r.Name)
	if utf8.RuneCountInString(r.Name) > MaxName {
		r.Name = string([]rune(r.Name)[:MaxName])
	}
	if !validMap(r.Map) {
		r.Map = ""
	}
	if r.Map == "" {
		r.Pos = nil
		r.At = time.Time{}
	}
	return r
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
