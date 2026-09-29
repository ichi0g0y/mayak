package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A relay that welcomes each member and then only reads.
func squadTestRelay(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"t": "welcome", "id": "m1"})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/squad/"
}

// Joining the squad already joined (the shell does at every reload) only
// reports again; it once deadlocked, and every squad call after it hung.
func TestSquadJoinAgainDoesNotHang(t *testing.T) {
	t.Setenv("MAYAK_SQUAD_RELAY", squadTestRelay(t))
	t.Setenv("APPDATA", t.TempDir())
	a := NewApp()
	defer a.SquadLeave()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := a.SquadJoin("ABCD-1234", "Alice"); err != nil {
			t.Error(err)
			return
		}
		time.Sleep(100 * time.Millisecond)
		if code, err := a.SquadJoin("abcd1234", "Alice"); err != nil || code != "ABCD-1234" {
			t.Errorf("join again = %q, %v", code, err)
		}
		if s := a.SquadState(); s == nil || s.Code != "ABCD1234" {
			t.Errorf("state = %+v", s)
		}
		if _, err := a.SquadJoin("ZZZZ-9999", "Alice"); err != nil {
			t.Error(err)
		}
		a.SquadLeave()
		if a.SquadState() != nil {
			t.Error("still in a squad after leaving")
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("squad calls hung")
	}
}
