package remote

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// CommandKey identifies a command by what it does (type, value, map), so a
// command sent and the same command relayed back by the server compare equal:
// the relayed copy drops the session ID and may format numbers differently.
func CommandKey(msg []byte) string {
	var message struct {
		Type string `json:"type"`
		Data struct {
			Type  string `json:"type"`
			Value string `json:"value"`
			Map   string `json:"map"`
		} `json:"data"`
	}
	if json.Unmarshal(msg, &message) != nil || message.Type != "command" {
		return ""
	}
	return message.Data.Type + "|" + message.Data.Value + "|" + message.Data.Map
}

// Watch joins the Remote Control session id the way a tarkov.dev page does
// and reports each command that arrives (CommandKey) until done is closed.
// It reconnects after a lost connection, waiting longer after each failure.
func Watch(done <-chan struct{}, id string, onCommand func(key string)) {
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("sessionid", id)
	u.RawQuery = q.Encode()
	wait := 5 * time.Second
	for {
		d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
		conn, _, err := d.Dial(u.String(), http.Header{"User-Agent": []string{userAgent}})
		if err == nil {
			wait = 5 * time.Second
			closed := make(chan struct{})
			go func() {
				select {
				case <-done:
				case <-closed:
				}
				_ = conn.Close()
			}()
			watchConn(conn, onCommand)
			close(closed)
		}
		select {
		case <-done:
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, time.Minute)
	}
}

func watchConn(conn *websocket.Conn, onCommand func(key string)) {
	for {
		_, body, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var message struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(body, &message) != nil {
			continue
		}
		switch message.Type {
		case "ping":
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		case "command":
			if key := CommandKey(body); key != "" {
				onCommand(key)
			}
		}
	}
}
