package snapnote

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCreateSaveAndReopen(t *testing.T) {
	s := New(t.TempDir())
	note, err := s.Create(testPNG(t, 40, 30), Note{Title: "  Customs   extracts ", URL: "https://tarkov.dev/map/customs", PageTitle: "Customs", Linked: true})
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(note.ID) || note.Width != 40 || note.Height != 30 || note.Title != "Customs extracts" || !note.Linked {
		t.Fatalf("created %+v", note)
	}
	strokes := json.RawMessage(`[{"c":"#ff3b30","w":6,"p":[[1,2],[3,4]]}]`)
	if _, err := s.Save(note.ID, "", strokes, testJPEG(t, 8, 6)); err != nil {
		t.Fatal(err)
	}
	// A new store on the same folder (a restart) finds it all again.
	again := New(s.Dir)
	got, err := again.Get(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Strokes) != string(strokes) || got.Title != "Customs extracts" {
		t.Fatalf("reopened %+v", got)
	}
	if thumb, _ := again.Thumb(note.ID); len(thumb) == 0 {
		t.Fatal("no thumbnail")
	}
	if img, _ := again.Image(note.ID); len(img) == 0 {
		t.Fatal("no image")
	}
	list, err := again.List()
	if err != nil || len(list) != 1 || list[0].Strokes != nil {
		t.Fatalf("list %+v %v", list, err)
	}
}

func TestUnlinkKeepsThePage(t *testing.T) {
	s := New(t.TempDir())
	linked, _ := s.Create(testPNG(t, 4, 4), Note{URL: "https://example.com/", Linked: true})
	note, err := s.SetLinked(linked.ID, false)
	if err != nil || note.Linked || note.URL != "https://example.com/" {
		t.Fatalf("unlinked %+v %v", note, err)
	}
	if note, err = s.SetLinked(linked.ID, true); err != nil || !note.Linked {
		t.Fatalf("linked again %+v %v", note, err)
	}
	blank, _ := s.Create(testJPEG(t, 4, 4), Note{Linked: true})
	if blank.Linked {
		t.Fatal("a note without a page is linked")
	}
	if _, err := s.SetLinked(blank.ID, true); err == nil {
		t.Fatal("linked a note without a page")
	}
	if img, _ := s.Image(blank.ID); len(img) == 0 {
		t.Fatal("a JPEG was not stored")
	} else if _, format, _ := image.DecodeConfig(bytes.NewReader(img)); format != "png" {
		t.Fatalf("stored as %s", format)
	}
}

func TestRejectsBadInput(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Create([]byte("not an image"), Note{}); err == nil {
		t.Fatal("stored a non-image")
	}
	note, _ := s.Create(testPNG(t, 4, 4), Note{})
	if _, err := s.Save(note.ID, "", json.RawMessage(`{broken`), nil); err == nil {
		t.Fatal("stored broken strokes")
	}
	if _, err := s.Save(note.ID, "", nil, testPNG(t, 4, 4)); err == nil {
		t.Fatal("stored a PNG as the thumbnail")
	}
	for _, id := range []string{"..", "../x", "20260927000000-zzzzzzzz", ""} {
		if _, err := s.Get(id); err == nil {
			t.Fatalf("read note %q", id)
		}
		if err := s.Delete(id); err == nil {
			t.Fatalf("deleted %q", id)
		}
	}
	if err := s.Delete(note.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatal("deleted note still listed")
	}
}

func TestFavorite(t *testing.T) {
	s := New(t.TempDir())
	created, _ := s.Create(testPNG(t, 4, 4), Note{})
	note, err := s.SetFavorite(created.ID, true)
	if err != nil || !note.Favorite {
		t.Fatalf("starred %+v %v", note, err)
	}
	// Saving the drawing keeps the star.
	if note, err = s.Save(created.ID, "renamed", json.RawMessage(`[]`), nil); err != nil || !note.Favorite || note.Title != "renamed" {
		t.Fatalf("saved %+v %v", note, err)
	}
	if list, _ := s.List(); len(list) != 1 || !list[0].Favorite {
		t.Fatalf("list %+v", list)
	}
	if note, err = s.SetFavorite(created.ID, false); err != nil || note.Favorite {
		t.Fatalf("unstarred %+v %v", note, err)
	}
	if _, err := s.SetFavorite("20260101000000-zzzzzzzz", true); err == nil {
		t.Fatal("starred a missing note")
	}
}
