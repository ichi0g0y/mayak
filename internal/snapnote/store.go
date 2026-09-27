// Package snapnote keeps the snap notes: an image (a capture of a page, a
// pasted picture or a blank sheet) with drawing over it. A note taken from a
// page remembers that page (URL, title) and can be unlinked from it, which
// keeps the page on record so it can be linked again.
//
// Each note is a folder <dir>/<id>/ with note.json (the Note, strokes
// included), base.png (the image drawn on) and thumb.jpg (a small picture of
// the image with the drawing, made by the shell, for the lists).
package snapnote

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Note is one snap note. Strokes is the shell's drawing, kept as it sends it.
type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	PageTitle string `json:"pageTitle,omitempty"`
	Linked    bool   `json:"linked"`
	Favorite  bool   `json:"favorite,omitempty"`
	// Spot is where a note from a game screenshot was taken, when its file
	// name has the position.
	Spot      *Spot           `json:"spot,omitempty"`
	Full      bool            `json:"full,omitempty"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Strokes   json.RawMessage `json:"strokes,omitempty"`
}

// Limits: images come from the page capture (up to 15000 CSS px high) or a
// paste; strokes and thumbnails from the shell.
const (
	MaxImageBytes  = 64 << 20
	MaxStrokeBytes = 16 << 20
	MaxThumbBytes  = 2 << 20
	MaxTitle       = 160
)

var idPattern = regexp.MustCompile(`^[0-9]{14}-[0-9a-f]{8}$`)

// ValidID reports whether id names a note (and so is safe as a folder name).
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Store is the notes in one folder.
type Store struct {
	Dir string
	mu  sync.Mutex
}

// New returns a store in dir.
func New(dir string) *Store { return &Store{Dir: dir} }

func newID(now time.Time) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return now.UTC().Format("20060102150405") + "-" + hex.EncodeToString(b[:]), nil
}

func (s *Store) folder(id string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("invalid note ID")
	}
	return filepath.Join(s.Dir, id), nil
}

// Create stores a new note on image (PNG or JPEG), saved as PNG. note gives
// the title and the page; its ID, size and times are set here.
func (s *Store) Create(image []byte, note Note) (Note, error) {
	if len(image) == 0 || len(image) > MaxImageBytes {
		return Note{}, errors.New("the image is empty or too large")
	}
	width, height, pngData, err := asPNG(image)
	if err != nil {
		return Note{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	id, err := newID(now)
	if err != nil {
		return Note{}, err
	}
	note.ID, note.Width, note.Height = id, width, height
	note.CreatedAt, note.UpdatedAt = now.UTC(), now.UTC()
	note.Title = cleanTitle(note.Title)
	if note.Title == "" {
		note.Title = now.Format("2006-01-02 15:04")
	}
	note.Linked = note.Linked && note.URL != ""
	note.Strokes = nil
	dir := filepath.Join(s.Dir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Note{}, err
	}
	if err := writeFile(filepath.Join(dir, "base.png"), pngData); err != nil {
		os.RemoveAll(dir)
		return Note{}, err
	}
	if err := s.writeNote(note); err != nil {
		os.RemoveAll(dir)
		return Note{}, err
	}
	return note, nil
}

// asPNG checks that data is an image and returns it as PNG with its size.
func asPNG(data []byte) (int, int, []byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, errors.New("not a PNG or JPEG image")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 16384 || config.Height > 32768 {
		return 0, 0, nil, errors.New("the image is too large")
	}
	if format == "png" {
		return config.Width, config.Height, data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return 0, 0, nil, err
	}
	return config.Width, config.Height, out.Bytes(), nil
}

func cleanTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	if r := []rune(title); len(r) > MaxTitle {
		title = string(r[:MaxTitle])
	}
	return title
}

// List returns every note, the latest changed first. A folder that does not
// hold a readable note is skipped.
func (s *Store) List() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Note{}, nil
	}
	if err != nil {
		return nil, err
	}
	notes := make([]Note, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !ValidID(entry.Name()) {
			continue
		}
		note, err := s.readNote(entry.Name())
		if err != nil {
			continue
		}
		note.Strokes = nil
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].UpdatedAt.After(notes[j].UpdatedAt) })
	return notes, nil
}

// Get returns a note with its strokes.
func (s *Store) Get(id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readNote(id)
}

// Image returns the note's base image (PNG).
func (s *Store) Image(id string) ([]byte, error) {
	dir, err := s.folder(id)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "base.png"))
}

// Thumb returns the note's thumbnail (JPEG), or nothing before the shell
// has saved one.
func (s *Store) Thumb(id string) ([]byte, error) {
	dir, err := s.folder(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "thumb.jpg"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// Save stores the note's title, its drawing and its thumbnail (JPEG; none
// keeps the one it has).
func (s *Store) Save(id, title string, strokes json.RawMessage, thumb []byte) (Note, error) {
	if len(strokes) > MaxStrokeBytes {
		return Note{}, errors.New("the drawing is too large")
	}
	if len(strokes) > 0 && !json.Valid(strokes) {
		return Note{}, errors.New("the drawing is not valid")
	}
	if len(thumb) > MaxThumbBytes {
		return Note{}, errors.New("the thumbnail is too large")
	}
	if len(thumb) > 0 {
		if _, format, err := image.DecodeConfig(bytes.NewReader(thumb)); err != nil || format != "jpeg" {
			return Note{}, errors.New("the thumbnail is not a JPEG image")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	note, err := s.readNote(id)
	if err != nil {
		return Note{}, err
	}
	if title = cleanTitle(title); title != "" {
		note.Title = title
	}
	note.Strokes = strokes
	note.UpdatedAt = time.Now().UTC()
	if len(thumb) > 0 {
		if err := writeFile(filepath.Join(s.Dir, id, "thumb.jpg"), thumb); err != nil {
			return Note{}, err
		}
	}
	if err := s.writeNote(note); err != nil {
		return Note{}, err
	}
	note.Strokes = nil
	return note, nil
}

// SetLinked links the note to the page it was taken from, or unlinks it; the
// page stays on record either way.
func (s *Store) SetLinked(id string, linked bool) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, err := s.readNote(id)
	if err != nil {
		return Note{}, err
	}
	if linked && note.URL == "" {
		return Note{}, errors.New("this note was not taken from a page")
	}
	note.Linked = linked
	if err := s.writeNote(note); err != nil {
		return Note{}, err
	}
	note.Strokes = nil
	return note, nil
}

// Spot is a place in a raid: the map (tarkov.dev's name for it, "" when not
// known), the position and the direction faced (degrees).
type Spot struct {
	Map      string  `json:"map,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	Rotation float64 `json:"rotation"`
}

