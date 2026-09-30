package squad

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The squad's store on the relay (relay/worker SquadRoom): a slot per member
// that the relay keeps a week after the squad last used it, so that the
// squad pen's lines are still there when everyone left and someone comes
// back. A slot holds what the shell stores for this member (its own lines),
// sealed with the squad's code as the messages are; the slot's name is a
// hash of the member key, which the relay cannot turn back.

// MaxStored is the most a member stores, before sealing.
const MaxStored = 360 * 1024

var storeClient = &http.Client{Timeout: 20 * time.Second}

// storeEvery is how far apart a slot is written when the relay does not say
// (its welcome's limits.storeEvery): the relay answers one written sooner
// with 429, so the client waits rather than finds it.
var storeEvery = 5 * time.Second

// retryAfter is a 429's Retry-After (seconds), else fallback.
func retryAfter(res *http.Response, fallback time.Duration) time.Duration {
	var s int
	if res != nil {
		if _, err := fmt.Sscan(res.Header.Get("Retry-After"), &s); err == nil && s > 0 && s <= 600 {
			return time.Duration(s) * time.Second
		}
	}
	return fallback
}

// pace waits until slot name may be written again (the relay's storeEvery
// since it last was), and counts this write. It tells false once the client
// is closed.
func (c *Client) pace(name string) bool {
	c.mu.Lock()
	every := c.storeEvery
	c.mu.Unlock()
	if every <= 0 {
		every = storeEvery
	}
	c.storedMu.Lock()
	if c.storedAt == nil {
		c.storedAt = map[string]time.Time{}
	}
	wait := every - time.Since(c.storedAt[name])
	c.storedAt[name] = time.Now().Add(max(wait, 0))
	c.storedMu.Unlock()
	if wait <= 0 {
		return true
	}
	select {
	case <-c.done:
		return false
	case <-time.After(wait):
		return true
	}
}

// storeURL is the room's store over HTTP(S): the room's address with http
// for ws, and /store after it.
func (c *Client) storeURL() string {
	u := c.url
	switch {
	case strings.HasPrefix(u, "wss://"):
		u = "https://" + strings.TrimPrefix(u, "wss://")
	case strings.HasPrefix(u, "ws://"):
		u = "http://" + strings.TrimPrefix(u, "ws://")
	}
	return u + "/store"
}

// slot names this PC's slot: a hash of its member key.
func slot(key string) string {
	sum := sha256.Sum256([]byte("mayak-squad-slot\x00" + key))
	return hex.EncodeToString(sum[:])
}

// Store keeps data (JSON; empty to let go of the slot) in this member's
// slot, sealed.
func (c *Client) Store(data json.RawMessage) error { return c.StoreIn("", data) }

// StoreIn is Store in another of this member's slots, named by kind (the
// squad pen's lines are in the first, "", a picture in "picture"), so each
// is replaced on its own.
func (c *Client) StoreIn(kind string, data json.RawMessage) error {
	key := c.Mine().Key
	if !ValidKey(key) {
		return errors.New("no member key")
	}
	if len(data) > MaxStored {
		return errors.New("too much to store")
	}
	body := ""
	if len(data) > 0 {
		plain, err := json.Marshal(sealedMessage{V: 2, D: data})
		if err != nil {
			return err
		}
		body = c.seal.seal(plain)
	}
	name := slot(key)
	if kind != "" {
		name = slot(key + "\x00" + kind)
	}
	// Once more after a 429 (a write the relay found too soon), when it says.
	for try := 0; ; try++ {
		if !c.pace(name) {
			return errors.New("squad left")
		}
		req, err := http.NewRequest(http.MethodPut, c.storeURL()+"/"+name, bytes.NewReader([]byte(body)))
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.agent)
		res, err := storeClient.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && try == 0 {
			wait := retryAfter(res, storeEvery)
			c.storedMu.Lock()
			c.storedAt[name] = time.Now().Add(wait)
			c.storedMu.Unlock()
			continue
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("store: %s", res.Status)
		}
		return nil
	}
}

// Stored returns what the members keep in the store (each slot opened with
// the squad's key; one that does not open is left out). A relay without a
// store gives none.
func (c *Client) Stored() ([]json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodGet, c.storeURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.agent)
	res, err := storeClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("store: %s", res.Status)
	}
	var slots map[string]string
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&slots); err != nil {
		return nil, err
	}
	out := []json.RawMessage{}
	for _, text := range slots {
		if d, ok := c.openMessage(text); ok {
			out = append(out, d)
		}
	}
	return out, nil
}
