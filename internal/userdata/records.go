package userdata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Records is an ordered collection (bookmarks), record by record: each has
// its id, its place in the order, when it last changed and, once removed, a
// tombstone, so that two copies merge record by record and a removal is not
// undone by a copy that still has the record.
type Records struct {
	Version int      `json:"version"`
	Items   []Record `json:"records"`
}

type Record struct {
	ID        string          `json:"id"`
	Value     json.RawMessage `json:"value,omitempty"`
	Order     float64         `json:"order"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Deleted   bool            `json:"deleted,omitempty"`
}

const recordsVersion = 1

// tombstoneAge is how long a removal is kept, for copies not seen since.
const tombstoneAge = 180 * 24 * time.Hour

// LoadRecords reads a Records file; a missing one is empty (ok is false), a
// damaged one is read from its .bak.
func LoadRecords(path string) (doc Records, ok bool, err error) {
	doc = Records{Version: recordsVersion}
	for _, p := range []string{path, path + ".bak"} {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			if p == path && !os.IsNotExist(readErr) {
				err = readErr
			}
			continue
		}
		var got Records
		if json.Unmarshal(b, &got) != nil {
			err = errors.New(filepath.Base(path) + " is damaged")
			continue
		}
		got.Version = recordsVersion
		return got, true, nil
	}
	return doc, false, err
}

// SaveRecords writes the file atomically, the one before kept as .bak.
func SaveRecords(path string, doc Records) error {
	doc.Version = recordsVersion
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return WriteWithBackup(path, b)
}

// Live is the records not removed, in order.
func (d Records) Live() []Record {
	live := []Record{}
	for _, r := range d.Items {
		if !r.Deleted {
			live = append(live, r)
		}
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].Order < live[j].Order })
	return live
}

// SetList makes the collection the list given (each value with its id, in
// order): a record new or changed (its value or its place) is stamped with
// now, one no longer in the list becomes a tombstone, and old tombstones go.
// It tells whether anything changed.
func (d *Records) SetList(ids []string, values []json.RawMessage, now time.Time) bool {
	now = now.UTC()
	index := map[string]int{}
	for i, r := range d.Items {
		index[r.ID] = i
	}
	seen := map[string]bool{}
	changed := false
	for i, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		order := float64(i)
		if at, ok := index[id]; ok {
			r := &d.Items[at]
			if !r.Deleted && r.Order == order && jsonEqual(r.Value, values[i]) {
				continue
			}
			r.Value, r.Order, r.Deleted, r.UpdatedAt = values[i], order, false, now
		} else {
			d.Items = append(d.Items, Record{ID: id, Value: values[i], Order: order, UpdatedAt: now})
		}
		changed = true
	}
	kept := d.Items[:0]
	for _, r := range d.Items {
		if !seen[r.ID] && !r.Deleted {
			r.Deleted, r.Value, r.UpdatedAt = true, nil, now
			changed = true
		}
		if r.Deleted && now.Sub(r.UpdatedAt) > tombstoneAge {
			changed = true
			continue
		}
		kept = append(kept, r)
	}
	d.Items = kept
	return changed
}

// Merge takes from other the records it changed later (for a sync).
func (d *Records) Merge(other Records) bool {
	index := map[string]int{}
	for i, r := range d.Items {
		index[r.ID] = i
	}
	changed := false
	for _, theirs := range other.Items {
		at, ok := index[theirs.ID]
		switch {
		case !ok:
			d.Items = append(d.Items, theirs)
		case theirs.UpdatedAt.After(d.Items[at].UpdatedAt):
			d.Items[at] = theirs
		default:
			continue
		}
		changed = true
	}
	return changed
}