// SetSpotMap sets the map of the note's spot (for one whose map was not
// known when it was made).
func (s *Store) SetSpotMap(id, mapName string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, err := s.readNote(id)
	if err != nil {
		return Note{}, err
	}
	if note.Spot == nil {
		return Note{}, errors.New("this note has no position")
	}
	note.Spot.Map = mapName
	if err := s.writeNote(note); err != nil {
		return Note{}, err
	}
	note.Strokes = nil
	return note, nil
}

// SetFavorite stars the note, or takes its star off.
func (s *Store) SetFavorite(id string, favorite bool) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, err := s.readNote(id)
	if err != nil {
		return Note{}, err
	}
	note.Favorite = favorite
	if err := s.writeNote(note); err != nil {
		return Note{}, err
	}
	note.Strokes = nil
	return note, nil
}

// Delete removes the note.
func (s *Store) Delete(id string) error {
	dir, err := s.folder(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(dir)
}

func (s *Store) readNote(id string) (Note, error) {
	dir, err := s.folder(id)
	if err != nil {
		return Note{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "note.json"))
	if err != nil {
		return Note{}, err
	}
	var note Note
	if err := json.Unmarshal(data, &note); err != nil || note.ID != id {
		return Note{}, errors.New("the note is damaged")
	}
	return note, nil
}

func (s *Store) writeNote(note Note) error {
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(s.Dir, note.ID, "note.json"), data)
}

// writeFile replaces path through a temporary file, so a crash never leaves
// half a file.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snap-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
