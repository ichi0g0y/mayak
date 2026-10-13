package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeSocket answers isConnected as tarkov.dev's socket does: connected when
// a socket with the asked session ID is open.
func fakeSocket(t *testing.T) (*httptest.Server, func(id string, open bool)) {
	t.Helper()
	var mu sync.Mutex
	pages := map[string]bool{}
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		self := r.URL.Query().Get("sessionid")
		for {
			var message struct {
				Type      string `json:"type"`
				SessionID string `json:"sessionID"`
			}
			if conn.ReadJSON(&message) != nil {
				return
			}
			if message.Type != "isConnected" || !strings.HasPrefix(self, message.SessionID) {
				continue
			}
			mu.Lock()
			connected := pages[message.SessionID]
			mu.Unlock()
			_ = conn.WriteJSON(map[string]any{"type": "connectionStatus", "data": map[string]bool{"connected": connected}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func(id string, open bool) {
		mu.Lock()
		pages[id] = open
		mu.Unlock()
	}
}

func TestViewingAsksWhetherAPageIsConnected(t *testing.T) {
	srv, page := fakeSocket(t)
	old := endpoint
	endpoint = "ws" + strings.TrimPrefix(srv.URL, "http")
	t.Cleanup(func() { endpoint = old })
	c := New("ABCD")
	defer c.Close()
	ask := func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		viewing, err := c.Viewing(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return viewing
	}
	if ask() {
		t.Fatal("no page is open")
	}
	page("ABCD", true)
	if !ask() {
		t.Fatal("the page is open")
	}
	page("ABCD", false)
	if ask() {
		t.Fatal("the page closed")
	}
}

func TestViewingFailsWhenTheSocketCannotBeReached(t *testing.T) {
	old := endpoint
	endpoint = "ws://127.0.0.1:1"
	t.Cleanup(func() { endpoint = old })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if viewing, err := New("ABCD").Viewing(ctx); viewing || err == nil {
		t.Fatalf("viewing=%v err=%v", viewing, err)
	}
}
