package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/version"
)

// endpoint is tarkov.dev's socket (a variable for the tests).
var endpoint = "wss://socket.tarkov.dev"

// userAgent is the User-Agent the remote client sends.
func userAgent() string { return version.UserAgent() }

type Client struct {
	remoteID string
	mu       sync.Mutex
	conn     *websocket.Conn
	// viewing takes the answer to an isConnected question (Viewing).
	viewing chan bool
}

// ErrUnreachable marks a failure to connect to tarkov.dev's socket for an ID
// (the dial or its handshake): nothing is connected there, so nothing could
// be sent.
var ErrUnreachable = errors.New("remote control not reachable")

func New(remoteID string) *Client  { return &Client{remoteID: remoteID} }
func (c *Client) RemoteID() string { return c.remoteID }

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectLocked(ctx)
}
func (c *Client) connectLocked(ctx context.Context) error {
	if c.remoteID == "" {
		return errors.New("Remote ID is required")
	}
	if c.conn != nil {
		return nil
	}
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("sessionid", c.remoteID+"-esc")
	u.RawQuery = q.Encode()
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	h := http.Header{"User-Agent": []string{userAgent()}}
	conn, _, err := d.DialContext(ctx, u.String(), h)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	c.conn = conn
	go c.readLoop(conn)
	return nil
}

func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		_, body, err := conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.mu.Unlock()
			_ = conn.Close()
			return
		}
		var message struct {
			Type string `json:"type"`
			Data struct {
				Connected bool `json:"connected"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &message) != nil {
			continue
		}
		if message.Type == "connectionStatus" {
			c.mu.Lock()
			if c.viewing != nil {
				select {
				case c.viewing <- message.Data.Connected:
				default:
				}
			}
			c.mu.Unlock()
			continue
		}
		if message.Type != "ping" {
			continue
		}
		c.mu.Lock()
		if c.conn == conn {
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		}
		c.mu.Unlock()
	}
}

// Viewing asks tarkov.dev whether a page is connected on the ID (its map or
// task page with Remote Control on): the socket answers an isConnected
// question with a connectionStatus. MAYAK's own connection is no sign of a
// page, as the socket takes it with or without one.
func (c *Client) Viewing(ctx context.Context) (bool, error) {
	c.mu.Lock()
	if err := c.connectLocked(ctx); err != nil {
		c.mu.Unlock()
		return false, err
	}
	answer := make(chan bool, 1)
	c.viewing = answer
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err := c.conn.WriteJSON(map[string]string{"type": "isConnected", "sessionID": c.remoteID})
	if err != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	if err != nil {
		return false, err
	}
	defer func() {
		c.mu.Lock()
		if c.viewing == answer {
			c.viewing = nil
		}
		c.mu.Unlock()
	}()
	select {
	case connected := <-answer:
		return connected, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

type envelope struct {
	Type      string `json:"type"`
	Data      any    `json:"data"`
	SessionID string `json:"sessionID"`
}
type command struct {
	Type     string   `json:"type"`
	Value    string   `json:"value,omitempty"`
	Map      string   `json:"map,omitempty"`
	Position *point   `json:"position,omitempty"`
	Rotation *float64 `json:"rotation,omitempty"`
}
type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func MapPayload(remoteID, mapName string) ([]byte, error) {
	return json.Marshal(envelope{Type: "command", SessionID: remoteID, Data: command{Type: "map", Value: tarkovDevMapName(mapName)}})
}
func TaskPayload(remoteID, normalizedName string) ([]byte, error) {
	return json.Marshal(envelope{Type: "command", SessionID: remoteID, Data: command{Type: "task", Value: normalizedName}})
}
func PositionPayload(remoteID, mapName string, p model.Position) ([]byte, error) {
	r := p.Rotation
	return json.Marshal(envelope{Type: "command", SessionID: remoteID, Data: command{Type: "playerPosition", Map: tarkovDevMapName(mapName), Position: &point{p.X, p.Y, p.Z}, Rotation: &r}})
}

// tarkovDevMapName converts EFT variants to the stable map route used by the
// tarkov.dev remote. Ground Zero 21+ uses the same interactive map and
// coordinate transform as Ground Zero, while its alternate route is not
// consistently available during map-data updates.
func tarkovDevMapName(mapName string) string {
	if mapName == "ground-zero-21" {
		return "ground-zero"
	}
	return mapName
}

func PositionMessages(remoteID, mapName string, p model.Position, navigateMap bool) ([][]byte, error) {
	positionMessage, err := PositionPayload(remoteID, mapName, p)
	if err != nil {
		return nil, err
	}
	messages := [][]byte{positionMessage}
	if navigateMap {
		mapMessage, err := MapPayload(remoteID, mapName)
		if err != nil {
			return nil, err
		}
		messages = append(messages, mapMessage)
	}
	return messages, nil
}

func (c *Client) SendPosition(ctx context.Context, mapName string, p model.Position, navigateMap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connectLocked(ctx); err != nil {
		return err
	}
	messages, err := PositionMessages(c.remoteID, mapName, p, navigateMap)
	if err != nil {
		return err
	}
	// Store the marker first so optional navigation can immediately select the
	// correct floor from the position's Y coordinate.
	for _, msg := range messages {
		_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			_ = c.conn.Close()
			c.conn = nil
			return err
		}
	}
	return nil
}

func (c *Client) SendMap(ctx context.Context, mapName string) error {
	msg, err := MapPayload(c.remoteID, mapName)
	return c.send(ctx, msg, err)
}

func (c *Client) SendTask(ctx context.Context, normalizedName string) error {
	msg, err := TaskPayload(c.remoteID, normalizedName)
	return c.send(ctx, msg, err)
}

func (c *Client) send(ctx context.Context, msg []byte, payloadErr error) error {
	if payloadErr != nil {
		return payloadErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connectLocked(ctx); err != nil {
		return err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		_ = c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
