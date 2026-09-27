package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/imaging"
	"github.com/local/mayak/internal/snapnote"
)

// Snap notes: the shell captures a page (or starts from a pasted picture or a
// blank sheet) and draws on it; internal/snapnote keeps them under
// %AppData%\Mayak\snapnotes. The shell is told of every change with
// "snapnote:changed".

var (
	snapNotesOnce  sync.Once
	snapNotesStore *snapnote.Store
	snapNotesErr   error
)

func snapNotes() (*snapnote.Store, error) {
	snapNotesOnce.Do(func() {
		dir, err := appdir.Path("snapnotes")
		if err != nil {
			snapNotesErr = err
			return
		}
		snapNotesStore = snapnote.New(dir)
	})
	return snapNotesStore, snapNotesErr
}

// SnapNoteData is a note opened for drawing: the note with its strokes and
// its image as a data URL.
type SnapNoteData struct {
	Note  snapnote.Note `json:"note"`
	Image string        `json:"image"`
}

func (a *App) snapNoteChanged() { a.emitEvent("snapnote:changed") }

// SnapNoteCapture captures the page in tab viewID (what is on screen, or with
// full the whole page) as a new note linked to that page. translated says
// the page shows through Google Translate, whose bar is hidden for it.
func (a *App) SnapNoteCapture(viewID, pageURL, pageTitle string, full, translated bool) (snapnote.Note, error) {
	if a.browserViews == nil {
		return snapnote.Note{}, errors.New("browser is not ready")
	}
	if !browserViewID.MatchString(viewID) {
		return snapnote.Note{}, errors.New("invalid browser ID")
	}
	if len(pageURL) > 4096 || !(strings.HasPrefix(pageURL, "https://") || strings.HasPrefix(pageURL, "http://")) {
		return snapnote.Note{}, errors.New("invalid page URL")
	}
	store, err := snapNotes()
	if err != nil {
		return snapnote.Note{}, err
	}
	prepare, restore := "", ""
	if translated {
		prepare, restore = hideTranslateBar, showTranslateBar
	}
	png, err := a.browserViews.Capture(viewID, full, prepare, restore)
	if err != nil {
		a.addLog("Warn", "SnapNote", "Could not capture the page: "+err.Error())
		return snapnote.Note{}, err
	}
	note, err := store.Create(png, snapnote.Note{Title: pageTitle, URL: pageURL, PageTitle: pageTitle, Linked: true, Full: full})
	if err != nil {
		return snapnote.Note{}, err
	}
	a.snapNoteChanged()
	return note, nil
}

// SnapNoteCreate starts a note on its own (not linked to a page) from an
// image data URL: a pasted picture, an image file or a blank sheet.
func (a *App) SnapNoteCreate(imageDataURL, title string) (snapnote.Note, error) {
	data, err := imageFromDataURL(imageDataURL, snapnote.MaxImageBytes)
	if err != nil {
		return snapnote.Note{}, err
	}
	store, err := snapNotes()
	if err != nil {
		return snapnote.Note{}, err
	}
	note, err := store.Create(data, snapnote.Note{Title: title})
	if err != nil {
		return snapnote.Note{}, err
	}
	a.snapNoteChanged()
	return note, nil
}

// SnapNoteFromScreenshot starts a note on its own (not linked to a page)
// from a game screenshot in the Screenshots folder, named as the gallery
// lists it.
func (a *App) SnapNoteFromScreenshot(name, title string) (snapnote.Note, error) {
	dir := a.screenshotDir()
	// Only a file directly in the screenshot folder: no paths.
	if dir == "" || name != filepath.Base(name) || !screenshotName(name) {
		return snapnote.Note{}, errors.New("invalid screenshot")
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return snapnote.Note{}, err
	}
	store, err := snapNotes()
	if err != nil {
		return snapnote.Note{}, err
	}
	note, err := store.Create(data, snapnote.Note{Title: title})
	if err != nil {
		return snapnote.Note{}, err
	}
	a.snapNoteChanged()
	return note, nil
}

// SnapNoteList returns every note (without strokes), the latest changed first.
func (a *App) SnapNoteList() ([]snapnote.Note, error) {
	store, err := snapNotes()
	if err != nil {
		return nil, err
	}
	return store.List()
}

// SnapNoteOpen returns a note with its strokes and image.
func (a *App) SnapNoteOpen(id string) (SnapNoteData, error) {
	store, err := snapNotes()
	if err != nil {
		return SnapNoteData{}, err
	}
	note, err := store.Get(id)
	if err != nil {
		return SnapNoteData{}, err
	}
	img, err := store.Image(id)
	if err != nil {
		return SnapNoteData{}, err
	}
	return SnapNoteData{Note: note, Image: imaging.DataURL("image/png", img)}, nil
}

// SnapNoteThumb returns a note's thumbnail as a data URL, empty before one
// was saved.
func (a *App) SnapNoteThumb(id string) (string, error) {
	store, err := snapNotes()
	if err != nil {
		return "", err
	}
	thumb, err := store.Thumb(id)
	if err != nil || len(thumb) == 0 {
		return "", err
	}
	return imaging.DataURL("image/jpeg", thumb), nil
}

// SnapNoteSave stores a note's title, drawing (JSON) and thumbnail (a JPEG
// data URL; empty keeps the one it has).
func (a *App) SnapNoteSave(id, title, strokes, thumbDataURL string) (snapnote.Note, error) {
	store, err := snapNotes()
	if err != nil {
		return snapnote.Note{}, err
	}
	var thumb []byte
	if thumbDataURL != "" {
		if thumb, err = imageFromDataURL(thumbDataURL, snapnote.MaxThumbBytes); err != nil {
			return snapnote.Note{}, err
		}
	}
	note, err := store.Save(id, title, json.RawMessage(strokes), thumb)
	if err != nil {
		return snapnote.Note{}, err
	}
	a.snapNoteChanged()
	return note, nil
}

// SnapNoteLink links a note to the page it was taken from, or unlinks it.
func (a *App) SnapNoteLink(id string, linked bool) (snapnote.Note, error) {
	store, err := snapNotes()
	if err != nil {
		return snapnote.Note{}, err
	}
	note, err := store.SetLinked(id, linked)
	if err != nil {
		return snapnote.Note{}, err
	}
	a.snapNoteChanged()
	return note, nil
}

// SnapNoteDelete removes a note.
func (a *App) SnapNoteDelete(id string) error {
	store, err := snapNotes()
	if err != nil {
		return err
	}
	if err := store.Delete(id); err != nil {
		return err
	}
	a.snapNoteChanged()
	return nil
}

// imageFromDataURL decodes a PNG or JPEG data URL of at most limit bytes.
func imageFromDataURL(value string, limit int) ([]byte, error) {
	for _, prefix := range []string{"data:image/png;base64,", "data:image/jpeg;base64,"} {
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		encoded := value[len(prefix):]
		if base64.StdEncoding.DecodedLen(len(encoded)) > limit {
			return nil, errors.New("the image is too large")
		}
		return base64.StdEncoding.DecodeString(encoded)
	}
	return nil, errors.New("not a PNG or JPEG image")
}
