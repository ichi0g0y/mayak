// Package userdata keeps the user's data that follows them from PC to PC
// (and may be synced one day) in forms that merge: preferences key by key,
// each with when it last changed, and collections record by record, each
// with an id, when it last changed and whether it was deleted. What belongs
// to one PC (folders, window places, pairing ids) and what can be made again
// (caches) stays out of it. docs/user-data.md has what is where.
package userdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Keyed is a set of preferences, each with when it last changed, so that two
// copies merge key by key: the later change of a key wins.
type Keyed struct {
	Version   int                        `json:"version"`
	Values    map[string]json.RawMessage `json:"values"`
	UpdatedAt map[string]time.Time       `json:"updatedAt"`
}

const keyedVersion = 1

// LoadKeyed reads a Keyed file; a missing one is empty (ok is false), a
// damaged one is read from its .bak.
func LoadKeyed(path string) (doc Keyed, ok bool, err error) {
	doc = Keyed{Version: keyedVersion, Values: map[string]json.RawMessage{}, UpdatedAt: map[string]time.Time{}}
	for _, p := range []string{path, path + ".bak"} {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			if p == path && !os.IsNotExist(readErr) {
				err = readErr
			}
			continue
		}
		var got Keyed
		if json.Unmarshal(b, &got) != nil || got.Values == nil {
			err = errors.New(filepath.Base(path) + " is damaged")
			continue
		}
		if got.UpdatedAt == nil {
			got.UpdatedAt = map[string]time.Time{}
		}
		got.Version = keyedVersion
		return got, true, nil
	}
	return doc, false, err
}

// Set puts values in, stamping the keys whose value changed (or is new) with
// now; it tells whether anything changed.
func (d *Keyed) Set(values map[string]json.RawMessage, now time.Time) bool {
	changed := false
	for key, value := range values {
		if old, ok := d.Values[key]; ok && jsonEqual(old, value) {
			continue
		}
		d.Values[key] = value
		d.UpdatedAt[key] = now.UTC()
		changed = true
	}
	return changed
}

// Merge takes from other the keys it changed later (for a sync).
func (d *Keyed) Merge(other Keyed) bool {
	changed := false
	for key, value := range other.Values {
		if theirs, mine := other.UpdatedAt[key], d.UpdatedAt[key]; theirs.After(mine) {
			d.Values[key] = value
			d.UpdatedAt[key] = theirs
			changed = true
		}
	}
	return changed
}

// SaveKeyed writes the file atomically, the one before kept as .bak.
func SaveKeyed(path string, doc Keyed) error {
	doc.Version = keyedVersion
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return WriteWithBackup(path, b)
}

func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return bytes.Equal(ax, by)
}

// WriteWithBackup writes a file atomically and keeps the valid file it
// replaces as .bak.
func WriteWithBackup(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if current, err := os.ReadFile(path); err == nil && json.Valid(current) {
		_ = WriteAtomic(path+".bak", current)
	}
	return WriteAtomic(path, data)
}

// WriteAtomic writes a file through a temporary one, synced, then renamed
// over it.
func WriteAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0o600)
	return os.Rename(tmp, path)
}
