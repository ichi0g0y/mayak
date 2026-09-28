package squad

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNormalizeAcceptsTypedForms(t *testing.T) {
	for _, in := range []string{"abcd-1234", "ABCD 1234", "ABCD1234", " abcd-1234 "} {
		got, err := Normalize(in)
		if err != nil || got != "ABCD1234" {
			t.Fatalf("Normalize(%q) = %q, %v", in, got, err)
		}
	}
	if got, _ := Normalize("OIL0-0000"); got != "01100000" {
		t.Fatalf("O, I and L read as digits: %q", got)
	}
	for _, in := range []string{"", "ABC", "ABCD-12345", "ABCU-1234", "ABCD-12!4"} {
		if _, err := Normalize(in); err == nil {
			t.Fatalf("Normalize(%q) accepted", in)
		}
	}
}

func TestNewCodeIsCanonical(t *testing.T) {
	for range 100 {
		code := NewCode()
		if got, err := Normalize(code); err != nil || got != code {
			t.Fatalf("NewCode() = %q, Normalize = %q, %v", code, got, err)
		}
	}
	if Format("ABCD1234") != "ABCD-1234" {
		t.Fatal("Format")
	}
}

func TestRoomIDHidesTheCode(t *testing.T) {
	id := RoomID("ABCD1234")
	if len(id) != 64 || strings.Contains(id, "ABCD1234") || id == RoomID("ABCD1235") {
		t.Fatalf("RoomID = %q", id)
	}
}

func TestSealOpensOnlyWithTheSameCode(t *testing.T) {
	a, _ := newSealer("ABCD1234")
	b, _ := newSealer("ABCD1235")
	text := a.seal([]byte("hello"))
	if plain, err := a.open(text); err != nil || string(plain) != "hello" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	if _, err := b.open(text); err == nil {
		t.Fatal("another code's key opened the message")
	}
	if _, err := a.open("not base64!"); err == nil {
		t.Fatal("garbage opened")
	}
}

func TestCleanBoundsReports(t *testing.T) {
	r := clean(Report{Name: "  " + strings.Repeat("あ", 40) + " ", Map: "Customs", Pos: &Position{X: 1}, At: time.Now()})
	if len([]rune(r.Name)) != MaxName || r.Map != "" || r.Pos != nil || !r.At.IsZero() {
		t.Fatalf("clean = %+v", r)
	}
	if r := clean(Report{Map: "ground-zero-21", Pos: &Position{X: 1}}); r.Map != "ground-zero-21" || r.Pos == nil {
		t.Fatalf("a valid map was dropped: %+v", r)
	}
}

// fakeRelay behaves like relay/worker/index.js for one room.
type fakeRelay struct {
	mu      sync.Mutex
	next    int
	members map[*websocket.Conn]*fakeMember
}

type fakeMember struct {
	id   string
	last string
	wmu  sync.Mutex
}

func (f *fakeRelay) send(conn *websocket.Conn, v any) {
	m := f.members[conn]
	m.wmu.Lock()
	defer m.wmu.Unlock()
	_ = conn.WriteJSON(v)
}

func (f *fakeRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	up := websocket.Upgrader{}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.next++
	me := &fakeMember{id: fmt.Sprint("m", f.next)}
	var others []map[string]string
	for _, m := range f.members {
		others = append(others, map[string]string{"id": m.id, "last": m.last})
	}
	f.members[conn] = me
	f.send(conn, map[string]any{"t": "welcome", "id": me.id, "members": others})
	for c := range f.members {
		if c != conn {
			f.send(c, map[string]string{"t": "join", "id": me.id})
		}
	}
	f.mu.Unlock()
	for {
		_, body, err := conn.ReadMessage()
		if err != nil {
			break
		}
		f.mu.Lock()
		if string(body) == "ping" {
			f.members[conn].wmu.Lock()
			_ = conn.WriteMessage(websocket.TextMessage, []byte("pong"))
			f.members[conn].wmu.Unlock()
		} else {
			me.last = string(body)
			for c := range f.members {
				if c != conn {
					f.send(c, map[string]string{"t": "msg", "from": me.id, "data": me.last})
				}
			}
		}
		f.mu.Unlock()
	}
	f.mu.Lock()
	delete(f.members, conn)
	for c := range f.members {
		f.send(c, map[string]string{"t": "leave", "id": me.id})
	}
	f.mu.Unlock()
	_ = conn.Close()
}

func waitFor(t *testing.T, states <-chan State, ok func(State) bool) State {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case s := <-states:
			if ok(s) {
				return s
			}
		case <-deadline:
			t.Fatal("timed out waiting for the room")
		}
	}
}

func member(s State, name string) *Member {
	for i := range s.Members {
		if s.Members[i].Name == name && !s.Members[i].Me {
			return &s.Members[i]
		}
	}
	return nil
}

func TestMembersSeeEachOthersReports(t *testing.T) {
	relay := &fakeRelay{members: map[*websocket.Conn]*fakeMember{}}
	srv := httptest.NewServer(relay)
	defer srv.Close()
	endpoint := "ws" + strings.TrimPrefix(srv.URL, "http") + "/squad/"

	aStates := make(chan State, 64)
	a, err := Join("ABCD1234", endpoint, "test", Report{Name: "Alice"}, func(s State) { aStates <- s })
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	waitFor(t, aStates, func(s State) bool { return s.Phase == PhaseConnected })

	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	a.Report(Report{Name: "Alice", Map: "customs", Pos: &Position{X: 1, Y: 2, Z: 3, Rot: 90}, At: at})

	bStates := make(chan State, 64)
	b, err := Join("ABCD1234", endpoint, "test", Report{Name: "Bob"}, func(s State) { bStates <- s })
	if err != nil {
		t.Fatal(err)
	}
	// Bob gets Alice's last report from the welcome.
	s := waitFor(t, bStates, func(s State) bool { return member(s, "Alice") != nil })
	if m := member(s, "Alice"); m.Map != "customs" || m.Pos == nil || m.Pos.Rot != 90 || !m.At.Equal(at) {
		t.Fatalf("Alice as Bob sees her: %+v", m)
	}
	// Alice gets Bob's hello.
	waitFor(t, aStates, func(s State) bool { return member(s, "Bob") != nil })

	// A different code in the same room cannot be read.
	cStates := make(chan State, 64)
	c, _ := Join("ZZZZ9999", endpoint, "test", Report{Name: "Eve"}, func(s State) { cStates <- s })
	s = waitFor(t, cStates, func(s State) bool { return s.Phase == PhaseConnected })
	if len(s.Members) != 1 {
		t.Fatalf("another code's client read the room: %+v", s.Members)
	}
	c.Close()

	b.Close()
	waitFor(t, aStates, func(s State) bool { return member(s, "Bob") == nil && s.Phase == PhaseConnected })
}

func TestReportsAreSealedOnTheWire(t *testing.T) {
	seen := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"t": "welcome", "id": "m1"})
		_, body, err := conn.ReadMessage()
		if err == nil {
			seen <- string(body)
		}
	}))
	defer srv.Close()
	c, _ := Join("ABCD1234", "ws"+strings.TrimPrefix(srv.URL, "http")+"/squad/", "test", Report{Name: "Alice", Map: "customs", Pos: &Position{X: 1}}, nil)
	defer c.Close()
	select {
	case body := <-seen:
		if strings.Contains(body, "Alice") || strings.Contains(body, "customs") || json.Valid([]byte(body)) {
			t.Fatalf("the relay could read the report: %q", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was sent")
	}
}
