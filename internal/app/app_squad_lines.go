package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/squad"
	"github.com/local/mayak/internal/userdata"
)

// What the squad pen (map-draw.js) keeps on this PC: its member key, which
// its lines in a squad are owned by across reconnections and restarts, and
// its own lines in each squad lately, to send again when it comes back (the
// relay keeps nothing). Both belong to this PC (docs/user-data.md).

var (
	squadKeyOnce sync.Once
	squadKey     string

	squadLinesMu sync.Mutex
)

// squadLinesAge is how long a squad's lines are kept after they last changed.
const squadLinesAge = 24 * time.Hour

// maxSquadLinesBytes bounds the lines of one squad the shell may hand over.
const maxSquadLinesBytes = 4 << 20

// squadMemberKey is this PC's member key, made once and kept in squad-key.txt.
func squadMemberKey() string {
	squadKeyOnce.Do(func() {
		p, err := appdir.Path("squad-key.txt")
		if err != nil {
			return
		}
		if b, err := os.ReadFile(p); err == nil && squad.ValidKey(strings.TrimSpace(string(b))) {
			squadKey = strings.TrimSpace(string(b))
			return
		}
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		key := hex.EncodeToString(b)
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
		if userdata.WriteAtomic(p, []byte(key)) == nil {
			squadKey = key
		}
	})
	return squadKey
}

// SquadMemberKey returns this PC's member key ("" when it could not be kept).
func (a *App) SquadMemberKey() string { return squadMemberKey() }

type squadLinesEntry struct {
	SavedAt time.Time       `json:"savedAt"`
	Data    json.RawMessage `json:"data"`
}

func loadSquadLines() (map[string]squadLinesEntry, string, error) {
	p, err := appdir.Path("squad-lines.json")
	if err != nil {
		return nil, "", err
	}
	all := map[string]squadLinesEntry{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &all)
	}
	for room, e := range all {
		if time.Since(e.SavedAt) > squadLinesAge {
			delete(all, room)
		}
	}
	return all, p, nil
}

// SquadLinesLoad returns what the shell kept for the squad of code (its own
// lines there), or "" for none or when older than a day.
func (a *App) SquadLinesLoad(code string) (string, error) {
	canonical, err := squad.Normalize(code)
	if err != nil {
		return "", err
	}
	squadLinesMu.Lock()
	defer squadLinesMu.Unlock()
	all, _, err := loadSquadLines()
	if err != nil {
		return "", err
	}
	return string(all[squad.RoomID(canonical)].Data), nil
}

// SquadLinesSave keeps what the shell hands over for the squad of code
// (JSON), under the squad's room ID rather than its code.
func (a *App) SquadLinesSave(code, data string) error {
	canonical, err := squad.Normalize(code)
	if err != nil {
		return err
	}
	if len(data) > maxSquadLinesBytes || !json.Valid([]byte(data)) {
		return errors.New("invalid squad lines")
	}
	squadLinesMu.Lock()
	defer squadLinesMu.Unlock()
	all, p, err := loadSquadLines()
	if err != nil {
		return err
	}
	all[squad.RoomID(canonical)] = squadLinesEntry{SavedAt: time.Now().UTC(), Data: json.RawMessage(data)}
	b, err := json.Marshal(all)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return userdata.WriteAtomic(p, b)
}
