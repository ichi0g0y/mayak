package screenshotstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screenshots.json")
	x := NewIndex(path)
	if err := x.Put("../x.png", Record{}); err == nil {
		t.Fatal("a path was accepted as a name")
	}
	at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	if err := x.Put("a.png", Record{At: at, Type: "tasks", Match: "Debut", Confidence: .9}); err != nil {
		t.Fatal(err)
	}
	// A new index reads the file.
	r, ok := NewIndex(path).Get("a.png")
	if !ok || r.Type != "tasks" || r.Match != "Debut" || !r.At.Equal(at) {
		t.Fatalf("record = %+v, %v", r, ok)
	}
	if _, ok := x.Get("b.png"); ok {
		t.Fatal("an unknown screenshot has a record")
	}
	// Past the bound the oldest go.
	for i := 0; i < maxRecords; i++ {
		if err := x.Put(fmt.Sprintf("n%04d.png", i), Record{At: at.Add(time.Duration(i+1) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := x.Get("a.png"); ok {
		t.Fatal("the oldest record was kept")
	}
	if _, ok := x.Get("n0000.png"); !ok || len(x.records) != maxRecords {
		t.Fatalf("records = %d", len(x.records))
	}
}

func TestDebugRecords(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DebugDirectory)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"capturedAt":"2026-09-20T10:00:00Z","sourceFile":"s1.png","screenshotType":"item","detectionLayout":"flea-offer","item":{"id":"i1","name":"Saury","shortName":"Saury","confidence":0.9},"itemCandidates":[{"name":"Saury","confidence":0.9},{"name":"Sprats","confidence":0.5}]}`)
	// A later analysis of the same screenshot wins.
	write("b.json", `{"capturedAt":"2026-09-20T09:00:00Z","sourceFile":"s1.png","screenshotType":"unknown"}`)
	write("c.json", `{"capturedAt":"2026-09-20T11:00:00Z","sourceFile":"s2.png","screenshotType":"tasks","quest":{"id":"q1","name":"Debut","trader":"Prapor","confidence":0.95},"ocrRaw":"Debut"}`)
	write("broken.json", `{`)
	records := DebugRecords(root)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	item := records["s1.png"]
	if item.Type != "item" || item.Layout != "flea-offer" || item.Match != "Saury" || item.Detail != "Saury" || len(item.Candidates) != 2 || item.Candidates[1] != "Sprats 50%" {
		t.Fatalf("item = %+v", item)
	}
	task := records["s2.png"]
	if task.Match != "Debut" || task.Detail != "Prapor" || task.OCR != "Debut" {
		t.Fatalf("task = %+v", task)
	}
	x := NewIndex(filepath.Join(root, "index.json"))
	if err := x.Put("s2.png", Record{Type: "position"}); err != nil {
		t.Fatal(err)
	}
	// Fill keeps what the index has.
	if err := x.Fill(records); err != nil {
		t.Fatal(err)
	}
	if r, _ := x.Get("s2.png"); r.Type != "position" {
		t.Fatalf("s2 = %+v", r)
	}
	if r, ok := x.Get("s1.png"); !ok || r.Match != "Saury" {
		t.Fatalf("s1 = %+v", r)
	}
}
